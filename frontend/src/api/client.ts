// Единственная точка сети. Компоненты не знают про fetch/URL.
// Базовый URL из VITE_API_BASE_URL; VITE_USE_MOCK=1 отдаёт мок без сети (спека §5).
// Переключение на живой API = смена env, а не кода.

import type {
  HealthResponse,
  Item,
  RecommendationsRequest,
  RecommendationsResponse,
} from '@/types/api'
import mock from '@mock/recommendations.json'

const BASE = import.meta.env.VITE_API_BASE_URL ?? ''
export const USE_MOCK = import.meta.env.VITE_USE_MOCK === '1'

/** Dev-сценарии мока (см. переключатель в UI). В live-режиме игнорируются. */
export type MockScenario = 'success' | 'empty' | 'error' | 'slow'

const KNOWN_STATUS = new Set(['overdue', 'due_soon', 'upcoming'])
const KNOWN_SEVERITY = new Set(['low', 'medium', 'high'])
const KNOWN_CATEGORY = new Set(['regular', 'known_issue'])

/** Лёгкий runtime-guard: TS-типы только compile-time; на живом ответе форму проверяем сами. */
function coerceItem(raw: unknown, i: number): Item {
  const o = (raw ?? {}) as Record<string, unknown>
  const num = (v: unknown, d = 0) => (typeof v === 'number' && Number.isFinite(v) ? v : d)
  const str = (v: unknown, d = '') => (typeof v === 'string' ? v : d)
  const status = str(o.status)
  const severity = str(o.severity)
  const category = str(o.category)
  return {
    id: str(o.id, `item-${i}`),
    title: str(o.title, 'Без названия'),
    category: (KNOWN_CATEGORY.has(category) ? category : 'regular') as Item['category'],
    severity: (KNOWN_SEVERITY.has(severity) ? severity : 'low') as Item['severity'],
    interval_km: num(o.interval_km),
    due_at_km: num(o.due_at_km),
    status: (KNOWN_STATUS.has(status) ? status : 'upcoming') as Item['status'],
    note: str(o.note),
  }
}

function normalize(raw: unknown, fallbackCar: RecommendationsRequest): RecommendationsResponse {
  const o = (raw ?? {}) as Record<string, unknown>
  const items = Array.isArray(o.items) ? o.items.map(coerceItem) : []
  const car = (o.car as RecommendationsResponse['car']) ?? { ...fallbackCar }
  return {
    car,
    items,
    generated_by: o.generated_by === 'llm' ? 'llm' : 'cache',
    cached: Boolean(o.cached),
  }
}

function delay(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) return reject(new DOMException('Aborted', 'AbortError'))
    const t = setTimeout(resolve, ms)
    signal?.addEventListener('abort', () => {
      clearTimeout(t)
      reject(new DOMException('Aborted', 'AbortError'))
    })
  })
}

async function mockRecommendations(
  req: RecommendationsRequest,
  scenario: MockScenario,
  signal?: AbortSignal,
): Promise<RecommendationsResponse> {
  await delay(scenario === 'slow' ? 2200 : 600, signal)
  if (scenario === 'error') throw new Error('Мок: имитация ошибки сети')
  const base = normalize(mock, req)
  return {
    ...base,
    car: { ...req },
    items: scenario === 'empty' ? [] : base.items,
  }
}

export interface FetchOptions {
  signal?: AbortSignal
  /** Только в mock-режиме: какой сценарий вернуть. */
  scenario?: MockScenario
}

export async function postRecommendations(
  req: RecommendationsRequest,
  opts: FetchOptions = {},
): Promise<RecommendationsResponse> {
  if (USE_MOCK) return mockRecommendations(req, opts.scenario ?? 'success', opts.signal)

  const res = await fetch(`${BASE}/api/v1/recommendations`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
    signal: opts.signal,
  })
  if (!res.ok) throw new Error(`API вернул ${res.status}`)
  return normalize(await res.json(), req)
}

export async function health(signal?: AbortSignal): Promise<HealthResponse> {
  if (USE_MOCK) return { status: 'ok' }
  const res = await fetch(`${BASE}/api/v1/health`, { signal })
  if (!res.ok) throw new Error(`API вернул ${res.status}`)
  return res.json()
}

// b2b-слой (/api/v1/b2b). Live-запросы идут через единый apiFetch (Bearer добавляется там),
// авторизованные вызовы обёрнуты withAuthRetry (401→refresh→повтор). USE_MOCK отдаёт мок без сети.
// scan-all — операторский: авторизуется заголовком X-Admin-Token (режим оператора в UI).

import { USE_MOCK } from '@/api/client'
import { apiFetch } from '@/api/http'
import { ApiError } from '@/api/errors'
import { assertMockAccess, withAuthRetry } from '@/api/auth'
import type {
  ConnectServiceCenterRequest,
  ScanReport,
  ScanSummary,
  ServiceCenter,
  ServiceCentersResponse,
} from '@/types/b2b'

/* -------------------------------- мок ---------------------------------- */
/** Dev-сценарии мока (переключатель в UI). В live-режиме игнорируются. */
export type B2BScenario = 'success' | 'empty' | 'error' | 'slow' | 'disabled'

const b2bDisabled = () => new ApiError('B2B_DISABLED', 'b2b выключен: задайте APP_SECRET_KEY', 503)

// In-memory «хранилище» СТО для мок-режима — connect добавляет сюда, list читает отсюда.
const mockStore = {
  seq: 1002,
  centers: [
    { id: 'sc-1001', name: 'АвтоТехЦентр «Магистраль»', responsible_id: 12 },
    { id: 'sc-1002', name: 'Дилер Тойота Центр', responsible_id: 7 },
  ] as ServiceCenter[],
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

function mockDelay(scenario: B2BScenario, signal?: AbortSignal): Promise<void> {
  return delay(scenario === 'slow' ? 1800 : 500, signal)
}

/** Похоже ли на входящий вебхук Bitrix24: http(s)://<portal>/rest/<user>/<code>/ */
function looksLikeWebhook(w: string): boolean {
  try {
    const u = new URL(w)
    return (u.protocol === 'https:' || u.protocol === 'http:') && /\/rest\//.test(u.pathname)
  } catch {
    return false
  }
}

async function mockConnect(
  req: ConnectServiceCenterRequest,
  scenario: B2BScenario,
  signal?: AbortSignal,
): Promise<ServiceCenter> {
  await mockDelay(scenario, signal)
  assertMockAccess() // b2b требует авторизации — mock тоже
  if (scenario === 'disabled') throw b2bDisabled()
  if (scenario === 'error') throw new ApiError('STORE_ERROR', 'Мок: не удалось сохранить СТО', 500)
  if (!looksLikeWebhook(req.webhook)) {
    throw new ApiError('INVALID_WEBHOOK', 'Мок: вебхук не похож на входящий вебхук Bitrix24 (…/rest/…).', 400)
  }
  const sc: ServiceCenter = { id: `sc-${++mockStore.seq}`, name: req.name, responsible_id: req.responsible_id }
  mockStore.centers = [...mockStore.centers, sc]
  return sc
}

async function mockList(scenario: B2BScenario, signal?: AbortSignal): Promise<ServiceCentersResponse> {
  await mockDelay(scenario, signal)
  assertMockAccess()
  if (scenario === 'disabled') throw b2bDisabled()
  if (scenario === 'error') throw new ApiError('STORE_ERROR', 'Мок: не удалось загрузить список СТО', 500)
  return { service_centers: scenario === 'empty' ? [] : [...mockStore.centers] }
}

async function mockScan(scenario: B2BScenario, signal?: AbortSignal): Promise<ScanReport> {
  await mockDelay(scenario, signal)
  assertMockAccess()
  if (scenario === 'disabled') throw b2bDisabled()
  if (scenario === 'error') throw new ApiError('SCAN_ERROR', 'Мок: Bitrix не ответил при сканировании', 502)
  if (scenario === 'empty') return { cars: 0, due_items: 0, pushed: 0, skipped: 0 }
  return { cars: 18, due_items: 6, pushed: 5, skipped: 1 }
}

async function mockScanAll(
  scenario: B2BScenario,
  adminToken: string,
  signal?: AbortSignal,
): Promise<ScanSummary> {
  await mockDelay(scenario, signal)
  if (!adminToken) throw new ApiError('ADMIN_DISABLED', 'Мок: нужен X-Admin-Token', 503)
  if (scenario === 'disabled') throw b2bDisabled()
  if (scenario === 'error') throw new ApiError('SCAN_ERROR', 'Мок: массовый скан не удался', 502)
  const n = scenario === 'empty' ? 0 : mockStore.centers.length
  return {
    centers: n,
    due_items: n * 4,
    pushed: n * 3,
    skipped: n,
    errors: n > 1 ? ['sc-1002: crm.contact.list вернул 401 — проверьте вебхук'] : undefined,
  }
}

/* ------------------------------- вызовы -------------------------------- */
export interface B2BRequestOptions {
  scenario?: B2BScenario
  signal?: AbortSignal
}

export interface ScanAllOptions extends B2BRequestOptions {
  adminToken?: string
}

export function connectServiceCenter(
  req: ConnectServiceCenterRequest,
  opts: B2BRequestOptions = {},
): Promise<ServiceCenter> {
  return withAuthRetry(() => {
    if (USE_MOCK) return mockConnect(req, opts.scenario ?? 'success', opts.signal)
    return apiFetch<ServiceCenter>('/b2b/service-centers', {
      method: 'POST',
      auth: true,
      body: req,
      signal: opts.signal,
    })
  })
}

export function listServiceCenters(opts: B2BRequestOptions = {}): Promise<ServiceCentersResponse> {
  return withAuthRetry(() => {
    if (USE_MOCK) return mockList(opts.scenario ?? 'success', opts.signal)
    return apiFetch<ServiceCentersResponse>('/b2b/service-centers', { auth: true, signal: opts.signal })
  })
}

export function scanServiceCenter(id: string, opts: B2BRequestOptions = {}): Promise<ScanReport> {
  return withAuthRetry(() => {
    if (USE_MOCK) return mockScan(opts.scenario ?? 'success', opts.signal)
    return apiFetch<ScanReport>(`/b2b/service-centers/${encodeURIComponent(id)}/scan`, {
      method: 'POST',
      auth: true,
      signal: opts.signal,
    })
  })
}

/** Операторский массовый скан. Авторизуется X-Admin-Token (не Bearer-пользователем). */
export function scanAll(opts: ScanAllOptions = {}): Promise<ScanSummary> {
  const adminToken = opts.adminToken ?? ''
  if (USE_MOCK) return mockScanAll(opts.scenario ?? 'success', adminToken, opts.signal)
  return apiFetch<ScanSummary>('/b2b/scan-all', {
    method: 'POST',
    adminToken,
    signal: opts.signal,
  })
}

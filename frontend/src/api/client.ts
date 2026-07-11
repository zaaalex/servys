// b2c-слой контракта §4.A (vehicles/alerts). Теперь привязан к аккаунту: запросы идут через
// единый сетевой слой (api/http.apiFetch) с заголовком Authorization: Bearer <access> и
// обёрнуты withAuthRetry (401→refresh→повтор). Гостевого X-Client-ID больше нет.
// VITE_USE_MOCK=1 — работаем по моку без сети (in-memory стор, чтобы add/patch жили в сессии).
// Переключение на живой API = смена env, а не кода.

import type {
  Alert,
  AlertStatus,
  CommunityNote,
  CreateVehicleRequest,
  HealthResponse,
  MaintenanceCategory,
  OdometerUpdate,
  ServiceEventRequest,
  Vehicle,
  VinResolveResult,
} from '@/types/api'
import { apiFetch } from '@/api/http'
import { withAuthRetry } from '@/api/auth'
import { decodeVin } from '@/data/vin'
import seedAlerts from '@mock/alerts.json'

export const USE_MOCK = import.meta.env.VITE_USE_MOCK === '1'

export type MockScenario = 'success' | 'empty' | 'error' | 'slow'

type RawVehicle = Record<string, unknown>
const str = (v: unknown, fallback = '') => typeof v === 'string' ? v : fallback
const num = (v: unknown, fallback = 0) => typeof v === 'number' && Number.isFinite(v) ? v : fallback

/** community из бэка (snake_case) → CommunityNote (camelCase); null/невалид → undefined. */
function communityFromAPI(v: unknown): CommunityNote | undefined {
  if (v == null || typeof v !== 'object') return undefined
  const c = v as Record<string, unknown>
  return {
    realIntervalKm: num(c.real_interval_km),
    note: str(c.note),
    source: str(c.source, 'demo'),
    reports: num(c.reports),
  }
}

function vehicleFromAPI(raw: RawVehicle): Vehicle {
  return {
    id: str(raw.id), vin: str(raw.vin), make: str(raw.make), model: str(raw.model), year: num(raw.year),
    engineCc: num(raw.engine_cc), powerHp: num(raw.power_hp), color: str(raw.color, '#1fbfb0'),
    bodyType: str(raw.body_type, 'sedan') as Vehicle['bodyType'], fuelType: str(raw.fuel_type, 'gasoline') as Vehicle['fuelType'],
    identificationSource: str(raw.identification_source, raw.vin ? 'drom' : 'manual') as Vehicle['identificationSource'],
    currentOdometer: num(raw.mileage_km), odometerUpdatedAt: str(raw.odometer_updated_at),
  }
}

export function normalizeVinResponse(raw: RawVehicle): VinResolveResult {
  const vehicle = vehicleFromAPI(raw)
  return { vin: vehicle.vin, signature: { make: vehicle.make, model: vehicle.model, year: vehicle.year, engineDisplacementCc: vehicle.engineCc, powerHp: vehicle.powerHp, bodyType: vehicle.bodyType, fuelType: vehicle.fuelType }, matchLevel: 'partial', identificationSource: vehicle.identificationSource }
}

/* ---- мок: in-memory стор (стартует пустым — гараж наполняет пользователь) ---- */
const store: Vehicle[] = []
// выполненные работы: vehicleId -> ruleCode -> пробег на момент выполнения
const serviceEvents: Record<string, Record<string, number>> = {}

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

/** Пересчёт статуса по dueAtKm относительно текущего пробега (мок вместо reminder-движка). */
function recomputeStatus(a: Alert, odo: number): AlertStatus {
  if (a.type === 'INSPECTION_REQUIRED' || a.status === 'INSPECTION_REQUIRED') return 'INSPECTION_REQUIRED'
  if (!a.dueAtKm || a.dueAtKm <= 0) return 'NO_INTERVAL'
  const delta = a.dueAtKm - odo
  if (delta < 0) return 'OVERDUE'
  if (delta <= 1500) return 'DUE'
  if (delta <= 5000) return 'SOON'
  return 'OK'
}

/* ---- публичное API (live-запросы авторизованы Bearer + 401→refresh→повтор) ---- */
export async function health(signal?: AbortSignal): Promise<HealthResponse> {
  if (USE_MOCK) return { status: 'ok' }
  return apiFetch<HealthResponse>('/health', { signal })
}

export async function resolveVin(vin: string, signal?: AbortSignal): Promise<VinResolveResult> {
  if (USE_MOCK) {
    await delay(400, signal)
    const d = decodeVin(vin)
    if ('err' in d) throw new Error(d.err)
    return {
      vin: vin.trim().toUpperCase(),
      signature: {
        make: d.make,
        model: d.model,
        year: d.year,
        engineDisplacementCc: d.engineCc,
        powerHp: d.powerHp,
        bodyType: d.bodyType,
        fuelType: d.fuelType,
      },
      matchLevel: d.matchLevel,
      identificationSource: 'drom',
    }
  }
  const raw = await withAuthRetry(() =>
    apiFetch<RawVehicle>('/vin/resolve', { method: 'POST', auth: true, body: { vin }, signal }),
  )
  return normalizeVinResponse(raw)
}

export async function listVehicles(signal?: AbortSignal): Promise<Vehicle[]> {
  if (USE_MOCK) return store.map((v) => ({ ...v }))
  const response = await withAuthRetry(() =>
    apiFetch<{ vehicles: RawVehicle[] }>('/vehicles', { auth: true, signal }),
  )
  return (response.vehicles ?? []).map(vehicleFromAPI)
}

export async function createVehicle(body: CreateVehicleRequest, signal?: AbortSignal): Promise<Vehicle> {
  if (USE_MOCK) {
    await delay(300, signal)
    const vehicle: Vehicle = {
      id: `veh-${store.length + 1}-${Date.now().toString(36)}`,
      vin: body.vin ?? '',
      make: body.make ?? 'Авто',
      model: body.model ?? '',
      year: body.year ?? 2020,
      engineCc: 0,
      powerHp: 0,
      color: body.color ?? '#1fbfb0',
      bodyType: body.bodyType ?? 'sedan',
      fuelType: body.fuelType ?? 'gasoline',
      identificationSource: body.vin ? 'drom' : 'manual',
      currentOdometer: Math.max(0, body.odometer),
      odometerUpdatedAt: new Date().toISOString(),
    }
    store.push(vehicle)
    return { ...vehicle }
  }
  const raw = await withAuthRetry(() =>
    apiFetch<RawVehicle>('/vehicles', {
      method: 'POST',
      auth: true,
      body: { vin: body.vin, make: body.make, model: body.model, year: body.year, engine_cc: body.engineCc ?? 0, power_hp: body.powerHp ?? 0, mileage_km: body.odometer },
      signal,
    }),
  )
  return vehicleFromAPI(raw)
}

export async function deleteVehicle(id: string, signal?: AbortSignal): Promise<void> {
  if (USE_MOCK) {
    await delay(200, signal)
    const i = store.findIndex((x) => x.id === id)
    if (i < 0) throw new Error('Машина не найдена')
    store.splice(i, 1)
    delete serviceEvents[id]
    return
  }
  await withAuthRetry(() =>
    apiFetch<void>(`/vehicles/${id}`, { method: 'DELETE', auth: true, signal }),
  )
}

export async function updateOdometer(id: string, odometer: number, signal?: AbortSignal): Promise<Vehicle> {
  if (USE_MOCK) {
    await delay(250, signal)
    const v = store.find((x) => x.id === id)
    if (!v) throw new Error('Машина не найдена')
    v.currentOdometer = Math.max(v.currentOdometer, Math.round(odometer)) // нельзя уменьшать
    v.odometerUpdatedAt = new Date().toISOString()
    return { ...v }
  }
  const body: OdometerUpdate = { odometer }
  const raw = await withAuthRetry(() =>
    apiFetch<RawVehicle>(`/vehicles/${id}/odometer`, { method: 'PATCH', auth: true, body: { mileage_km: body.odometer }, signal }),
  )
  return vehicleFromAPI(raw)
}

export async function addServiceEvent(id: string, body: ServiceEventRequest, signal?: AbortSignal): Promise<void> {
  if (USE_MOCK) {
    await delay(250, signal)
    ;(serviceEvents[id] ??= {})[body.componentCode] = body.odometer
    return
  }
  await withAuthRetry(() =>
    apiFetch<void>(`/vehicles/${id}/service-events`, { method: 'POST', auth: true, body: { rule_code: body.componentCode, odometer: body.odometer }, signal }),
  )
}

export interface AlertsOptions {
  signal?: AbortSignal
  /** Только в mock-режиме. */
  scenario?: MockScenario
}

export async function getAlerts(vehicle: Vehicle, opts: AlertsOptions = {}): Promise<Alert[]> {
  if (USE_MOCK) {
    await delay(opts.scenario === 'slow' ? 2200 : 550, opts.signal)
    if (opts.scenario === 'error') throw new Error('Мок: имитация ошибки сети')
    if (opts.scenario === 'empty') return []
    const done = serviceEvents[vehicle.id] ?? {}
    return (seedAlerts as Alert[]).map((a) => {
      const at = done[a.ruleCode]
      if (at != null) {
        // работа выполнена → следующий срок отодвинут, статус «в норме»
        return { ...a, vehicleId: vehicle.id, dueAtKm: at + 12000, status: 'OK' as AlertStatus }
      }
      return { ...a, vehicleId: vehicle.id, status: recomputeStatus(a, vehicle.currentOdometer) }
    })
  }
  const response = await withAuthRetry(() =>
    apiFetch<{ alerts: Array<Record<string, unknown>> }>(`/vehicles/${vehicle.id}/alerts`, { auth: true, signal: opts.signal }),
  )
  return (response.alerts ?? []).map((raw) => ({
    id: str(raw.id), vehicleId: vehicle.id, type: str(raw.type) as Alert['type'], ruleCode: str(raw.rule_code),
    severity: str(raw.severity, 'low') as Alert['severity'], status: str(raw.type).replace('MAINTENANCE_', '') as Alert['status'],
    title: str(raw.title), description: str(raw.description), dueAtKm: num(raw.due_at_km),
    // MAINTENANCE_OK → status 'OK' проходит через тот же replace; category с fallback 'primary'
    category: str(raw.category, 'primary') as MaintenanceCategory, community: communityFromAPI(raw.community),
  }))
}

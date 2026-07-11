// Единственная точка сети (контракт §4.A: vehicles/alerts). Идентификация — X-Client-ID.
// VITE_USE_MOCK=1 — работаем по моку без сети (in-memory стор, чтобы add/patch жили в сессии).
// Переключение на живой API = смена env, а не кода. Точные формы бэка сверить с Dev 1.

import type {
  Alert,
  AlertStatus,
  CreateVehicleRequest,
  HealthResponse,
  Me,
  OdometerUpdate,
  ServiceEventRequest,
  Vehicle,
  VinResolveResult,
} from '@/types/api'
import { decodeVin } from '@/data/vin'
import seedVehicles from '@mock/vehicles.json'
import seedAlerts from '@mock/alerts.json'

const BASE = import.meta.env.VITE_API_BASE_URL ?? ''
export const USE_MOCK = import.meta.env.VITE_USE_MOCK === '1'

export type MockScenario = 'success' | 'empty' | 'error' | 'slow'

/* ---- идентификация: browser-token в localStorage (MVP, не production-auth) ---- */
function clientId(): string {
  const KEY = 'servys.clientId'
  try {
    let id = localStorage.getItem(KEY)
    if (!id) {
      id = crypto.randomUUID()
      localStorage.setItem(KEY, id)
    }
    return id
  } catch {
    return 'anonymous'
  }
}

function headers(json = false): HeadersInit {
  const h: Record<string, string> = { 'X-Client-ID': clientId() }
  if (json) h['Content-Type'] = 'application/json'
  return h
}

async function req<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(`${BASE}${path}`, { ...init, headers: { ...headers(init.body != null), ...init.headers } })
  if (!res.ok) throw new Error(`API вернул ${res.status}`)
  return res.json() as Promise<T>
}

/* ---- мок: in-memory стор ---- */
const store: Vehicle[] = (seedVehicles as Vehicle[]).map((v) => ({ ...v }))

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

/* ---- публичное API ---- */
export async function health(signal?: AbortSignal): Promise<HealthResponse> {
  if (USE_MOCK) return { status: 'ok' }
  return req<HealthResponse>('/api/v1/health', { signal })
}

export async function getMe(signal?: AbortSignal): Promise<Me> {
  if (USE_MOCK) return { id: 'mock-user', clientKey: clientId(), tenantType: 'b2c' }
  return req<Me>('/api/v1/me', { signal })
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
  return req<VinResolveResult>('/api/v1/vin/resolve', { method: 'POST', body: JSON.stringify({ vin }) })
}

export async function listVehicles(signal?: AbortSignal): Promise<Vehicle[]> {
  if (USE_MOCK) return store.map((v) => ({ ...v }))
  return req<Vehicle[]>('/api/v1/vehicles', { signal })
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
  return req<Vehicle>('/api/v1/vehicles', { method: 'POST', body: JSON.stringify(body) })
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
  return req<Vehicle>(`/api/v1/vehicles/${id}/odometer`, { method: 'PATCH', body: JSON.stringify(body) })
}

export async function addServiceEvent(id: string, body: ServiceEventRequest, signal?: AbortSignal): Promise<void> {
  if (USE_MOCK) {
    await delay(250, signal)
    return
  }
  await req<unknown>(`/api/v1/vehicles/${id}/service-events`, { method: 'POST', body: JSON.stringify(body) })
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
    return (seedAlerts as Alert[]).map((a) => ({
      ...a,
      vehicleId: vehicle.id,
      status: recomputeStatus(a, vehicle.currentOdometer),
    }))
  }
  return req<Alert[]>(`/api/v1/vehicles/${vehicle.id}/alerts`, { signal: opts.signal })
}

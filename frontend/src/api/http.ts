// Единый сетевой слой (live). Все реальные запросы к /api/v1 идут через apiFetch —
// здесь централизованно добавляется Authorization: Bearer <access> (forward-compat auth)
// и X-Admin-Token (операторские эндпоинты), и единообразно разбирается формат ошибок.
// Логика 401→refresh→повтор живёт в api/auth.ts (withAuthRetry), чтобы работать и в mock-режиме.

import { getAccessToken } from '@/api/session'
import { ApiError, type ApiErrorBody } from '@/api/errors'

const BASE = import.meta.env.VITE_API_BASE_URL ?? ''

export interface ApiFetchOptions {
  method?: string
  body?: unknown
  /** добавить заголовок Authorization из текущего access-токена */
  auth?: boolean
  /** X-Admin-Token для операторских эндпоинтов (scan-all) */
  adminToken?: string
  signal?: AbortSignal
}

export async function apiFetch<T>(path: string, opts: ApiFetchOptions = {}): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (opts.auth) {
    const token = getAccessToken()
    if (token) headers.Authorization = `Bearer ${token}`
  }
  if (opts.adminToken) headers['X-Admin-Token'] = opts.adminToken

  const res = await fetch(`${BASE}/api/v1${path}`, {
    method: opts.method ?? 'GET',
    headers,
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
    signal: opts.signal,
  })

  if (!res.ok) {
    let code = `HTTP_${res.status}`
    let message = `API вернул ${res.status}`
    try {
      const b = (await res.json()) as ApiErrorBody
      if (b?.error?.code) code = b.error.code
      if (b?.error?.message) message = b.error.message
    } catch {
      /* тело не JSON — оставляем дефолт */
    }
    throw new ApiError(code, message, res.status)
  }

  if (res.status === 204) return undefined as T
  const text = await res.text()
  return (text ? (JSON.parse(text) as T) : (undefined as T))
}

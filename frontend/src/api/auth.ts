// Авторизация: /api/v1/auth/*. Токены хранит api/session; live-запросы идут через api/http.apiFetch,
// mock-режим — через in-memory фейк-сессию. withAuthRetry — общий перехватчик 401→refresh→повтор
// (работает и в mock, и в live), refresh дедуплицируется (параллельные 401 → один refresh).

import { USE_MOCK } from '@/api/client'
import { apiFetch } from '@/api/http'
import { ApiError, isUnauthorized } from '@/api/errors'
import {
  clearTokens,
  getAccessToken,
  getRefreshToken,
  setAccessToken,
  setTokens,
} from '@/api/session'
import type { AuthContext, Credentials, MeResponse, SwitchResponse, TokenResponse } from '@/types/auth'

/* =============================== MOCK ================================= */
interface MockSession {
  accountId: string
  contexts: AuthContext[]
  active: AuthContext
}

const mock = {
  validRefresh: null as string | null,
  accessExpired: false,
  session: null as MockSession | null,
  takenEmails: new Set<string>(['taken@servys.app']), // для демонстрации EMAIL_TAKEN
}

function rand(): string {
  return Math.random().toString(36).slice(2, 12)
}

function mockIssue(): TokenResponse {
  const access_token = `mock-access-${rand()}`
  const refresh_token = `mock-refresh-${rand()}`
  mock.validRefresh = refresh_token
  mock.accessExpired = false
  setTokens(access_token, refresh_token)
  return { access_token, refresh_token, expires_in: 900 }
}

function mockDefaultContexts(): AuthContext[] {
  // Демо-аккаунт с обоими контекстами, чтобы был виден переключатель и b2b-раздел.
  return [
    { ctx_type: 'b2c', tenant_id: 't-personal', role: 'owner' },
    { ctx_type: 'b2b', tenant_id: 'sc-1001', role: 'owner' },
  ]
}

// На перезагрузке страницы токены живут в localStorage, а mock.session — нет: восстановим лениво.
function mockEnsureSession(): MockSession {
  if (!mock.session) {
    const contexts = mockDefaultContexts()
    mock.session = { accountId: 'acc-mock-1', contexts, active: contexts[0] }
    if (mock.validRefresh === null) mock.validRefresh = getRefreshToken()
  }
  return mock.session
}

function mockAssertAccess(): void {
  if (!getAccessToken()) throw new ApiError('NO_TOKEN', 'Нет токена', 401)
  if (mock.accessExpired) throw new ApiError('INVALID_TOKEN', 'Access истёк', 401)
}

const delay = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))

/** Dev-хелпер (mock): пометить access протухшим, чтобы вручную проверить 401→refresh. */
export function expireMockAccess(): void {
  mock.accessExpired = true
}

/* ============================= ENDPOINTS ============================= */

export async function register(cred: Credentials): Promise<void> {
  if (USE_MOCK) {
    await delay(400)
    if (mock.takenEmails.has(cred.email.toLowerCase())) {
      throw new ApiError('EMAIL_TAKEN', 'email занят', 409)
    }
    mock.takenEmails.add(cred.email.toLowerCase())
    const contexts = mockDefaultContexts()
    mock.session = { accountId: `acc-${rand()}`, contexts, active: contexts[0] }
    mockIssue()
    return
  }
  const t = await apiFetch<TokenResponse>('/auth/register', { method: 'POST', body: cred })
  setTokens(t.access_token, t.refresh_token)
}

export async function login(cred: Credentials): Promise<void> {
  if (USE_MOCK) {
    await delay(400)
    if (cred.password === 'wrong') throw new ApiError('BAD_CREDENTIALS', 'неверный пароль', 401)
    const contexts = mockDefaultContexts()
    mock.session = { accountId: 'acc-mock-1', contexts, active: contexts[0] }
    mockIssue()
    return
  }
  const t = await apiFetch<TokenResponse>('/auth/login', { method: 'POST', body: cred })
  setTokens(t.access_token, t.refresh_token)
}

export async function loginTelegram(initData: string): Promise<void> {
  if (USE_MOCK) {
    await delay(300)
    // Без окружения мини-аппа считаем Telegram недоступным (демо-503).
    throw new ApiError('TELEGRAM_DISABLED', 'Telegram не настроен', 503)
  }
  const t = await apiFetch<TokenResponse>('/auth/telegram', { method: 'POST', body: { init_data: initData } })
  setTokens(t.access_token, t.refresh_token)
}

/** Ротация refresh: старый refresh инвалидируется. Кидает при неудаче. */
async function refreshTokens(): Promise<void> {
  const rt = getRefreshToken()
  if (!rt) throw new ApiError('INVALID_REFRESH', 'нет refresh', 401)
  if (USE_MOCK) {
    await delay(250)
    if (rt !== mock.validRefresh) throw new ApiError('INVALID_REFRESH', 'refresh инвалиден', 401)
    mockIssue() // ротация: новый access+refresh, снимаем «протухание»
    return
  }
  const t = await apiFetch<TokenResponse>('/auth/refresh', { method: 'POST', body: { refresh_token: rt } })
  setTokens(t.access_token, t.refresh_token)
}

export async function logout(): Promise<void> {
  const rt = getRefreshToken()
  try {
    if (USE_MOCK) {
      await delay(150)
    } else if (rt) {
      await apiFetch<void>('/auth/logout', { method: 'POST', body: { refresh_token: rt } })
    }
  } finally {
    mock.session = null
    mock.validRefresh = null
    mock.accessExpired = false
    clearTokens() // уведомит подписчиков → UI покажет вход
  }
}

async function rawMe(): Promise<MeResponse> {
  if (USE_MOCK) {
    await delay(200)
    mockAssertAccess()
    const s = mockEnsureSession()
    return { account_id: s.accountId, active_context: s.active, contexts: s.contexts }
  }
  return apiFetch<MeResponse>('/auth/me', { auth: true })
}

async function rawSwitch(ctxType: string, tenantId: string): Promise<void> {
  if (USE_MOCK) {
    await delay(250)
    mockAssertAccess()
    const s = mockEnsureSession()
    const ctx = s.contexts.find((c) => c.ctx_type === ctxType && c.tenant_id === tenantId)
    if (!ctx) throw new ApiError('NO_MEMBERSHIP', 'нет членства', 403)
    s.active = ctx
    setAccessToken(`mock-access-${rand()}`) // новый access под новый контекст
    return
  }
  const r = await apiFetch<SwitchResponse>('/auth/switch', {
    method: 'POST',
    auth: true,
    body: { ctx_type: ctxType, tenant_id: tenantId },
  })
  setAccessToken(r.access_token)
}

/* ========================= 401 → refresh → retry ===================== */
let refreshInFlight: Promise<boolean> | null = null

async function attemptRefresh(): Promise<boolean> {
  try {
    await refreshTokens()
    return true
  } catch {
    clearTokens() // refresh не удался → разлогин (подписчики покажут вход)
    return false
  }
}

/** Дедуплицированный refresh: параллельные 401 ждут один и тот же запрос. */
function sharedRefresh(): Promise<boolean> {
  if (!refreshInFlight) {
    refreshInFlight = attemptRefresh().finally(() => {
      refreshInFlight = null
    })
  }
  return refreshInFlight
}

/**
 * Обернуть авторизованный вызов: на 401 один раз дёрнуть refresh и повторить.
 * `call` должен КАЖДЫЙ раз перечитывать токен (apiFetch/mock так и делают).
 */
export async function withAuthRetry<T>(call: () => Promise<T>): Promise<T> {
  try {
    return await call()
  } catch (e) {
    if (!isUnauthorized(e)) throw e
    const ok = await sharedRefresh()
    if (!ok) throw e
    return call() // повтор ровно один раз
  }
}

/* ====================== публичные авторизованные ==================== */
export function me(): Promise<MeResponse> {
  return withAuthRetry(rawMe)
}

export function switchContext(ctxType: string, tenantId: string): Promise<void> {
  return withAuthRetry(() => rawSwitch(ctxType, tenantId))
}

/** Для b2b-слоя: убедиться, что mock-сессия валидна (иначе 401 → refresh). */
export function assertMockAccess(): void {
  mockAssertAccess()
}

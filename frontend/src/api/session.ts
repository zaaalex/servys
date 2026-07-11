// Хранилище токенов access/refresh: память + localStorage. Единая точка правды по токенам —
// сетевой слой (api/http.ts) читает access отсюда, авторизация пишет сюда.

const ACCESS_KEY = 'servys.auth.access'
const REFRESH_KEY = 'servys.auth.refresh'

function readLS(k: string): string | null {
  try {
    return localStorage.getItem(k)
  } catch {
    return null // приватный режим / недоступный storage
  }
}

function writeLS(k: string, v: string | null): void {
  try {
    if (v === null) localStorage.removeItem(k)
    else localStorage.setItem(k, v)
  } catch {
    /* игнорируем — токен всё равно живёт в памяти */
  }
}

let access: string | null = readLS(ACCESS_KEY)
let refresh: string | null = readLS(REFRESH_KEY)

const clearedListeners = new Set<() => void>()

export function getAccessToken(): string | null {
  return access
}

export function getRefreshToken(): string | null {
  return refresh
}

/** Обновить только access (используется /auth/switch). */
export function setAccessToken(a: string): void {
  access = a
  writeLS(ACCESS_KEY, a)
}

/** Сохранить пару токенов (login/register/refresh-ротация). */
export function setTokens(a: string, r: string): void {
  access = a
  refresh = r
  writeLS(ACCESS_KEY, a)
  writeLS(REFRESH_KEY, r)
}

/** Сбросить сессию и уведомить подписчиков (→ UI показывает вход). */
export function clearTokens(): void {
  access = null
  refresh = null
  writeLS(ACCESS_KEY, null)
  writeLS(REFRESH_KEY, null)
  clearedListeners.forEach((cb) => cb())
}

export function hasSession(): boolean {
  return access !== null
}

/** Подписка на сброс сессии (refresh не удался / logout). Возвращает отписку. */
export function onTokensCleared(cb: () => void): () => void {
  clearedListeners.add(cb)
  return () => clearedListeners.delete(cb)
}

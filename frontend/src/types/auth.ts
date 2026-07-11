// Контракт /api/v1/auth/* как TypeScript-типы. 1:1 с контрактом координатора.
// Формат ошибок общий: {"error":{"code","message"}} (см. api/errors.ts).

/** Ответ register/login/telegram/refresh — пара токенов + время жизни access (секунды). */
export interface TokenResponse {
  access_token: string
  refresh_token: string
  expires_in: number
}

/** Ответ /auth/switch — только новый access (refresh не меняется). */
export interface SwitchResponse {
  access_token: string
}

/** Контекст доступа аккаунта: b2c (личный) или b2b (членство в СТО). */
export interface AuthContext {
  ctx_type: string // 'b2c' | 'b2b'
  tenant_id: string
  role: string
}

/** Ответ /auth/me. */
export interface MeResponse {
  account_id: string
  active_context: AuthContext
  contexts: AuthContext[]
}

export interface Credentials {
  email: string
  password: string
}

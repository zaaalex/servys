/** Ответ telegram/max/refresh — пара токенов и время жизни access в секундах. */
export interface TokenResponse {
  access_token: string
  refresh_token: string
  expires_in: number
}

/** Личный контекст аккаунта (b2c-гараж). */
export interface AuthContext {
  ctx_type: string
  tenant_id: string
  role: string
}

/** Ответ /auth/me. */
export interface MeResponse {
  account_id: string
  active_context: AuthContext
  contexts: AuthContext[]
  is_admin?: boolean
  telegram_id?: string
  name?: string
  username?: string
}


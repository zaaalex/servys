// Реактивное состояние авторизации — модульный синглтон (как useGarage).
// Держит аккаунт/контексты/активный контекст и статус приложения (loading/anonymous/authed).
// Все сетевые детали — в api/auth. На сброс сессии (refresh не удался) подписываемся один раз.

import { computed, reactive } from 'vue'
import * as auth from '@/api/auth'
import { hasSession, onTokensCleared } from '@/api/session'
import { describeError, type UiError } from '@/api/errors'
import type { AuthContext, MeResponse } from '@/types/auth'

type AuthStatus = 'loading' | 'anonymous' | 'authed'

const state = reactive({
  status: 'loading' as AuthStatus,
  accountId: '',
  active: null as AuthContext | null,
  contexts: [] as AuthContext[],
  busy: false,
  error: null as UiError | null,
})

// Сброс токенов (logout / провал refresh) → показать вход.
onTokensCleared(() => {
  state.status = 'anonymous'
  state.accountId = ''
  state.active = null
  state.contexts = []
})

function applyMe(me: MeResponse): void {
  state.accountId = me.account_id
  state.active = me.active_context
  state.contexts = me.contexts
  state.status = 'authed'
}

let initialized = false

export function useAuth() {
  async function init(): Promise<void> {
    if (initialized) return
    initialized = true
    if (!hasSession()) {
      state.status = 'anonymous'
      return
    }
    state.status = 'loading'
    try {
      applyMe(await auth.me())
    } catch {
      state.status = 'anonymous' // onTokensCleared мог уже сработать
    }
  }

  async function submit(fn: () => Promise<void>): Promise<boolean> {
    if (state.busy) return false
    state.busy = true
    state.error = null
    try {
      await fn()
      applyMe(await auth.me())
      return true
    } catch (e) {
      state.error = describeError(e)
      return false
    } finally {
      state.busy = false
    }
  }

  function login(email: string, password: string): Promise<boolean> {
    return submit(() => auth.login({ email, password }))
  }

  function register(email: string, password: string): Promise<boolean> {
    return submit(() => auth.register({ email, password }))
  }

  function loginTelegram(): Promise<boolean> {
    const tg = (window as unknown as { Telegram?: { WebApp?: { initData?: string } } }).Telegram
    const initData = tg?.WebApp?.initData ?? ''
    return submit(() => auth.loginTelegram(initData))
  }

  async function logout(): Promise<void> {
    try {
      await auth.logout() // clearTokens внутри → onTokensCleared сбросит состояние
    } catch {
      /* всё равно разлогинены локально */
    }
  }

  /** Активировать контекст указанного типа (если он есть у аккаунта и ещё не активен). */
  async function switchTo(ctxType: string): Promise<boolean> {
    const ctx = state.contexts.find((c) => c.ctx_type === ctxType)
    if (!ctx) return false
    if (state.active && state.active.ctx_type === ctxType && state.active.tenant_id === ctx.tenant_id) {
      return true
    }
    state.busy = true
    state.error = null
    try {
      await auth.switchContext(ctx.ctx_type, ctx.tenant_id)
      applyMe(await auth.me())
      return true
    } catch (e) {
      state.error = describeError(e)
      return false
    } finally {
      state.busy = false
    }
  }

  /** Перечитать /auth/me (например, после подключения первого СТО — появился b2b-контекст). */
  async function reloadMe(): Promise<void> {
    try {
      applyMe(await auth.me())
    } catch {
      /* сессия слетела — onTokensCleared покажет вход */
    }
  }

  return {
    status: computed(() => state.status),
    accountId: computed(() => state.accountId),
    active: computed(() => state.active),
    contexts: computed(() => state.contexts),
    busy: computed(() => state.busy),
    error: computed(() => state.error),
    isAuthed: computed(() => state.status === 'authed'),
    hasB2B: computed(() => state.contexts.some((c) => c.ctx_type === 'b2b')),
    activeCtxType: computed(() => state.active?.ctx_type ?? 'b2c'),
    init,
    login,
    register,
    loginTelegram,
    logout,
    switchTo,
    reloadMe,
  }
}

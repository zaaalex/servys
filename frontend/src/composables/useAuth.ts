// Реактивное состояние авторизации — модульный синглтон (как useGarage).
// Держит аккаунт и статус приложения (loading/anonymous/authed).
// Все сетевые детали — в api/auth. На сброс сессии (refresh не удался) подписываемся один раз.

import { computed, reactive } from 'vue'
import * as auth from '@/api/auth'
import { readBridge, resolveLaunch, type BridgeWindow, type PlatformId } from '@/platform/resolve'
import { hasSession, onTokensCleared } from '@/api/session'
import { describeError, type UiError } from '@/api/errors'
import type { MeResponse } from '@/types/auth'

type AuthStatus = 'loading' | 'anonymous' | 'authed'

export type AuthPlace = 'telegram' | 'max' | 'dev' | ''

const PLACE_KEY = 'servys.place'

const state = reactive({
  status: 'loading' as AuthStatus,
  accountId: '',
  place: (sessionStorage.getItem(PLACE_KEY) ?? '') as AuthPlace,
  busy: false,
  error: null as UiError | null,
})

function rememberPlace(place: Exclude<AuthPlace, ''>): void {
  state.place = place
  sessionStorage.setItem(PLACE_KEY, place)
}

// Сброс токенов (logout / провал refresh) → показать вход.
onTokensCleared(() => {
  state.status = 'anonymous'
  state.accountId = ''
  state.place = ''
  sessionStorage.removeItem(PLACE_KEY)
})

function applyMe(me: MeResponse): void {
  state.accountId = me.account_id
  state.status = 'authed'
}

let initialized = false

export function useAuth() {
  function loginMessenger(provider: 'telegram' | 'max', initData: string): Promise<boolean> {
    const call = provider === 'max' ? auth.loginMax(initData) : auth.loginTelegram(initData)
    return submit(() => call)
  }

  function messengerLaunch(): { id: PlatformId; initData: string; encoded: boolean } {
    const launch = resolveLaunch({ hash: window.location.hash, search: window.location.search })
    if (launch.initData && (launch.id === 'telegram' || launch.id === 'max')) {
      return { id: launch.id, initData: launch.initData, encoded: true }
    }
    const bridge = readBridge(window as BridgeWindow)
    return { id: bridge.id, initData: bridge.initData, encoded: false }
  }

  async function enter(): Promise<void> {
    if (hasSession()) {
      state.status = 'loading'
      try {
        applyMe(await auth.me())
        return
      } catch {
        /* сессия не подошла — пробуем мессенджер или локальный гараж */
      }
    }
    const launch = messengerLaunch()
    if (launch.initData && (launch.id === 'telegram' || launch.id === 'max')) {
      rememberPlace(launch.id)
      let data = launch.initData
      if (launch.encoded) {
        try {
          data = decodeURIComponent(launch.initData)
        } catch {
          /* значение уже без кодирования */
        }
      }
      const ok = await loginMessenger(launch.id, data)
      if (ok) return
    }
    // Как локальный просмотр trainys: vite без мессенджера сразу открывает гараж.
    if (import.meta.env.DEV) {
      try {
        await auth.loginDev()
        rememberPlace('dev')
        applyMe(await auth.me())
        return
      } catch {
        /* на этом API локальный вход выключен */
      }
    }
    state.status = 'anonymous'
  }

  async function init(): Promise<void> {
    if (initialized) return
    initialized = true
    await enter()
  }

  function retry(): Promise<void> {
    return enter()
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

  async function logout(): Promise<void> {
    try {
      await auth.logout() // clearTokens внутри → onTokensCleared сбросит состояние
    } catch {
      /* всё равно разлогинены локально */
    }
  }

  return {
    status: computed(() => state.status),
    accountId: computed(() => state.accountId),
    place: computed(() => state.place),
    busy: computed(() => state.busy),
    error: computed(() => state.error),
    isAuthed: computed(() => state.status === 'authed'),
    init,
    retry,
    logout,
  }
}

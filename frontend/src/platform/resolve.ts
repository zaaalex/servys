export type PlatformId = 'telegram' | 'max' | 'unknown'

export interface BridgeWindow {
  Telegram?: { WebApp?: { initData?: string } }
  WebApp?: { initData?: string }
}

// readBridge читает initData уже загруженного SDK. Пустая строка и оба
// мессенджера сразу — это не вход.
export function readBridge(win: BridgeWindow): { id: PlatformId; initData: string } {
  const telegram = win.Telegram?.WebApp?.initData ?? ''
  const max = win.WebApp?.initData ?? ''
  if (telegram && max) {
    return { id: 'unknown', initData: '' }
  }
  if (telegram) {
    return { id: 'telegram', initData: telegram }
  }
  if (max) {
    return { id: 'max', initData: max }
  }
  return { id: 'unknown', initData: '' }
}

// resolveLaunch смотрит фрагмент и query. Значение initData остаётся как в URL,
// без decode. Фрагмент важнее query и, выбрав одну платформу, query другой не читает.
export function resolveLaunch(input: { hash: string; search: string }): { id: PlatformId; initData: string } {
  const fragment = launchParams(input.hash)
  const picked = pick(fragment)
  if (picked) {
    return picked
  }
  return pick(launchParams(input.search)) ?? { id: 'unknown', initData: '' }
}

function pick(found: { telegram?: string; max?: string }): { id: PlatformId; initData: string } | null {
  if (found.telegram && found.max) {
    return { id: 'unknown', initData: '' }
  }
  if (found.telegram) {
    return { id: 'telegram', initData: found.telegram }
  }
  if (found.max) {
    return { id: 'max', initData: found.max }
  }
  return null
}

function launchParams(source: string): { telegram?: string; max?: string } {
  let raw = source
  if (raw.startsWith('#') || raw.startsWith('?')) {
    raw = raw.slice(1)
  }
  const found: { telegram?: string; max?: string } = {}
  for (const part of raw.split('&')) {
    const eq = part.indexOf('=')
    if (eq <= 0) {
      continue
    }
    const key = part.slice(0, eq)
    const value = part.slice(eq + 1)
    if (value === '') {
      continue
    }
    if (key === 'tgWebAppData' && found.telegram === undefined) {
      found.telegram = value
    } else if (key === 'WebAppData' && found.max === undefined) {
      found.max = value
    }
  }
  return found
}

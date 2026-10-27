type ChatHost = {
  Telegram?: { WebApp?: { close?: () => void } }
  WebApp?: { close?: () => void }
}

/** Закрывает мини-приложение и возвращает человека в чат бота. */
export function closeToChat(): boolean {
  const host = window as ChatHost
  const close = host.Telegram?.WebApp?.close ?? host.WebApp?.close
  if (!close) return false
  close()
  return true
}

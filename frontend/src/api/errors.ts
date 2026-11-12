// Единый тип ошибки API + перевод кодов бэка в дружелюбный текст для UI.
// Формат тела ошибки на бэке: {"error":{"code","message"}}.

export interface ApiErrorBody {
  error: { code: string; message?: string }
}

/** Ошибка API с кодом из тела и HTTP-статусом. */
export class ApiError extends Error {
  code: string
  status: number
  constructor(code: string, message: string, status = 0) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

/** Форма ошибки для отрисовки: технический код + человекочитаемый текст. */
export interface UiError {
  code: string | null
  message: string
}

// Неизвестный код → показываем message с сервера.
const FRIENDLY: Record<string, string> = {
  NOT_FOUND: 'Не найдено.',
  STORE_ERROR: 'Внутренняя ошибка хранилища. Попробуйте позже.',
  INVALID_VIN: 'Некорректный VIN: нужно ровно 17 символов (латиница и цифры, без I, O, Q).',
  NO_TOKEN: 'Нужно войти в аккаунт.',
  INVALID_TOKEN: 'Сессия истекла — обновляем…',
  INVALID_REFRESH: 'Сессия истекла. Войдите заново.',
  AUTH_DISABLED: 'Авторизация выключена на сервере.',
  TELEGRAM_DISABLED: 'Вход через Telegram не настроен на сервере.',
  MAX_DISABLED: 'Вход через MAX не настроен на сервере.',
  TELEGRAM_INVALID: 'Не удалось проверить вход из Telegram.',
  NOT_ADMIN: 'Этот раздел только для администратора.',
  BAD_TELEGRAM_ID: 'Нужен числовой номер Telegram.',
  BAD_QUESTION: 'Напишите вопрос от 1 до 300 знаков.',
  MAX_INVALID: 'Не удалось проверить вход из MAX.',
}

export function describeError(e: unknown): UiError {
  if (e instanceof ApiError) {
    return { code: e.code, message: FRIENDLY[e.code] ?? e.message ?? 'Ошибка запроса.' }
  }
  if (e instanceof DOMException && e.name === 'AbortError') {
    return { code: 'ABORTED', message: 'Запрос отменён.' }
  }
  if (e instanceof TypeError) {
    return { code: 'NETWORK', message: 'Нет связи с сервером. Проверьте, что бэкенд запущен.' }
  }
  return { code: null, message: e instanceof Error ? e.message : 'Неизвестная ошибка.' }
}

/** 401 / протухший access — сигнал попробовать refresh. */
export function isUnauthorized(e: unknown): boolean {
  return e instanceof ApiError && (e.status === 401 || e.code === 'NO_TOKEN' || e.code === 'INVALID_TOKEN')
}

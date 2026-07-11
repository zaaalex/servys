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
  // b2b
  B2B_DISABLED: 'b2b-модуль выключен на сервере. Нужен APP_SECRET_KEY в окружении бэкенда.',
  INVALID_WEBHOOK: 'Вебхук не прошёл проверку. Укажите рабочий входящий вебхук Bitrix24 (…/rest/…).',
  NOT_FOUND: 'СТО не найдено — возможно, оно было удалено. Обновите список.',
  SCAN_ERROR: 'Не удалось просканировать автопарк: Bitrix не ответил. Попробуйте ещё раз.',
  STORE_ERROR: 'Внутренняя ошибка хранилища. Попробуйте позже.',
  // vin
  INVALID_VIN: 'Некорректный VIN: нужно ровно 17 символов (латиница и цифры, без I, O, Q).',
  // auth
  BAD_CREDENTIALS: 'Неверный email или пароль.',
  EMAIL_TAKEN: 'Этот email уже зарегистрирован — попробуйте войти.',
  NO_TOKEN: 'Нужно войти в аккаунт.',
  INVALID_TOKEN: 'Сессия истекла — обновляем…',
  INVALID_REFRESH: 'Сессия истекла. Войдите заново.',
  NO_MEMBERSHIP: 'Нет доступа к b2b: аккаунт не состоит ни в одном СТО.',
  FORBIDDEN: 'Доступ запрещён: это СТО принадлежит другому аккаунту.',
  AUTH_DISABLED: 'Авторизация выключена на сервере.',
  ADMIN_DISABLED: 'Операторский режим выключен на сервере (нет ADMIN_TOKEN).',
  TELEGRAM_DISABLED: 'Вход через Telegram не настроен на сервере.',
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

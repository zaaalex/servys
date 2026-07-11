// Контракт b2b-API (/api/v1/b2b) как TypeScript-типы — единственный источник формы данных b2b-слоя.
// 1:1 с backend/api/b2b.go и backend/b2b/README.md. При расхождении с бэком правим здесь.
// b2b — самостоятельный контракт («движок удержания для СТО»), не пересекается с b2c (vehicles/alerts).

/** СТО в списке и в ответе connect. */
export interface ServiceCenter {
  id: string
  name: string
  responsible_id: number
}

/** POST /service-centers — подключить СТО по входящему вебхуку Bitrix24. */
export interface ConnectServiceCenterRequest {
  name: string
  webhook: string
  responsible_id: number
}

/** GET /service-centers */
export interface ServiceCentersResponse {
  service_centers: ServiceCenter[]
}

/** POST /service-centers/{id}/scan — отчёт по одному СТО. */
export interface ScanReport {
  cars: number
  due_items: number
  pushed: number
  skipped: number
  /** непустой при частичных сбоях (напр. одно авто не прочиталось из CRM). */
  errors?: string[]
}

/** POST /scan-all — сводка по всем СТО. */
export interface ScanSummary {
  centers: number
  due_items: number
  pushed: number
  skipped: number
  errors?: string[]
}

/** Формат ошибки b2b-API: {"error":{"code","message"}}. */
export interface ApiErrorBody {
  error: { code: string; message?: string }
}

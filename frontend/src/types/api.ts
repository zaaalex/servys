// Контракт §4.A (модель vehicles/alerts, ADR-001). JSON — camelCase (ADR §5.2).
// Backend ещё не поднят: точные формы/казинг сверить с Dev 1, когда появится API.
// Мок (mock/*.json) — текущий якорь этих типов.

export type FuelType = 'gasoline' | 'diesel' | 'hybrid' | 'phev' | 'bev'
export type ApiBodyType = 'sedan' | 'hatchback' | 'wagon' | 'suv' | 'coupe' | 'pickup' | 'minivan' | 'unknown'
export type IdentificationSource = 'drom' | 'manual'

/** Статусы карточек (ADR §8.6). */
export type AlertStatus =
  | 'OK'
  | 'SOON'
  | 'DUE'
  | 'OVERDUE'
  | 'RESEARCHING'
  | 'NO_INTERVAL'
  | 'INSPECTION_REQUIRED'

/** Типы событий/алертов (ADR §8.7). */
export type AlertType =
  | 'ODOMETER_UPDATE_REQUIRED'
  | 'MAINTENANCE_SOON'
  | 'MAINTENANCE_DUE'
  | 'MAINTENANCE_OVERDUE'
  | 'MAINTENANCE_OK'
  | 'INSPECTION_REQUIRED'
  | 'RISK_DIAGNOSTIC_RECOMMENDED'
  | 'KNOWLEDGE_RESEARCH_FAILED'

export type Severity = 'low' | 'medium' | 'high'

/** Категория важности компонента из каталога (спека §«Каталог компонентов»). */
export type MaintenanceCategory = 'primary' | 'secondary'

/** Данные из отзывов/форумов владельцев — НЕ официальный регламент (спека §«Контракт данных»). */
export interface CommunityNote {
  /** Реальный интервал по опыту владельцев, км (напр. 45000). */
  realIntervalKm: number
  /** Человекочитаемый вывод сообщества. */
  note: string
  /** URL источника или "demo". */
  source: string
  /** Сколько отзывов/источников — сила консенсуса. */
  reports: number
}

export interface Me {
  id: string
  clientKey: string
  tenantType: 'b2c' | 'b2b'
}

/** Нормализованная подпись авто из VIN-резолва (ADR §5.2). */
export interface VehicleSignature {
  make: string
  model: string
  year: number
  engineDisplacementCc?: number | null
  powerHp?: number | null
  bodyType: ApiBodyType
  fuelType?: FuelType | null
  marketHint?: string | null
}

export interface VinResolveResult {
  vin: string
  signature: VehicleSignature
  matchLevel: 'exact' | 'partial' | 'none'
  identificationSource: IdentificationSource
}

export interface Vehicle {
  id: string
  vin: string
  make: string
  model: string
  year: number
  engineCc: number
  powerHp: number
  color: string
  bodyType: ApiBodyType
  fuelType: FuelType
  identificationSource: IdentificationSource
  currentOdometer: number
  /** ISO-8601 */
  odometerUpdatedAt: string
}

export interface CreateVehicleRequest {
  vin?: string
  make?: string
  model?: string
  year?: number
  engineCc?: number
  powerHp?: number
  bodyType?: ApiBodyType
  fuelType?: FuelType
  color?: string
  /** первый пробег */
  odometer: number
}

export interface OdometerUpdate {
  odometer: number
}

/** Отметка выполненной работы (ADR §8.4). */
export interface ServiceEventRequest {
  componentCode: string
  operation?: string
  /** ISO-8601 date */
  date: string
  odometer: number
  note?: string
}

export interface Alert {
  id: string
  vehicleId: string
  type: AlertType
  ruleCode: string
  severity: Severity
  status: AlertStatus
  title: string
  description: string
  /** 0, если интервал неприменим (NO_INTERVAL/INSPECTION_REQUIRED) */
  dueAtKm: number
  /** Секция витрины: основные / дополнительные (из каталога, fallback 'primary'). */
  category: MaintenanceCategory
  /** Данные из отзывов владельцев рядом с регламентом; отсутствуют — undefined. */
  community?: CommunityNote | null
}

export interface HealthResponse {
  status: string
}

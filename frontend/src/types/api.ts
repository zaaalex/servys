// Контракт A (HTTP API, pull) как TypeScript-типы — единственный источник формы данных на фронте.
// 1:1 с mock/recommendations.json и спекой §4.A. При расхождении с моком/бэком правим здесь.

export type Category = 'regular' | 'known_issue'
export type Severity = 'low' | 'medium' | 'high'
export type Status = 'overdue' | 'due_soon' | 'upcoming'
export type GeneratedBy = 'llm' | 'cache'

export interface Car {
  make: string
  model: string
  year: number
  mileage_km: number
}

export interface Item {
  id: string
  title: string
  category: Category
  severity: Severity
  /** 0, если неприменимо */
  interval_km: number
  due_at_km: number
  status: Status
  note: string
}

export interface RecommendationsRequest {
  make: string
  model: string
  year: number
  mileage_km: number
}

export interface RecommendationsResponse {
  car: Car
  items: Item[]
  generated_by: GeneratedBy
  cached: boolean
}

export interface HealthResponse {
  status: string
}

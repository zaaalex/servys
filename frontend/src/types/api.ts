export type FuelType = 'gasoline' | 'diesel' | 'hybrid' | 'phev' | 'bev'
export type ApiBodyType = 'sedan' | 'hatchback' | 'wagon' | 'suv' | 'coupe' | 'pickup' | 'minivan' | 'unknown'
export type IdentificationSource = 'manual' | 'drom' | 'nhtsa'

export type AlertStatus = 'OK' | 'SOON' | 'DUE' | 'OVERDUE' | 'INSPECT'

export type AlertType =
  | 'ODOMETER_UPDATE_REQUIRED'
  | 'MAINTENANCE_HISTORY_REQUIRED'
  | 'MAINTENANCE_SOON'
  | 'MAINTENANCE_DUE'
  | 'MAINTENANCE_OVERDUE'
  | 'MAINTENANCE_OK'
  | 'MAINTENANCE_INSPECT'
  | 'REGULATION_NOT_FOUND'

export type Severity = 'low' | 'medium' | 'high'

export type MaintenanceCategory = 'primary' | 'secondary'

export interface CommunityNote {
  realIntervalKm: number
  note: string
  source: string
  reports: number
}

export interface Me {
  id: string
  clientKey: string
  tenantType: 'b2c'
}

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
  identificationSource?: IdentificationSource
  odometer: number
}

export interface OdometerUpdate {
  odometer: number
}

export interface ServiceEventRequest {
  ruleCode: string
  operation?: string
  date: string
  odometer: number
  note?: string
  costRub?: number
  place?: string
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
  dueAtKm: number
  category: MaintenanceCategory
  community?: CommunityNote | null
  basis?: 'profile' | 'typical' | 'found'
}

export interface HealthResponse {
  status: string
}

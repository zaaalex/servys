import { apiFetch } from '@/api/http'

export interface GarageUser {
  account_id: string
  telegram_id: string
  name: string
  username: string
  role: string
  created_at: string
}

export interface PulseDay {
  date: string
  users: number
  cars: number
}

export interface Inspection {
  vehicle_id: string
  make: string
  model: string
  year: number
  vin: string
  mileage_km: number
  checked_at: string
  next_at: string
}

export interface AdminPulse {
  users: number
  cars: number
  users_today: number
  cars_today: number
  checks_today: number
  check_every_days: number
  days: PulseDay[]
  inspections: Inspection[]
}

export interface RunningModel {
  role: 'search' | 'text' | 'book' | string
  name: string
}

export function listModels(): Promise<{ models: RunningModel[] }> {
  return apiFetch('/admin/models', { auth: true })
}

export function loadPulse(): Promise<AdminPulse> {
  return apiFetch('/admin/overview', { auth: true })
}

export function listUsers(): Promise<{ users: GarageUser[] }> {
  return apiFetch('/admin/users', { auth: true })
}

export function addUser(telegramId: string, name: string): Promise<GarageUser> {
  return apiFetch('/admin/users', {
    method: 'POST',
    auth: true,
    body: { telegram_id: telegramId, name },
  })
}

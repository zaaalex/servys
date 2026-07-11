// Мок-декодер VIN (заглушка вместо Drom-резолвера, ADR §5.1). По номеру определяем марку
// (WMI-префикс), модель, год (10-й символ), тип кузова и характеристики. Используется client.resolveVin
// в mock-режиме; на живом бэке заменяется POST /api/v1/vin/resolve.

import type { ApiBodyType, FuelType } from '@/types/api'

export interface DecodedVin {
  make: string
  model: string
  year: number
  bodyType: ApiBodyType
  fuelType: FuelType
  engineCc: number
  powerHp: number
  matchLevel: 'exact' | 'partial'
}

export type VinResult = DecodedVin | { err: string }

interface DbEntry {
  make: string
  model: string
  year: number
  bodyType: ApiBodyType
  engineCc: number
  powerHp: number
}

const VIN_DB: Record<string, DbEntry> = {
  JTDBE32K700261000: { make: 'Toyota', model: 'Camry', year: 2018, bodyType: 'sedan', engineCc: 2494, powerHp: 181 },
  WVWZZZ1KZAW000001: { make: 'Volkswagen', model: 'Golf', year: 2020, bodyType: 'hatchback', engineCc: 1395, powerHp: 150 },
  '5UXWX7C5XBA000001': { make: 'BMW', model: 'X5', year: 2021, bodyType: 'suv', engineCc: 2998, powerHp: 340 },
}

const WMI: Record<string, string> = {
  JTD: 'Toyota', JT: 'Toyota', JN: 'Nissan', JH: 'Honda', JM: 'Mazda',
  WVW: 'Volkswagen', WV: 'Volkswagen', WAU: 'Audi', WBA: 'BMW', '5UX': 'BMW',
  WDB: 'Mercedes-Benz', WDD: 'Mercedes-Benz', XTA: 'Lada', XW: 'Kia', KN: 'Kia',
  KM: 'Hyundai', '1HG': 'Honda', '1G': 'Chevrolet', VF: 'Renault', YV: 'Volvo',
  SAL: 'Land Rover', ZFA: 'Fiat',
}

const MODELS: Record<string, string[]> = {
  Toyota: ['Camry', 'Corolla', 'RAV4', 'Land Cruiser'],
  Volkswagen: ['Golf', 'Passat', 'Tiguan', 'Polo'],
  BMW: ['3 Series', '5 Series', 'X5', 'X3'],
  Audi: ['A4', 'A6', 'Q5', 'Q7'],
  'Mercedes-Benz': ['C-Class', 'E-Class', 'GLC'],
  Nissan: ['Qashqai', 'X-Trail', 'Almera'],
  Honda: ['Civic', 'CR-V', 'Accord'],
  Kia: ['Rio', 'Sportage', 'Ceed'],
  Hyundai: ['Solaris', 'Tucson', 'Creta'],
  Lada: ['Vesta', 'Granta', 'Niva'],
  Renault: ['Logan', 'Duster', 'Sandero'],
  Volvo: ['XC60', 'XC90', 'S60'],
  'Land Rover': ['Discovery', 'Range Rover'],
  Chevrolet: ['Cruze', 'Niva'],
  Mazda: ['3', '6', 'CX-5'],
  Fiat: ['500', 'Punto'],
  _: ['Sedan', 'Cross', 'Hatch'],
}

const SUV = new Set([
  'RAV4', 'Land Cruiser', 'Tiguan', 'X5', 'X3', 'Q5', 'Q7', 'GLC', 'Qashqai', 'X-Trail',
  'CR-V', 'Sportage', 'Tucson', 'Creta', 'Niva', 'Duster', 'XC60', 'XC90', 'Discovery',
  'Range Rover', 'CX-5', 'Cross',
])
const HATCH = new Set(['Golf', 'Polo', 'Rio', 'Ceed', 'Solaris', 'Civic', '500', 'Punto', 'Granta', 'Hatch', 'Vesta', 'Sandero'])

const YEARMAP = 'ABCDEFGHJKLMNPRSTVWXY123456789'

function hash(s: string): number {
  let h = 0
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0
  return h
}
function wmiMake(vin: string): string {
  return WMI[vin.slice(0, 3)] ?? WMI[vin.slice(0, 2)] ?? WMI[vin.slice(0, 1)] ?? 'Авто'
}
function guessBody(model: string, h: number): ApiBodyType {
  if (SUV.has(model)) return 'suv'
  if (HATCH.has(model)) return 'hatchback'
  return h % 4 === 0 ? 'coupe' : 'sedan'
}
function decodeYear(ch: string, h: number): number {
  const i = YEARMAP.indexOf(ch)
  const y = i < 0 ? 2012 + (h % 12) : i < 21 ? 2010 + i : 2001 + (i - 21)
  return Math.min(y, 2026)
}

export function decodeVin(raw: string): VinResult {
  const vin = (raw ?? '').trim().toUpperCase().replace(/\s+/g, '')
  if (vin.length < 11) return { err: 'Введите VIN (не короче 11 символов).' }

  const known = VIN_DB[vin]
  if (known) {
    return { ...known, fuelType: 'gasoline', matchLevel: 'exact' }
  }

  const h = hash(vin)
  const make = wmiMake(vin)
  const models = MODELS[make] ?? MODELS._
  const model = models[h % models.length]
  return {
    make,
    model,
    year: decodeYear(vin.charAt(9), h),
    bodyType: guessBody(model, h),
    fuelType: 'gasoline',
    engineCc: 1400 + (h % 16) * 100,
    powerHp: 90 + (h % 22) * 10,
    matchLevel: 'partial',
  }
}

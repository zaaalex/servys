// Аватар машины: цвет кузова + тип кузова. Цвета — в бренд-гамме (teal→azure и рядом).

import type { ApiBodyType } from '@/types/api'

export type RGB = [number, number, number]

export interface ColorPreset {
  name: string
  rgb: RGB
  css: string
}

export const COLOR_PRESETS: ColorPreset[] = [
  { name: 'Бирюза', rgb: [0.12, 0.75, 0.69], css: '#1fbfb0' },
  { name: 'Азур', rgb: [0.3, 0.55, 1.0], css: '#4d8dff' },
  { name: 'Индиго', rgb: [0.55, 0.48, 1.0], css: '#8b7bff' },
  { name: 'Коралл', rgb: [1.0, 0.42, 0.35], css: '#ff6b5a' },
  { name: 'Янтарь', rgb: [1.0, 0.7, 0.24], css: '#ffb43d' },
  { name: 'Графит', rgb: [0.42, 0.47, 0.57], css: '#6b7488' },
]

/** Типы кузова у 3D-движка (car3d/engine.ts). */
export type SceneBody = 'sedan' | 'hatch' | 'suv' | 'coupe' | 'pickup'

/** Пикер типа кузова в UI — значения контракта (ApiBodyType). */
export const BODY_TYPES: { id: ApiBodyType; name: string }[] = [
  { id: 'sedan', name: 'Седан' },
  { id: 'hatchback', name: 'Хэтчбек' },
  { id: 'suv', name: 'Внедорожник' },
  { id: 'coupe', name: 'Купе' },
  { id: 'pickup', name: 'Пикап' },
]

/** ApiBodyType → тип кузова 3D-сцены. */
export function apiBodyToScene(b: ApiBodyType): SceneBody {
  switch (b) {
    case 'hatchback':
      return 'hatch'
    case 'suv':
    case 'minivan':
      return 'suv'
    case 'coupe':
      return 'coupe'
    case 'pickup':
      return 'pickup'
    default:
      return 'sedan'
  }
}

/** '#rrggbb' → RGB 0..1 для 3D. Неизвестное → нейтральный teal. */
export function hexToRgb(hex: string): RGB {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex ?? '')
  if (!m) return [0.16, 0.5, 0.56]
  const n = parseInt(m[1], 16)
  return [((n >> 16) & 255) / 255, ((n >> 8) & 255) / 255, (n & 255) / 255]
}

/** Ближайший цвет-пресет к hex (для подсветки выбранного свотча). */
export function colorIndexOf(hex: string): number {
  const i = COLOR_PRESETS.findIndex((c) => c.css.toLowerCase() === (hex ?? '').toLowerCase())
  return i < 0 ? 0 : i
}

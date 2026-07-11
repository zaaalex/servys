// Аватар машины: цвет кузова + тип кузова. Цвета — в бренд-гамме (teal→azure и рядом).

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

export type BodyType = 'sedan' | 'hatch' | 'suv' | 'coupe' | 'pickup'

export interface BodyTypeOption {
  id: BodyType
  name: string
}

export const BODY_TYPES: BodyTypeOption[] = [
  { id: 'sedan', name: 'Седан' },
  { id: 'hatch', name: 'Хэтчбек' },
  { id: 'suv', name: 'Внедорожник' },
  { id: 'coupe', name: 'Купе' },
  { id: 'pickup', name: 'Пикап' },
]

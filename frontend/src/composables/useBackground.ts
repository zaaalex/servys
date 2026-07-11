// Выбор фона приложения. Синглтон + persist в localStorage.

import { ref } from 'vue'

export type BgId = 'aurora' | 'grid' | 'stars' | 'plain'

export interface BgOption {
  id: BgId
  name: string
}

export const BACKGROUNDS: BgOption[] = [
  { id: 'aurora', name: 'Аврора' },
  { id: 'grid', name: 'Сетка' },
  { id: 'stars', name: 'Звёзды' },
  { id: 'plain', name: 'Минимал' },
]

const KEY = 'servys.bg'

function load(): BgId {
  try {
    const v = localStorage.getItem(KEY) as BgId | null
    if (v && BACKGROUNDS.some((b) => b.id === v)) return v
  } catch {
    /* ignore */
  }
  return 'aurora'
}

const current = ref<BgId>(load())

export function useBackground() {
  function set(id: BgId): void {
    current.value = id
    try {
      localStorage.setItem(KEY, id)
    } catch {
      /* ignore */
    }
  }
  return { current, set }
}

// Гараж пользователя: список машин, активная, добавление. Состояние — модульный синглтон,
// общий для всех компонентов. Отредактированный регламент хранится прямо в машине (persist на клиенте).

import { computed, reactive } from 'vue'
import type { BodyType } from '@/data/presets'
import type { Item } from '@/types/api'

export interface Vehicle {
  id: number
  make: string
  model: string
  year: number
  mileage_km: number
  /** индекс в COLOR_PRESETS */
  colorIndex: number
  type: BodyType
  /** сохранённый/отредактированный регламент; undefined — ещё не загружали */
  items?: Item[]
}

const state = reactive({
  cars: [] as Vehicle[],
  activeId: 0,
  nextId: 1,
})

function seed(): void {
  if (state.cars.length) return
  state.cars.push(
    { id: state.nextId++, make: 'Toyota', model: 'Camry', year: 2018, mileage_km: 95000, colorIndex: 0, type: 'sedan' },
    { id: state.nextId++, make: 'Volkswagen', model: 'Golf', year: 2020, mileage_km: 41000, colorIndex: 1, type: 'hatch' },
  )
  state.activeId = state.cars[0].id
}
seed()

export function useGarage() {
  const cars = computed(() => state.cars)
  const activeId = computed(() => state.activeId)
  const activeCar = computed(() => state.cars.find((c) => c.id === state.activeId) ?? null)

  function setActive(id: number): void {
    state.activeId = id
  }

  function addVehicle(v: Omit<Vehicle, 'id'>): Vehicle {
    const car: Vehicle = { ...v, id: state.nextId++ }
    state.cars.push(car)
    state.activeId = car.id
    return car
  }

  return { cars, activeId, activeCar, setActive, addVehicle }
}

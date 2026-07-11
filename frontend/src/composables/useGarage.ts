// Гараж пользователя поверх контракта §4.A. Состояние — модульный синглтон, общий для компонентов.
// В mock-режиме client держит in-memory стор; в live — реальные /vehicles.

import { computed, reactive } from 'vue'
import { createVehicle, deleteVehicle, listVehicles, updateOdometer } from '@/api/client'
import type { CreateVehicleRequest, Vehicle } from '@/types/api'

const state = reactive({
  vehicles: [] as Vehicle[],
  activeId: '' as string,
  loading: false,
  error: '' as string,
})

let loaded = false

async function loadGarage(): Promise<void> {
  state.loading = true
  state.error = ''
  try {
    state.vehicles = await listVehicles()
    if (!state.activeId && state.vehicles.length) state.activeId = state.vehicles[0].id
  } catch (e) {
    state.error = e instanceof Error ? e.message : 'Не удалось загрузить гараж'
  } finally {
    state.loading = false
  }
}

export function useGarage() {
  if (!loaded) {
    loaded = true
    void loadGarage()
  }

  const vehicles = computed(() => state.vehicles)
  const activeId = computed(() => state.activeId)
  const activeVehicle = computed(() => state.vehicles.find((v) => v.id === state.activeId) ?? null)
  const loading = computed(() => state.loading)
  const error = computed(() => state.error)

  function setActive(id: string): void {
    state.activeId = id
  }

  async function addVehicle(body: CreateVehicleRequest): Promise<Vehicle> {
    const v = await createVehicle(body)
    state.vehicles.push(v)
    state.activeId = v.id
    return v
  }

  async function removeVehicle(id: string): Promise<void> {
    await deleteVehicle(id)
    const i = state.vehicles.findIndex((v) => v.id === id)
    if (i >= 0) state.vehicles.splice(i, 1)
    if (state.activeId === id) {
      state.activeId = state.vehicles[0]?.id ?? ''
    }
  }

  async function setOdometer(id: string, odometer: number): Promise<void> {
    const updated = await updateOdometer(id, odometer)
    const i = state.vehicles.findIndex((v) => v.id === id)
    if (i >= 0) state.vehicles[i] = updated
  }

  return { vehicles, activeId, activeVehicle, loading, error, setActive, addVehicle, removeVehicle, setOdometer, reload: loadGarage }
}

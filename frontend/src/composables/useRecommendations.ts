// Состояние экрана регламента: статус idle/loading/success/error + список алертов.
// Защита от гонки — устаревший ответ отбрасывается через AbortController.

import { ref, shallowRef } from 'vue'
import { getAlerts, type MockScenario } from '@/api/client'
import type { Alert, Vehicle } from '@/types/api'

export type LoadStatus = 'idle' | 'loading' | 'success' | 'error'

export function useRecommendations() {
  const status = ref<LoadStatus>('idle')
  const alerts = shallowRef<Alert[]>([])
  const error = ref<string | null>(null)
  let controller: AbortController | null = null

  async function load(vehicle: Vehicle, scenario?: MockScenario): Promise<void> {
    controller?.abort() // отбрасываем предыдущий запрос
    controller = new AbortController()
    const { signal } = controller
    status.value = 'loading'
    error.value = null

    try {
      const res = await getAlerts(vehicle, { signal, scenario })
      if (signal.aborted) return
      alerts.value = res
      status.value = 'success'
    } catch (e) {
      if (signal.aborted || (e instanceof DOMException && e.name === 'AbortError')) return
      error.value = e instanceof Error ? e.message : 'Не удалось получить рекомендации'
      status.value = 'error'
    }
  }

  return { status, alerts, error, load }
}

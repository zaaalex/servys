// Состояние экрана регламента: статус idle/loading/success/error + список алертов.
// Защита от гонки — устаревший ответ отбрасывается через AbortController.

import { ref, shallowRef } from 'vue'
import { getAlerts, type MockScenario } from '@/api/client'
import type { Alert, Vehicle } from '@/types/api'

export type LoadStatus = 'idle' | 'loading' | 'success' | 'error'

export interface LoadOptions {
  scenario?: MockScenario
  /** true — тихая перезагрузка: не показываем скелет (status не → 'loading'), просто заменяем alerts. */
  silent?: boolean
}

export function useRecommendations() {
  const status = ref<LoadStatus>('idle')
  const alerts = shallowRef<Alert[]>([])
  const error = ref<string | null>(null)
  let controller: AbortController | null = null

  async function load(vehicle: Vehicle, opts: LoadOptions = {}): Promise<void> {
    controller?.abort() // отбрасываем предыдущий запрос
    controller = new AbortController()
    const { signal } = controller
    if (!opts.silent) status.value = 'loading'
    error.value = null

    try {
      const res = await getAlerts(vehicle, { signal, scenario: opts.scenario })
      if (signal.aborted) return
      alerts.value = res
      status.value = 'success'
    } catch (e) {
      if (signal.aborted || (e instanceof DOMException && e.name === 'AbortError')) return
      // при тихой перезагрузке не рушим успешный экран из-за фоновой ошибки
      if (opts.silent) return
      error.value = e instanceof Error ? e.message : 'Не удалось получить рекомендации'
      status.value = 'error'
    }
  }

  /**
   * Оптимистично отмечаем работу(ы) выполненной: соответствующие ruleCode алерты → «в норме»,
   * срок отодвигаем (как это делает мок/бэк). status не трогаем — карточка мгновенно меняет вид.
   */
  function markDone(ruleCode: string): void {
    alerts.value = alerts.value.map((a) =>
      a.ruleCode === ruleCode
        ? { ...a, status: 'OK' as Alert['status'], dueAtKm: a.dueAtKm > 0 ? a.dueAtKm + 12000 : a.dueAtKm }
        : a,
    )
  }

  return { status, alerts, error, load, markDone }
}

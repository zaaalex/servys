// Состояние экрана рекомендаций: статус idle/loading/success/error + результат.
// Вся асинхронщина и защита от гонки (устаревший ответ отбрасывается через AbortController) — здесь.

import { ref, shallowRef } from 'vue'
import { postRecommendations, type MockScenario } from '@/api/client'
import type { RecommendationsRequest, RecommendationsResponse } from '@/types/api'

export type LoadStatus = 'idle' | 'loading' | 'success' | 'error'

export function useRecommendations() {
  const status = ref<LoadStatus>('idle')
  const response = shallowRef<RecommendationsResponse | null>(null)
  const error = ref<string | null>(null)
  let controller: AbortController | null = null

  async function load(req: RecommendationsRequest, scenario?: MockScenario): Promise<void> {
    controller?.abort() // отбрасываем предыдущий запрос — в UI попадёт только последний
    controller = new AbortController()
    const { signal } = controller
    status.value = 'loading'
    error.value = null

    try {
      const res = await postRecommendations(req, { signal, scenario })
      if (signal.aborted) return
      response.value = res
      status.value = 'success'
    } catch (e) {
      if (signal.aborted || (e instanceof DOMException && e.name === 'AbortError')) return
      error.value = e instanceof Error ? e.message : 'Не удалось получить рекомендации'
      status.value = 'error'
    }
  }

  return { status, response, error, load }
}

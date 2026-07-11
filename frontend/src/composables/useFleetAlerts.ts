// Срочность по всему гаражу: для каждой машины грузим алерты и отмечаем,
// есть ли среди них критичные (tone 'crit' = OVERDUE/DUE). Нужно для красных
// точек в списке «Мой гараж» и рядом с «Что пора обслужить».
// Пересчёт — при изменении состава гаража или пробега любой машины.

import { computed, reactive, watch } from 'vue'
import { getAlerts } from '@/api/client'
import { useGarage } from '@/composables/useGarage'
import { statusMeta } from '@/ui/status'
import type { Alert } from '@/types/api'

// vehicleId -> есть ли критичные работы
const urgentMap = reactive<Record<string, boolean>>({})

function isUrgent(alert: Alert): boolean {
  return statusMeta(alert.status).tone === 'crit'
}

let started = false

export function useFleetAlerts() {
  const { vehicles } = useGarage()

  if (!started) {
    started = true
    watch(
      // подпись «id:пробег» — меняется и при добавлении/удалении, и при обновлении пробега
      () => vehicles.value.map((v) => `${v.id}:${v.currentOdometer}`).join('|'),
      async () => {
        const list = vehicles.value
        const alive = new Set(list.map((v) => v.id))
        Object.keys(urgentMap).forEach((id) => {
          if (!alive.has(id)) delete urgentMap[id]
        })
        await Promise.all(
          list.map(async (v) => {
            try {
              const alerts = await getAlerts(v)
              urgentMap[v.id] = alerts.some(isUrgent)
            } catch {
              /* не удалось получить — точку не рисуем */
            }
          }),
        )
      },
      { immediate: true },
    )
  }

  const urgentIds = computed(() => Object.keys(urgentMap).filter((id) => urgentMap[id]))
  const hasUrgent = (id: string): boolean => !!urgentMap[id]

  return { urgentIds, hasUrgent }
}

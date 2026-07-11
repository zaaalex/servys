// Единый маппинг severity/status → лейбл, тон, порядок сортировки. С обязательным fallback:
// неизвестное значение (недетерминизм LLM / дрейф бэка) → нейтральный тон, а не undefined/краш.

import type { Item, Severity, Status } from '@/types/api'

export type Tone = 'crit' | 'warn' | 'calm'

interface StatusMeta {
  label: string
  tone: Tone
  cls: string
  rank: number
}

const STATUS: Record<Status, StatusMeta> = {
  overdue: { label: 'Просрочено', tone: 'crit', cls: 'is-crit', rank: 0 },
  due_soon: { label: 'Скоро', tone: 'warn', cls: 'is-warn', rank: 1 },
  upcoming: { label: 'Впереди', tone: 'calm', cls: 'is-calm', rank: 2 },
}
const STATUS_FALLBACK: StatusMeta = { label: 'Прочее', tone: 'calm', cls: 'is-calm', rank: 99 }

const SEVERITY_TONE: Record<Severity, Tone> = {
  high: 'crit',
  medium: 'warn',
  low: 'calm',
}

export function statusMeta(status: string): StatusMeta {
  return STATUS[status as Status] ?? STATUS_FALLBACK
}

export function severityTone(severity: string): Tone {
  return SEVERITY_TONE[severity as Severity] ?? 'calm'
}

/** Сортировка: overdue → due_soon → upcoming, неизвестные — в конец. */
export function byUrgency(a: Item, b: Item): number {
  return statusMeta(a.status).rank - statusMeta(b.status).rank
}

/** Текст срока относительно текущего пробега. */
export function whenText(item: Item, currentKm: number): string {
  const d = item.due_at_km - currentKm
  const km = new Intl.NumberFormat('ru-RU').format(Math.abs(d))
  return d < 0 ? `просрочено на ${km} км` : `через ${km} км`
}

import type { Alert, AlertStatus, Severity } from '@/types/api'

export type Tone = 'crit' | 'warn' | 'calm'

interface StatusMeta {
  label: string
  tone: Tone
  cls: string
  rank: number
}

const STATUS: Record<AlertStatus, StatusMeta> = {
  OVERDUE: { label: 'Просрочено', tone: 'crit', cls: 'is-crit', rank: 0 },
  DUE: { label: 'Пора', tone: 'crit', cls: 'is-crit', rank: 1 },
  SOON: { label: 'Скоро', tone: 'warn', cls: 'is-warn', rank: 2 },
  OK: { label: 'В норме', tone: 'calm', cls: 'is-calm', rank: 3 },
}
const STATUS_FALLBACK: StatusMeta = { label: '—', tone: 'calm', cls: 'is-calm', rank: 99 }

const SEVERITY_TONE: Record<Severity, Tone> = { high: 'crit', medium: 'warn', low: 'calm' }

export function statusMeta(status: string): StatusMeta {
  return STATUS[status as AlertStatus] ?? STATUS_FALLBACK
}

export function severityTone(severity: string): Tone {
  return SEVERITY_TONE[severity as Severity] ?? 'calm'
}

export function byUrgency(a: Alert, b: Alert): number {
  return statusMeta(a.status).rank - statusMeta(b.status).rank
}

export function whenText(alert: Alert, currentKm: number): string {
  if (!alert.dueAtKm || alert.dueAtKm <= 0) return 'по состоянию'
  const d = alert.dueAtKm - currentKm
  const km = new Intl.NumberFormat('ru-RU').format(Math.abs(d))
  return d < 0 ? `просрочено на ${km} км` : `через ${km} км`
}

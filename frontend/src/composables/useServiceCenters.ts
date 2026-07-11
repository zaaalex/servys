// Состояние «Кабинета СТО» (b2b): список СТО, подключение, скан одного/всех.
// Вся асинхронщина, защита от двойного сабмита и разбор ошибок — здесь; компоненты только рисуют.

import { reactive, ref } from 'vue'
import {
  connectServiceCenter,
  listServiceCenters,
  scanAll,
  scanServiceCenter,
  type B2BScenario,
} from '@/api/b2b'
import { describeError, type UiError } from '@/api/errors'
import type { ConnectServiceCenterRequest, ScanReport, ScanSummary, ServiceCenter } from '@/types/b2b'

export type LoadStatus = 'idle' | 'loading' | 'success' | 'error'

/** Состояние скана одного СТО (по id). */
export interface ScanState {
  scanning: boolean
  report: ScanReport | null
  error: UiError | null
}

export function useServiceCenters() {
  const listStatus = ref<LoadStatus>('idle')
  const centers = ref<ServiceCenter[]>([])
  const listError = ref<UiError | null>(null)

  const connecting = ref(false)
  const connectError = ref<UiError | null>(null)

  const scans = reactive<Record<string, ScanState>>({})

  const scanningAll = ref(false)
  const summary = ref<ScanSummary | null>(null)
  const scanAllError = ref<UiError | null>(null)

  function scanStateFor(id: string): ScanState {
    if (!scans[id]) scans[id] = { scanning: false, report: null, error: null }
    return scans[id]
  }

  async function loadList(scenario?: B2BScenario): Promise<void> {
    listStatus.value = 'loading'
    listError.value = null
    try {
      const res = await listServiceCenters({ scenario })
      centers.value = res.service_centers
      listStatus.value = 'success'
    } catch (e) {
      listError.value = describeError(e)
      listStatus.value = 'error'
    }
  }

  /** Подключить СТО. Возвращает созданное СТО или null при ошибке. Защита от двойного сабмита. */
  async function connect(
    req: ConnectServiceCenterRequest,
    scenario?: B2BScenario,
  ): Promise<ServiceCenter | null> {
    if (connecting.value) return null
    connecting.value = true
    connectError.value = null
    try {
      const sc = await connectServiceCenter(req, { scenario })
      centers.value = [...centers.value, sc] // добавляем без повторного запроса списка
      if (listStatus.value !== 'success') listStatus.value = 'success'
      return sc
    } catch (e) {
      connectError.value = describeError(e)
      return null
    } finally {
      connecting.value = false
    }
  }

  async function scan(id: string, scenario?: B2BScenario): Promise<void> {
    const st = scanStateFor(id)
    if (st.scanning) return
    st.scanning = true
    st.error = null
    try {
      st.report = await scanServiceCenter(id, { scenario })
    } catch (e) {
      st.error = describeError(e)
    } finally {
      st.scanning = false
    }
  }

  async function runScanAll(adminToken: string, scenario?: B2BScenario): Promise<void> {
    if (scanningAll.value) return
    scanningAll.value = true
    scanAllError.value = null
    try {
      summary.value = await scanAll({ scenario, adminToken })
    } catch (e) {
      scanAllError.value = describeError(e)
    } finally {
      scanningAll.value = false
    }
  }

  return {
    listStatus,
    centers,
    listError,
    connecting,
    connectError,
    scans,
    scanningAll,
    summary,
    scanAllError,
    scanStateFor,
    loadList,
    connect,
    scan,
    runScanAll,
  }
}

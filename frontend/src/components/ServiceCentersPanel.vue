<script setup lang="ts">
// «Кабинет СТО» (b2b): подключение СТО, список, скан одного, операторский массовый скан.
// Состояния loading/empty/error с повтором. Dev-переключатель мок-сценариев + проверка 401→refresh.
import { computed, onMounted, ref, watch } from 'vue'
import { USE_MOCK } from '@/api/client'
import { type B2BScenario } from '@/api/b2b'
import { expireMockAccess } from '@/api/auth'
import { useServiceCenters } from '@/composables/useServiceCenters'
import ConnectServiceCenterForm from '@/components/ConnectServiceCenterForm.vue'
import ServiceCenterCard from '@/components/ServiceCenterCard.vue'
import ScanReportTiles, { type Tile } from '@/components/ScanReportTiles.vue'
import type { ConnectServiceCenterRequest } from '@/types/b2b'

const emit = defineEmits<{ connected: [] }>()

const {
  listStatus,
  centers,
  listError,
  connecting,
  connectError,
  scans,
  scanningAll,
  summary,
  scanAllError,
  loadList,
  connect,
  scan,
  runScanAll,
} = useServiceCenters()

const scenario = ref<B2BScenario>('success')
const scenarios: B2BScenario[] = ['success', 'empty', 'error', 'slow', 'disabled']
const useMock = USE_MOCK

// key для сброса формы после успешного подключения
const formKey = ref(0)

// режим оператора (массовый скан по X-Admin-Token)
const operatorOpen = ref(false)
const adminToken = ref('')
const adminError = ref('')

const summaryTiles = computed<Tile[]>(() => {
  const s = summary.value
  if (!s) return []
  return [
    { label: 'СТО', value: s.centers, tone: 'calm' },
    { label: 'подошло ТО', value: s.due_items, tone: 'warn' },
    { label: 'дел создано', value: s.pushed, tone: 'accent' },
    { label: 'пропущено', value: s.skipped, tone: 'calm' },
  ]
})

async function onConnect(req: ConnectServiceCenterRequest): Promise<void> {
  const sc = await connect(req, scenario.value)
  if (sc) {
    formKey.value++ // сброс полей формы
    emit('connected') // родитель перечитает /auth/me — мог появиться b2b-контекст
  }
}

function onScan(id: string): void {
  void scan(id, scenario.value)
}

function onScanAll(): void {
  adminError.value = adminToken.value.trim() ? '' : 'Нужен операторский токен (X-Admin-Token)'
  if (adminError.value) return
  void runScanAll(adminToken.value.trim(), scenario.value)
}

function reloadList(): void {
  void loadList(scenario.value)
}

function setScenario(s: B2BScenario): void {
  scenario.value = s
}

// dev: пометить access протухшим и дёрнуть список → увидим 401→refresh→повтор (mock)
function onExpireToken(): void {
  expireMockAccess()
  reloadList()
}

onMounted(reloadList)
// смена мок-сценария → перезагрузить список и сбросить сводку
watch(scenario, () => {
  summary.value = null
  scanAllError.value = null
  reloadList()
})
</script>

<template>
  <div class="b2b-view">
    <div class="b2b-wrap">
      <header class="b2b-head">
        <div>
          <span class="eyebrow">servys · b2b</span>
          <h1 class="b2b-title">Кабинет СТО</h1>
          <p class="b2b-sub">Подключайте автосервисы и запускайте скан автопарка — servys создаёт ретеншн-дела в CRM.</p>
        </div>
        <button class="btn-ghost b2b-op-toggle" type="button" :aria-pressed="operatorOpen" @click="operatorOpen = !operatorOpen">
          {{ operatorOpen ? 'Скрыть режим оператора' : 'Режим оператора' }}
        </button>
      </header>

      <!-- режим оператора: массовый скан по X-Admin-Token -->
      <section v-if="operatorOpen" class="b2b-card b2b-operator">
        <h2 class="b2b-card-title">Массовый скан всех СТО</h2>
        <p class="b2b-card-sub">Операторская операция: прогоняет все подключённые СТО. Требует токен оператора (заголовок X-Admin-Token).</p>
        <div class="field b2b-admin-field">
          <label for="adminTok">X-Admin-Token</label>
          <div class="box"><input id="adminTok" v-model="adminToken" type="password" autocomplete="off" placeholder="операторский токен" /></div>
          <div v-if="adminError" class="err">{{ adminError }}</div>
        </div>
        <button class="go" type="button" :disabled="scanningAll" @click="onScanAll">
          <span v-if="scanningAll" class="spin" aria-hidden="true"></span>
          {{ scanningAll ? 'Сканирую все…' : 'Сканировать все' }}
        </button>
      </section>

      <!-- сводка массового скана -->
      <section v-if="scanAllError" class="b2b-banner-err" role="alert">
        <strong>Массовый скан не удался.</strong> {{ scanAllError.message }}
        <span v-if="scanAllError.code" class="b2b-code">{{ scanAllError.code }}</span>
      </section>
      <section v-else-if="summary" class="b2b-card b2b-summary">
        <h2 class="b2b-card-title">Итог массового скана</h2>
        <ScanReportTiles :tiles="summaryTiles" :errors="summary.errors" />
      </section>

      <!-- подключение -->
      <ConnectServiceCenterForm :key="formKey" :connecting="connecting" :error="connectError" @submit="onConnect" />

      <!-- список -->
      <section class="b2b-list-sec">
        <div class="sec-head"><span class="n">→</span><h2>Подключённые СТО</h2></div>

        <!-- loading -->
        <div v-if="listStatus === 'loading'" class="skel">
          <div v-for="i in 3" :key="i" class="skc"></div>
        </div>

        <!-- error -->
        <div v-else-if="listStatus === 'error'" class="state error">
          <div class="big" aria-hidden="true">⚡</div>
          <h3>Не удалось загрузить список</h3>
          <p>{{ listError?.message || 'Сервис не ответил.' }}</p>
          <button class="retry" type="button" @click="reloadList">Повторить</button>
        </div>

        <!-- empty -->
        <div v-else-if="centers.length === 0" class="state">
          <div class="big" aria-hidden="true">✦</div>
          <h3>Пока нет ни одного СТО</h3>
          <p>Подключите первый автосервис через форму выше — он появится здесь.</p>
        </div>

        <!-- success -->
        <div v-else class="b2b-cards">
          <ServiceCenterCard
            v-for="c in centers"
            :key="c.id"
            :center="c"
            :scan="scans[c.id]"
            @scan="onScan"
          />
        </div>
      </section>
    </div>

    <div v-if="useMock" class="dev" role="group" aria-label="Dev-инструменты b2b">
      <span class="k">b2b_mock_scenario</span>
      <div class="seg">
        <button
          v-for="s in scenarios"
          :key="s"
          type="button"
          :aria-pressed="scenario === s"
          @click="setScenario(s)"
        >
          {{ s }}
        </button>
      </div>
      <button class="b2b-link" type="button" @click="onExpireToken">истёк токен → refresh</button>
    </div>
  </div>
</template>

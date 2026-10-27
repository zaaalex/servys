<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { addServiceEvent } from '@/api/client'
import DriveGame from '@/components/DriveGame.vue'
import { useRecommendations } from '@/composables/useRecommendations'
import { useBookStamp } from '@/composables/useBookStamp'
import { useGarage } from '@/composables/useGarage'
import type { Alert, Vehicle } from '@/types/api'
import { byUrgency, statusMeta, whenText } from '@/ui/status'

const props = defineProps<{ car: Vehicle | null }>()

const fmt = new Intl.NumberFormat('ru-RU')
const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches

const ICONS = {
  regular:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a4 4 0 0 0-5.4 5.4l-6 6a1.5 1.5 0 0 0 2.1 2.1l6-6a4 4 0 0 0 5.4-5.4l-2.6 2.6-2.1-2.1 2.6-2.6Z"/></svg>',
  issue:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3 2 20h20L12 3Z"/><path d="M12 10v4"/><path d="M12 17.5h.01"/></svg>',
}
function iconFor(a: Alert): string {
  return a.severity === 'high' ? ICONS.issue : ICONS.regular
}

const { status, alerts, error, load } = useRecommendations()
const { setOdometer } = useGarage()

const odo = ref(0)
const barsReady = ref(false)

const editingOdo = ref(false)
const odoDraft = ref(0)
const odoError = ref('')
const odoInput = ref<HTMLInputElement | null>(null)
const odoMode = ref<'manual' | 'drive'>('manual')
const driveKm = ref(0)
function onDrive(km: number): void {
  driveKm.value = km
}

/* ---- отметка «выполнено» ---- */
const doneAlert = ref<Alert | null>(null)
const doneDate = ref('')
const doneOdo = ref(0)
const doneNote = ref('')
const doneError = ref('')
const today = new Date().toISOString().slice(0, 10)

const viewState = computed<'loading' | 'success' | 'empty' | 'error'>(() => {
  if (status.value === 'loading') return 'loading'
  if (status.value === 'error') return 'error'
  if (status.value === 'success' && alerts.value.length === 0) return 'empty'
  if (status.value === 'success') return 'success'
  return 'loading'
})

const carLabel = computed(() => {
  const c = props.car
  return c ? `${c.make} ${c.model} · ${c.year} · ${fmt.format(c.currentOdometer)} км` : ''
})

const basisText =
  'Сроки из раздела документации, который совпал с годом, рынком и мощностью. Форумы перечитываются в фоне и эти строки не заменяют.'

const elapsedSec = ref(0)
let elapsedTimer = 0

function stopElapsed(): void {
  window.clearInterval(elapsedTimer)
  elapsedTimer = 0
}

const elapsedLabel = computed(() => {
  const s = elapsedSec.value
  if (s < 5) return 'только началось'
  const m = Math.floor(s / 60)
  const r = s % 60
  if (m === 0) return `уже ${r} с`
  return `уже ${m} мин ${r.toString().padStart(2, '0')} с`
})

const subtitle = computed(() => {
  if (!props.car) return 'Добавьте машину по VIN — агент ищет документацию по её обслуживанию.'
  if (viewState.value === 'loading') return 'Агент ещё читает документацию по этой машине. Списка работ пока нет.'
  if (viewState.value === 'success') return basisText
  if (viewState.value === 'empty') return 'Документация по этой комплектации не нашлась.'
  if (viewState.value === 'error') return 'Анализ остановился. Это ошибка, список работ из неё не собирается.'
  return ''
})

function showCommunity(a: Alert): boolean {
  return !!a.community && a.community.source !== 'demo' && (a.community.note !== '' || a.community.realIntervalKm > 0)
}
function communityLabel(a: Alert): string {
  const src = (a.community?.source ?? '').toLowerCase()
  if (src.includes('reglament') || src.includes('regulations-to')) return 'Из регламента'
  return 'По отзывам'
}
const primary = computed(() => alerts.value.filter((a) => a.category === 'primary').sort(byUrgency))
const secondary = computed(() => alerts.value.filter((a) => a.category === 'secondary').sort(byUrgency))
const secondaryOpen = ref(false)

const counts = computed(() => {
  const c = { crit: 0, warn: 0, calm: 0 }
  for (const a of alerts.value) c[statusMeta(a.status).tone]++
  return c
})
const total = computed(() => alerts.value.length || 1)

function barStyle(n: number): Record<string, string> {
  return { width: barsReady.value ? `${(n / total.value) * 100}%` : '0%' }
}

/** Прогресс к сроку: пробег относительно dueAtKm (без интервала — полный нейтральный). */
function meterStyle(a: Alert): Record<string, string> {
  let pct = 100
  if (a.dueAtKm > 0 && props.car) {
    pct = Math.max(4, Math.min(100, (props.car.currentOdometer / a.dueAtKm) * 100))
  }
  return { width: barsReady.value ? `${pct}%` : '0%' }
}

let odoRaf = 0
function animateOdo(to: number): void {
  cancelAnimationFrame(odoRaf)
  if (reduce) {
    odo.value = to
    return
  }
  let t0: number | null = null
  const step = (ts: number): void => {
    if (t0 === null) t0 = ts
    const p = Math.min(1, (ts - t0) / 900)
    odo.value = Math.round(to * (1 - Math.pow(1 - p, 3)))
    if (p < 1) odoRaf = requestAnimationFrame(step)
  }
  odoRaf = requestAnimationFrame(step)
}

/* ---- reveal по скроллу ---- */
const section = ref<HTMLElement | null>(null)
const entered = ref(false)
let isVisible = false
let revealedForLoad = false
let observer: IntersectionObserver | null = null

function reveal(): void {
  revealedForLoad = true
  entered.value = true
  animateOdo(props.car?.currentOdometer ?? 0)
  barsReady.value = false
  requestAnimationFrame(() => {
    requestAnimationFrame(() => (barsReady.value = true))
  })
}
function maybeReveal(): void {
  if (isVisible && viewState.value === 'success' && !revealedForLoad) reveal()
}

function refresh(): void {
  const c = props.car
  if (!c) return
  editingOdo.value = false
  barsReady.value = false
  entered.value = false
  revealedForLoad = false
  void load(c)
}

watch(viewState, maybeReveal)

function startOdo(): void {
  odoDraft.value = props.car?.currentOdometer ?? 0
  odoError.value = ''
  odoMode.value = 'manual'
  driveKm.value = 0
  editingOdo.value = true
  void nextTick(() => odoInput.value?.focus())
}
async function saveOdo(): Promise<void> {
  const c = props.car
  if (!c) return
  if (odoMode.value === 'drive' && driveKm.value <= 0) {
    odoError.value = 'Проедьте хотя бы немного'
    return
  }
  const target = odoMode.value === 'drive' ? c.currentOdometer + driveKm.value : odoDraft.value
  if (!Number.isFinite(target) || target < c.currentOdometer) {
    odoError.value = `Не меньше ${fmt.format(c.currentOdometer)} км`
    return
  }
  odoError.value = ''
  editingOdo.value = false
  await setOdometer(c.id, target) // reload алертов случится по watch на currentOdometer
}

function openDone(a: Alert): void {
  doneAlert.value = a
  doneDate.value = today
  doneOdo.value = props.car?.currentOdometer ?? 0
  doneNote.value = ''
  doneError.value = ''
}
async function saveDone(): Promise<void> {
  const c = props.car
  const a = doneAlert.value
  if (!c || !a) return
  if (!Number.isFinite(doneOdo.value) || doneOdo.value < 0) {
    doneError.value = 'Укажите корректный пробег'
    return
  }
  const odometer = Math.round(doneOdo.value)
  doneError.value = ''
  doneAlert.value = null
  await addServiceEvent(c.id, {
    ruleCode: a.ruleCode,
    operation: 'replace',
    date: doneDate.value,
    odometer,
    note: doneNote.value.trim() || undefined,
  })
  void load(c)
}

watch(() => [props.car?.id, props.car?.currentOdometer], refresh, { immediate: true })

const bookStamp = useBookStamp()
watch(bookStamp, () => {
  const c = props.car
  if (!c) return
  void load(c)
})

watch(
  () => Boolean(props.car) && viewState.value === 'loading',
  (on) => {
    stopElapsed()
    elapsedSec.value = 0
    if (!on) return
    const started = Date.now()
    elapsedTimer = window.setInterval(() => {
      elapsedSec.value = Math.floor((Date.now() - started) / 1000)
    }, 1000)
  },
  { immediate: true },
)

function onKeydown(e: KeyboardEvent): void {
  if (e.key !== 'Escape') return
  if (editingOdo.value) editingOdo.value = false
  if (doneAlert.value) doneAlert.value = null
}

onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => {
      isVisible = entries[0]?.isIntersecting ?? false
      if (isVisible) maybeReveal()
    },
    { threshold: 0 },
  )
  if (section.value) observer.observe(section.value)
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(odoRaf)
  stopElapsed()
  observer?.disconnect()
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <header class="rp-top">
    <div class="rp-right">
      <span class="rp-car">{{ carLabel }}</span>
      <button v-if="car && (viewState === 'success' || viewState === 'empty')" class="edit-btn" type="button" @click="startOdo">
        Обновить пробег
      </button>
    </div>
  </header>

  <main class="results-wrap">
    <span class="eyebrow">регламент</span>
    <div class="sec-head"><h2>Что пора обслужить</h2></div>
    <p class="sec-sub">{{ subtitle }}</p>
    <p v-if="car && viewState === 'loading'" class="analyze-line">
      <span class="analyze-kicker">идёт анализ</span>
      <span class="analyze-time">{{ elapsedLabel }}</span>
    </p>
    <section class="results" ref="section">
      <div v-if="!car" class="state">
        <h3>Нет активной машины</h3>
        <p>Добавьте авто по VIN. Агент читает документацию по обслуживанию этой машины.</p>
      </div>

      <div v-else-if="viewState === 'loading'" class="analyze-wait">
        Ищет техническую документацию {{ car.make }} {{ car.model }} {{ car.year }}: что менять и на каком пробеге.
      </div>

      <div v-else-if="viewState === 'error'" class="state error">
        <h3>Анализ остановился</h3>
        <p>{{ error || 'Поиск документации не ответил. Повторите запрос.' }}</p>
        <button class="retry" type="button" @click="refresh">Повторить</button>
      </div>

      <div v-else-if="viewState === 'empty'" class="state">
        <h3>Агент закончил анализ</h3>
        <p v-if="car">Для {{ car.make }} {{ car.model }} на пробеге {{ fmt.format(car.currentOdometer) }} км раздел обслуживания под эту комплектацию не нашёлся.</p>
      </div>

      <div v-else-if="viewState === 'success' && car" class="rec-content" :class="{ entered }">
        <div class="summary">
          <div class="sum-top">
            <span class="who">{{ car.make }} {{ car.model }}</span>
            <span class="odo">
              <span class="num">{{ fmt.format(odo) }}</span>
              <span class="unit">км · {{ car.year }}</span>
            </span>
          </div>
          <div class="dist">
            <span class="s crit" :style="barStyle(counts.crit)"></span>
            <span class="s warn" :style="barStyle(counts.warn)"></span>
            <span class="s calm" :style="barStyle(counts.calm)"></span>
          </div>
          <div class="dist-legend">
            <span class="lg crit"><span class="d"></span>Срочно <b>{{ counts.crit }}</b></span>
            <span class="lg warn"><span class="d"></span>Скоро <b>{{ counts.warn }}</b></span>
            <span class="lg calm"><span class="d"></span>В норме <b>{{ counts.calm }}</b></span>
          </div>
        </div>

        <!-- ===== Основные (primary) — вертикальный список, самое срочное сверху ===== -->
        <section v-if="primary.length" class="al-sec">
          <div class="al-sec-head">
            <h3 class="al-sec-title">Основные <span class="al-sec-count">{{ primary.length }}</span></h3>
          </div>

          <div class="al-list">
            <article
              v-for="a in primary"
              :key="a.id"
              class="al-card"
              :class="statusMeta(a.status).cls"
            >
              <div class="al-top">
                <div class="al-ic" v-html="iconFor(a)"></div>
                <span class="al-chip"><span class="d"></span>{{ statusMeta(a.status).label }}</span>
              </div>
              <div class="al-body">
                <h3 class="al-title">{{ a.title }}</h3>
                <p class="al-desc">{{ a.description }}</p>
                <div v-if="showCommunity(a)" class="al-comm">
                  <span class="al-comm-badge"><span class="d"></span>{{ communityLabel(a) }}</span>
                  <p v-if="a.community && a.community.realIntervalKm > 0" class="al-comm-lead">Меняют на <b>{{ fmt.format(a.community.realIntervalKm) }} км</b></p>
                  <p v-if="a.community" class="al-comm-note">{{ a.community.note }}</p>
                  <p v-if="a.community && a.community.reports > 0" class="al-comm-meta">Данные владельцев, не официальный регламент · {{ a.community.reports }} отзывов</p>
                </div>
              </div>
              <div class="al-foot">
                <div class="al-when">
                  <span class="al-when-val">{{ whenText(a, car.currentOdometer) }}</span>
                  <span v-if="a.dueAtKm > 0" class="al-due">срок {{ fmt.format(a.dueAtKm) }} км</span>
                </div>
                <div class="al-meter"><span :style="meterStyle(a)"></span></div>
                <button v-if="a.status !== 'OK'" class="al-done" type="button" @click="openDone(a)">
                  Отметить выполненным
                </button>
              </div>
            </article>
          </div>
        </section>

        <!-- ===== Дополнительные (secondary) — свёрнуты, чтобы первый экран оставался коротким ===== -->
        <section v-if="secondary.length" class="al-sec al-sec-secondary">
          <div class="al-sec-head">
            <button class="al-sec-toggle" type="button" :aria-expanded="secondaryOpen" @click="secondaryOpen = !secondaryOpen">
              <span class="al-sec-title">Дополнительные <span class="al-sec-count">{{ secondary.length }}</span></span>
              <span class="al-sec-caret" :class="{ open: secondaryOpen }" aria-hidden="true">▾</span>
            </button>
          </div>

          <div v-show="secondaryOpen" class="al-list">
            <article
              v-for="a in secondary"
              :key="a.id"
              class="al-card"
              :class="statusMeta(a.status).cls"
            >
              <div class="al-top">
                <div class="al-ic" v-html="iconFor(a)"></div>
                <span class="al-chip"><span class="d"></span>{{ statusMeta(a.status).label }}</span>
              </div>
              <div class="al-body">
                <h3 class="al-title">{{ a.title }}</h3>
                <p class="al-desc">{{ a.description }}</p>
                <div v-if="showCommunity(a)" class="al-comm">
                  <span class="al-comm-badge"><span class="d"></span>{{ communityLabel(a) }}</span>
                  <p v-if="a.community && a.community.realIntervalKm > 0" class="al-comm-lead">Меняют на <b>{{ fmt.format(a.community.realIntervalKm) }} км</b></p>
                  <p v-if="a.community" class="al-comm-note">{{ a.community.note }}</p>
                  <p v-if="a.community && a.community.reports > 0" class="al-comm-meta">Данные владельцев, не официальный регламент · {{ a.community.reports }} отзывов</p>
                </div>
              </div>
              <div class="al-foot">
                <div class="al-when">
                  <span class="al-when-val">{{ whenText(a, car.currentOdometer) }}</span>
                  <span v-if="a.dueAtKm > 0" class="al-due">срок {{ fmt.format(a.dueAtKm) }} км</span>
                </div>
                <div class="al-meter"><span :style="meterStyle(a)"></span></div>
                <button v-if="a.status !== 'OK'" class="al-done" type="button" @click="openDone(a)">
                  Отметить выполненным
                </button>
              </div>
            </article>
          </div>
        </section>
      </div>
    </section>
  </main>

  <div v-if="editingOdo && car" class="modal">
    <div class="modal-backdrop" @click="editingOdo = false"></div>
    <div class="modal-card modal-card-sm" role="dialog" aria-modal="true" aria-label="Обновить пробег">
      <div class="modal-head">
        <h3>Обновить пробег</h3>
        <button class="modal-x" type="button" aria-label="Закрыть" @click="editingOdo = false">✕</button>
      </div>
      <div class="mode-seg">
        <button type="button" :aria-pressed="odoMode === 'manual'" @click="odoMode = 'manual'">Вручную</button>
        <button type="button" :aria-pressed="odoMode === 'drive'" @click="odoMode = 'drive'">Проехать</button>
      </div>
      <form novalidate @submit.prevent="saveOdo">
        <div v-if="odoMode === 'manual'" class="odo-field">
          <label for="odoEdit">Текущий пробег</label>
          <div class="odo-box">
            <input id="odoEdit" ref="odoInput" v-model.number="odoDraft" type="number" :min="car.currentOdometer" inputmode="numeric" placeholder="0" />
            <span class="odo-unit">км</span>
          </div>
        </div>
        <template v-else>
          <DriveGame @distance="onDrive" />
          <p class="drive-info">Проехали <b>+{{ fmt.format(driveKm) }} км</b> → станет <b>{{ fmt.format(car.currentOdometer + driveKm) }} км</b></p>
        </template>
        <div class="vin-result" :class="{ bad: odoError }">
          {{ odoError || `Нельзя уменьшать — сейчас ${fmt.format(car.currentOdometer)} км` }}
        </div>
        <div class="modal-actions">
          <button type="button" class="btn-ghost" @click="editingOdo = false">Отмена</button>
          <button type="submit" class="go go-sm">Сохранить</button>
        </div>
      </form>
    </div>
  </div>

  <div v-if="doneAlert && car" class="modal">
    <div class="modal-backdrop" @click="doneAlert = null"></div>
    <div class="modal-card modal-card-sm" role="dialog" aria-modal="true" aria-label="Отметить выполненным">
      <div class="modal-head">
        <h3>Отметить выполненным</h3>
        <button class="modal-x" type="button" aria-label="Закрыть" @click="doneAlert = null">✕</button>
      </div>
      <p class="done-sub">{{ doneAlert.title }}</p>
      <form novalidate @submit.prevent="saveDone">
        <div class="odo-field">
          <label for="doneOdo">Пробег на момент работ</label>
          <div class="odo-box">
            <input id="doneOdo" v-model.number="doneOdo" type="number" min="0" inputmode="numeric" placeholder="0" />
            <span class="odo-unit">км</span>
          </div>
        </div>
        <div class="vin-field" style="margin-top: 14px">
          <label for="doneDate">Дата обслуживания</label>
          <div class="m-box"><input id="doneDate" v-model="doneDate" type="date" :max="today" /></div>
        </div>
        <div class="vin-field" style="margin-top: 14px">
          <label for="doneNote">Заметка (необязательно)</label>
          <div class="m-box"><input id="doneNote" v-model="doneNote" type="text" placeholder="напр. сервис, артикул" /></div>
        </div>
        <div v-if="doneError" class="vin-result bad" style="margin-top: 10px">{{ doneError }}</div>
        <div class="modal-actions">
          <button type="button" class="btn-ghost" @click="doneAlert = null">Отмена</button>
          <button type="submit" class="go go-sm">Готово</button>
        </div>
      </form>
    </div>
  </div>
</template>

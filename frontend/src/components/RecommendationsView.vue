<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { USE_MOCK, type MockScenario } from '@/api/client'
import { useRecommendations } from '@/composables/useRecommendations'
import { useGarage } from '@/composables/useGarage'
import type { Alert, Vehicle } from '@/types/api'
import { byUrgency, statusMeta, whenText } from '@/ui/status'

const props = defineProps<{ car: Vehicle | null }>()
const emit = defineEmits<{ back: [] }>()

const fmt = new Intl.NumberFormat('ru-RU')
const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches

const ICONS = {
  regular:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a4 4 0 0 0-5.4 5.4l-6 6a1.5 1.5 0 0 0 2.1 2.1l6-6a4 4 0 0 0 5.4-5.4l-2.6 2.6-2.1-2.1 2.6-2.6Z"/></svg>',
  issue:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3 2 20h20L12 3Z"/><path d="M12 10v4"/><path d="M12 17.5h.01"/></svg>',
}
function iconFor(a: Alert): string {
  return a.type === 'RISK_DIAGNOSTIC_RECOMMENDED' ? ICONS.issue : ICONS.regular
}

const { status, alerts, error, load } = useRecommendations()
const { setOdometer } = useGarage()

const scenario = ref<MockScenario>('success')
const scenarios: MockScenario[] = ['success', 'empty', 'error', 'slow']
const useMock = USE_MOCK

const odo = ref(0)
const barsReady = ref(false)

const editingOdo = ref(false)
const odoDraft = ref(0)

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
const sorted = computed(() => alerts.value.slice().sort(byUrgency))
const counts = computed(() => {
  const c = { crit: 0, warn: 0, calm: 0 }
  for (const a of alerts.value) c[statusMeta(a.status).tone]++
  return c
})
const total = computed(() => alerts.value.length || 1)

function cardDelay(i: number): Record<string, string> {
  return reduce ? {} : { '--d': `${(0.06 * i + 0.05).toFixed(2)}s` }
}
function barStyle(n: number): Record<string, string> {
  return { width: barsReady.value ? `${(n / total.value) * 100}%` : '0%' }
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

function refresh(): void {
  const c = props.car
  if (!c) return
  editingOdo.value = false
  barsReady.value = false
  void load(c, useMock ? scenario.value : undefined)
}

// одометр-счётчик + бар — когда пришёл успех
watch(viewState, (s) => {
  if (s === 'success') {
    animateOdo(props.car?.currentOdometer ?? 0)
    barsReady.value = false
    requestAnimationFrame(() => requestAnimationFrame(() => (barsReady.value = true)))
  }
})

function setScenario(s: MockScenario): void {
  scenario.value = s
}

function startOdo(): void {
  odoDraft.value = props.car?.currentOdometer ?? 0
  editingOdo.value = true
}
async function saveOdo(): Promise<void> {
  const c = props.car
  if (!c) return
  editingOdo.value = false
  if (Number.isFinite(odoDraft.value) && odoDraft.value >= c.currentOdometer) {
    await setOdometer(c.id, odoDraft.value) // reload алертов случится по watch на currentOdometer
  }
}

watch(() => [props.car?.id, props.car?.currentOdometer], refresh, { immediate: true })
watch(scenario, refresh)

onBeforeUnmount(() => cancelAnimationFrame(odoRaf))
</script>

<template>
  <header class="rp-top">
    <button class="back" type="button" @click="emit('back')"><span aria-hidden="true">↑</span> Гараж</button>
    <div class="rp-right">
      <span class="rp-car">{{ carLabel }}</span>
      <button v-if="viewState === 'success' || viewState === 'empty'" class="edit-btn" type="button" @click="startOdo">
        Обновить пробег
      </button>
    </div>
  </header>

  <main class="results-wrap">
    <div class="sec-head"><span class="n">01</span><h2>Что пора обслужить</h2></div>
    <p class="sec-sub">Регламентные работы и типовые поломки, привязанные к вашему пробегу.</p>

    <section class="results" aria-live="polite">
      <div v-if="editingOdo && car" class="edit-head">
        <label for="odoEdit">Текущий пробег, км (нельзя уменьшать)</label>
        <input id="odoEdit" v-model.number="odoDraft" type="number" :min="car.currentOdometer" />
        <div class="modal-actions" style="margin-top: 12px; justify-content: flex-start">
          <button class="go go-sm" type="button" @click="saveOdo">Сохранить</button>
          <button class="btn-ghost" type="button" @click="editingOdo = false">Отмена</button>
        </div>
      </div>

      <template v-if="viewState === 'loading'">
        <div class="skel"><div v-for="i in 4" :key="i" class="skc"></div></div>
      </template>

      <div v-else-if="viewState === 'error'" class="state error">
        <div class="big" aria-hidden="true">⚡</div>
        <h3>Не удалось получить рекомендации</h3>
        <p>{{ error || 'Сервис не ответил. Попробуйте ещё раз.' }}</p>
        <button class="retry" type="button" @click="refresh">Повторить</button>
      </div>

      <div v-else-if="viewState === 'empty' && !editingOdo" class="state">
        <div class="big" aria-hidden="true">✦</div>
        <h3>Срочных работ нет</h3>
        <p v-if="car">Для {{ car.make }} {{ car.model }} на пробеге {{ fmt.format(car.currentOdometer) }} км рекомендаций нет.</p>
      </div>

      <template v-else-if="viewState === 'success' && car">
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

        <div class="list">
          <article
            v-for="(a, i) in sorted"
            :key="a.id"
            class="card"
            :class="statusMeta(a.status).cls"
            :style="cardDelay(i)"
          >
            <div class="ic" v-html="iconFor(a)"></div>
            <div class="c-main">
              <div class="c-title">{{ a.title }}</div>
              <div class="c-note">{{ a.description }}</div>
            </div>
            <div class="c-side">
              <span class="chip"><span class="d"></span>{{ statusMeta(a.status).label }}</span>
              <span class="c-when">{{ whenText(a, car.currentOdometer) }}</span>
              <span v-if="a.dueAtKm > 0" class="c-due">срок {{ fmt.format(a.dueAtKm) }} км</span>
            </div>
          </article>
        </div>
      </template>
    </section>
  </main>

  <div v-if="useMock" class="dev" role="group" aria-label="Сценарий ответа API">
    <span class="k">mock_scenario</span>
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
  </div>
</template>

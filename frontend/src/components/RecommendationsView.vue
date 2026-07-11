<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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

const odo = ref(0)
const barsReady = ref(false)

const editingOdo = ref(false)
const odoDraft = ref(0)
const odoError = ref('')
const odoInput = ref<HTMLInputElement | null>(null)

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

function barStyle(n: number): Record<string, string> {
  return { width: barsReady.value ? `${(n / total.value) * 100}%` : '0%' }
}

/* ---- карусель алертов ---- */
const carousel = ref<HTMLElement | null>(null)
const activeIndex = ref(0)

function stepPx(): number {
  const el = carousel.value
  if (!el || el.children.length === 0) return 1
  const a = el.children[0] as HTMLElement
  const b = el.children[1] as HTMLElement | undefined
  return b ? b.offsetLeft - a.offsetLeft : a.clientWidth
}
function onScroll(): void {
  const el = carousel.value
  if (!el) return
  activeIndex.value = Math.round(el.scrollLeft / stepPx())
}
function scrollToIndex(i: number): void {
  const el = carousel.value
  if (!el) return
  const idx = Math.max(0, Math.min(sorted.value.length - 1, i))
  const child = el.children[idx] as HTMLElement | undefined
  if (!child) return
  el.scrollTo({ left: child.offsetLeft - (el.clientWidth - child.clientWidth) / 2, behavior: reduce ? 'auto' : 'smooth' })
  activeIndex.value = idx
}
function nudge(dir: number): void {
  scrollToIndex(activeIndex.value + dir)
}
const atStart = computed(() => activeIndex.value <= 0)
const atEnd = computed(() => activeIndex.value >= sorted.value.length - 1)

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
  activeIndex.value = 0
  animateOdo(props.car?.currentOdometer ?? 0)
  barsReady.value = false
  requestAnimationFrame(() => {
    if (carousel.value) carousel.value.scrollLeft = 0
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
  activeIndex.value = 0
  entered.value = false
  revealedForLoad = false
  void load(c)
}

watch(viewState, maybeReveal)

function startOdo(): void {
  odoDraft.value = props.car?.currentOdometer ?? 0
  odoError.value = ''
  editingOdo.value = true
  void nextTick(() => odoInput.value?.focus())
}
async function saveOdo(): Promise<void> {
  const c = props.car
  if (!c) return
  if (!Number.isFinite(odoDraft.value) || odoDraft.value < c.currentOdometer) {
    odoError.value = `Не меньше ${fmt.format(c.currentOdometer)} км`
    return
  }
  odoError.value = ''
  editingOdo.value = false
  await setOdometer(c.id, odoDraft.value) // reload алертов случится по watch на currentOdometer
}

watch(() => [props.car?.id, props.car?.currentOdometer], refresh, { immediate: true })

function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'Escape' && editingOdo.value) editingOdo.value = false
}

onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => {
      isVisible = entries[0]?.isIntersecting ?? false
      if (isVisible) maybeReveal()
    },
    { threshold: 0.2 },
  )
  if (section.value) observer.observe(section.value)
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(odoRaf)
  observer?.disconnect()
  document.removeEventListener('keydown', onKeydown)
})
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
    <span class="eyebrow">регламент</span>
    <div class="sec-head"><h2>Что пора обслужить</h2></div>
    <p class="sec-sub">Регламентные работы и типовые поломки, привязанные к вашему пробегу.</p>

    <section class="results" ref="section" aria-live="polite">
      <template v-if="viewState === 'loading'">
        <div class="skel"><div v-for="i in 4" :key="i" class="skc"></div></div>
      </template>

      <div v-else-if="viewState === 'error'" class="state error">
        <div class="big" aria-hidden="true">⚡</div>
        <h3>Не удалось получить рекомендации</h3>
        <p>{{ error || 'Сервис не ответил. Попробуйте ещё раз.' }}</p>
        <button class="retry" type="button" @click="refresh">Повторить</button>
      </div>

      <div v-else-if="viewState === 'empty'" class="state">
        <div class="big" aria-hidden="true">✦</div>
        <h3>Срочных работ нет</h3>
        <p v-if="car">Для {{ car.make }} {{ car.model }} на пробеге {{ fmt.format(car.currentOdometer) }} км рекомендаций нет.</p>
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

        <div class="carousel-head">
          <span class="carousel-count">{{ sorted.length ? activeIndex + 1 : 0 }} / {{ sorted.length }}</span>
          <div class="carousel-nav">
            <button class="cbtn" type="button" aria-label="Предыдущая" :disabled="atStart" @click="nudge(-1)">‹</button>
            <button class="cbtn" type="button" aria-label="Следующая" :disabled="atEnd" @click="nudge(1)">›</button>
          </div>
        </div>

        <div class="al-carousel" ref="carousel" @scroll="onScroll">
          <article
            v-for="(a, i) in sorted"
            :key="a.id"
            class="al-card"
            :class="[statusMeta(a.status).cls, { 'is-focus': i === activeIndex }]"
          >
            <div class="al-top">
              <div class="al-ic" v-html="iconFor(a)"></div>
              <span class="al-chip"><span class="d"></span>{{ statusMeta(a.status).label }}</span>
            </div>
            <div class="al-body">
              <h3 class="al-title">{{ a.title }}</h3>
              <p class="al-desc">{{ a.description }}</p>
            </div>
            <div class="al-foot">
              <div class="al-when">
                <span class="al-when-val">{{ whenText(a, car.currentOdometer) }}</span>
                <span v-if="a.dueAtKm > 0" class="al-due">срок {{ fmt.format(a.dueAtKm) }} км</span>
              </div>
              <div class="al-meter"><span :style="meterStyle(a)"></span></div>
            </div>
          </article>
        </div>

        <div class="al-dots">
          <button
            v-for="(a, i) in sorted"
            :key="a.id"
            class="al-dot"
            :class="{ on: i === activeIndex }"
            type="button"
            :aria-label="`Карточка ${i + 1}`"
            @click="scrollToIndex(i)"
          ></button>
        </div>
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
      <form novalidate @submit.prevent="saveOdo">
        <div class="vin-field">
          <label for="odoEdit">Текущий пробег, км</label>
          <div class="vin-row">
            <div class="box"><input id="odoEdit" ref="odoInput" v-model.number="odoDraft" type="number" :min="car.currentOdometer" /></div>
          </div>
          <div class="vin-result" :class="{ bad: odoError }">
            {{ odoError || `Нельзя уменьшать — сейчас ${fmt.format(car.currentOdometer)} км` }}
          </div>
        </div>
        <div class="modal-actions">
          <button type="button" class="btn-ghost" @click="editingOdo = false">Отмена</button>
          <button type="submit" class="go go-sm">Сохранить</button>
        </div>
      </form>
    </div>
  </div>
</template>

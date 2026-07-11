<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { USE_MOCK, type MockScenario } from '@/api/client'
import { useRecommendations } from '@/composables/useRecommendations'
import type { Vehicle } from '@/composables/useGarage'
import type { Category, Item, Status } from '@/types/api'
import { byUrgency, statusMeta, whenText } from '@/ui/status'

const props = defineProps<{ car: Vehicle | null }>()
const emit = defineEmits<{ back: [] }>()

const fmt = new Intl.NumberFormat('ru-RU')
const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches

const ICONS: Record<Category, string> = {
  regular:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a4 4 0 0 0-5.4 5.4l-6 6a1.5 1.5 0 0 0 2.1 2.1l6-6a4 4 0 0 0 5.4-5.4l-2.6 2.6-2.1-2.1 2.6-2.6Z"/></svg>',
  known_issue:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3 2 20h20L12 3Z"/><path d="M12 10v4"/><path d="M12 17.5h.01"/></svg>',
}

type ViewState = 'loading' | 'success' | 'empty' | 'error'
const { status, response, error, load } = useRecommendations()

const viewState = ref<ViewState>('loading')
const shownItems = ref<Item[]>([])
const odo = ref(0)
const barsReady = ref(false)

const scenario = ref<MockScenario>('success')
const scenarios: MockScenario[] = ['success', 'empty', 'error', 'slow']
const useMock = USE_MOCK

/* ---- editing ---- */
interface EditRow {
  title: string
  status: Status
  category: Category
  due_at_km: number
  note: string
}
const editing = ref(false)
const editItems = ref<EditRow[]>([])
const editMileage = ref(0)

const carLabel = computed(() => {
  const c = props.car
  return c ? `${c.make} ${c.model} · ${c.year} · ${fmt.format(c.mileage_km)} км` : ''
})
const sortedItems = computed(() => shownItems.value.slice().sort(byUrgency))
const counts = computed(() => {
  const c = { overdue: 0, due_soon: 0, upcoming: 0 }
  for (const it of shownItems.value) if (it.status in c) c[it.status as keyof typeof c]++
  return c
})
const total = computed(() => shownItems.value.length || 1)

function cloneItem(it: Item): Item {
  return { ...it }
}
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

function settle(): void {
  viewState.value = 'success'
  animateOdo(props.car?.mileage_km ?? 0)
  barsReady.value = false
  requestAnimationFrame(() => requestAnimationFrame(() => (barsReady.value = true)))
}

async function refresh(): Promise<void> {
  const c = props.car
  if (!c) return
  editing.value = false
  const sc = scenario.value

  // success с уже загруженным/отредактированным регламентом — показываем без сети
  if (sc === 'success' && c.items) {
    shownItems.value = c.items
    settle()
    return
  }

  viewState.value = 'loading'
  barsReady.value = false
  await load({ make: c.make, model: c.model, year: c.year, mileage_km: c.mileage_km }, useMock ? sc : undefined)

  if (props.car?.id !== c.id) return // активная машина сменилась во время загрузки
  if (status.value === 'error') {
    viewState.value = 'error'
    return
  }
  const res = response.value
  if (!res) return // запрос отброшен (гонка)
  if (sc === 'empty') {
    viewState.value = 'empty'
    return
  }
  if (!c.items) c.items = res.items.map(cloneItem)
  shownItems.value = c.items
  settle()
}

function setScenario(s: MockScenario): void {
  scenario.value = s
}

/* ---- edit actions ---- */
function startEdit(): void {
  const c = props.car
  if (!c) return
  editItems.value = (c.items ?? []).map((it) => ({
    title: it.title,
    status: it.status,
    category: it.category,
    due_at_km: it.due_at_km,
    note: it.note,
  }))
  editMileage.value = c.mileage_km
  editing.value = true
}
function addWork(): void {
  editItems.value.push({
    title: '',
    status: 'due_soon',
    category: 'regular',
    due_at_km: props.car?.mileage_km ?? 0,
    note: '',
  })
}
function removeWork(i: number): void {
  editItems.value.splice(i, 1)
}
function saveEdit(): void {
  const c = props.car
  if (!c) return
  if (Number.isFinite(editMileage.value) && editMileage.value >= 0) c.mileage_km = editMileage.value
  const severityByStatus = (s: Status): Item['severity'] =>
    s === 'overdue' ? 'high' : s === 'due_soon' ? 'medium' : 'low'
  c.items = editItems.value
    .filter((r) => r.title.trim())
    .map((r, i) => ({
      id: `i${i}`,
      title: r.title.trim(),
      category: r.category,
      severity: severityByStatus(r.status),
      interval_km: 0,
      due_at_km: Number(r.due_at_km) || 0,
      status: r.status,
      note: r.note.trim(),
    }))
  editing.value = false
  shownItems.value = c.items
  settle()
}
function toggleEdit(): void {
  if (editing.value) saveEdit()
  else startEdit()
}

watch(() => props.car?.id, refresh, { immediate: true })
watch(scenario, refresh)

onBeforeUnmount(() => cancelAnimationFrame(odoRaf))
</script>

<template>
  <header class="rp-top">
    <button class="back" type="button" @click="emit('back')"><span aria-hidden="true">↑</span> Гараж</button>
    <div class="rp-right">
      <span class="rp-car">{{ carLabel }}</span>
      <button v-if="viewState === 'success'" class="edit-btn" type="button" @click="toggleEdit">
        {{ editing ? 'Готово' : 'Редактировать' }}
      </button>
    </div>
  </header>

  <main class="results-wrap">
    <div class="sec-head"><span class="n">01</span><h2>Что пора обслужить</h2></div>
    <p class="sec-sub">Регламентные работы и типовые поломки, привязанные к вашему пробегу.</p>

    <section class="results" aria-live="polite">
      <!-- loading -->
      <div v-if="viewState === 'loading'" class="skel">
        <div v-for="i in 4" :key="i" class="skc"></div>
      </div>

      <!-- error -->
      <div v-else-if="viewState === 'error'" class="state error">
        <div class="big" aria-hidden="true">⚡</div>
        <h3>Не удалось получить рекомендации</h3>
        <p>{{ error || 'Сервис не ответил. Попробуйте ещё раз.' }}</p>
        <button class="retry" type="button" @click="refresh">Повторить</button>
      </div>

      <!-- empty -->
      <div v-else-if="viewState === 'empty'" class="state">
        <div class="big" aria-hidden="true">✦</div>
        <h3>Срочных работ нет</h3>
        <p v-if="car">Для {{ car.make }} {{ car.model }} на пробеге {{ fmt.format(car.mileage_km) }} км рекомендаций нет.</p>
      </div>

      <!-- success: edit mode -->
      <template v-else-if="editing && car">
        <div class="edit-head">
          <label for="eMileage">Пробег, км</label>
          <input id="eMileage" v-model.number="editMileage" type="number" min="0" />
        </div>
        <div class="list">
          <div v-for="(row, i) in editItems" :key="i" class="card edit">
            <input v-model="row.title" class="e-title" placeholder="Название работы" />
            <div class="e-row">
              <select v-model="row.status" class="e-status">
                <option value="overdue">Просрочено</option>
                <option value="due_soon">Скоро</option>
                <option value="upcoming">Впереди</option>
              </select>
              <select v-model="row.category" class="e-cat">
                <option value="regular">Регламент</option>
                <option value="known_issue">Поломка</option>
              </select>
              <input v-model.number="row.due_at_km" class="e-due" type="number" min="0" placeholder="срок, км" />
              <button class="e-del" type="button" aria-label="Удалить" @click="removeWork(i)">✕</button>
            </div>
            <input v-model="row.note" class="e-note" placeholder="Заметка" />
          </div>
        </div>
        <button class="add-work" type="button" @click="addWork">＋ Добавить работу</button>
      </template>

      <!-- success: read mode -->
      <template v-else-if="car">
        <div class="summary">
          <div class="sum-top">
            <span class="who">{{ car.make }} {{ car.model }}</span>
            <span class="odo">
              <span class="num">{{ fmt.format(odo) }}</span>
              <span class="unit">км · {{ car.year }}</span>
            </span>
          </div>
          <div class="dist">
            <span class="s crit" :style="barStyle(counts.overdue)"></span>
            <span class="s warn" :style="barStyle(counts.due_soon)"></span>
            <span class="s calm" :style="barStyle(counts.upcoming)"></span>
          </div>
          <div class="dist-legend">
            <span class="lg crit"><span class="d"></span>Просрочено <b>{{ counts.overdue }}</b></span>
            <span class="lg warn"><span class="d"></span>Скоро <b>{{ counts.due_soon }}</b></span>
            <span class="lg calm"><span class="d"></span>Впереди <b>{{ counts.upcoming }}</b></span>
          </div>
        </div>

        <div class="list">
          <article
            v-for="(item, i) in sortedItems"
            :key="item.id"
            class="card"
            :class="statusMeta(item.status).cls"
            :style="cardDelay(i)"
          >
            <div class="ic" v-html="ICONS[item.category] || ICONS.regular"></div>
            <div class="c-main">
              <div class="c-title">{{ item.title }}</div>
              <div class="c-note">{{ item.note }}</div>
            </div>
            <div class="c-side">
              <span class="chip"><span class="d"></span>{{ statusMeta(item.status).label }}</span>
              <span class="c-when">{{ whenText(item, car.mileage_km) }}</span>
              <span class="c-due">срок {{ fmt.format(item.due_at_km) }} км</span>
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

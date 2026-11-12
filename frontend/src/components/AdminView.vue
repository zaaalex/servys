<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { listModels, listUsers, loadPulse, type AdminPulse, type GarageUser, type Inspection, type RunningModel } from '@/api/admin'
import { describeError } from '@/api/errors'

const emit = defineEmits<{ close: [] }>()

const section = ref<'models' | 'data'>('data')
const shelf = ref<'cars' | 'people' | 'checks'>('cars')
const view = ref<'all' | 'fresh' | 'due' | 'admins'>('all')
const searchOpen = ref(false)
const openId = ref('')
const pulse = ref<AdminPulse | null>(null)
const models = ref<RunningModel[]>([])
const users = ref<GarageUser[]>([])
const query = ref('')
const error = ref('')
const loading = ref(true)

const modelCopy: Record<string, { title: string, note: string }> = {
  search: { title: 'Поиск', note: 'Ищет страницы и отвечает на вопрос. Перед чтением текста выгружается.' },
  text: { title: 'Текст', note: 'Читает страницу, если таблица регламента её не разобрала.' },
  book: { title: 'Книжка', note: 'Читает снимок сервисной книжки. В очереди отдельно от этих двух.' },
}

onMounted(() => {
  void refresh()
})

async function refresh(): Promise<void> {
  error.value = ''
  loading.value = true
  try {
    const [board, people, running] = await Promise.all([loadPulse(), listUsers(), listModels()])
    pulse.value = board
    users.value = people.users
    models.value = running.models
  } catch (e) {
    error.value = describeError(e).message
  } finally {
    loading.value = false
  }
}

const everyDays = computed(() => pulse.value?.check_every_days ?? 7)

function everyLabel(days: number): string {
  const mod10 = days % 10
  const mod100 = days % 100
  let word = 'дней'
  if (mod10 === 1 && mod100 !== 11) word = 'день'
  else if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) word = 'дня'
  return days + ' ' + word
}

const cars = computed(() => pulse.value?.inspections ?? [])

function isDue(car: Inspection): boolean {
  if (!car.checked_at || !car.next_at) return false
  return new Date(car.next_at).getTime() <= Date.now()
}

function isFresh(car: Inspection): boolean {
  return Boolean(car.checked_at) && !isDue(car)
}

function isNewPerson(user: GarageUser): boolean {
  const key = user.created_at.slice(0, 10)
  return (pulse.value?.days ?? []).some((day) => day.date === key)
}

function pickShelf(next: 'cars' | 'people' | 'checks'): void {
  shelf.value = next
  view.value = 'all'
  openId.value = ''
  query.value = ''
}

const viewItems = computed(() => {
  if (shelf.value === 'people') {
    return [
      { key: 'all' as const, label: 'Все', count: users.value.length },
      { key: 'fresh' as const, label: 'Новые', count: users.value.filter(isNewPerson).length },
      { key: 'admins' as const, label: 'Админы', count: users.value.filter((user) => user.role === 'admin').length },
    ]
  }
  return [
    { key: 'all' as const, label: 'Все', count: cars.value.length },
    { key: 'fresh' as const, label: 'Свежие', count: cars.value.filter(isFresh).length },
    { key: 'due' as const, label: 'Пора', count: cars.value.filter(isDue).length },
  ]
})

function matchesQuery(text: string): boolean {
  const q = query.value.trim().toLowerCase()
  return !q || text.toLowerCase().includes(q)
}

interface DataRow {
  id: string
  eyebrow: string
  title: string
  status: string
  tone: 'ok' | 'warn' | 'neutral'
  facts: { label: string, value: string }[]
  noteLabel: string
  note: string
  detail: string
}

const rows = computed((): DataRow[] => {
  if (shelf.value === 'people') {
    return users.value
      .filter((user) => view.value === 'all' || (view.value === 'fresh' ? isNewPerson(user) : view.value === 'admins' && user.role === 'admin'))
      .filter((user) => matchesQuery([user.name, user.username, user.telegram_id].join(' ')))
      .map((user) => ({
        id: user.account_id,
        eyebrow: 'Пользователь',
        title: userTitle(user),
        status: user.role === 'admin' ? 'Админ' : 'В гараже',
        tone: user.role === 'admin' ? 'ok' : 'neutral',
        facts: [
          { label: 'Номер', value: user.telegram_id || '—' },
          { label: 'Имя', value: user.username ? '@' + user.username : '—' },
          { label: 'Вход', value: when(user.created_at) || '—' },
        ],
        noteLabel: 'Аккаунт',
        note: 'Появляется сам, когда человек открывает бота.',
        detail: user.telegram_id,
      }))
  }
  return cars.value
    .filter((car) => view.value === 'all' || (view.value === 'fresh' ? isFresh(car) : view.value === 'due' && isDue(car)))
    .filter((car) => matchesQuery([car.make, car.model, String(car.year), car.vin].join(' ')))
    .map((car) => shelf.value === 'checks' ? checkRow(car) : carRow(car))
})

function km(value: number): string {
  return new Intl.NumberFormat('ru-RU').format(value || 0)
}

function shortDate(iso: string): string {
  if (!iso) return '—'
  const parsed = new Date(iso)
  if (Number.isNaN(parsed.getTime())) return '—'
  return parsed.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
}

function carTone(car: Inspection): 'ok' | 'warn' | 'neutral' {
  if (!car.checked_at) return 'neutral'
  return isDue(car) ? 'warn' : 'ok'
}

function carStatus(car: Inspection): string {
  if (!car.checked_at) return 'Нет разбора'
  return isDue(car) ? 'Пора' : 'Свежая'
}

function carRow(car: Inspection): DataRow {
  return {
    id: car.vehicle_id,
    eyebrow: 'Машина',
    title: car.make + ' ' + car.model,
    status: carStatus(car),
    tone: carTone(car),
    facts: [
      { label: 'Год', value: String(car.year || '—') },
      { label: 'Пробег', value: km(car.mileage_km) },
      { label: 'Проверка', value: shortDate(car.checked_at) },
    ],
    noteLabel: 'Следующая',
    note: car.next_at ? ruDate(car.next_at) : 'когда откроют машину',
    detail: [car.vin, checkLine(car)].filter(Boolean).join(' · '),
  }
}

function checkRow(car: Inspection): DataRow {
  return {
    id: 'check-' + car.vehicle_id,
    eyebrow: 'Проверка',
    title: car.make + ' ' + car.model,
    status: carStatus(car),
    tone: carTone(car),
    facts: [
      { label: 'Была', value: shortDate(car.checked_at) },
      { label: 'Следующая', value: shortDate(car.next_at) },
      { label: 'Срок', value: pulse.value?.check_every_days === 0 ? 'нет' : everyLabel(everyDays.value) },
    ],
    noteLabel: 'Источники',
    note: pulse.value?.check_every_days === 0 ? 'Повторный разбор не запущен.' : 'Страницы по этой машине. Новое попадёт в следующий разбор.',
    detail: checkLine(car),
  }
}

const emptyTitle = computed(() => query.value.trim() ? 'Ничего не нашлось' : 'Записей пока нет')

const emptyText = computed(() => {
  if (query.value.trim()) return 'Записи в разделе есть, но под фильтр не попали.'
  if (shelf.value === 'people') return 'Аккаунт появится, когда человек откроет бота.'
  if (shelf.value === 'checks') return 'Проверка появится вместе с машиной.'
  return 'Машина появится, когда её добавят в гараж.'
})

const searchPlaceholder = computed(() => shelf.value === 'people' ? 'имя, @username или номер' : 'марка, модель или VIN')

const shelfTitle = computed(() => shelf.value === 'cars' ? 'Машины' : shelf.value === 'people' ? 'Пользователи' : 'Проверки')

const shelfLead = computed(() => {
  if (shelf.value === 'people') return 'Человек появляется сам, когда открывает бота.'
  if (shelf.value === 'checks') {
    if (pulse.value?.check_every_days === 0) return 'Повторной проверки нет: разбор не запущен.'
    return 'Источники по машине перечитываются раз в ' + everyLabel(everyDays.value) + '.'
  }
  return 'Машины из гаражей. Карточка открывается по нажатию.'
})

function userTitle(user: GarageUser): string {
  return user.name || (user.telegram_id ? 'Telegram ' + user.telegram_id : 'Без имени')
}

function when(raw: string): string {
  const [y, m, d] = raw.slice(0, 10).split('-')
  if (!d || !m || !y) return ''
  return d + '.' + m + '.' + y
}

function ruDate(iso: string): string {
  const parsed = new Date(iso)
  if (Number.isNaN(parsed.getTime())) return ''
  return parsed.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })
}

function checkLine(car: Inspection): string {
  if (!car.checked_at) return 'Ещё не проверяли. Первый разбор начнётся, когда откроют машину.'
  const checked = ruDate(car.checked_at)
  const next = ruDate(car.next_at)
  if (car.next_at && new Date(car.next_at).getTime() <= Date.now()) {
    return 'Проверена ' + checked + '. Следующий разбор уже пора.'
  }
  return 'Проверена ' + checked + '. Следующая ' + next + '.'
}

function modelTitle(model: RunningModel): string {
  return modelCopy[model.role]?.title || model.role
}

function modelNote(model: RunningModel): string {
  return modelCopy[model.role]?.note || ''
}
</script>

<template>
  <section class="screen">
    <header class="ticket-intro">
      <p class="screen-kicker">Админка</p>
      <h1>{{ section === 'models' ? 'Модели' : shelfTitle }}</h1>
      <p class="screen-lead">{{ section === 'models' ? 'Поиск, текст страницы и книжка. Две модели Ollama вместе не держатся.' : shelfLead }}</p>
    </header>

    <p v-if="error" class="err">{{ error }}</p>
    <p v-else-if="loading" class="book-empty">Считаю.</p>

    <div v-else-if="section === 'models'">
      <article v-for="model in models" :key="model.role" class="sheet-card">
        <span class="sheet-kicker">{{ modelTitle(model) }}</span>
        <span class="sheet-title">{{ model.name }}</span>
        <span v-if="modelNote(model)" class="sheet-note">{{ modelNote(model) }}</span>
      </article>
      <p v-if="!models.length" class="book-empty">Модели не заданы.</p>
    </div>

    <div v-else class="ex">
      <div class="ex-bar">
        <div class="ex-chips" role="tablist" aria-label="Раздел данных">
          <button type="button" role="tab" :aria-selected="shelf === 'cars'" :class="{ 'is-on': shelf === 'cars' }" @click="pickShelf('cars')">Машины <em>{{ cars.length }}</em></button>
          <button type="button" role="tab" :aria-selected="shelf === 'people'" :class="{ 'is-on': shelf === 'people' }" @click="pickShelf('people')">Пользователи <em>{{ users.length }}</em></button>
          <button type="button" role="tab" :aria-selected="shelf === 'checks'" :class="{ 'is-on': shelf === 'checks' }" @click="pickShelf('checks')">Проверки <em>{{ cars.length }}</em></button>
        </div>
        <button type="button" class="ex-icon" :class="{ 'is-on': searchOpen }" :aria-expanded="searchOpen ? 'true' : 'false'" :aria-label="searchOpen ? 'Скрыть поиск' : 'Поиск'" @click="searchOpen = !searchOpen">
          <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.6-3.6" /></svg>
        </button>
      </div>
      <form v-if="searchOpen" class="ex-search" role="search" @submit.prevent>
        <input v-model="query" :placeholder="searchPlaceholder" aria-label="Поиск по разделу">
        <button type="button" @click="searchOpen = false">Скрыть</button>
      </form>
      <div class="ex-views" role="tablist" aria-label="Вид">
        <button v-for="item in viewItems" :key="item.key" type="button" role="tab" :aria-selected="view === item.key" :class="{ 'is-on': view === item.key }" @click="view = item.key; openId = ''">
          {{ item.label }} <em>{{ item.count }}</em>
        </button>
      </div>
      <p v-if="!rows.length" class="ex-empty" role="status"><b>{{ emptyTitle }}</b><span>{{ emptyText }}</span></p>
      <article v-for="row in rows" :key="row.id" class="ex-card" role="button" tabindex="0" :aria-expanded="openId === row.id ? 'true' : 'false'" @click="openId = openId === row.id ? '' : row.id" @keydown.enter.prevent="openId = openId === row.id ? '' : row.id" @keydown.space.prevent="openId = openId === row.id ? '' : row.id">
        <header>
          <div>
            <span class="ex-eye">{{ row.eyebrow }}</span>
            <strong>{{ row.title }}</strong>
          </div>
          <span class="ex-status" :class="'tone-' + row.tone">{{ row.status }}</span>
        </header>
        <div class="ex-facts">
          <div v-for="fact in row.facts" :key="fact.label"><small>{{ fact.label }}</small><b>{{ fact.value }}</b></div>
        </div>
        <p class="ex-note"><small>{{ row.noteLabel }}</small><span>{{ row.note }}</span></p>
        <p v-if="openId === row.id" class="ex-detail" @click.stop>{{ row.detail }} <button type="button" @click="openId = ''">Закрыть</button></p>
      </article>
    </div>

    <nav class="tabbar" aria-label="Админка">
      <button type="button" :aria-current="section === 'data' ? 'page' : undefined" @click="section = 'data'">Данные</button>
      <button type="button" :aria-current="section === 'models' ? 'page' : undefined" @click="section = 'models'">Модели</button>
      <button type="button" aria-label="Пользовательский интерфейс" title="Пользовательский интерфейс" @click="emit('close')">Гараж</button>
    </nav>
  </section>
</template>

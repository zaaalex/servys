<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useGarage } from '@/composables/useGarage'
import { listServiceEvents, addServiceEvent, type ServiceLogEntry } from '@/api/client'
import { describeError } from '@/api/errors'
import { WORKS, workTitle } from '@/data/works'
import ServiceHistory from '@/components/ServiceHistory.vue'
import { touchBook } from '@/composables/useBookStamp'

const { activeVehicle } = useGarage()

const entries = ref<ServiceLogEntry[]>([])
const loading = ref(false)
const loaded = ref(false)
const loadError = ref('')
const saving = ref(false)
const formError = ref('')
const saved = ref(false)

const code = ref(WORKS[0]?.code ?? '')
const date = ref(localToday())
const km = ref<number | string>(activeVehicle.value?.currentOdometer ?? 0)

// Ответ предыдущей машины не должен перезаписать список текущей.
let flight = 0

const ruDate = new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})

const opening = computed(() => loading.value && !loaded.value && entries.value.length === 0)
const showEmpty = computed(
  () => loaded.value && !loading.value && !loadError.value && entries.value.length === 0,
)

function localToday(): string {
  const now = new Date()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${now.getFullYear()}-${month}-${day}`
}

function formatRuDate(value: Date): string {
  const parts = ruDate.formatToParts(value)
  const day = parts.find((part) => part.type === 'day')?.value
  const month = parts.find((part) => part.type === 'month')?.value
  const year = parts.find((part) => part.type === 'year')?.value
  if (!day || !month || !year) return ruDate.format(value)
  return `${day} ${month} ${year}`
}

function performedLabel(raw: string): string | null {
  const text = raw.trim()
  if (!text) return null
  // performed_at — RFC3339 с полуночью UTC; в строке берём календарную дату.
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(text)
  if (match) {
    const year = Number(match[1])
    const month = Number(match[2])
    const day = Number(match[3])
    const parsed = new Date(year, month - 1, day)
    if (parsed.getFullYear() !== year || parsed.getMonth() !== month - 1 || parsed.getDate() !== day) return null
    return formatRuDate(parsed)
  }
  const parsed = new Date(text)
  if (Number.isNaN(parsed.getTime())) return null
  return formatRuDate(parsed)
}

function lineOf(entry: ServiceLogEntry): string {
  const kmText = `${entry.odometer.toLocaleString('ru-RU')} км`
  const when = performedLabel(entry.performedAt)
  return when ? `${when} · ${kmText}` : kmText
}

function dismissSaved(): void {
  saved.value = false
}

async function reload(): Promise<void> {
  const vehicle = activeVehicle.value
  if (!vehicle) return
  const id = vehicle.id
  const mine = ++flight
  loading.value = true
  loadError.value = ''
  try {
    const list = await listServiceEvents(id)
    if (mine !== flight) return
    entries.value = list
    loaded.value = true
  } catch (e) {
    if (mine !== flight) return
    loadError.value = describeError(e).message
    loaded.value = true
  } finally {
    if (mine === flight) loading.value = false
  }
}

async function onSubmit(): Promise<void> {
  const vehicle = activeVehicle.value
  if (!vehicle || saving.value) return
  const work = code.value.trim()
  const when = date.value.trim()
  const odometer = typeof km.value === 'number' ? km.value : Number.NaN
  if (!work || !when || !Number.isFinite(odometer) || odometer < 0) {
    formError.value = 'Укажите работу, дату и пробег.'
    return
  }
  saving.value = true
  formError.value = ''
  try {
    await addServiceEvent(vehicle.id, {
      ruleCode: work,
      odometer: Math.round(odometer),
      date: when,
    })
    if (activeVehicle.value?.id === vehicle.id) await reload()
    touchBook()
    if (activeVehicle.value?.id === vehicle.id) saved.value = true
  } catch (e) {
    formError.value = describeError(e).message
  } finally {
    saving.value = false
  }
}

async function onPasted(): Promise<void> {
  await reload()
  touchBook()
}

watch(
  () => activeVehicle.value?.id,
  (id) => {
    entries.value = []
    loadError.value = ''
    loaded.value = false
    saved.value = false
    formError.value = ''
    const vehicle = activeVehicle.value
    if (vehicle && id) km.value = vehicle.currentOdometer
    if (!id) {
      flight += 1
      loading.value = false
      return
    }
    void reload()
  },
  { immediate: true },
)
</script>

<template>
  <section class="screen">
    <p class="screen-kicker">Сервисная книжка</p>
    <h1>Сервисная книжка</h1>
    <p v-if="!activeVehicle" class="screen-lead">Сначала добавьте машину в гараже.</p>
    <template v-else>
      <p class="screen-lead">
        Что меняли и когда. {{ activeVehicle.make }} {{ activeVehicle.model }}, {{ activeVehicle.year }}.
      </p>
      <p v-if="saved" class="book-empty">Записано в книжку.</p>
      <form class="book-form" novalidate @submit.prevent="onSubmit">
        <div class="field">
          <label for="book-work">Что меняли</label>
          <div class="box">
            <select id="book-work" v-model="code" @change="dismissSaved">
              <option v-for="work in WORKS" :key="work.code" :value="work.code">{{ work.title }}</option>
            </select>
          </div>
        </div>
        <div class="field">
          <label for="book-date">Когда</label>
          <div class="box">
            <input id="book-date" v-model="date" type="date" @input="dismissSaved" />
          </div>
        </div>
        <div class="field">
          <label for="book-km">Пробег, км</label>
          <div class="box">
            <input id="book-km" v-model.number="km" type="number" min="0" inputmode="numeric" @input="dismissSaved" />
          </div>
        </div>
        <p v-if="formError" class="err">{{ formError }}</p>
        <button class="go" type="submit" :disabled="saving">{{ saving ? 'Секунду…' : 'Записать' }}</button>
      </form>
      <div class="book-block">
        <p v-if="opening" class="book-empty">Книжка открывается…</p>
        <p v-if="loadError" class="err">{{ loadError }}</p>
        <p v-if="showEmpty" class="book-empty">Пока пусто. Запишите, что и когда меняли — книжка хранит это для машины.</p>
        <div v-for="entry in entries" :key="entry.id" class="book-row">
          <b>{{ workTitle(entry.ruleCode) }}</b>
          <span>{{ lineOf(entry) }}</span>
        </div>
      </div>
      <ServiceHistory :vehicle-id="activeVehicle.id" @saved="onPasted" />
    </template>
  </section>
</template>

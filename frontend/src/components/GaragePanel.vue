<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Vehicle } from '@/types/api'
import { BACKGROUNDS, useBackground } from '@/composables/useBackground'

const { current: bg, set: setBg } = useBackground()

const props = defineProps<{
  cars: Vehicle[]
  activeId: string
  loading?: boolean
  collapsed?: boolean
  urgentIds?: string[]
}>()

const urgentSet = computed(() => new Set(props.urgentIds ?? []))
const anyUrgent = computed(() => props.cars.some((c) => urgentSet.value.has(c.id)))

const emit = defineEmits<{
  select: [id: string]
  add: []
  remove: [id: string]
  toggle: []
}>()

const fmt = new Intl.NumberFormat('ru-RU')

// id машины, для которой показываем инлайн-подтверждение удаления
const confirmId = ref<string>('')
// id машины, которую сейчас удаляем (блокируем повторный клик)
const removingId = ref<string>('')

function askRemove(id: string): void {
  confirmId.value = id
}

function cancelRemove(): void {
  confirmId.value = ''
}

async function doRemove(id: string): Promise<void> {
  if (removingId.value) return
  removingId.value = id
  try {
    emit('remove', id)
  } finally {
    removingId.value = ''
    confirmId.value = ''
  }
}

function plural(n: number, a: string, b: string, c: string): string {
  const m = n % 100
  const d = n % 10
  if (m >= 11 && m <= 14) return c
  if (d === 1) return a
  if (d >= 2 && d <= 4) return b
  return c
}

const countLabel = computed(() =>
  props.loading
    ? 'загрузка…'
    : `${props.cars.length} ${plural(props.cars.length, 'автомобиль', 'автомобиля', 'автомобилей')}`,
)
</script>

<template>
  <aside class="garage" :class="{ 'is-collapsed': collapsed }">
    <button
      class="garage-toggle"
      type="button"
      :aria-label="collapsed ? 'Развернуть гараж' : 'Свернуть гараж'"
      :aria-expanded="!collapsed"
      :title="collapsed ? 'Развернуть гараж' : 'Свернуть гараж'"
      @click="emit('toggle')"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M15 18l-6-6 6-6" />
      </svg>
    </button>

    <!-- Свёрнутое состояние: тонкий рельс -->
    <div v-if="collapsed" class="garage-rail">
      <div class="pf-av rail-av" aria-hidden="true">
        КД
        <span v-if="anyUrgent" class="urgent-dot rail-dot" title="Есть срочные работы"></span>
      </div>
      <span class="rail-label">Мой гараж · {{ cars.length }}</span>
    </div>

    <!-- Развёрнутое состояние -->
    <template v-else>
      <div class="profile">
        <div class="pf-av">КД</div>
        <div class="pf-txt">
          <div class="pf-name">Мой гараж</div>
          <div class="pf-sub">{{ countLabel }}</div>
        </div>
      </div>

      <div class="car-list">
        <div
          v-for="car in cars"
          :key="car.id"
          class="car-row"
          :class="{ 'is-active': car.id === activeId }"
        >
          <button
            type="button"
            class="car-item"
            :class="{ 'is-active': car.id === activeId }"
            @click="emit('select', car.id)"
          >
            <span class="ci-dot" :style="{ background: car.color }"></span>
            <span class="ci-main">
              <span class="ci-name">{{ car.make }} {{ car.model }}</span>
              <span class="ci-meta">{{ car.year }} · {{ fmt.format(car.currentOdometer) }} км</span>
            </span>
          </button>

          <span
            v-if="urgentSet.has(car.id)"
            class="urgent-dot car-urgent"
            title="Есть срочные работы"
            aria-label="Есть срочные работы"
          ></span>

          <button
            v-if="confirmId !== car.id"
            type="button"
            class="car-del"
            :aria-label="`Удалить ${car.make} ${car.model}`"
            title="Удалить машину"
            @click.stop="askRemove(car.id)"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" />
            </svg>
          </button>

          <div v-else class="car-confirm">
            <span class="cc-text">Удалить?</span>
            <button type="button" class="cc-yes" :disabled="removingId === car.id" @click.stop="doRemove(car.id)">Да</button>
            <button type="button" class="cc-no" @click.stop="cancelRemove">Нет</button>
          </div>
        </div>

        <div v-if="loading && cars.length === 0" class="garage-empty">Загрузка гаража…</div>
        <div v-else-if="cars.length === 0" class="garage-empty">
          <p class="ge-title">Гараж пуст</p>
          <p class="ge-sub">Добавьте первую машину по VIN — ниже.</p>
        </div>
      </div>

      <button class="add-car" type="button" @click="emit('add')">
        <span>＋</span> Добавить машину
      </button>

      <div class="bg-switch">
        <span class="bg-switch-label">Фон</span>
        <div class="bg-swatches">
          <button
            v-for="b in BACKGROUNDS"
            :key="b.id"
            type="button"
            class="bg-sw"
            :class="[`bg-sw-${b.id}`, { sel: b.id === bg }]"
            :title="b.name"
            :aria-label="`Фон: ${b.name}`"
            :aria-pressed="b.id === bg"
            @click="setBg(b.id)"
          ></button>
        </div>
      </div>
    </template>
  </aside>
</template>

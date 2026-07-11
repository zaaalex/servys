<script setup lang="ts">
import { computed } from 'vue'
import type { Vehicle } from '@/types/api'
import { BACKGROUNDS, useBackground } from '@/composables/useBackground'

const { current: bg, set: setBg } = useBackground()

const props = defineProps<{
  cars: Vehicle[]
  activeId: string
  loading?: boolean
}>()

const emit = defineEmits<{
  select: [id: string]
  add: []
}>()

const fmt = new Intl.NumberFormat('ru-RU')

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
  <aside class="garage">
    <div class="profile">
      <div class="pf-av">КД</div>
      <div class="pf-txt">
        <div class="pf-name">Мой гараж</div>
        <div class="pf-sub">{{ countLabel }}</div>
      </div>
    </div>

    <div class="car-list">
      <button
        v-for="car in cars"
        :key="car.id"
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
  </aside>
</template>

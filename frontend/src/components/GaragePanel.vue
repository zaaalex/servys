<script setup lang="ts">
import { computed } from 'vue'
import { COLOR_PRESETS } from '@/data/presets'
import type { Vehicle } from '@/composables/useGarage'

const props = defineProps<{
  cars: Vehicle[]
  activeId: number
}>()

const emit = defineEmits<{
  select: [id: number]
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

const countLabel = computed(
  () => `${props.cars.length} ${plural(props.cars.length, 'автомобиль', 'автомобиля', 'автомобилей')}`,
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
        <span class="ci-dot" :style="{ background: COLOR_PRESETS[car.colorIndex].css }"></span>
        <span class="ci-main">
          <span class="ci-name">{{ car.make }} {{ car.model }}</span>
          <span class="ci-meta">{{ car.year }} · {{ fmt.format(car.mileage_km) }} км</span>
        </span>
      </button>
    </div>

    <button class="add-car" type="button" @click="emit('add')">
      <span>＋</span> Добавить машину
    </button>
  </aside>
</template>

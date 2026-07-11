<script setup lang="ts">
// Наглядный отчёт скана: плитки с числами + список ошибок (частичные сбои).
import { computed } from 'vue'

export type TileTone = 'accent' | 'warn' | 'calm' | 'crit'
export interface Tile {
  label: string
  value: number
  tone: TileTone
}

const props = defineProps<{
  tiles: Tile[]
  errors?: string[]
}>()

const fmt = new Intl.NumberFormat('ru-RU')
const errors = computed(() => props.errors ?? [])
</script>

<template>
  <div class="b2b-report">
    <div class="b2b-tiles">
      <div v-for="t in tiles" :key="t.label" class="b2b-tile" :class="`tone-${t.tone}`">
        <span class="b2b-tile-num">{{ fmt.format(t.value) }}</span>
        <span class="b2b-tile-lbl">{{ t.label }}</span>
      </div>
    </div>
    <ul v-if="errors.length" class="b2b-errors">
      <li v-for="(e, i) in errors" :key="i">{{ e }}</li>
    </ul>
  </div>
</template>

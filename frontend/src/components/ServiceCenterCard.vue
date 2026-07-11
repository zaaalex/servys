<script setup lang="ts">
// Карточка одного СТО: реквизиты + кнопка «Сканировать» + отчёт/ошибка скана.
import { computed } from 'vue'
import ScanReportTiles, { type Tile } from '@/components/ScanReportTiles.vue'
import type { ScanState } from '@/composables/useServiceCenters'
import type { ServiceCenter } from '@/types/b2b'

const props = defineProps<{
  center: ServiceCenter
  scan: ScanState | undefined
}>()

const emit = defineEmits<{ scan: [id: string] }>()

const scanning = computed(() => props.scan?.scanning ?? false)
const report = computed(() => props.scan?.report ?? null)
const error = computed(() => props.scan?.error ?? null)

const tiles = computed<Tile[]>(() => {
  const r = report.value
  if (!r) return []
  return [
    { label: 'авто', value: r.cars, tone: 'calm' },
    { label: 'подошло ТО', value: r.due_items, tone: 'warn' },
    { label: 'дел создано', value: r.pushed, tone: 'accent' },
    { label: 'пропущено', value: r.skipped, tone: 'calm' },
  ]
})
</script>

<template>
  <article class="b2b-sc-card">
    <div class="b2b-sc-head">
      <div class="b2b-sc-main">
        <div class="b2b-sc-name">{{ center.name }}</div>
        <div class="b2b-sc-meta">
          <span class="b2b-sc-id">{{ center.id }}</span>
          <span class="b2b-sc-resp">ответственный #{{ center.responsible_id }}</span>
        </div>
      </div>
      <button class="go go-sm" type="button" :disabled="scanning" @click="emit('scan', center.id)">
        <span v-if="scanning" class="spin" aria-hidden="true"></span>
        {{ scanning ? 'Сканирую…' : 'Сканировать' }}
      </button>
    </div>

    <div v-if="error" class="b2b-inline-err">
      <span>{{ error.message }}</span>
      <button class="b2b-link" type="button" :disabled="scanning" @click="emit('scan', center.id)">
        Повторить
      </button>
    </div>

    <ScanReportTiles v-else-if="report" :tiles="tiles" :errors="report.errors" />
  </article>
</template>

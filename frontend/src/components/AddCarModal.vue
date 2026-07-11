<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import CarScene from '@/components/CarScene.vue'
import { BODY_TYPES, COLOR_PRESETS, type BodyType, type RGB } from '@/data/presets'
import { decodeVin, type DecodedVin } from '@/data/vin'

const emit = defineEmits<{
  close: []
  add: [vehicle: { make: string; model: string; year: number; mileage_km: number; colorIndex: number; type: BodyType }]
}>()

const fmt = new Intl.NumberFormat('ru-RU')

const vin = ref('')
const vinMessage = ref('')
const vinOk = ref(false)
const decoded = ref<DecodedVin | null>(null)

const selectedType = ref<BodyType>('sedan')
const selectedColor = ref(0)
const previewColor = computed<RGB>(() => COLOR_PRESETS[selectedColor.value].rgb)

const scene = ref<InstanceType<typeof CarScene> | null>(null)
const vinInput = ref<HTMLInputElement | null>(null)

onMounted(() => vinInput.value?.focus())

function runLookup(): DecodedVin | null {
  const result = decodeVin(vin.value)
  if ('err' in result) {
    decoded.value = null
    vinOk.value = false
    vinMessage.value = result.err
    return null
  }
  decoded.value = result
  selectedType.value = result.type
  vinOk.value = true
  vinMessage.value = `✓ ${result.make} ${result.model} · ${result.year} · ~${fmt.format(result.mileage_km)} км`
  scene.value?.flourish()
  return result
}

function submit(): void {
  const d = decoded.value ?? runLookup()
  if (!d) return
  emit('add', {
    make: d.make,
    model: d.model,
    year: d.year,
    mileage_km: d.mileage_km,
    colorIndex: selectedColor.value,
    type: selectedType.value,
  })
}
</script>

<template>
  <div class="modal">
    <div class="modal-backdrop" @click="emit('close')"></div>
    <div class="modal-card" role="dialog" aria-modal="true" aria-label="Добавить машину">
      <div class="modal-head">
        <h3>Добавить машину</h3>
        <button class="modal-x" type="button" aria-label="Закрыть" @click="emit('close')">✕</button>
      </div>

      <div class="mini-stage">
        <CarScene
          ref="scene"
          class="mini-scene"
          :type="selectedType"
          :color="previewColor"
          :interactive="false"
          :dist="7.6"
          :cam-y="-0.15"
        />
      </div>

      <form novalidate @submit.prevent="submit">
        <div class="vin-field">
          <label for="vin">VIN-номер</label>
          <div class="vin-row">
            <div class="box">
              <input
                id="vin"
                ref="vinInput"
                v-model="vin"
                type="text"
                autocomplete="off"
                spellcheck="false"
                maxlength="17"
                placeholder="напр. JTDBE32K700261000"
                @keydown.enter.prevent="runLookup"
              />
            </div>
            <button type="button" class="vin-find" @click="runLookup">Найти</button>
          </div>
          <div class="vin-result" :class="{ ok: vinOk, bad: !vinOk && vinMessage }">{{ vinMessage }}</div>
        </div>

        <div class="avatar-pick">
          <label class="ap-label">Тип кузова</label>
          <div class="types">
            <button
              v-for="t in BODY_TYPES"
              :key="t.id"
              type="button"
              class="ty"
              :class="{ sel: t.id === selectedType }"
              @click="selectedType = t.id"
            >
              {{ t.name }}
            </button>
          </div>
        </div>

        <div class="avatar-pick">
          <label class="ap-label">Аватар — цвет кузова</label>
          <div class="swatches">
            <button
              v-for="(c, i) in COLOR_PRESETS"
              :key="c.name"
              type="button"
              class="sw"
              :class="{ sel: i === selectedColor }"
              :style="{ background: c.css }"
              :aria-label="c.name"
              @click="selectedColor = i"
            ></button>
          </div>
        </div>

        <div class="modal-actions">
          <button type="button" class="btn-ghost" @click="emit('close')">Отмена</button>
          <button type="submit" class="go go-sm">Добавить в гараж</button>
        </div>
      </form>
    </div>
  </div>
</template>

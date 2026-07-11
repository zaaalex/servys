<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import CarScene from '@/components/CarScene.vue'
import { resolveVin } from '@/api/client'
import { apiBodyToScene, BODY_TYPES, COLOR_PRESETS, type RGB } from '@/data/presets'
import type { ApiBodyType, CreateVehicleRequest, VinResolveResult } from '@/types/api'

const emit = defineEmits<{
  close: []
  add: [vehicle: CreateVehicleRequest]
}>()

const vin = ref('')
const vinMessage = ref('')
const vinOk = ref(false)
const looking = ref(false)
const resolved = ref<VinResolveResult | null>(null)

const selectedBody = ref<ApiBodyType>('sedan')
const selectedColor = ref(0)
const odometer = ref<number | null>(null)
const formError = ref('')

const previewColor = computed<RGB>(() => COLOR_PRESETS[selectedColor.value].rgb)
const previewScene = computed(() => apiBodyToScene(selectedBody.value))

const scene = ref<InstanceType<typeof CarScene> | null>(null)
const vinInput = ref<HTMLInputElement | null>(null)
const odoInput = ref<HTMLInputElement | null>(null)

onMounted(() => vinInput.value?.focus())

async function runLookup(): Promise<void> {
  if (looking.value) return
  looking.value = true
  vinMessage.value = 'Ищем по VIN…'
  vinOk.value = false
  try {
    const r = await resolveVin(vin.value)
    resolved.value = r
    selectedBody.value = r.signature.bodyType
    vinOk.value = true
    vinMessage.value = `✓ ${r.signature.make} ${r.signature.model} · ${r.signature.year}${
      r.matchLevel === 'partial' ? ' · совпадение частичное' : ''
    }`
    scene.value?.flourish()
    odoInput.value?.focus()
  } catch (e) {
    resolved.value = null
    vinOk.value = false
    vinMessage.value = e instanceof Error ? e.message : 'Не удалось распознать VIN'
  } finally {
    looking.value = false
  }
}

function submit(): void {
  const r = resolved.value
  if (!r) {
    formError.value = 'Сначала найдите машину по VIN.'
    return
  }
  if (!Number.isFinite(odometer.value) || (odometer.value ?? 0) < 0) {
    formError.value = 'Укажите текущий пробег.'
    return
  }
  formError.value = ''
  emit('add', {
    vin: r.vin,
    make: r.signature.make,
    model: r.signature.model,
    year: r.signature.year,
    bodyType: selectedBody.value,
    fuelType: r.signature.fuelType ?? 'gasoline',
    color: COLOR_PRESETS[selectedColor.value].css,
    odometer: Math.round(odometer.value ?? 0),
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
          :type="previewScene"
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
            <button type="button" class="vin-find" :disabled="looking" @click="runLookup">
              {{ looking ? '…' : 'Найти' }}
            </button>
          </div>
          <div class="vin-result" :class="{ ok: vinOk, bad: !vinOk && vinMessage }">{{ vinMessage }}</div>
        </div>

        <div class="vin-field" v-if="resolved">
          <label for="odo">Текущий пробег, км</label>
          <div class="box">
            <input id="odo" ref="odoInput" v-model.number="odometer" type="number" min="0" placeholder="95000" />
          </div>
        </div>

        <div class="avatar-pick">
          <label class="ap-label">Тип кузова</label>
          <div class="types">
            <button
              v-for="t in BODY_TYPES"
              :key="t.id"
              type="button"
              class="ty"
              :class="{ sel: t.id === selectedBody }"
              @click="selectedBody = t.id"
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

        <div class="err">{{ formError }}</div>
        <div class="modal-actions">
          <button type="button" class="btn-ghost" @click="emit('close')">Отмена</button>
          <button type="submit" class="go go-sm">Добавить в гараж</button>
        </div>
      </form>
    </div>
  </div>
</template>

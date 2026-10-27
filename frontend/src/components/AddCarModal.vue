<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import CarScene from '@/components/CarScene.vue'
import { resolveVin } from '@/api/client'
import { describeError } from '@/api/errors'
import { apiBodyToScene, BODY_TYPES, COLOR_PRESETS, type RGB } from '@/data/presets'
import type { ApiBodyType, CreateVehicleRequest, IdentificationSource, VinResolveResult } from '@/types/api'

const emit = defineEmits<{
  close: []
  add: [vehicle: CreateVehicleRequest]
}>()

const vin = ref('')
const vinMessage = ref('')
const vinOk = ref(false)
const looking = ref(false)

const mMake = ref('')
const mModel = ref('')
const mYear = ref<number | null>(null)
const mEngineCc = ref<number | null>(null)
const mPowerHp = ref<number | null>(null)

const selectedBody = ref<ApiBodyType>('sedan')
const selectedColor = ref(0)
const identificationSource = ref<IdentificationSource>('manual')
const odometer = ref<number | null>(null)
const formError = ref('')

const previewColor = computed<RGB>(() => COLOR_PRESETS[selectedColor.value].rgb)
const previewScene = computed(() => apiBodyToScene(selectedBody.value))
const bodyLabel = computed(() => BODY_TYPES.find((t) => t.id === selectedBody.value)?.name ?? '')

const scene = ref<InstanceType<typeof CarScene> | null>(null)
const vinInput = ref<HTMLInputElement | null>(null)
const odoInput = ref<HTMLInputElement | null>(null)

onMounted(() => {
  document.getElementById('deck')?.classList.add('deck-locked')
  vinInput.value?.focus()
})
onBeforeUnmount(() => document.getElementById('deck')?.classList.remove('deck-locked'))

function scrollField(e: FocusEvent): void {
  const el = e.target
  if (el instanceof HTMLElement) el.scrollIntoView({ block: 'center' })
}

function fillFrom(r: VinResolveResult): void {
  mMake.value = r.signature.make
  mModel.value = r.signature.model
  mYear.value = r.signature.year || null
  mEngineCc.value = r.signature.engineDisplacementCc || null
  mPowerHp.value = r.signature.powerHp || null
  if (r.signature.bodyType) selectedBody.value = r.signature.bodyType
  identificationSource.value = r.identificationSource
}

async function runLookup(): Promise<void> {
  if (looking.value) return
  looking.value = true
  vinMessage.value = 'Ищем по VIN…'
  vinOk.value = false
  identificationSource.value = 'manual'
  try {
    const r = await resolveVin(vin.value)
    fillFrom(r)
    const named = [r.signature.make, r.signature.model, r.signature.year].filter(Boolean).join(' · ')
    if (!r.signature.make || !r.signature.model) {
      vinMessage.value = named ? `Определили частично: ${named}. Дополните марку, модель и год.` : 'VIN узнали не до конца. Дополните марку, модель и год.'
      return
    }
    vinOk.value = true
    vinMessage.value = `✓ ${r.signature.make} ${r.signature.model} · ${r.signature.year} · ${bodyLabel.value}`
    scene.value?.flourish()
    void nextTick(() => odoInput.value?.focus())
  } catch (e) {
    vinOk.value = false
    vinMessage.value = describeError(e).message
  } finally {
    looking.value = false
  }
}

function submit(): void {
  if (!mMake.value.trim() || !mModel.value.trim()) {
    formError.value = 'Укажите марку и модель. Год и мотор помогают найти именно эту машину.'
    return
  }
  const year = mYear.value ?? 0
  if (!Number.isFinite(year) || year < 1950 || year > 2027) {
    formError.value = 'Укажите корректный год (1950–2027).'
    return
  }
  if (!Number.isFinite(odometer.value) || (odometer.value ?? 0) < 0) {
    formError.value = 'Укажите текущий пробег.'
    return
  }
  const vinValue = vin.value.trim().toUpperCase()
  formError.value = ''
  emit('add', {
    vin: vinValue || undefined,
    make: mMake.value.trim(),
    model: mModel.value.trim(),
    year,
    engineCc: mEngineCc.value ?? undefined,
    powerHp: mPowerHp.value ?? undefined,
    bodyType: selectedBody.value,
    fuelType: 'gasoline',
    color: COLOR_PRESETS[selectedColor.value].css,
    identificationSource: identificationSource.value,
    odometer: Math.round(odometer.value ?? 0),
  })
}
</script>

<template>
  <div class="modal">
    <div class="modal-backdrop" @click="emit('close')"></div>
    <div class="modal-card add-sheet" role="dialog" aria-modal="true" aria-label="Добавить машину">
      <div class="modal-head">
        <h3>Добавить машину</h3>
        <button class="modal-x" type="button" aria-label="Закрыть" @click="emit('close')">✕</button>
      </div>

      <form class="add-form" novalidate @submit.prevent="submit">
      <div class="modal-scroll">
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

        <div class="vin-field">
          <label for="vin">VIN, или год и марка</label>
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
          <p class="who-hint">VIN заполняет поля сам. Без VIN достаточно года, марки и модели. Объём и мощность сужают регламент.</p>
        </div>

        <div class="manual-block">
          <div class="m-grid2">
            <div class="vin-field">
              <label for="mMake">Марка</label>
              <div class="mf-box"><input id="mMake" v-model="mMake" type="text" autocomplete="off" placeholder="напр. KIA" /></div>
            </div>
            <div class="vin-field">
              <label for="mModel">Модель</label>
              <div class="mf-box"><input id="mModel" v-model="mModel" type="text" autocomplete="off" placeholder="напр. K3" /></div>
            </div>
          </div>
          <div class="m-grid3">
            <div class="vin-field">
              <label for="mYear">Год</label>
              <div class="mf-box"><input id="mYear" v-model.number="mYear" type="number" min="1950" max="2027" inputmode="numeric" placeholder="2020" /></div>
            </div>
            <div class="vin-field">
              <label for="mCc">Объём, см³</label>
              <div class="mf-box"><input id="mCc" v-model.number="mEngineCc" type="number" min="0" inputmode="numeric" placeholder="1353" /></div>
            </div>
            <div class="vin-field">
              <label for="mHp">Мощность, л.с.</label>
              <div class="mf-box"><input id="mHp" v-model.number="mPowerHp" type="number" min="0" inputmode="numeric" placeholder="130" /></div>
            </div>
          </div>
          <div class="vin-field">
            <label for="mBody">Тип кузова</label>
            <div class="mf-box"><select id="mBody" v-model="selectedBody">
              <option v-for="t in BODY_TYPES" :key="t.id" :value="t.id">{{ t.name }}</option>
            </select></div>
          </div>
        </div>

        <div class="odo-field">
          <label for="odo">Текущий пробег</label>
          <div class="odo-box">
            <input id="odo" ref="odoInput" v-model.number="odometer" type="number" min="0" inputmode="numeric" placeholder="0" @focus="scrollField" />
            <span class="odo-unit">км</span>
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
      </div>
        <div class="modal-actions">
          <button type="button" class="btn-ghost" @click="emit('close')">Отмена</button>
          <button type="submit" class="go go-sm">Добавить в гараж</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.add-sheet .modal-head {
  position: sticky;
  top: 0;
  z-index: 2;
  background: var(--bg-2);
  padding-bottom: 10px;
}
.modal-scroll { min-width: 0; }
.add-sheet .modal-actions {
  position: sticky;
  bottom: 0;
  z-index: 2;
  margin-top: 14px;
  padding-top: 14px;
  background: var(--bg-2);
  border-top: 1px solid var(--line);
}
@media (max-height: 760px) {
  .add-sheet :deep(.mini-stage) { height: 92px; margin-bottom: 12px; }
}
.who-hint {
  margin: 0;
  color: var(--ink-3);
  font-size: 12.5px;
}
.manual-toggle {
  justify-self: start;
  appearance: none;
  border: none;
  background: transparent;
  padding: 2px 0;
  margin-top: 2px;
  color: var(--ink-3);
  font: inherit;
  font-size: 12.5px;
  text-decoration: underline;
  text-underline-offset: 3px;
  cursor: pointer;
  transition: color 0.18s;
}
.manual-toggle:hover {
  color: var(--g1);
}
.manual-block {
  display: grid;
  gap: 14px;
  margin-top: 14px;
}
/* minmax(0,1fr): иначе врождённая ширина input/select задаёт минимум трека,
   колонки получаются неравными и сетка вылезает за модалку */
.m-grid2 {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}
.m-grid3 {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}
/* поля ручной формы — единый вид с полем VIN (та же высота/фон/радиус) */
.mf-box {
  min-width: 0;
  display: flex;
  align-items: center;
  background: var(--bg);
  border: 1px solid var(--line-2);
  border-radius: 13px;
  padding: 0 14px;
  height: 52px;
  transition: border-color 0.2s, box-shadow 0.2s;
}
.mf-box:focus-within {
  border-color: transparent;
  box-shadow: 0 0 0 2px var(--g1);
}
.mf-box input,
.mf-box select {
  flex: 1;
  min-width: 0;
  border: none;
  background: transparent;
  color: var(--ink);
  font: inherit;
  font-size: 15px;
  padding: 0;
}
.mf-box select {
  cursor: pointer;
}
.mf-box input:focus,
.mf-box select:focus {
  outline: none;
}
.mf-box input::placeholder {
  color: var(--ink-3);
}
.mf-box select option {
  color: initial;
}
/* без спиннеров у числовых полей — ровный вид */
.mf-box input::-webkit-outer-spin-button,
.mf-box input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}
.mf-box input[type='number'] {
  -moz-appearance: textfield;
  appearance: textfield;
}
</style>

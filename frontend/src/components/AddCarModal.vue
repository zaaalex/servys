<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import CarScene from '@/components/CarScene.vue'
import { resolveVin } from '@/api/client'
import { describeError } from '@/api/errors'
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

/* ---- ручной ввод: fallback, когда автоопределение по VIN не сработало ---- */
const manual = ref(false)
const mMake = ref('')
const mModel = ref('')
const mYear = ref<number | null>(null)
const mEngineCc = ref<number | null>(null)
const mPowerHp = ref<number | null>(null)

const selectedBody = ref<ApiBodyType>('sedan')
const selectedColor = ref(0)
const odometer = ref<number | null>(null)
const formError = ref('')

const previewColor = computed<RGB>(() => COLOR_PRESETS[selectedColor.value].rgb)
const previewScene = computed(() => apiBodyToScene(selectedBody.value))
const bodyLabel = computed(() => BODY_TYPES.find((t) => t.id === selectedBody.value)?.name ?? '')
/** Пробег вводим, когда есть что добавлять: распознали по VIN или заполняем вручную. */
const showOdo = computed(() => resolved.value !== null || manual.value)

const scene = ref<InstanceType<typeof CarScene> | null>(null)
const vinInput = ref<HTMLInputElement | null>(null)
const odoInput = ref<HTMLInputElement | null>(null)
const makeInput = ref<HTMLInputElement | null>(null)

onMounted(() => vinInput.value?.focus())

function openManual(focus = true): void {
  manual.value = true
  resolved.value = null
  vinOk.value = false
  formError.value = ''
  if (focus) void nextTick(() => makeInput.value?.focus())
}

async function runLookup(): Promise<void> {
  if (looking.value) return
  looking.value = true
  vinMessage.value = 'Ищем по VIN…'
  vinOk.value = false
  try {
    const r = await resolveVin(vin.value)
    if (!r.signature.make || !r.signature.model) {
      // частичное распознавание (напр., европейский VIN: марка+год без модели) —
      // префиллим ручную форму тем, что нашли, остальное дополнит пользователь
      openManual()
      mMake.value = r.signature.make
      mModel.value = r.signature.model
      mYear.value = r.signature.year || null
      mEngineCc.value = r.signature.engineDisplacementCc || null
      mPowerHp.value = r.signature.powerHp || null
      vinMessage.value = `Определили частично: ${[r.signature.make, r.signature.year].filter(Boolean).join(' · ')} — дополните остальное.`
      return
    }
    resolved.value = r
    manual.value = false // распознали — ручная форма не нужна
    selectedBody.value = r.signature.bodyType
    vinOk.value = true
    vinMessage.value = `✓ ${r.signature.make} ${r.signature.model} · ${r.signature.year} · ${bodyLabel.value}`
    scene.value?.flourish()
    void nextTick(() => odoInput.value?.focus())
  } catch (e) {
    resolved.value = null
    vinOk.value = false
    vinMessage.value = describeError(e).message
    // автоопределение недоступно — не тупик: сразу открываем ручной ввод (данные не выдумываем)
    openManual()
  } finally {
    looking.value = false
  }
}

function submit(): void {
  const r = resolved.value

  // 1) распознали по VIN — как раньше
  if (r) {
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
      engineCc: r.signature.engineDisplacementCc ?? undefined,
      powerHp: r.signature.powerHp ?? undefined,
      bodyType: selectedBody.value,
      fuelType: r.signature.fuelType ?? 'gasoline',
      color: COLOR_PRESETS[selectedColor.value].css,
      odometer: Math.round(odometer.value ?? 0),
    })
    return
  }

  // 2) ручной ввод
  if (manual.value) {
    if (!mMake.value.trim() || !mModel.value.trim()) {
      formError.value = 'Укажите марку и модель.'
      return
    }
    const year = mYear.value ?? 0
    if (!Number.isFinite(year) || year < 1950 || year > 2026) {
      formError.value = 'Укажите корректный год (1950–2026).'
      return
    }
    if (!Number.isFinite(odometer.value) || (odometer.value ?? 0) < 0) {
      formError.value = 'Укажите текущий пробег.'
      return
    }
    formError.value = ''
    emit('add', {
      vin: vin.value.trim().toUpperCase() || undefined,
      make: mMake.value.trim(),
      model: mModel.value.trim(),
      year,
      engineCc: mEngineCc.value ?? undefined,
      powerHp: mPowerHp.value ?? undefined,
      bodyType: selectedBody.value,
      fuelType: 'gasoline',
      color: COLOR_PRESETS[selectedColor.value].css,
      odometer: Math.round(odometer.value ?? 0),
    })
    return
  }

  formError.value = 'Найдите машину по VIN или заполните вручную.'
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
          <button v-if="!manual && !resolved" type="button" class="manual-toggle" @click="openManual()">
            Нет VIN? Заполнить вручную
          </button>
        </div>

        <!-- ===== Ручной ввод (fallback / без VIN) ===== -->
        <div v-if="manual && !resolved" class="manual-block">
          <div class="m-grid2">
            <div class="vin-field">
              <label for="mMake">Марка</label>
              <div class="mf-box"><input id="mMake" ref="makeInput" v-model="mMake" type="text" autocomplete="off" placeholder="напр. KIA" /></div>
            </div>
            <div class="vin-field">
              <label for="mModel">Модель</label>
              <div class="mf-box"><input id="mModel" v-model="mModel" type="text" autocomplete="off" placeholder="напр. K3" /></div>
            </div>
          </div>
          <div class="m-grid3">
            <div class="vin-field">
              <label for="mYear">Год</label>
              <div class="mf-box"><input id="mYear" v-model.number="mYear" type="number" min="1950" max="2026" inputmode="numeric" placeholder="2020" /></div>
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

        <div class="odo-field" v-if="showOdo">
          <label for="odo">Текущий пробег</label>
          <div class="odo-box">
            <input id="odo" ref="odoInput" v-model.number="odometer" type="number" min="0" inputmode="numeric" placeholder="0" />
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
        <div class="modal-actions">
          <button type="button" class="btn-ghost" @click="emit('close')">Отмена</button>
          <button type="submit" class="go go-sm">Добавить в гараж</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
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
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}
/* поля ручной формы — единый вид с полем VIN (та же высота/фон/радиус) */
.mf-box {
  min-width: 0;
  display: flex;
  align-items: center;
  background: rgba(255, 255, 255, 0.05);
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
@media (max-width: 460px) {
  .m-grid3 {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>

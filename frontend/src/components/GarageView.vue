<script setup lang="ts">
// Личный гараж: колонка мини-приложения, сцена сверху, список работ ниже.
import { computed, ref, watch } from 'vue'
import BackgroundLayer from '@/components/BackgroundLayer.vue'
import CarScene from '@/components/CarScene.vue'
import GaragePanel from '@/components/GaragePanel.vue'
import AddCarModal from '@/components/AddCarModal.vue'
import RecommendationsView from '@/components/RecommendationsView.vue'
import { useGarage } from '@/composables/useGarage'
import { useFleetAlerts } from '@/composables/useFleetAlerts'
import { apiBodyToScene, hexToRgb, type RGB, type SceneBody } from '@/data/presets'
import type { CreateVehicleRequest } from '@/types/api'

const { vehicles, activeId, activeVehicle, loading, setActive, addVehicle, removeVehicle } = useGarage()
const { urgentIds } = useFleetAlerts()

async function onRemove(id: string): Promise<void> {
  await removeVehicle(id)
}

const fmt = new Intl.NumberFormat('ru-RU')

const heroType = computed<SceneBody>(() => apiBodyToScene(activeVehicle.value?.bodyType ?? 'sedan'))
const heroColor = computed<RGB>(() => hexToRgb(activeVehicle.value?.color ?? '#1fbfb0'))
const isEmpty = computed(() => !loading.value && !activeVehicle.value)
const heroSub = computed(() => {
  if (activeVehicle.value) return `${activeVehicle.value.year} · ${fmt.format(activeVehicle.value.currentOdometer)} км`
  return loading.value ? 'гараж загружается…' : 'добавьте машину, чтобы начать'
})

const modalOpen = ref(false)
const heroScene = ref<InstanceType<typeof CarScene> | null>(null)

async function onAdd(body: CreateVehicleRequest): Promise<void> {
  await addVehicle(body)
  modalOpen.value = false
}

watch(
  () => activeVehicle.value?.id,
  () => heroScene.value?.flourish(),
)
</script>

<template>
  <div class="garage-view" style="display: contents">
    <BackgroundLayer />
    <div id="deck">
      <section class="slide" id="slideHero">
        <div class="hero-grid">
          <GaragePanel
            :cars="vehicles"
            :active-id="activeId"
            :loading="loading"
            :collapsed="false"
            :urgent-ids="urgentIds"
            @select="setActive"
            @add="modalOpen = true"
            @remove="onRemove"
          />

          <div class="stage">
            <div class="stage-ui">
              <div class="nav">
                <span class="mark">serv<span class="g">ys</span></span>
              </div>
            </div>
            <div class="stage-scene">
              <div class="scene-glow" aria-hidden="true"></div>
              <CarScene ref="heroScene" class="hero-canvas" :type="heroType" :color="heroColor" :interactive="true" :silhouette="isEmpty" :dist="5.6" :cam-y="0.12" />
              <div class="car-shadow" aria-hidden="true"></div>
            </div>
            <div class="stage-copy">
              <div class="headline" :key="activeVehicle?.id">
                <span class="eyebrow">твой гараж</span>
                <h1>{{ activeVehicle ? `${activeVehicle.make} ${activeVehicle.model}` : isEmpty ? 'Гараж пуст' : 'servys' }}</h1>
                <p class="hero-sub">{{ heroSub }}</p>
              </div>
              <div class="stage-bottom">
                <button v-if="isEmpty" class="go go-sm" type="button" @click="modalOpen = true">＋ Добавить машину</button>
                <div v-else class="hint2"><span></span> потяни машину, чтобы повращать</div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <AddCarModal v-if="modalOpen" @close="modalOpen = false" @add="onAdd" />

      <section class="slide" id="slideResults">
        <RecommendationsView :car="activeVehicle" />
      </section>
    </div>
  </div>
</template>

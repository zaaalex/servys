<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import CarScene from '@/components/CarScene.vue'
import GaragePanel from '@/components/GaragePanel.vue'
import AddCarModal from '@/components/AddCarModal.vue'
import RecommendationsView from '@/components/RecommendationsView.vue'
import { useGarage } from '@/composables/useGarage'
import { apiBodyToScene, hexToRgb, type RGB, type SceneBody } from '@/data/presets'
import type { CreateVehicleRequest } from '@/types/api'

const { vehicles, activeId, activeVehicle, loading, setActive, addVehicle } = useGarage()

const fmt = new Intl.NumberFormat('ru-RU')
const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches

const heroType = computed<SceneBody>(() => apiBodyToScene(activeVehicle.value?.bodyType ?? 'sedan'))
const heroColor = computed<RGB>(() => hexToRgb(activeVehicle.value?.color ?? '#1fbfb0'))
const heroSub = computed(() =>
  activeVehicle.value
    ? `${activeVehicle.value.year} · ${fmt.format(activeVehicle.value.currentOdometer)} км`
    : 'гараж загружается…',
)

const modalOpen = ref(false)
const slideHero = ref<HTMLElement | null>(null)
const slideResults = ref<HTMLElement | null>(null)
const heroScene = ref<InstanceType<typeof CarScene> | null>(null)

function scrollTo(el: HTMLElement | null): void {
  el?.scrollIntoView({ behavior: reduce ? 'auto' : 'smooth', block: 'start' })
}

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
  <div id="deck">
    <section class="slide" id="slideHero" ref="slideHero">
      <div class="hero-grid">
        <GaragePanel :cars="vehicles" :active-id="activeId" :loading="loading" @select="setActive" @add="modalOpen = true" />

        <div class="stage">
          <div class="scene-glow" aria-hidden="true"></div>
          <CarScene ref="heroScene" class="hero-canvas" :type="heroType" :color="heroColor" :interactive="true" />
          <div class="car-shadow" aria-hidden="true"></div>
          <div class="hero-scrim" aria-hidden="true"></div>

          <div class="stage-ui">
            <div class="nav">
              <span class="mark">serv<span class="g">ys</span></span>
              <span class="tag">preventive care</span>
            </div>
            <div class="headline" :key="activeVehicle?.id">
              <span class="eyebrow">твой гараж</span>
              <h1>{{ activeVehicle ? `${activeVehicle.make} ${activeVehicle.model}` : 'servys' }}</h1>
              <p class="hero-sub">{{ heroSub }}</p>
            </div>
            <div class="stage-bottom">
              <div class="hint2"><span></span> потяни машину, чтобы повращать</div>
            </div>
          </div>
        </div>
      </div>

      <button class="down-arrow" type="button" @click="scrollTo(slideResults)">
        <span class="lbl">Регламент</span>
        <span class="chev">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M6 9l6 6 6-6" />
          </svg>
        </span>
      </button>
    </section>

    <AddCarModal v-if="modalOpen" @close="modalOpen = false" @add="onAdd" />

    <section class="slide" id="slideResults" ref="slideResults">
      <RecommendationsView :car="activeVehicle" @back="scrollTo(slideHero)" />
    </section>
  </div>
</template>

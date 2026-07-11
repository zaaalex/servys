<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { createCarScene, type CarScene, type RGB } from '@/car3d/engine'
import type { SceneBody as BodyType } from '@/data/presets'

const props = withDefaults(
  defineProps<{
    type?: BodyType
    color?: RGB
    interactive?: boolean
    autoSpin?: boolean
    dist?: number
    camY?: number
    silhouette?: boolean
  }>(),
  { type: 'sedan', interactive: false, autoSpin: true, silhouette: false },
)

const canvas = ref<HTMLCanvasElement | null>(null)
let scene: CarScene | null = null

onMounted(() => {
  if (!canvas.value) return
  scene = createCarScene(canvas.value, {
    interactive: props.interactive,
    autoSpin: props.autoSpin,
    dist: props.dist,
    camY: props.camY,
    type: props.type,
    color: props.color,
    silhouette: props.silhouette,
  })
})

onBeforeUnmount(() => {
  scene?.destroy()
  scene = null
})

watch(
  () => props.type,
  (t) => t && scene?.setType(t),
)
watch(
  () => props.color,
  (c) => c && scene?.setColorRGB(c),
  { deep: true },
)
watch(
  () => props.silhouette,
  (s) => scene?.setSilhouette(!!s),
)

defineExpose({
  flourish: () => scene?.flourish(),
  resize: () => scene?.resize(),
})
</script>

<template>
  <canvas ref="canvas" class="car-canvas" aria-label="3D-модель автомобиля"></canvas>
</template>

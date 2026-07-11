<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useBackground } from '@/composables/useBackground'

const { current } = useBackground()
const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches

const canvas = ref<HTMLCanvasElement | null>(null)
let raf = 0
let onResize: (() => void) | null = null

interface Star {
  x: number
  y: number
  r: number
  vy: number
  tw: number
}

function stopStars(): void {
  cancelAnimationFrame(raf)
  raf = 0
  if (onResize) {
    window.removeEventListener('resize', onResize)
    onResize = null
  }
}

function startStars(): void {
  const cv = canvas.value
  if (!cv) return
  const ctx = cv.getContext('2d')
  if (!ctx) return
  const dpr = Math.min(window.devicePixelRatio || 1, 2)

  onResize = () => {
    cv.width = Math.floor(window.innerWidth * dpr)
    cv.height = Math.floor(window.innerHeight * dpr)
  }
  onResize()
  window.addEventListener('resize', onResize)

  const stars: Star[] = Array.from({ length: 90 }, () => ({
    x: Math.random(),
    y: Math.random(),
    r: (Math.random() * 1.3 + 0.3) * dpr,
    vy: Math.random() * 0.0004 + 0.0001,
    tw: Math.random() * Math.PI * 2,
  }))

  const draw = (): void => {
    ctx.clearRect(0, 0, cv.width, cv.height)
    for (const s of stars) {
      if (!reduce) {
        s.y += s.vy
        if (s.y > 1) s.y = 0
        s.tw += 0.02
      }
      const alpha = 0.3 + 0.35 * Math.sin(s.tw)
      ctx.beginPath()
      ctx.arc(s.x * cv.width, s.y * cv.height, s.r, 0, Math.PI * 2)
      ctx.fillStyle = `rgba(180, 210, 255, ${Math.max(0, alpha)})`
      ctx.fill()
    }
    if (!reduce) raf = requestAnimationFrame(draw)
  }
  draw()
}

function sync(): void {
  stopStars()
  if (current.value === 'stars') void nextTick(startStars)
}

watch(current, sync)
onMounted(sync)
onBeforeUnmount(stopStars)
</script>

<template>
  <div class="bg-layer" :class="`bg-${current}`" aria-hidden="true">
    <template v-if="current === 'aurora'">
      <span class="ab ab1"></span>
      <span class="ab ab2"></span>
      <span class="ab ab3"></span>
    </template>
    <canvas v-else-if="current === 'stars'" ref="canvas" class="bg-canvas"></canvas>
  </div>
</template>

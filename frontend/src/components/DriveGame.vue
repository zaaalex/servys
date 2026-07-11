<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

const emit = defineEmits<{ distance: [km: number] }>()

const canvas = ref<HTMLCanvasElement | null>(null)
const running = ref(false)
const km = ref(0)

let ctx: CanvasRenderingContext2D | null = null
let raf = 0
let last = 0
let W = 0
let H = 0
let dpr = 1

let roadOffset = 0
let carX = 0.5 // целевая/текущая позиция в полосе 0..1
let targetX = 0.5
let speed = 0 // км/с
let hitFlash = 0
let spawnT = 0

interface Ob {
  x: number
  y: number
  hit: boolean
}
let obs: Ob[] = []

const BASE = 44 // км/с базовая
const PXPK = 0.9 // px прокрутки на км

function resize(): void {
  const cv = canvas.value
  if (!cv || !ctx) return
  dpr = Math.min(window.devicePixelRatio || 1, 2)
  W = cv.clientWidth
  H = cv.clientHeight
  cv.width = Math.floor(W * dpr)
  cv.height = Math.floor(H * dpr)
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  draw()
}

function laneX(x: number): number {
  const m = W * 0.16
  return m + x * (W - 2 * m)
}

function roundRect(c: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number): void {
  c.beginPath()
  c.moveTo(x + r, y)
  c.arcTo(x + w, y, x + w, y + h, r)
  c.arcTo(x + w, y + h, x, y + h, r)
  c.arcTo(x, y + h, x, y, r)
  c.arcTo(x, y, x + w, y, r)
  c.closePath()
}

function draw(): void {
  const c = ctx
  if (!c) return
  c.clearRect(0, 0, W, H)
  const rl = W * 0.16
  const rr = W - W * 0.16
  c.fillStyle = '#12151d'
  c.fillRect(rl - 10, 0, rr - rl + 20, H)
  c.strokeStyle = 'rgba(255,255,255,.14)'
  c.lineWidth = 2
  c.beginPath()
  c.moveTo(rl - 8, 0)
  c.lineTo(rl - 8, H)
  c.moveTo(rr + 8, 0)
  c.lineTo(rr + 8, H)
  c.stroke()
  c.strokeStyle = 'rgba(120,180,255,.35)'
  c.lineWidth = 3
  c.setLineDash([16, 22])
  c.lineDashOffset = -roadOffset
  c.beginPath()
  c.moveTo((rl + rr) / 2, 0)
  c.lineTo((rl + rr) / 2, H)
  c.stroke()
  c.setLineDash([])

  for (const o of obs) {
    c.fillStyle = o.hit ? 'rgba(255,120,90,.45)' : '#ff8a3d'
    roundRect(c, laneX(o.x) - 12, o.y - 16, 24, 32, 6)
    c.fill()
  }

  const cx = laneX(carX)
  const cy = H - 46
  c.save()
  if (hitFlash > 0 && Math.floor(hitFlash * 20) % 2 === 0) c.globalAlpha = 0.5
  c.fillStyle = '#1fbfb0'
  roundRect(c, cx - 15, cy - 22, 30, 44, 8)
  c.fill()
  c.fillStyle = 'rgba(255,255,255,.22)'
  roundRect(c, cx - 11, cy - 12, 22, 14, 4)
  c.fill()
  c.restore()
}

function loop(ts: number): void {
  const dt = last ? Math.min(0.05, (ts - last) / 1000) : 0
  last = ts

  const target = hitFlash > 0 ? BASE * 0.35 : BASE
  speed += (target - speed) * Math.min(1, dt * 4)
  if (hitFlash > 0) hitFlash -= dt
  km.value += speed * dt
  roadOffset = (roadOffset + speed * dt * PXPK) % 40

  carX += (targetX - carX) * Math.min(1, dt * 10)
  carX = Math.max(0, Math.min(1, carX))

  spawnT -= dt
  if (spawnT <= 0) {
    spawnT = 0.5 + Math.random() * 0.6
    obs.push({ x: 0.1 + Math.random() * 0.8, y: -30, hit: false })
  }
  const carY = H - 46
  for (const o of obs) {
    o.y += speed * dt * PXPK
    if (!o.hit && Math.abs(laneX(o.x) - laneX(carX)) < 30 && Math.abs(o.y - carY) < 30) {
      o.hit = true
      hitFlash = 0.5
    }
  }
  obs = obs.filter((o) => o.y < H + 40)

  draw()
  emit('distance', Math.round(km.value))
  if (running.value) raf = requestAnimationFrame(loop)
}

function start(): void {
  if (running.value) return
  running.value = true
  last = 0
  raf = requestAnimationFrame(loop)
}
function stop(): void {
  running.value = false
  cancelAnimationFrame(raf)
}
function toggle(): void {
  running.value ? stop() : start()
}
function reset(): void {
  km.value = 0
  carX = 0.5
  targetX = 0.5
  speed = 0
  obs = []
  roadOffset = 0
  hitFlash = 0
  spawnT = 0
  emit('distance', 0)
  draw()
}

function steer(e: PointerEvent): void {
  const cv = canvas.value
  if (!cv) return
  const rect = cv.getBoundingClientRect()
  const rl = W * 0.16
  const rr = W - W * 0.16
  targetX = Math.max(0, Math.min(1, (e.clientX - rect.left - rl) / (rr - rl)))
}
function onResize(): void {
  resize()
}

onMounted(() => {
  const cv = canvas.value
  if (!cv) return
  ctx = cv.getContext('2d')
  resize()
  window.addEventListener('resize', onResize)
  cv.addEventListener('pointermove', steer)
  cv.addEventListener('pointerdown', steer)
  start()
})

onBeforeUnmount(() => {
  stop()
  window.removeEventListener('resize', onResize)
  canvas.value?.removeEventListener('pointermove', steer)
  canvas.value?.removeEventListener('pointerdown', steer)
})

defineExpose({ reset })
</script>

<template>
  <div class="drive-game">
    <canvas ref="canvas" class="drive-canvas"></canvas>
    <div class="drive-hud">
      <span class="drive-km">{{ Math.round(km) }} км</span>
      <div class="drive-ctrls">
        <button type="button" class="drive-btn" @click="toggle">{{ running ? 'Пауза' : 'Старт' }}</button>
        <button type="button" class="drive-btn ghost" @click="reset">Сброс</button>
      </div>
    </div>
    <p class="drive-hint">Веди пальцем или мышью по дороге, чтобы объезжать препятствия</p>
  </div>
</template>

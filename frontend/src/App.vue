<script setup lang="ts">
// Оболочка приложения: гейт авторизации + переключатель разделов (b2c «Гараж» / b2b «Кабинет СТО»).
// Раздел синхронизирован с location.hash (#/b2b) — линкуемо и переживает перезагрузку, без роутера.
// b2c-дек (UI Карины) рендерится через v-show (сохраняет 3D-сцену); при входе в b2b активируем
// контекст (/auth/switch). По умолчанию приземляемся на активный контекст из /auth/me.
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useAuth } from '@/composables/useAuth'
import AuthView from '@/components/AuthView.vue'
import AppBar from '@/components/AppBar.vue'
import GarageView from '@/components/GarageView.vue'
import ServiceCentersPanel from '@/components/ServiceCentersPanel.vue'

type Section = 'b2c' | 'b2b'

const { status, isAuthed, activeCtxType, hasB2B, init, switchTo, reloadMe } = useAuth()

function sectionFromHash(): Section {
  return window.location.hash.replace(/^#\/?/, '') === 'b2b' ? 'b2b' : 'b2c'
}

const section = ref<Section>(sectionFromHash())

function applySection(s: Section): void {
  section.value = s
  const target = s === 'b2b' ? '#/b2b' : '#/'
  if (window.location.hash !== target) window.location.hash = target
  // если у аккаунта есть соответствующий контекст — активируем его на сервере
  if (s === 'b2b' && hasB2B.value) void switchTo('b2b')
  if (s === 'b2c') void switchTo('b2c')
}

function onHashChange(): void {
  applySection(sectionFromHash())
}

onMounted(async () => {
  window.addEventListener('hashchange', onHashChange)
  await init()
  // после входа приземляемся на активный контекст, если раздел не задан явно в hash
  if (window.location.hash.replace(/^#\/?/, '') === '') {
    section.value = activeCtxType.value === 'b2b' ? 'b2b' : 'b2c'
  }
})
onBeforeUnmount(() => window.removeEventListener('hashchange', onHashChange))

// при разлогине возвращаемся в b2c-раздел
watch(isAuthed, (v) => {
  if (!v) section.value = 'b2c'
})

// после подключения первого СТО у аккаунта появляется b2b-контекст — перечитаем /auth/me и активируем его
async function onConnected(): Promise<void> {
  await reloadMe()
  if (hasB2B.value) void switchTo('b2b')
}
</script>

<template>
  <div v-if="status === 'loading'" class="auth-splash">
    <span class="mark">serv<span class="g">ys</span></span>
    <span class="spin auth-splash-spin" aria-hidden="true"></span>
  </div>

  <AuthView v-else-if="status === 'anonymous'" />

  <template v-else>
    <AppBar :section="section" @select="applySection" />
    <GarageView v-show="section === 'b2c'" />
    <ServiceCentersPanel v-if="section === 'b2b'" @connected="onConnected" />
  </template>
</template>

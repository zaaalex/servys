<script setup lang="ts">
// Оболочка: гейт авторизации, затем три раздела мини-приложения.
import { onMounted, ref } from 'vue'
import { useAuth } from '@/composables/useAuth'
import AuthView from '@/components/AuthView.vue'
import GarageView from '@/components/GarageView.vue'
import ProfileView from '@/components/ProfileView.vue'
import ServiceBookView from '@/components/ServiceBookView.vue'

const { status, init } = useAuth()
const tab = ref<'profile' | 'garage' | 'book'>('garage')

onMounted(() => {
  void init()
})
</script>

<template>
  <div class="phone">
    <div v-if="status === 'loading'" class="auth-splash">
      <span class="mark">serv<span class="g">ys</span></span>
      <span class="spin auth-splash-spin" aria-hidden="true"></span>
    </div>

    <AuthView v-else-if="status === 'anonymous'" />

    <template v-else>
      <GarageView v-show="tab === 'garage'" />
      <ProfileView v-if="tab === 'profile'" />
      <ServiceBookView v-if="tab === 'book'" />
      <nav class="tabbar" aria-label="Разделы">
        <button type="button" :aria-current="tab === 'garage' ? 'page' : undefined" @click="tab = 'garage'">Гараж</button>
        <button type="button" :aria-current="tab === 'book' ? 'page' : undefined" @click="tab = 'book'">Книжка</button>
        <button type="button" :aria-current="tab === 'profile' ? 'page' : undefined" @click="tab = 'profile'">Профиль</button>
      </nav>
    </template>
  </div>
</template>

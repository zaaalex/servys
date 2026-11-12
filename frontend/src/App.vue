<script setup lang="ts">
// Оболочка: гейт авторизации, затем три раздела мини-приложения.
import { onMounted, ref } from 'vue'
import { useAuth } from '@/composables/useAuth'
import AuthView from '@/components/AuthView.vue'
import GarageView from '@/components/GarageView.vue'
import ProfileView from '@/components/ProfileView.vue'
import ServiceBookView from '@/components/ServiceBookView.vue'
import AdminView from '@/components/AdminView.vue'

const { status, init, isAdmin } = useAuth()
const tab = ref<'profile' | 'garage' | 'book'>('garage')
const adminOpen = ref(false)

function openAdmin(): void {
  adminOpen.value = true
}

function closeAdmin(): void {
  adminOpen.value = false
  tab.value = 'garage'
}

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
      <AdminView v-if="adminOpen && isAdmin" @close="closeAdmin" />
      <template v-else>
        <GarageView v-show="tab === 'garage'" />
        <ProfileView v-if="tab === 'profile'" />
        <ServiceBookView v-if="tab === 'book'" />
        <nav class="tabbar" aria-label="Разделы">
          <button type="button" :aria-current="tab === 'garage' ? 'page' : undefined" @click="tab = 'garage'">Гараж</button>
          <button type="button" :aria-current="tab === 'book' ? 'page' : undefined" @click="tab = 'book'">Книжка</button>
          <button type="button" :aria-current="tab === 'profile' ? 'page' : undefined" @click="tab = 'profile'">Профиль</button>
          <button v-if="isAdmin" type="button" class="is-tool" aria-label="Админка" title="Админка" @click="openAdmin">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M12 3.5 5.5 6v5.25c0 4.05 2.67 7.88 6.5 9.25 3.83-1.37 6.5-5.2 6.5-9.25V6L12 3.5Z" />
              <path d="m9.75 12 1.55 1.55 3.45-3.7" />
            </svg>
          </button>
        </nav>
      </template>
    </template>
  </div>
</template>

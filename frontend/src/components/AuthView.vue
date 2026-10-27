<script setup lang="ts">
// Вход только из Telegram или MAX. В браузере гаража нет — как у trainys.
import { useAuth } from '@/composables/useAuth'

const { busy, error, retry } = useAuth()
</script>

<template>
  <div class="auth-view">
    <div class="auth-card">
      <span class="mark auth-mark">serv<span class="g">ys</span></span>
      <span class="eyebrow">гараж</span>
      <h1 class="auth-title">Откройте из Telegram или MAX</h1>
      <p class="auth-sub">Гараж открывается из бота. Напишите /start и нажмите «Открыть гараж». В браузере войти нельзя.</p>

      <div v-if="error" class="banner-err" role="alert">
        <span>{{ error.message }}</span>
      </div>

      <button v-if="error" class="go" type="button" :disabled="busy" @click="retry">
        <span v-if="busy" class="spin" aria-hidden="true"></span>
        {{ busy ? 'Секунду…' : 'Повторить' }}
      </button>
    </div>
  </div>
</template>

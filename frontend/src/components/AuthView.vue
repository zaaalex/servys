<script setup lang="ts">
// Экран входа/регистрации по email+паролю. Telegram — опциональный слот (кнопка);
// при отсутствии окружения мини-аппа бэк вернёт 503 TELEGRAM_DISABLED, показываем текст.
import { computed, reactive, ref } from 'vue'
import { useAuth } from '@/composables/useAuth'

const { busy, error, login, register, loginTelegram } = useAuth()

type Mode = 'login' | 'register'
const mode = ref<Mode>('login')

const email = ref('')
const password = ref('')
const local = reactive({ email: '', password: '' })

const title = computed(() => (mode.value === 'login' ? 'Вход' : 'Регистрация'))
const submitLabel = computed(() => (mode.value === 'login' ? 'Войти' : 'Создать аккаунт'))

function validEmail(v: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v)
}

async function onSubmit(): Promise<void> {
  local.email = !email.value.trim() ? 'Укажите email' : validEmail(email.value.trim()) ? '' : 'Некорректный email'
  local.password = password.value.length >= 6 ? '' : 'Минимум 6 символов'
  if (local.email || local.password) return
  const fn = mode.value === 'login' ? login : register
  await fn(email.value.trim(), password.value)
}

function setMode(m: Mode): void {
  mode.value = m
}
</script>

<template>
  <div class="auth-view">
    <div class="auth-card">
      <span class="mark auth-mark">serv<span class="g">ys</span></span>
      <span class="eyebrow">кабинет</span>
      <h1 class="auth-title">{{ title }}</h1>
      <p class="auth-sub">Вход в платформу — гараж (b2c) и Кабинет СТО (b2b).</p>

      <div class="auth-tabs" role="tablist">
        <button type="button" role="tab" :aria-selected="mode === 'login'" :class="{ on: mode === 'login' }" @click="setMode('login')">
          Вход
        </button>
        <button type="button" role="tab" :aria-selected="mode === 'register'" :class="{ on: mode === 'register' }" @click="setMode('register')">
          Регистрация
        </button>
      </div>

      <form novalidate @submit.prevent="onSubmit">
        <div class="field">
          <label for="authEmail">Email</label>
          <div class="box"><input id="authEmail" v-model="email" type="email" autocomplete="username" placeholder="you@servys.app" /></div>
          <div v-if="local.email" class="err">{{ local.email }}</div>
        </div>

        <div class="field">
          <label for="authPass">Пароль</label>
          <div class="box">
            <input
              id="authPass"
              v-model="password"
              type="password"
              :autocomplete="mode === 'login' ? 'current-password' : 'new-password'"
              placeholder="•••••••"
            />
          </div>
          <div v-if="local.password" class="err">{{ local.password }}</div>
        </div>

        <div v-if="error" class="b2b-banner-err" role="alert">
          <span>{{ error.message }}</span>
          <span v-if="error.code" class="b2b-code">{{ error.code }}</span>
        </div>

        <button class="go" type="submit" :disabled="busy">
          <span v-if="busy" class="spin" aria-hidden="true"></span>
          {{ busy ? 'Секунду…' : submitLabel }}
        </button>
      </form>

      <div class="auth-divider"><span>или</span></div>
      <button class="btn-ghost auth-tg" type="button" :disabled="busy" @click="loginTelegram">
        Войти через Telegram
      </button>
    </div>
  </div>
</template>

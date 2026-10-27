<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAuth } from '@/composables/useAuth'
import { closeToChat } from '@/platform/closeChat'

const { place } = useAuth()
const supportOpen = ref(false)
const chatError = ref('')

const lead = computed(() => {
  if (place.value === 'telegram') return 'Вход из Telegram'
  if (place.value === 'max') return 'Вход из MAX'
  if (place.value === 'dev') return 'Локальный просмотр'
  return 'Вход из Telegram или MAX'
})

function openSupport(): void {
  supportOpen.value = true
}

function back(): void {
  supportOpen.value = false
  chatError.value = ''
}

function returnToChat(): void {
  if (closeToChat()) {
    chatError.value = ''
    return
  }
  chatError.value = 'Откройте приложение из Telegram или MAX и напишите боту.'
}
</script>

<template>
  <section class="screen">
    <header class="ticket-intro">
      <p class="screen-kicker">Профиль</p>
      <h1>Кабинет</h1>
      <p class="screen-lead">{{ lead }}</p>
    </header>

    <button v-if="!supportOpen" type="button" class="sheet-card" @click="openSupport">
      <span class="sheet-kicker">Поддержка</span>
      <span class="sheet-title">Обратная связь</span>
      <span class="sheet-note">Вопрос или идея. Ответ придёт в чат.</span>
    </button>

    <div v-else class="sheet-card">
      <button type="button" class="text-back" @click="back">Назад</button>
      <span class="sheet-kicker">Поддержка</span>
      <span class="sheet-title">Напишите боту</span>
      <span class="sheet-note">Ответ придёт в чат, из которого открыто приложение. Сообщение отсюда не отправляется.</span>
      <button type="button" class="go" @click="returnToChat">Вернуться в чат</button>
      <p v-if="chatError" class="err">{{ chatError }}</p>
    </div>
  </section>
</template>

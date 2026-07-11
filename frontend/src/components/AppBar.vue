<script setup lang="ts">
// Верхняя панель авторизованного приложения: переключатель разделов (Гараж / Кабинет СТО)
// и блок аккаунта с выходом. Раздел b2b доступен всегда (чтобы подключить первый СТО);
// если у аккаунта есть b2b-контекст — переключение активирует его через /auth/switch.
import { computed } from 'vue'
import { useAuth } from '@/composables/useAuth'

type Section = 'b2c' | 'b2b'

const props = defineProps<{ section: Section }>()
const emit = defineEmits<{ select: [section: Section] }>()

const { accountId, hasB2B, busy, logout } = useAuth()

const acc = computed(() => (accountId.value ? accountId.value.slice(0, 10) : 'аккаунт'))

function select(s: Section): void {
  if (s !== props.section) emit('select', s)
}
</script>

<template>
  <nav class="app-switch" role="tablist" aria-label="Разделы servys">
    <button
      type="button"
      role="tab"
      :aria-selected="section === 'b2c'"
      :class="{ on: section === 'b2c' }"
      @click="select('b2c')"
    >
      Гараж
    </button>
    <button
      type="button"
      role="tab"
      :aria-selected="section === 'b2b'"
      :class="{ on: section === 'b2b' }"
      @click="select('b2b')"
    >
      Кабинет СТО<span v-if="!hasB2B" class="app-switch-dot" title="СТО ещё не подключено" aria-hidden="true">•</span>
    </button>
  </nav>

  <div class="app-account">
    <span class="app-acc-id" :title="accountId">{{ acc }}</span>
    <button class="app-logout" type="button" :disabled="busy" @click="logout">Выйти</button>
  </div>
</template>

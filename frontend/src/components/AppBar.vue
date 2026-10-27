<script setup lang="ts">
import { useAuth } from '@/composables/useAuth'

const { busy, logout } = useAuth()
</script>

<template>
  <div class="app-account">
    <button class="app-logout" type="button" :disabled="busy" @click="logout">Выйти</button>
  </div>
</template>

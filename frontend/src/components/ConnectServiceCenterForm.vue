<script setup lang="ts">
// Форма подключения СТО: название, webhook-URL, responsible_id.
// Валидация — на клиенте (до сети); серверные ошибки (INVALID_WEBHOOK, B2B_DISABLED и т.п.)
// приходят пропом `error` и показываются баннером. Кнопка блокируется на время запроса.
import { reactive, ref } from 'vue'
import type { UiError } from '@/api/errors'
import type { ConnectServiceCenterRequest } from '@/types/b2b'

const props = defineProps<{
  connecting: boolean
  error: UiError | null
}>()

const emit = defineEmits<{ submit: [req: ConnectServiceCenterRequest] }>()

const name = ref('')
const webhook = ref('')
// v-model на <input type="number"> отдаёт number (или '' когда пусто) — держим оба варианта.
const responsibleId = ref<number | string>('')

const errors = reactive<{ name: string; webhook: string; responsibleId: string }>({
  name: '',
  webhook: '',
  responsibleId: '',
})

function isUrl(v: string): boolean {
  try {
    const u = new URL(v)
    return u.protocol === 'https:' || u.protocol === 'http:'
  } catch {
    return false
  }
}

function validate(): ConnectServiceCenterRequest | null {
  errors.name = name.value.trim() ? '' : 'Укажите название СТО'
  errors.webhook = !webhook.value.trim()
    ? 'Укажите webhook-URL'
    : isUrl(webhook.value.trim())
      ? ''
      : 'Похоже, это не URL (ожидается https://…/rest/…)'
  const raw = responsibleId.value
  const isEmpty = raw === '' || (typeof raw === 'string' && raw.trim() === '')
  const rid = typeof raw === 'number' ? raw : Number(String(raw).trim())
  errors.responsibleId = isEmpty
    ? 'Укажите ID ответственного'
    : Number.isInteger(rid) && rid > 0
      ? ''
      : 'Целое число больше нуля'

  if (errors.name || errors.webhook || errors.responsibleId) return null
  return {
    name: name.value.trim(),
    webhook: webhook.value.trim(),
    responsible_id: rid,
  }
}

function onSubmit(): void {
  if (props.connecting) return // защита от двойного сабмита
  const req = validate()
  if (req) emit('submit', req)
}
</script>

<template>
  <form class="b2b-card b2b-connect" novalidate @submit.prevent="onSubmit">
    <h2 class="b2b-card-title">Подключить СТО</h2>
    <p class="b2b-card-sub">
      СТО отдаёт входящий вебхук Bitrix24 — servys читает автопарк и создаёт ретеншн-дела.
    </p>

    <div class="b2b-fields">
      <div class="field">
        <label for="scName">Название</label>
        <div class="box"><input id="scName" v-model="name" type="text" placeholder="АвтоТехЦентр «Магистраль»" /></div>
        <div v-if="errors.name" class="err">{{ errors.name }}</div>
      </div>

      <div class="field">
        <label for="scWebhook">Входящий webhook Bitrix24</label>
        <div class="box">
          <input
            id="scWebhook"
            v-model="webhook"
            type="url"
            autocomplete="off"
            spellcheck="false"
            placeholder="https://portal.bitrix24.ru/rest/1/xxxxxxxxxxxx/"
          />
        </div>
        <div v-if="errors.webhook" class="err">{{ errors.webhook }}</div>
      </div>

      <div class="field b2b-field-narrow num">
        <label for="scResp">ID ответственного</label>
        <div class="box"><input id="scResp" v-model="responsibleId" type="number" min="1" step="1" placeholder="12" /></div>
        <div v-if="errors.responsibleId" class="err">{{ errors.responsibleId }}</div>
      </div>
    </div>

    <div v-if="error" class="b2b-banner-err" role="alert">
      <strong>Не удалось подключить.</strong> {{ error.message }}
      <span v-if="error.code" class="b2b-code">{{ error.code }}</span>
    </div>

    <button class="go" type="submit" :disabled="connecting">
      <span v-if="connecting" class="spin" aria-hidden="true"></span>
      {{ connecting ? 'Подключаю…' : 'Подключить СТО' }}
    </button>
  </form>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { addServiceEvent, submitHistory, submitHistoryFile, type HistorySuggestion } from '@/api/client'
import { describeError } from '@/api/errors'

const props = defineProps<{ vehicleId: string }>()
const emit = defineEmits<{ saved: [] }>()

const text = ref('')
const busy = ref(false)
const message = ref('')
const bad = ref(false)
const suggestions = ref<HistorySuggestion[]>([])
const picked = ref<boolean[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
const fileName = ref('')

function onFile(): void {
  fileName.value = fileInput.value?.files?.[0]?.name ?? ''
}

async function read(): Promise<void> {
  const file = fileInput.value?.files?.[0]
  if (!text.value.trim() && !file) {
    bad.value = true
    message.value = 'Вставьте строки с датой, работой и пробегом или выберите txt.'
    return
  }
  busy.value = true
  bad.value = false
  message.value = ''
  try {
    const fromText = text.value.trim() ? await submitHistory(props.vehicleId, text.value.trim()) : []
    const fromFile = file ? await submitHistoryFile(props.vehicleId, file) : { suggestions: [], notice: '' }
    suggestions.value = [...fromText, ...fromFile.suggestions]
    picked.value = suggestions.value.map(() => true)
    if (fromFile.notice) message.value = fromFile.notice
    else if (suggestions.value.length) message.value = 'Отметьте, что записать в книжку.'
    else message.value = 'В тексте не нашлось работы с пробегом. Запишите её формой выше.'
  } catch (e) {
    bad.value = true
    message.value = describeError(e).message
  } finally {
    busy.value = false
  }
}

async function commit(): Promise<void> {
  const chosen = suggestions.value.filter((_, i) => picked.value[i])
  if (!chosen.length) return
  busy.value = true
  bad.value = false
  try {
    for (const item of chosen) {
      await addServiceEvent(props.vehicleId, {
        ruleCode: item.ruleCode,
        odometer: item.odometer,
        date: item.performedAt ?? new Date().toISOString().slice(0, 10),
      })
    }
    suggestions.value = []
    picked.value = []
    message.value = 'Записано в книжку.'
    emit('saved')
  } catch (e) {
    bad.value = true
    message.value = describeError(e).message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="hist">
    <h3>Старые записи текстом</h3>
    <p>Если книжка уже написана, вставьте строки или приложите фото заказ-наряда. Сервис предложит отметить найденные работы. Снимок и поиск регламента читаются по очереди, не одновременно.</p>
    <textarea id="history-text" v-model="text" rows="3" placeholder="12.03.2024 замена масла, 45 000 км"></textarea>
    <label class="file-btn">
      <input ref="fileInput" type="file" accept=".txt,.pdf,image/jpeg,image/png,image/webp" @change="onFile" />
      <span>{{ fileName || 'Или файл txt' }}</span>
    </label>
    <button class="btn-ghost hist-find" type="button" :disabled="busy" @click="read">{{ busy ? 'Секунду…' : 'Найти работы' }}</button>
    <p v-if="message" class="msg" :class="{ bad }">{{ message }}</p>
    <ul v-if="suggestions.length" class="list">
      <li v-for="(item, i) in suggestions" :key="item.ruleCode + item.odometer + i">
        <label>
          <input v-model="picked[i]" type="checkbox" />
          {{ item.title }} · {{ item.odometer.toLocaleString('ru-RU') }} км
          <span v-if="item.performedAt"> · {{ item.performedAt }}</span>
        </label>
      </li>
    </ul>
    <button v-if="suggestions.length" class="go" type="button" :disabled="busy" @click="commit">Записать отмеченное</button>
  </section>
</template>

<style scoped>
.hist {
  margin: 0 0 22px;
  padding: 16px;
  border: 1px solid var(--line);
  border-radius: 16px;
  background: var(--bg-2);
}
.hist h3 { margin: 0 0 6px; font-size: 16px; }
.hist p { margin: 0 0 12px; color: var(--ink-2); font-size: 13.5px; }
textarea {
  width: 100%;
  min-height: 4.8em;
  max-height: 180px;
  color: var(--ink);
  background: var(--bg);
  border: 1px solid var(--line);
  border-radius: 12px;
  font: inherit;
  font-size: 14.5px;
  padding: 10px 12px;
  resize: vertical;
}
textarea:focus { outline: none; border-color: transparent; box-shadow: 0 0 0 2px var(--g1); }
.file-btn {
  display: flex;
  align-items: center;
  margin-top: 10px;
  min-height: 48px;
  padding: 0 14px;
  border-radius: 12px;
  border: 1px solid var(--line-2);
  background: var(--bg);
  color: var(--ink-2);
  cursor: pointer;
}
.file-btn span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.file-btn input {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  border: 0;
}
.file-btn:focus-within { box-shadow: 0 0 0 2px var(--g1); }
.hist .go, .hist-find { width: 100%; margin-top: 10px; }
.hist-find:disabled { opacity: .55; cursor: default; }
.msg { margin-top: 10px; }
.msg.bad { color: var(--crit); }
.list { list-style: none; margin: 12px 0; padding: 0; display: grid; gap: 8px; }
.list label { display: flex; gap: 8px; align-items: baseline; }
</style>

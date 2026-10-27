// Счётчик записей в книжке. Гараж подписан на него и тихо пересчитывает сроки.
import { ref } from 'vue'

const stamp = ref(0)

export function touchBook(): void {
  stamp.value += 1
}

export function useBookStamp() {
  return stamp
}

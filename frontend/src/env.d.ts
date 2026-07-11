/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>
  export default component
}

interface ImportMetaEnv {
  /** Базовый URL Go-API. Пусто → тот же origin (dev-proxy /api). */
  readonly VITE_API_BASE_URL?: string
  /** '1' → отдаём мок без сети, пока живой API не поднят. */
  readonly VITE_USE_MOCK?: string
  /** Куда dev-proxy шлёт /api (по умолчанию http://localhost:8080). */
  readonly VITE_API_TARGET?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

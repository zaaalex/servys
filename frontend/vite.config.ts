import { fileURLToPath, URL } from 'node:url'
import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

// Standalone Vue-фронт servys. Ходит в Go-API по контракту A.
// Dev-proxy /api → Go-бэк, чтобы локально не ловить CORS (см. спеку §5).
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  return {
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      // mock/recommendations.json вне src/ — общий якорь контракта A с бэком
      '@mock': fileURLToPath(new URL('./mock', import.meta.url)),
    },
  },
  server: {
    proxy: {
      '/api': {
        target: env.VITE_API_TARGET || 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  }
})

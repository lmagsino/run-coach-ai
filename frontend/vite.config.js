import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  // /api is proxied to the Go server so the dev UI shares its origin: the
  // session cookie just works and the backend needs no CORS (ADR 0007).
  server: { port: 5173, proxy: { '/api': 'http://localhost:8080' } },
  // e2e/ is driven by `npm run e2e` against a running app, not by vitest.
  test: { include: ['src/**/*.{test,spec}.js'] },
})

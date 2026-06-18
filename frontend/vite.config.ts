import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // Dev-only proxy: the browser calls /api/* same-origin and Vite forwards to
    // the Go backend on :8080. This sidesteps CORS entirely (the backend sets no
    // CORS headers — see PLAN.md), so no preflight, no Access-Control-* needed.
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})

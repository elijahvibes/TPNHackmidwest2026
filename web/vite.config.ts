import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      // In dev the Go API runs on :8080; in prod the Go binary serves this build itself.
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})

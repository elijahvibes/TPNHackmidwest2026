import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'

// API_PORT lets a second dev instance proxy to its own Go server (see .claude/launch.json).
const apiPort = process.env.API_PORT || '8080'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      // In dev the Go API runs on :8080; in prod the Go binary serves this build itself.
      '/api': { target: `http://localhost:${apiPort}`, changeOrigin: true },
    },
  },
})

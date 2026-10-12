import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// The api-gateway has no CORS, so the dev server proxies /api to the kind
// ingress (localhost:80) and the browser only ever talks to one origin.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.API_TARGET ?? 'http://localhost',
        changeOrigin: true,
      },
    },
  },
})

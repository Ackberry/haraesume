import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  envDir: '..',
  plugins: [react(), tailwindcss()],
  server: {
    host: true,          // binds to 0.0.0.0
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': { target: 'http://backend:3001', changeOrigin: true },
      '/health': { target: 'http://backend:3001', changeOrigin: true },
    },
  },
})

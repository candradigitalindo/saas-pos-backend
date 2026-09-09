import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const akar = path.dirname(fileURLToPath(import.meta.url))

// Server backend berjalan di :8080. Dev server mem-proxy /api supaya frontend
// dan API berbagi origin — tidak perlu CORS saat pengembangan.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': path.resolve(akar, './src') },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.VITE_API_PROXY ?? 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})

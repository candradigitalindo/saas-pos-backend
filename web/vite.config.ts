import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const akar = path.dirname(fileURLToPath(import.meta.url))

// Server backend berjalan di :8080. Dev server mem-proxy /api supaya frontend
// dan API berbagi origin — tidak perlu CORS saat pengembangan.
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.svg'],
      manifest: {
        name: 'Kasir UMKM',
        short_name: 'Kasir',
        description: 'Aplikasi kasir dan catatan usaha yang tetap jalan tanpa internet.',
        lang: 'id',
        start_url: '/',
        display: 'standalone',
        background_color: '#fafaf9',
        theme_color: '#047857',
        icons: [
          { src: 'ikon-192.png', sizes: '192x192', type: 'image/png' },
          { src: 'ikon-512.png', sizes: '512x512', type: 'image/png' },
          {
            src: 'ikon-512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable',
          },
        ],
      },
      workbox: {
        // Kerangka aplikasi di-cache supaya kasir tetap terbuka tanpa internet.
        globPatterns: ['**/*.{js,css,html,woff2}'],
        navigateFallback: 'index.html',
        // Permintaan API TIDAK di-cache: data basi yang terlihat segar lebih
        // berbahaya daripada layar yang jujur mengaku offline. Antrean Dexie
        // yang menangani penulisan saat jaringan mati.
        navigateFallbackDenylist: [/^\/api\//],
        runtimeCaching: [],
      },
    }),
  ],
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

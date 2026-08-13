import { defineConfig } from 'vite'
import { VitePWA } from 'vite-plugin-pwa'
const backend = process.env.MSP_DEV_BACKEND || 'http://127.0.0.1:8099'

export default defineConfig({
  plugins: [
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.ico', 'logo.svg'],
      manifest: false,
      workbox: {
        navigateFallbackDenylist: [/^\/api\//],
        cleanupOutdatedCaches: true,
        skipWaiting: true,
        clientsClaim: true,
        runtimeCaching: [
          {
            urlPattern: /^\/api\//,
            handler: 'NetworkOnly',
          },
        ],
      }
    })
  ],
  server: {
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: backend,
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  }
})

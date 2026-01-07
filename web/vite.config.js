import { defineConfig } from 'vite'
import { VitePWA } from 'vite-plugin-pwa'

const backend = process.env.MSP_DEV_BACKEND || 'http://127.0.0.1:8099'

export default defineConfig({
  plugins: [
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.ico', 'icon.svg'],
      workbox: {
        navigateFallbackDenylist: [/^\/api\//],
        runtimeCaching: [
          {
            urlPattern: /^\/api\//,
            handler: 'NetworkOnly',
          },
        ],
      },
      manifest: {
        name: 'MSP Media Share',
        short_name: 'MSP',
        description: 'Local LAN Media Share & Preview',
        theme_color: '#ffffff',
        icons: [
          {
            src: 'icon.svg',
            sizes: 'any',
            type: 'image/svg+xml'
          }
        ]
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

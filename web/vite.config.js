import { defineConfig } from 'vite'
import { VitePWA } from 'vite-plugin-pwa'
// import { OpenMemory } from "openmemory-js";

const backend = process.env.MSP_DEV_BACKEND || 'http://127.0.0.1:8099'

// const mem = new OpenMemory({
//   path: "./memory.sqlite",
//   tier: "fast",
//   embeddings: { provider: "synthetic" } // Use 'openai' for production
// });

// await mem.add("User prefers dark mode", { tags: ["preferences"] });

// const result = await mem.query("What does the user like?");
// console.log(result);

export default defineConfig({
  plugins: [
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.ico', 'logo.svg'],
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
            src: 'logo.svg',
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

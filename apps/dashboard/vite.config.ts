import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const __dirname = dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 7702,
    strictPort: true,
    proxy: {
      '/jobs': 'http://localhost:7700',
      '/deploy': 'http://localhost:7700',
      '/health': 'http://localhost:7700',
    },
  },
  build: {
    outDir: resolve(__dirname, '../server/public'),
    emptyOutDir: true,
  },
})

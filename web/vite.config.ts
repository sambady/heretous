import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: {
      // Preserve the browser's Host so same-origin protection also works in dev.
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: false },
    },
  },
})

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import inject from '@rollup/plugin-inject';
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    inject({
      Buffer: ['buffer', 'Buffer'],
    }),
    tailwindcss(),
  ],
  base: './',
  build: {
    outDir: '../server/assets/frontend',
    emptyOutDir: true,
    minify: false,
  },
  resolve: {
    alias: {
      '@': '/src',
    },
  },
})

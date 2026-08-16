import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

export default defineConfig({
  plugins: [svelte()],
  base: '/admin/',
  build: {
    outDir: path.resolve(__dirname, '../internal/admin/dist'),
    emptyOutDir: true,
    target: 'esnext',
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:3001',
        changeOrigin: true,
      },
      '/static': {
        target: 'http://127.0.0.1:3001',
        changeOrigin: true,
      },
      '/rules': {
        target: 'http://127.0.0.1:3001',
        changeOrigin: true,
      },
    },
  },
});

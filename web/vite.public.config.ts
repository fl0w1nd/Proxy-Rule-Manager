import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: path.resolve(__dirname, '../internal/site/dist'),
    emptyOutDir: true,
    target: 'esnext',
    lib: {
      entry: path.resolve(__dirname, 'src/public/main.ts'),
      formats: ['es'],
      fileName: () => 'public.js',
      cssFileName: 'public',
    },
  },
});

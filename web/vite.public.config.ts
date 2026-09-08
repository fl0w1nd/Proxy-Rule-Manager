import { defineConfig, type Plugin } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import fs from 'node:fs';
import path from 'node:path';

function copyPublicSidecars(): Plugin {
  const copies = [
    ['src/assets/fonts/zpix.woff2', 'zpix.woff2'],
    ['src/assets/fonts/VT323-Regular.ttf', 'VT323-Regular.ttf'],
    ['src/assets/icons/brand/prm.svg', 'prm.svg'],
  ] as const;
  return {
    name: 'copy-public-sidecars',
    closeBundle() {
      const outDir = path.resolve(__dirname, '../internal/site/dist');
      fs.mkdirSync(outDir, { recursive: true });
      for (const [src, name] of copies) {
        fs.copyFileSync(path.resolve(__dirname, src), path.join(outDir, name));
      }
    },
  };
}

export default defineConfig({
  plugins: [svelte(), copyPublicSidecars()],
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

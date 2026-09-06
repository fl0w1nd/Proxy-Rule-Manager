import { defineConfig, type Plugin } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import fs from 'node:fs';
import path from 'node:path';

function copyPublicFonts(): Plugin {
  const fontsDir = path.resolve(__dirname, 'src/assets/fonts');
  const files = ['zpix.woff2', 'VT323-Regular.ttf'];
  return {
    name: 'copy-public-fonts',
    closeBundle() {
      const outDir = path.resolve(__dirname, '../internal/site/dist');
      fs.mkdirSync(outDir, { recursive: true });
      for (const file of files) {
        fs.copyFileSync(path.join(fontsDir, file), path.join(outDir, file));
      }
    },
  };
}

export default defineConfig({
  plugins: [svelte(), copyPublicFonts()],
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

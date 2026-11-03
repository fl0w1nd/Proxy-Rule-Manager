import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/svelte';
import { afterEach } from 'vitest';

// Node 26 defines a global localStorage that returns undefined unless the
// process starts with --localstorage-file, and it shadows jsdom's Storage.
// Install an in-memory implementation so bare localStorage access in
// components behaves as it does in browsers.
if (typeof globalThis.localStorage === 'undefined') {
  const store = new Map<string, string>();
  const storage = {
    get length() { return store.size; },
    clear() { store.clear(); },
    getItem(key: string) { return store.get(key) ?? null; },
    key(index: number) { return [...store.keys()][index] ?? null; },
    removeItem(key: string) { store.delete(key); },
    setItem(key: string, value: string) { store.set(key, String(value)); },
  } as Storage;
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: storage });
}

afterEach(() => cleanup());

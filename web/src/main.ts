import { mount } from 'svelte';
import './fonts.css';
import './app.css';
import App from './App.svelte';
import { initRetroBodyScroll } from './utils/scrollbars';

initRetroBodyScroll();

const app = mount(App, {
  target: document.getElementById('app')!,
});

export default app;

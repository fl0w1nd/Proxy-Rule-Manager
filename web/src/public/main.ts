import { mount } from 'svelte';
import '../app.css';
import './public.css';
import PublicApp from './PublicApp.svelte';
import { readPublicData } from './utils';

const app = mount(PublicApp, {
  target: document.getElementById('public-app')!,
  props: { data: readPublicData() },
});

export default app;

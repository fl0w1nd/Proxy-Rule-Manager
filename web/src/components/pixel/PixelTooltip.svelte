<script lang="ts">
  import PixelIcon from './PixelIcon.svelte';

  interface Props { label: string; text: string; }
  let { label, text }: Props = $props();
  const id = $props.id();
  let root: HTMLDivElement;
  let open = $state(false);
  let timer = 0;

  function show() {
    clearTimeout(timer);
    open = true;
  }
  function hide() {
    clearTimeout(timer);
    open = false;
  }
  function toggle() {
    clearTimeout(timer);
    open = !open;
  }
</script>

<svelte:window onpointerdown={(event) => { if (!root?.contains(event.target as Node)) hide(); }} />
<div class="pixel-tooltip" bind:this={root}>
  <button type="button" aria-label="{label}说明" aria-expanded={open} aria-describedby={open ? id : undefined}
    onpointerenter={() => { timer = window.setTimeout(show, 160); }}
    onpointerleave={() => { if (!root.contains(document.activeElement)) hide(); }}
    onfocus={show} onblur={hide} onclick={toggle}
    onkeydown={(event) => { if (event.key === 'Escape') { event.stopPropagation(); hide(); } }}>
    <PixelIcon name="help" size={12} />
  </button>
  {#if open}
    <div {id} class="bubble" role="tooltip">{text}</div>
  {/if}
</div>

<style>
  .pixel-tooltip { position: relative; display: inline-flex; flex-shrink: 0; }
  button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    height: 16px;
    padding: 0;
    border: 1px solid transparent;
    border-radius: 3px;
    background: transparent;
    color: var(--dim);
    cursor: help;
    transition: color 80ms linear, background-color 80ms linear, border-color 80ms linear;
  }
  button:hover,
  button:focus-visible,
  button[aria-expanded='true'] {
    background: var(--surface-2);
    color: var(--text);
    border-color: var(--border-vis);
  }
  .bubble {
    position: absolute;
    bottom: calc(100% + 7px);
    left: -6px;
    z-index: 30;
    width: max-content;
    max-width: 240px;
    padding: 6px 10px;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    background: var(--surface-2);
    color: var(--text);
    box-shadow: var(--shadow-popup);
    font: 400 11px/17px var(--font-ui);
    overflow-wrap: anywhere;
    animation: pixel-fade 100ms linear;
    pointer-events: none;
  }
  .bubble::before,
  .bubble::after {
    content: '';
    position: absolute;
    left: 10px;
    width: 0;
    height: 0;
    border-left: 4px solid transparent;
    border-right: 4px solid transparent;
    pointer-events: none;
  }
  .bubble::before {
    bottom: -5px;
    border-top: 5px solid var(--border-vis);
  }
  .bubble::after {
    bottom: -4px;
    border-top: 4px solid var(--surface-2);
  }
</style>

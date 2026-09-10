<script lang="ts">
  import PixelButton from './PixelButton.svelte';
  import { retroScroll } from '../../utils/scrollbars';
  import { openModal, closeModalWithExit } from '../../utils/modal';

  export interface IconPickerItem {
    id: string;
    src: string;
    label?: string;
  }

  interface Props {
    open: boolean;
    title?: string;
    items: IconPickerItem[];
    value?: string;
    loading?: boolean;
    emptyLabel?: string;
    onconfirm?: (id: string) => void;
    onclear?: () => void;
    oncancel?: () => void;
  }

  let {
    open = $bindable(false),
    title = '选择图标',
    items,
    value = '',
    loading = false,
    emptyLabel = '还没有可用的客户端图标。',
    onconfirm,
    onclear,
    oncancel,
  }: Props = $props();

  const headingId = $props.id();
  let dialog = $state<HTMLDialogElement>();
  let picked = $state('');

  $effect(() => {
    if (open) picked = value ?? '';
  });

  $effect(() => {
    if (!open || !dialog) return;
    const element = dialog;
    const session = openModal(element);
    return () => {
      if (element.isConnected && element.hasAttribute('open')) {
        closeModalWithExit(element, 'modal-closing', () => session.finishClose());
      } else {
        session.finishClose();
      }
    };
  });

  function cancel() {
    open = false;
    oncancel?.();
  }

  function confirm() {
    if (!picked) return;
    open = false;
    onconfirm?.(picked);
  }

  function clear() {
    open = false;
    onclear?.();
  }

  function move(delta: number) {
    if (items.length === 0) return;
    const index = items.findIndex((item) => item.id === picked);
    const start = index < 0 ? (delta > 0 ? -1 : 0) : index;
    picked = items[(start + delta + items.length) % items.length].id;
  }
</script>

<dialog
  bind:this={dialog}
  class="icon-picker"
  aria-labelledby="{headingId}-title"
  oncancel={(event) => { event.preventDefault(); cancel(); }}
>
  <header><h2 id="{headingId}-title">{title}</h2></header>
  <div class="body" use:retroScroll>
    {#if loading}
      <p class="status" role="status">正在读取图标…</p>
    {:else if items.length === 0}
      <p class="status empty">{emptyLabel}</p>
    {:else}
      <div
        class="grid"
        role="radiogroup"
        tabindex="-1"
        aria-labelledby="{headingId}-title"
        onkeydown={(event) => {
          if (event.key === 'ArrowRight' || event.key === 'ArrowDown') { event.preventDefault(); move(1); }
          else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') { event.preventDefault(); move(-1); }
          else if (event.key === 'Enter') { event.preventDefault(); confirm(); }
        }}
      >
        {#each items as item (item.id)}
          <button
            type="button"
            class="tile"
            class:on={picked === item.id}
            role="radio"
            aria-checked={picked === item.id}
            onclick={() => { picked = item.id; }}
            ondblclick={() => { picked = item.id; confirm(); }}
          >
            <span class="mark" class:on={picked === item.id} aria-hidden="true"></span>
            <img src={item.src} width="48" height="48" alt="" />
            <strong>{item.label ?? item.id}</strong>
          </button>
        {/each}
      </div>
    {/if}
  </div>
  <footer>
    <PixelButton onclick={cancel}>取消</PixelButton>
    <PixelButton disabled={!value && !picked} onclick={clear}>清除</PixelButton>
    <PixelButton variant="primary" disabled={loading || !picked} onclick={confirm}>使用</PixelButton>
  </footer>
</dialog>

<style>
  .icon-picker {
    margin: auto;
    width: min(640px, calc(100vw - 32px));
    max-height: calc(100dvh - 32px);
    padding: 0;
    overflow: hidden;
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow-dialog);
  }
  .icon-picker::backdrop { background: var(--backdrop); }
  .icon-picker[open] {
    display: flex;
    flex-direction: column;
    animation: pixel-fade 120ms linear;
  }
  .icon-picker[open]::backdrop { animation: pixel-fade 120ms linear; }
  .icon-picker:global(.modal-closing),
  .icon-picker:global(.modal-closing)::backdrop { animation: pixel-fade 120ms linear reverse; }
  header {
    flex-shrink: 0;
    padding: 14px 20px;
    border-bottom: 1px solid var(--border-vis);
    background: var(--surface-2);
  }
  h2 {
    margin: 0;
    font: 400 24px/32px var(--font-ui);
    color: var(--display);
  }
  .body {
    flex: 1 1 auto;
    min-height: 0;
    overflow-y: auto;
    padding: 20px;
  }
  .status {
    margin: 0;
    font: 400 14px/22px var(--font-reading);
    color: var(--sec);
  }
  .status.empty { color: var(--dim); text-align: center; padding: 36px 0; }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(104px, 1fr));
    gap: 12px;
  }
  .tile {
    position: relative;
    display: grid;
    justify-items: center;
    gap: 8px;
    min-width: 0;
    padding: 14px 10px 12px;
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--highlight-top), var(--shadow-panel);
    cursor: pointer;
    text-align: center;
    transition: background-color 80ms linear;
  }
  .tile:hover:not(.on) { background: var(--surface-2); }
  .tile.on {
    background: var(--selected);
    color: var(--selected-text);
    box-shadow: var(--edge-pressed);
  }
  .tile:focus-visible { outline: 2px solid var(--focus); outline-offset: 3px; }
  .tile img {
    width: 48px;
    height: 48px;
    object-fit: contain;
  }
  .tile strong {
    display: block;
    max-width: 100%;
    overflow: hidden;
    color: inherit;
    font: 400 12px/20px var(--font-ui);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .mark {
    position: absolute;
    top: 8px;
    right: 8px;
    width: 10px;
    height: 10px;
    border: 1px solid currentColor;
    border-radius: 50%;
  }
  .mark.on::after {
    content: '';
    position: absolute;
    inset: 2px;
    border-radius: 50%;
    background: currentColor;
  }
  footer {
    flex-shrink: 0;
    display: flex;
    justify-content: flex-end;
    gap: 10px;
    padding: 12px 20px 20px;
    flex-wrap: wrap;
  }
</style>

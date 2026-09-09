<script lang="ts">
  import type { Snippet } from 'svelte';
  import PixelButton from './PixelButton.svelte';

  interface Props {
    open: boolean;
    title: string;
    confirmLabel?: string;
    cancelLabel?: string;
    danger?: boolean;
    showCancel?: boolean;
    onconfirm?: () => void;
    oncancel?: () => void;
    children?: Snippet;
  }
  let { open = $bindable(false), title, confirmLabel = '确认', cancelLabel = '取消', danger = false, showCancel = true, onconfirm, oncancel, children }: Props = $props();
  const id = $props.id();
  let dialog: HTMLDialogElement;

  function cancel() { open = false; oncancel?.(); }
  $effect(() => {
    if (!open || !dialog) return;
    const previousFocus = document.activeElement as HTMLElement | null;
    const overflow = document.body.style.overflow;
    dialog.showModal();
    document.body.style.overflow = 'hidden';
    return () => {
      dialog.close();
      document.body.style.overflow = overflow;
      previousFocus?.focus();
    };
  });
</script>

<dialog bind:this={dialog} aria-labelledby="{id}-title" oncancel={(event) => { event.preventDefault(); cancel(); }}>
  <header><h2 id="{id}-title">{title}</h2></header>
  <div class="body">{#if children}{@render children()}{/if}</div>
  <footer>
    {#if showCancel}
      <PixelButton onclick={cancel}>{cancelLabel}</PixelButton>
    {/if}
    <PixelButton variant={danger ? 'danger' : 'primary'} onclick={() => { open = false; onconfirm?.(); }}>{confirmLabel}</PixelButton>
  </footer>
</dialog>

<style>
  dialog {
    margin: auto;
    width: min(440px, calc(100vw - 32px));
    max-height: calc(100dvh - 32px);
    padding: 0;
    overflow: hidden;
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow-dialog);
  }
  dialog::backdrop { background: var(--backdrop); }
  dialog[open] {
    display: flex;
    flex-direction: column;
    animation: pixel-fade 120ms linear;
  }
  dialog[open]::backdrop { animation: pixel-fade 120ms linear; }
  header { flex-shrink: 0; padding: 14px 20px; border-bottom: 1px solid var(--border-vis); background: var(--surface-2); }
  h2 { margin: 0; font: 400 24px/32px var(--font-ui); color: var(--display); }
  .body { min-height: 0; overflow-y: auto; padding: 20px; font: 400 14px/22px var(--font-reading); overflow-wrap: anywhere; }
  footer { flex-shrink: 0; display: flex; justify-content: flex-end; gap: 10px; padding: 12px 20px 20px; flex-wrap: wrap; }
</style>

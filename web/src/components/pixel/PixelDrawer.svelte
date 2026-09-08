<script lang="ts">
  import type { Snippet } from 'svelte';
  import PixelButton from './PixelButton.svelte';
  import PixelIcon from './PixelIcon.svelte';
  import terminalIcon from '../../assets/icons/ui/terminal.svg';
  import { retroScroll } from '../../utils/scrollbars';

  interface Props {
    open: boolean;
    title?: string;
    width?: string;
    onclose?: () => void;
    onrequestclose?: () => boolean;
    icon?: string;
    children?: Snippet;
    footer?: Snippet;
  }

  let {
    open = $bindable(false),
    title = '终端控制台',
    width = '520px',
    onclose,
    onrequestclose,
    icon = terminalIcon,
    children,
    footer,
  }: Props = $props();

  let dialog = $state<HTMLDialogElement>();

  function close() {
    if (onrequestclose?.() === false) return;
    open = false;
    onclose?.();
  }

  $effect(() => {
    if (open && dialog) {
      const previousFocus = document.activeElement as HTMLElement | null;
      const element = dialog;
      element.showModal();
      return () => {
        element.close();
        previousFocus?.focus();
      };
    }
  });
</script>

{#if open}
    <dialog
      bind:this={dialog}
      class="drawer-panel"
      style="max-width: {width};"
      oncancel={(event) => { event.preventDefault(); close(); }}
      onclick={(event) => { if (dialog && event.target === dialog) { const box = dialog.getBoundingClientRect(); if (event.clientX < box.left || event.clientX > box.right || event.clientY < box.top || event.clientY > box.bottom) close(); } }}
      aria-label={title}
    >
      <div class="drawer-header">
        <div class="drawer-title">
          <img src={icon} class="drawer-pixel-icon" width="24" height="24" alt="" />
          <span>{title}</span>
        </div>
        <PixelButton size="sm" variant="ghost" onclick={close} aria-label="关闭抽屉">
          <PixelIcon name="cross" size={12} />
        </PixelButton>
      </div>

      <div class="drawer-body" use:retroScroll>
        {#if children}
          {@render children()}
        {/if}
      </div>

      {#if footer}
        <div class="drawer-footer">
          {@render footer()}
        </div>
      {/if}
    </dialog>
{/if}

<style>
  :global(body:has(.drawer-panel[open])) { overflow: hidden; }
  .drawer-panel::backdrop { background: var(--backdrop); }

  .drawer-panel {
    position: fixed;
    inset: 0 0 0 auto;
    margin: 0 0 0 auto;
    padding: 0;
    border: 0;
    max-height: 100dvh;
    color: var(--text);
    width: 100%;
    height: 100%;
    background: var(--surface);
    border-left: 1px solid var(--border-vis);
    box-shadow: var(--shadow-dialog);
    flex-direction: column;
    animation: pixel-drawer 140ms cubic-bezier(.2, .8, .2, 1) both;
  }

  .drawer-panel[open] { display: flex; }

  .drawer-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 18px;
    background: var(--surface-2);
    border-bottom: 1px solid var(--border-vis);
  }

  .drawer-title {
    display: flex;
    align-items: center;
    gap: 10px;
    font: 400 24px/32px var(--font-ui);
    color: var(--display);
    letter-spacing: 0;
    text-shadow: none;
  }

  .drawer-pixel-icon {
    width: 24px;
    height: 24px;
    image-rendering: pixelated;
    image-rendering: crisp-edges;
    flex-shrink: 0;
    display: block;
  }

  .drawer-body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 16px 18px;
    background: var(--surface);
    scrollbar-color: var(--bevel-dark) var(--surface-2);
    overscroll-behavior: contain;
  }

  .drawer-footer {
    padding: 12px 18px;
    background: var(--surface-2);
    border-top: 1px solid var(--border-vis);
    display: flex;
    justify-content: flex-end;
    flex-wrap: wrap;
    gap: 10px;
  }

  @media (max-width: 600px) {
    .drawer-panel {
      max-width: 100% !important;
      border-left: none;
    }
  }
</style>

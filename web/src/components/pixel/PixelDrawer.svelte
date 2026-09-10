<script lang="ts">
  import type { Snippet } from 'svelte';
  import PixelButton from './PixelButton.svelte';
  import PixelIcon from './PixelIcon.svelte';
  import terminalIcon from '../../assets/icons/ui/terminal.svg';
  import { retroScroll } from '../../utils/scrollbars';
  import { openModal, closeModalWithExit } from '../../utils/modal';

  interface Props {
    open: boolean;
    title?: string;
    width?: string;
    scrollable?: boolean;
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
    scrollable = true,
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

  function drawerScroll(node: HTMLElement) {
    if (!scrollable) return;
    return retroScroll(node);
  }

  $effect(() => {
    if (!open || !dialog) return;
    const element = dialog;
    const session = openModal(element);
    return () => {
      if (element.isConnected && element.hasAttribute('open')) {
        closeModalWithExit(element, 'closing', () => session.finishClose());
      } else {
        session.finishClose();
      }
    };
  });
</script>

<dialog
  bind:this={dialog}
  class="drawer-panel"
  style="max-width: min({width}, 100vw);"
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

  <div class="drawer-body" class:no-scroll={!scrollable} use:drawerScroll>
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

<style>
  :global(html.scroll-locked),
  :global(body.scroll-locked),
  :global(html:has(.drawer-panel[open])),
  :global(body:has(.drawer-panel[open])) {
    overflow: hidden !important;
    overscroll-behavior: none !important;
  }
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
    overscroll-behavior: contain;
  }

  .drawer-panel[open] {
    display: flex;
    animation: pixel-drawer 140ms cubic-bezier(.2, .8, .2, 1);
  }

  .drawer-panel:global(.closing) {
    animation: pixel-drawer-out 120ms cubic-bezier(.2, .8, .2, 1) forwards;
  }

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
    padding: 16px 36px 16px 18px;
    background: var(--surface);
    scrollbar-color: var(--bevel-dark) var(--surface-2);
    overscroll-behavior: contain;
  }

  .drawer-body.no-scroll {
    overflow: hidden;
    display: flex;
    flex-direction: column;
    padding: 16px 18px;
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

  @media (max-width: 768px) {
    .drawer-panel {
      max-width: 100% !important;
      border-left: none;
    }
  }
</style>

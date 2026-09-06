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
    children?: Snippet;
    footer?: Snippet;
  }

  let {
    open = $bindable(false),
    title = '终端控制台',
    width = '520px',
    onclose,
    children,
    footer,
  }: Props = $props();

  function handleKeydown(ev: KeyboardEvent) {
    if (ev.key === 'Escape' && open) {
      close();
    }
  }

  function close() {
    open = false;
    onclose?.();
  }

  $effect(() => {
    if (open) {
      const originalOverflow = document.body.style.overflow;
      document.body.style.overflow = 'hidden';
      return () => {
        document.body.style.overflow = originalOverflow;
      };
    }
  });
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
  <div class="drawer-backdrop" onclick={close} role="presentation">
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <div
      class="drawer-panel"
      style="max-width: {width};"
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      tabindex="-1"
    >
      <div class="drawer-header">
        <div class="drawer-title">
          <img src={terminalIcon} class="drawer-pixel-icon" width="24" height="24" alt="" />
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
    </div>
  </div>
{/if}

<style>
  .drawer-backdrop {
    position: fixed;
    inset: 0;
    z-index: 100;
    background: var(--backdrop);
    display: flex;
    justify-content: flex-end;
    overscroll-behavior: contain;
    animation: pixel-fade 120ms linear;
  }

  .drawer-panel {
    width: 100%;
    height: 100%;
    background: var(--surface);
    border-left: 1px solid var(--border-vis);
    box-shadow: var(--shadow-dialog);
    display: flex;
    flex-direction: column;
    animation: pixel-drawer 140ms cubic-bezier(.2, .8, .2, 1) both;
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
    gap: 10px;
  }

  @media (max-width: 600px) {
    .drawer-panel {
      max-width: 100% !important;
      border-left: none;
    }
  }
</style>

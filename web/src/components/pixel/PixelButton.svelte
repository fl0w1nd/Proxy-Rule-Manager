<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
    size?: 'sm' | 'md' | 'lg';
    disabled?: boolean;
    type?: 'button' | 'submit' | 'reset';
    title?: string;
    'aria-label'?: string;
    ariaLabel?: string;
    onclick?: (ev: MouseEvent) => void;
    class?: string;
    children?: Snippet;
  }

  let {
    variant = 'secondary',
    size = 'md',
    disabled = false,
    type = 'button',
    title,
    'aria-label': ariaLabelAttr,
    ariaLabel,
    onclick,
    class: className = '',
    children,
  }: Props = $props();
</script>

<button
  {type}
  {disabled}
  {title}
  aria-label={ariaLabelAttr || ariaLabel}
  {onclick}
  class="pixel-btn {variant} {size} {className}"
>
  {#if children}
    {@render children()}
  {/if}
</button>

<style>
  .pixel-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    background: var(--surface-2);
    color: var(--text);
    box-shadow: var(--edge-raised);
    font: 400 12px/20px var(--font-ui);
    letter-spacing: 0;
    text-shadow: none;
    cursor: pointer;
    text-decoration: none;
    white-space: nowrap;
    transition: background-color 80ms linear;
  }

  .pixel-btn.sm {
    min-height: 28px;
    padding: 0 10px;
  }
  .pixel-btn.md {
    min-height: 32px;
    padding: 0 12px;
  }
  .pixel-btn.lg {
    min-height: 40px;
    padding: 0 16px;
  }

  .pixel-btn.primary {
    background: var(--accent);
    color: var(--text);
  }
  .pixel-btn.primary:hover:not(:disabled) {
    background: var(--accent-hover);
  }

  .pixel-btn.secondary:hover:not(:disabled) {
    background: var(--surface);
  }

  .pixel-btn.danger {
    background: var(--status-error);
    color: var(--text);
  }
  .pixel-btn.danger:hover:not(:disabled) {
    background: var(--surface-3);
  }

  .pixel-btn.ghost {
    background: transparent;
    color: var(--sec);
    border-color: transparent;
    box-shadow: none;
  }
  .pixel-btn.ghost:hover:not(:disabled) {
    background: var(--surface-2);
    color: var(--text);
    border-color: transparent;
    box-shadow: none;
  }

  .pixel-btn:active:not(:disabled) {
    box-shadow: var(--edge-pressed);
    transform: translateY(1px);
  }
  .pixel-btn.ghost:active:not(:disabled) {
    box-shadow: var(--edge-pressed);
  }

  .pixel-btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
    transform: none;
    box-shadow: var(--edge-raised);
  }
  .pixel-btn.ghost:disabled {
    box-shadow: none;
  }
</style>

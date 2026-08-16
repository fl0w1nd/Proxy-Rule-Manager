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
    background: var(--surface);
    color: var(--text);
    border: 2px solid var(--border-vis);
    font-family: "Space Mono", monospace;
    font-weight: 700;
    letter-spacing: 0.05em;
    cursor: pointer;
    text-decoration: none;
    white-space: nowrap;
    box-shadow:
      inset 1px 1px 0 var(--bevel-light),
      inset -1px -1px 0 var(--bevel-dark),
      2px 2px 0 var(--shadow);
    transition: all 80ms steps(2, end);
  }

  /* Sizes */
  .pixel-btn.sm {
    padding: 4px 10px;
    font-size: 11px;
  }
  .pixel-btn.md {
    padding: 7px 16px;
    font-size: 12px;
  }
  .pixel-btn.lg {
    padding: 10px 22px;
    font-size: 13px;
  }

  /* Hover & Active states */
  .pixel-btn:hover:not(:disabled) {
    color: var(--display);
    border-color: var(--orange);
    transform: translate(-1px, -1px);
    box-shadow:
      inset 1px 1px 0 var(--bevel-light),
      inset -1px -1px 0 var(--bevel-dark),
      3px 3px 0 var(--shadow);
  }

  .pixel-btn:active:not(:disabled) {
    transform: translate(2px, 2px);
    box-shadow:
      inset -1px -1px 0 var(--bevel-light),
      inset 1px 1px 0 var(--bevel-dark),
      0 0 0 var(--shadow);
  }

  /* Variants */
  .pixel-btn.primary {
    background: var(--orange);
    color: #ffffff;
    border-color: var(--orange);
    box-shadow:
      inset 1px 1px 0 rgba(255, 255, 255, 0.3),
      inset -1px -1px 0 rgba(0, 0, 0, 0.3),
      2px 2px 0 var(--shadow);
  }
  .pixel-btn.primary:hover:not(:disabled) {
    background: #ff7830;
    border-color: #ff7830;
  }

  .pixel-btn.danger {
    color: var(--red);
    border-color: var(--red);
  }
  .pixel-btn.danger:hover:not(:disabled) {
    background: var(--red);
    color: #ffffff;
  }

  .pixel-btn.ghost {
    background: transparent;
    border-color: transparent;
    box-shadow: none;
  }
  .pixel-btn.ghost:hover:not(:disabled) {
    background: var(--surface-2);
    border-color: var(--border-vis);
    box-shadow: 2px 2px 0 var(--shadow);
  }

  .pixel-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
    filter: grayscale(0.5);
  }
</style>

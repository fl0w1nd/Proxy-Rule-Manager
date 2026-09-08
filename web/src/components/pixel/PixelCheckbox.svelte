<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    id?: string;
    label?: string;
    checked?: boolean;
    disabled?: boolean;
    title?: string;
    size?: 'sm' | 'md';
    class?: string;
    onchange?: (checked: boolean) => void;
    children?: Snippet;
  }

  let {
    id,
    label,
    checked = $bindable(false),
    disabled = false,
    title,
    size = 'md',
    class: className = '',
    onchange,
    children,
  }: Props = $props();

  function toggle(e: Event) {
    if (disabled) return;
    const target = e.currentTarget as HTMLInputElement;
    checked = target.checked;
    onchange?.(checked);
  }
</script>

<label class="pixel-checkbox {size} {className}" class:disabled class:checked {title}>
  <input
    {id}
    type="checkbox"
    bind:checked
    {disabled}
    onchange={toggle}
  />
  <span class="checkbox-box" class:checked aria-hidden="true">
    {#if checked}
      <svg
        class="pixel-check"
        width={size === 'sm' ? 8 : 10}
        height={size === 'sm' ? 8 : 10}
        viewBox="0 0 10 10"
        shape-rendering="crispEdges"
        aria-hidden="true"
      >
        <path
          d="M0 5h1v2H0z M1 6h1v2H1z M2 7h1v2H2z M3 6h1v2H3z M4 5h1v2H4z M5 4h1v2H5z M6 3h1v2H6z M7 2h1v2H7z M8 1h1v2H8z"
          fill="currentColor"
        />
      </svg>
    {/if}
  </span>
  {#if children}
    <span class="checkbox-label">{@render children()}</span>
  {:else if label}
    <span class="checkbox-label">{label}</span>
  {/if}
</label>

<style>
  .pixel-checkbox {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    position: relative;
    cursor: pointer;
    user-select: none;
    font: 400 12px/20px var(--font-ui);
    color: var(--text);
  }

  .pixel-checkbox.sm {
    gap: 6px;
    font: 400 11px/16px var(--font-ui);
  }

  .pixel-checkbox.disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  input[type="checkbox"] {
    position: absolute;
    opacity: 0;
    width: 0;
    height: 0;
    margin: 0;
    pointer-events: none;
  }

  .checkbox-box {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    height: 16px;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    background: var(--surface-2);
    box-shadow: var(--edge-inset);
    color: var(--selected-text);
    flex-shrink: 0;
    transition: background-color 80ms linear;
  }

  .pixel-checkbox.sm .checkbox-box {
    width: 14px;
    height: 14px;
    border-radius: 2px;
  }

  .pixel-checkbox:hover:not(.disabled) .checkbox-box:not(.checked) {
    background: var(--surface-3);
  }

  .checkbox-box.checked {
    background: var(--selected);
    color: var(--selected-text);
    box-shadow: var(--edge-pressed);
  }

  .pixel-checkbox:hover:not(.disabled) .checkbox-box.checked {
    background: var(--accent-hover);
  }

  .pixel-checkbox:has(input:focus-visible) .checkbox-box {
    outline: 2px solid var(--focus);
    outline-offset: 1px;
  }

  .checkbox-label {
    overflow-wrap: anywhere;
  }
</style>

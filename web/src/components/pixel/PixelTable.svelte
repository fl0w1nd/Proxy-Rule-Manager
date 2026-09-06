<script lang="ts">
  import type { Snippet } from 'svelte';
  import { retroScroll } from '../../utils/scrollbars';

  interface Props {
    minWidth?: string;
    class?: string;
    children?: Snippet;
  }

  let { minWidth = '100%', class: className = '', children }: Props = $props();
</script>

<div class="pixel-table-wrap {className}" use:retroScroll>
  <table class="pixel-table" style="min-width: {minWidth};">
    {#if children}
      {@render children()}
    {/if}
  </table>
</div>

<style>
  .pixel-table-wrap {
    width: 100%;
    overflow-x: auto;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    background: var(--surface);
    scrollbar-color: var(--bevel-dark) var(--surface-2);
    scrollbar-width: thin;
  }

  .pixel-table {
    width: 100%;
    border-collapse: collapse;
    font: 400 12px/20px var(--font-ui);
    text-align: left;
  }

  :global(.pixel-table thead th) {
    position: sticky;
    top: 0;
    z-index: 5;
    background: var(--surface-2);
    border-bottom: 1px solid var(--border-vis);
    color: var(--sec);
    font: 400 12px/20px var(--font-ui);
    letter-spacing: 0;
    padding: 10px 14px;
    white-space: nowrap;
  }

  :global(.pixel-table tbody td),
  :global(.pixel-table tbody th) {
    padding: 12px 14px;
    border-bottom: 1px solid var(--border);
    vertical-align: middle;
  }

  :global(.pixel-table tbody tr) {
    transition: background-color 80ms linear;
  }

  :global(.pixel-table tbody tr:hover) {
    background: var(--surface-2);
  }

  :global(.pixel-table tbody tr.selected),
  :global(.pixel-table tbody tr.selected:hover) {
    background: var(--selected);
    color: var(--selected-text);
  }

  :global(.pixel-table tbody tr.selected td),
  :global(.pixel-table tbody tr.selected th),
  :global(.pixel-table tbody tr.selected .font-name),
  :global(.pixel-table tbody tr.selected .text-sec),
  :global(.pixel-table tbody tr.selected .text-dim) {
    color: var(--selected-text);
  }

  :global(.pixel-table tbody tr.selected em) {
    background: transparent;
    border-color: currentColor;
    color: var(--selected-text);
  }

  :global(.pixel-table .num) {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
</style>

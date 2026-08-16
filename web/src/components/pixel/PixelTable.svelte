<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    minWidth?: string;
    class?: string;
    children?: Snippet;
  }

  let { minWidth = '100%', class: className = '', children }: Props = $props();
</script>

<div class="pixel-table-wrap {className}">
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
    border: 2px solid var(--border-vis);
    background: var(--surface);
    box-shadow:
      inset 1px 1px 0 var(--bevel-light),
      inset -1px -1px 0 var(--bevel-dark),
      3px 3px 0 var(--shadow);
    scrollbar-color: var(--border-vis) var(--bg);
    scrollbar-width: thin;
  }

  .pixel-table {
    width: 100%;
    border-collapse: collapse;
    font-family: "Space Mono", monospace;
    font-size: 12px;
    text-align: left;
  }

  :global(.pixel-table thead th) {
    position: sticky;
    top: 0;
    z-index: 5;
    background: var(--surface-2);
    border-bottom: 2px solid var(--border-vis);
    color: var(--sec);
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.05em;
    padding: 10px 14px;
    white-space: nowrap;
  }

  :global(.pixel-table tbody td),
  :global(.pixel-table tbody th) {
    padding: 11px 14px;
    border-bottom: 1px dashed var(--border);
    vertical-align: middle;
  }

  :global(.pixel-table tbody tr) {
    transition: background 80ms steps(2, end);
  }

  :global(.pixel-table tbody tr:hover) {
    background: var(--surface-2);
  }

  :global(.pixel-table tbody tr:hover > :first-child) {
    box-shadow: inset 4px 0 0 var(--orange);
  }

  :global(.pixel-table .num) {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
</style>

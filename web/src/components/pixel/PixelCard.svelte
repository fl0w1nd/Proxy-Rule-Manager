<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    title?: string;
    class?: string;
    children?: Snippet;
    actions?: Snippet;
  }

  let { title, class: className = '', children, actions }: Props = $props();
</script>

<div class="pixel-card {className}">
  {#if title || actions}
    <div class="pixel-card-header">
      {#if title}
        <h3 class="pixel-card-title">{title}</h3>
      {/if}
      {#if actions}
        <div class="pixel-card-actions">
          {@render actions()}
        </div>
      {/if}
    </div>
  {/if}
  <div class="pixel-card-body">
    {#if children}
      {@render children()}
    {/if}
  </div>
</div>

<style>
  .pixel-card {
    background: var(--surface);
    border: 2px solid var(--border-vis);
    box-shadow:
      inset 1px 1px 0 var(--bevel-light),
      inset -1px -1px 0 var(--bevel-dark),
      3px 3px 0 var(--shadow);
    padding: 16px 20px;
  }

  .pixel-card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding-bottom: 12px;
    margin-bottom: 14px;
    border-bottom: 2px dashed var(--border-vis);
  }

  .pixel-card-title {
    font-family: "Doto", "Space Mono", monospace;
    font-size: 15px;
    font-weight: 800;
    letter-spacing: 0.05em;
    color: var(--display);
    text-shadow: 1px 0 currentColor;
  }

  .pixel-card-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .pixel-card-body {
    min-width: 0;
  }
</style>

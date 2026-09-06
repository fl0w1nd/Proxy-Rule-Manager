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
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    box-shadow: var(--highlight-top), var(--shadow-panel);
    padding: 16px 20px;
  }

  .pixel-card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding-bottom: 12px;
    margin-bottom: 14px;
    border-bottom: 1px solid var(--border);
  }

  .pixel-card-title {
    font: 400 24px/32px var(--font-ui);
    color: var(--display);
    letter-spacing: 0;
    text-shadow: none;
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

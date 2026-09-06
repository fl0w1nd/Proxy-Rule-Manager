<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    status?: 'success' | 'warning' | 'error' | 'info' | 'neutral' | 'active';
    pulse?: boolean;
    class?: string;
    children?: Snippet;
  }

  let {
    status = 'neutral',
    pulse = false,
    class: className = '',
    children,
  }: Props = $props();
</script>

<span class="pixel-badge {status} {pulse ? 'pulse' : ''} {className}">
  <span class="indicator"></span>
  {#if children}
    <span class="label">{@render children()}</span>
  {/if}
</span>

<style>
  .pixel-badge {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 0 6px;
    min-height: 20px;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    background: var(--surface-2);
    color: var(--sec);
    font: 400 12px/20px var(--font-ui);
    letter-spacing: 0;
    text-shadow: none;
    white-space: nowrap;
  }

  .indicator {
    display: inline-block;
    width: 6px;
    height: 6px;
    background: currentColor;
    flex-shrink: 0;
  }

  .pixel-badge.success {
    background: var(--status-success);
    color: var(--text);
  }
  .pixel-badge.warning {
    background: var(--status-warning);
    color: var(--text);
  }
  .pixel-badge.error {
    background: var(--status-error);
    color: var(--text);
  }
  .pixel-badge.info,
  .pixel-badge.active {
    background: var(--status-info);
    color: var(--text);
  }

  .pixel-badge.pulse .indicator {
    animation: pixel-signal 800ms steps(2, end) infinite;
  }
</style>

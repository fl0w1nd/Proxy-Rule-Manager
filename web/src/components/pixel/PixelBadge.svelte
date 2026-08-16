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
    padding: 3px 8px;
    border: 1px solid var(--border-vis);
    background: var(--surface);
    font-family: "Space Mono", monospace;
    font-size: 11px;
    font-weight: 700;
    line-height: 1;
    white-space: nowrap;
    box-shadow: 1px 1px 0 var(--shadow);
  }

  .indicator {
    display: inline-block;
    width: 6px;
    height: 6px;
    background: var(--dim);
    flex-shrink: 0;
  }

  .pixel-badge.success {
    color: var(--green);
    border-color: var(--green);
    background: var(--green-dim);
  }
  .pixel-badge.success .indicator {
    background: var(--green);
  }

  .pixel-badge.warning {
    color: var(--orange);
    border-color: var(--orange);
    background: var(--orange-dim);
  }
  .pixel-badge.warning .indicator {
    background: var(--orange);
  }

  .pixel-badge.error {
    color: var(--red);
    border-color: var(--red);
    background: var(--red-dim);
  }
  .pixel-badge.error .indicator {
    background: var(--red);
  }

  .pixel-badge.info, .pixel-badge.active {
    color: var(--blue);
    border-color: var(--blue);
    background: var(--blue-dim);
  }
  .pixel-badge.info .indicator, .pixel-badge.active .indicator {
    background: var(--blue);
  }

  .pixel-badge.pulse .indicator {
    animation: pixel-signal 800ms steps(2, end) infinite;
  }
</style>

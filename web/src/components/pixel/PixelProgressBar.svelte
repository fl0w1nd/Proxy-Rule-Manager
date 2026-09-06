<script lang="ts">
  interface Props {
    current: number;
    total: number;
    showText?: boolean;
    class?: string;
  }

  let {
    current = 0,
    total = 100,
    showText = true,
    class: className = '',
  }: Props = $props();

  let percentage = $derived(
    total > 0 ? Math.min(100, Math.max(0, Math.round((current / total) * 100))) : 0
  );
</script>

<div class="progress-wrap {className}">
  <div class="progress-shell">
    <div class="progress-bar" style="transform: scaleX({percentage / 100});"></div>
  </div>
  {#if showText}
    <div class="progress-info">
      <span class="pct">{percentage}%</span>
      <span class="counts">{current} / {total}</span>
    </div>
  {/if}
</div>

<style>
  .progress-wrap {
    width: 100%;
  }

  .progress-shell {
    width: 100%;
    height: 14px;
    background: var(--surface-2);
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    box-shadow: var(--edge-inset);
    overflow: hidden;
  }

  .progress-bar {
    width: 100%;
    height: 100%;
    transform-origin: left center;
    background-color: var(--accent);
    background-image: repeating-linear-gradient(
      90deg,
      var(--accent) 0,
      var(--accent) 4px,
      transparent 4px,
      transparent 6px
    );
    transition: transform 120ms linear;
    will-change: transform;
  }

  @media (prefers-reduced-motion: reduce) {
    .progress-bar {
      transition: none;
    }
  }

  .progress-info {
    display: flex;
    justify-content: space-between;
    margin-top: 4px;
    color: var(--sec);
    font: 400 12px/20px var(--font-ui);
    font-variant-numeric: tabular-nums;
  }
</style>

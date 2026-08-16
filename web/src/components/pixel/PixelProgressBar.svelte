<script lang="ts">
  interface Props {
    current: number;
    total: number;
    color?: 'orange' | 'green' | 'red' | 'blue';
    showText?: boolean;
    class?: string;
  }

  let {
    current = 0,
    total = 100,
    color = 'orange',
    showText = true,
    class: className = '',
  }: Props = $props();

  let percentage = $derived(
    total > 0 ? Math.min(100, Math.max(0, Math.round((current / total) * 100))) : 0
  );
</script>

<div class="progress-wrap {className}">
  <div class="progress-shell {color}">
    <div class="progress-bar" style="width: {percentage}%;"></div>
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
    border: 2px solid var(--border-vis);
    box-shadow:
      inset 1px 1px 0 var(--bevel-dark),
      2px 2px 0 var(--shadow);
    padding: 2px;
    overflow: hidden;
  }

  .progress-bar {
    height: 100%;
    transition: width 120ms steps(8, end);
  }

  /* Segmented dither bar patterns */
  .progress-shell.orange .progress-bar {
    background-color: var(--orange);
    background-image: repeating-linear-gradient(
      90deg,
      transparent 0,
      transparent 4px,
      rgba(0, 0, 0, 0.4) 4px,
      rgba(0, 0, 0, 0.4) 6px
    );
  }

  .progress-shell.green .progress-bar {
    background-color: var(--green);
    background-image: repeating-linear-gradient(
      90deg,
      transparent 0,
      transparent 4px,
      rgba(0, 0, 0, 0.4) 4px,
      rgba(0, 0, 0, 0.4) 6px
    );
  }

  .progress-shell.red .progress-bar {
    background-color: var(--red);
    background-image: repeating-linear-gradient(
      90deg,
      transparent 0,
      transparent 4px,
      rgba(0, 0, 0, 0.4) 4px,
      rgba(0, 0, 0, 0.4) 6px
    );
  }

  .progress-shell.blue .progress-bar {
    background-color: var(--blue);
    background-image: repeating-linear-gradient(
      90deg,
      transparent 0,
      transparent 4px,
      rgba(0, 0, 0, 0.4) 4px,
      rgba(0, 0, 0, 0.4) 6px
    );
  }

  .progress-info {
    display: flex;
    justify-content: space-between;
    font-family: "Space Mono", monospace;
    font-size: 10px;
    font-weight: 700;
    color: var(--sec);
    margin-top: 4px;
  }
</style>

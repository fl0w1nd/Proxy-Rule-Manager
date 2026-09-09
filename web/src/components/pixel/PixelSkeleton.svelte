<script lang="ts">
  let {
    rows = 6,
    variant = 'list',
  }: {
    rows?: number;
    variant?: 'list' | 'preview';
  } = $props();

  const widths = $derived(variant === 'preview'
    ? ['72%', '88%', '64%', '80%', '70%', '84%', '58%', '76%']
    : ['48%', '62%', '54%', '70%', '44%', '66%', '58%', '50%', '64%', '46%']);
</script>

<div class="pixel-skeleton" data-variant={variant} aria-hidden="true">
  {#each Array.from({ length: rows }, (_, index) => widths[index % widths.length]) as width, index (index)}
    <div class="pixel-skeleton-row">
      <i style:width={width}></i>
    </div>
  {/each}
</div>

<style>
  .pixel-skeleton {
    display: grid;
    gap: 8px;
    padding: 10px 8px;
  }

  .pixel-skeleton-row {
    display: flex;
    align-items: center;
    min-height: 20px;
  }

  .pixel-skeleton-row i {
    display: block;
    height: 10px;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    background: var(--surface-3);
    box-shadow: var(--edge-inset);
    animation: pixel-skeleton 900ms steps(2, end) infinite;
  }

  [data-variant='preview'] .pixel-skeleton-row i {
    height: 8px;
  }

  @media (prefers-reduced-motion: reduce) {
    .pixel-skeleton-row i { animation: none; }
  }

  @keyframes pixel-skeleton {
    0%, 49% { opacity: 0.42; }
    50%, 100% { opacity: 0.88; }
  }
</style>

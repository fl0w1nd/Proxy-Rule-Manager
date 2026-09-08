<script lang="ts">
  interface Props {
    id: string;
    label: string;
    items: { value: string; label: string; disabled?: boolean }[];
    value: string;
    onchange?: (value: string) => void;
  }
  let { id, label, items, value = $bindable(), onchange }: Props = $props();

  function select(next: string) {
    value = next;
    onchange?.(next);
  }

  function navigate(event: KeyboardEvent, index: number) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    const enabled = items.filter((item) => !item.disabled);
    const current = enabled.findIndex((item) => item.value === items[index].value);
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? enabled.length - 1
      : (current + (event.key === 'ArrowRight' ? 1 : -1) + enabled.length) % enabled.length;
    if (!enabled[next]) return;
    select(enabled[next].value);
    document.getElementById(`${id}-tab-${enabled[next].value}`)?.focus();
  }
</script>

<div class="pixel-tabs" role="tablist" aria-label={label}>
  {#each items as item, index (item.value)}
    <button type="button" role="tab" id="{id}-tab-{item.value}"
      aria-controls="{id}-panel-{item.value}" aria-selected={value === item.value}
      tabindex={value === item.value ? 0 : -1} disabled={item.disabled}
      onclick={() => select(item.value)} onkeydown={(event) => navigate(event, index)}>
      {item.label}
    </button>
  {/each}
</div>

<style>
  .pixel-tabs { display: flex; width: fit-content; max-width: 100%; }
  button { min-height: 32px; padding: 4px 14px; border: 1px solid var(--border-vis); background: var(--surface-2); color: var(--text); box-shadow: var(--edge-raised); font: 400 12px/20px var(--font-ui); cursor: pointer; }
  button + button { border-left: 0; }
  button:first-child { border-radius: 4px 0 0 4px; }
  button:last-child { border-radius: 0 4px 4px 0; }
  button:only-child { border-radius: 4px; }
  button[aria-selected='false']:hover:not(:disabled) { background: var(--surface-3); }
  button[aria-selected='true'] { background: var(--selected); color: var(--selected-text); box-shadow: var(--edge-pressed); }
  button:disabled { opacity: .45; cursor: not-allowed; }
  button:focus-visible { position: relative; z-index: 1; }
</style>

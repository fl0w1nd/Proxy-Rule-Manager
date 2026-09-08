<script lang="ts">
  interface Props {
    id: string;
    label: string;
    options: { value: string; label: string }[];
    value: string;
    disabled?: boolean;
    onchange?: (value: string) => void;
  }
  let { id, label, options, value = $bindable(), disabled = false, onchange }: Props = $props();
  let root: HTMLDivElement;
  let open = $state(false);
  let highlighted = $state(0);
  const selected = $derived(options.find((option) => option.value === value));

  function show() {
    highlighted = Math.max(0, options.findIndex((option) => option.value === value));
    open = true;
  }
  function choose(index: number) {
    if (!options[index]) return;
    value = options[index].value;
    onchange?.(value);
    open = false;
  }
  function keydown(event: KeyboardEvent) {
    if (event.key === 'Tab') { open = false; return; }
    if (event.key === 'Escape') { if (open) event.stopPropagation(); open = false; return; }
    if (['ArrowDown', 'ArrowUp', 'Home', 'End', 'Enter', ' '].includes(event.key)) {
      event.preventDefault();
      if (!open) { show(); return; }
      if (event.key === 'Enter' || event.key === ' ') { choose(highlighted); return; }
      highlighted = event.key === 'Home' ? 0 : event.key === 'End' ? options.length - 1
        : (highlighted + (event.key === 'ArrowDown' ? 1 : -1) + options.length) % options.length;
    } else if (event.key.length === 1) {
      const index = options.findIndex((option) => option.label.toLowerCase().startsWith(event.key.toLowerCase()));
      if (index >= 0) { event.preventDefault(); open = true; highlighted = index; }
    }
  }
  $effect(() => {
    if (!open) return;
    document.getElementById(`${id}-option-${highlighted}`)?.scrollIntoView?.({ block: 'nearest' });
  });
</script>

<svelte:window onpointerdown={(event) => { if (!root?.contains(event.target as Node)) open = false; }} />
<div class="pixel-select" bind:this={root} onfocusout={(event) => { if (!root.contains(event.relatedTarget as Node)) open = false; }}>
  <button {id} type="button" role="combobox" aria-label={label} aria-haspopup="listbox"
    aria-expanded={open} aria-controls="{id}-options"
    aria-activedescendant={open ? `${id}-option-${highlighted}` : undefined}
    {disabled} onclick={() => open ? open = false : show()} onkeydown={keydown}>
    <span>{selected?.label ?? value}</span>
    <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true" shape-rendering="crispEdges"><path d="M2 4h8v2H8v2H4V6H2z" fill="currentColor" /></svg>
  </button>
  {#if open && !disabled}
    <div id="{id}-options" class="options" role="listbox" aria-label={label}>
      {#each options as option, index (option.value)}
        <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
        <div id="{id}-option-{index}" role="option" tabindex="-1" aria-selected={value === option.value}
          class:highlighted={highlighted === index}
          onpointermove={() => highlighted = index} onpointerdown={(event) => event.preventDefault()}
          onclick={() => choose(index)}>{option.label}</div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .pixel-select { position: relative; min-width: 0; }
  button { display: flex; align-items: center; justify-content: space-between; gap: 12px; width: 100%; min-height: 32px; padding: 4px 8px; border: 1px solid var(--border-vis); border-radius: 3px; background: var(--surface-2); color: var(--text); box-shadow: var(--edge-raised); font: 400 12px/20px var(--font-ui); text-align: left; cursor: pointer; transition: background-color 80ms linear; }
  button span { overflow: hidden; text-overflow: ellipsis; }
  svg { flex-shrink: 0; }
  button:hover:not(:disabled) { background: var(--surface); }
  button[aria-expanded='true'] { box-shadow: var(--edge-pressed); }
  button:disabled { opacity: .45; cursor: not-allowed; }
  .options { position: absolute; top: calc(100% + 4px); left: 0; right: 0; z-index: 20; max-height: 240px; overflow-y: auto; padding: 4px; border: 1px solid var(--border-vis); border-radius: 3px; background: var(--surface); color: var(--text); box-shadow: var(--shadow-popup); animation: pixel-fade 120ms linear; }
  [role='option'] { padding: 6px 8px; font: 400 12px/20px var(--font-ui); cursor: pointer; overflow-wrap: anywhere; }
  .highlighted { background: var(--selected); color: var(--selected-text); }
</style>

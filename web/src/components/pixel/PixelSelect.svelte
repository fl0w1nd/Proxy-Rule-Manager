<script lang="ts">
  interface Props {
    id: string;
    label: string;
    options: { value: string; label: string }[];
    value: string;
    disabled?: boolean;
    size?: 'sm' | 'md';
    class?: string;
    onchange?: (value: string) => void;
  }
  let { id, label, options, value = $bindable(), disabled = false, size = 'md', class: className = '', onchange }: Props = $props();
  let root: HTMLDivElement;
  let trigger: HTMLButtonElement;
  let open = $state(false);
  let highlighted = $state(0);
  let panel = $state({ top: 0, left: 0, width: 0, maxHeight: 240, flip: false });
  const selected = $derived(options.find((option) => option.value === value));

  function place() {
    if (!trigger) return;
    const box = trigger.getBoundingClientRect();
    const gap = 4;
    const below = window.innerHeight - box.bottom - gap;
    const above = box.top - gap;
    const flip = below < 96 && above > below;
    panel = {
      top: flip ? box.top - gap : box.bottom + gap,
      left: Math.min(box.left, Math.max(8, window.innerWidth - box.width - 8)),
      width: box.width,
      maxHeight: Math.max(80, Math.min(240, flip ? above : below)),
      flip,
    };
  }

  function show() {
    highlighted = Math.max(0, options.findIndex((option) => option.value === value));
    place();
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
    place();
    const onMove = () => place();
    window.addEventListener('resize', onMove);
    window.addEventListener('scroll', onMove, true);
    return () => {
      window.removeEventListener('resize', onMove);
      window.removeEventListener('scroll', onMove, true);
    };
  });
  $effect(() => {
    if (!open) return;
    document.getElementById(`${id}-option-${highlighted}`)?.scrollIntoView?.({ block: 'nearest' });
  });
</script>

<svelte:window onpointerdown={(event) => { if (!root?.contains(event.target as Node)) open = false; }} />
<div class="pixel-select {size} {className}" class:open bind:this={root} onfocusout={(event) => { if (!root.contains(event.relatedTarget as Node)) open = false; }}>
  <button bind:this={trigger} {id} type="button" role="combobox" aria-label={label} aria-haspopup="listbox"
    aria-expanded={open} aria-controls="{id}-options"
    aria-activedescendant={open ? `${id}-option-${highlighted}` : undefined}
    {disabled} onclick={() => open ? open = false : show()} onkeydown={keydown}>
    <span>{selected?.label ?? value}</span>
    <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true" shape-rendering="crispEdges"><path d="M2 4h8v2H8v2H4V6H2z" fill="currentColor" /></svg>
  </button>
  {#if open && !disabled}
    <div id="{id}-options" class="options" class:flip={panel.flip} role="listbox" aria-label={label}
      style="top: {panel.top}px; left: {panel.left}px; width: {panel.width}px; max-height: {panel.maxHeight}px;">
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
  .pixel-select.open { z-index: 30; }
  button { display: flex; align-items: center; justify-content: space-between; gap: 8px; width: 100%; min-height: 32px; padding: 4px 8px; border: 1px solid var(--border-vis); border-radius: 3px; background: var(--surface-2); color: var(--text); box-shadow: var(--edge-raised); font: 400 12px/20px var(--font-ui); text-align: left; cursor: pointer; transition: background-color 80ms linear; }
  .pixel-select.sm button { min-height: 28px; padding: 2px 8px; font: 400 11px/18px var(--font-ui); }
  button span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  svg { flex-shrink: 0; }
  button:hover:not(:disabled) { background: var(--surface); }
  button[aria-expanded='true'] { box-shadow: var(--edge-pressed); }
  button:disabled { opacity: .45; cursor: not-allowed; }
  .options { position: fixed; z-index: 40; overflow-y: auto; padding: 4px; border: 1px solid var(--border-vis); border-radius: 3px; background: var(--surface); color: var(--text); box-shadow: var(--shadow-popup); animation: pixel-fade 120ms linear; }
  .options.flip { transform: translateY(-100%); }
  [role='option'] { padding: 4px 8px; font: 400 12px/20px var(--font-ui); cursor: pointer; overflow-wrap: anywhere; border-radius: 2px; }
  .pixel-select.sm [role='option'] { padding: 3px 6px; font: 400 11px/18px var(--font-ui); }
  .highlighted { background: var(--selected); color: var(--selected-text); }
</style>

<script lang="ts">
  import { tick } from 'svelte';

  export interface MenuItem {
    label: string;
    onselect: () => void;
    disabled?: boolean;
    danger?: boolean;
  }

  interface Props {
    label: string;
    items: MenuItem[];
    disabled?: boolean;
    /** 触发按钮文字；默认显示 ⋯。 */
    text?: string;
  }
  let { label, items, disabled = false, text = '⋯' }: Props = $props();
  const id = $props.id();
  let root: HTMLDivElement;
  let trigger: HTMLButtonElement;
  let open = $state(false);
  let panel = $state({ top: 0, right: 0, flip: false });

  async function show() {
    const box = trigger.getBoundingClientRect();
    const below = window.innerHeight - box.bottom;
    const flip = below < 200 && box.top > below;
    panel = { top: flip ? box.top - 4 : box.bottom + 4, right: Math.max(8, window.innerWidth - box.right), flip };
    open = true;
    await tick();
    root.querySelector<HTMLElement>('[role="menuitem"]:not([aria-disabled="true"])')?.focus();
  }
  function hide(focusTrigger = false) {
    open = false;
    if (focusTrigger) trigger?.focus();
  }
  function choose(item: MenuItem) {
    if (item.disabled) return;
    hide(true);
    item.onselect();
  }
  function keydown(event: KeyboardEvent) {
    if (event.key === 'Escape') { event.stopPropagation(); event.preventDefault(); hide(true); return; }
    if (event.key === 'Tab') { hide(); return; }
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    const nodes = [...root.querySelectorAll<HTMLElement>('[role="menuitem"]:not([aria-disabled="true"])')];
    if (!nodes.length) return;
    const current = nodes.indexOf(document.activeElement as HTMLElement);
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? nodes.length - 1
      : (current + (event.key === 'ArrowDown' ? 1 : -1) + nodes.length) % nodes.length;
    nodes[next].focus();
  }
</script>

<svelte:window onpointerdown={(event) => { if (open && !root?.contains(event.target as Node)) hide(); }} />
<div class="pixel-menu" bind:this={root}>
  <button bind:this={trigger} type="button" class="trigger" aria-label={label} aria-haspopup="menu" aria-expanded={open}
    aria-controls="{id}-menu" {disabled} onclick={() => open ? hide() : show()}>{text}</button>
  {#if open}
    <!-- svelte-ignore a11y_interactive_supports_focus -->
    <div id="{id}-menu" class="menu" class:flip={panel.flip} role="menu" aria-label={label} tabindex="-1"
      style="top: {panel.top}px; right: {panel.right}px;" onkeydown={keydown}>
      {#each items as item, index (index)}
        <button type="button" role="menuitem" class:danger={item.danger} aria-disabled={item.disabled || undefined}
          tabindex="-1" onclick={() => choose(item)}>{item.label}</button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .pixel-menu { position: relative; display: inline-flex; flex-shrink: 0; }
  .trigger { min-width: 28px; min-height: 28px; padding: 0 8px; border: 1px solid var(--border-vis); border-radius: 3px; background: var(--surface-2); color: var(--text); box-shadow: var(--edge-raised); font: 400 14px/1 var(--font-ui); cursor: pointer; }
  .trigger:hover:not(:disabled) { background: var(--surface); }
  .trigger[aria-expanded='true'] { box-shadow: var(--edge-pressed); }
  .trigger:disabled { opacity: .45; cursor: not-allowed; }
  .menu { position: fixed; z-index: 40; min-width: 160px; max-width: 280px; max-height: 320px; overflow-y: auto; padding: 4px; display: grid; border: 1px solid var(--border-vis); border-radius: 3px; background: var(--surface); box-shadow: var(--shadow-popup); animation: pixel-fade 120ms linear; }
  .menu.flip { transform: translateY(-100%); }
  [role='menuitem'] { padding: 4px 8px; border: 0; border-radius: 2px; background: none; color: var(--text); font: 400 12px/20px var(--font-ui); text-align: left; cursor: pointer; overflow-wrap: anywhere; }
  [role='menuitem']:hover, [role='menuitem']:focus-visible { background: var(--selected); color: var(--selected-text); outline: none; }
  [role='menuitem'].danger { color: var(--error-border); }
  [role='menuitem'][aria-disabled='true'] { opacity: .45; cursor: not-allowed; background: none; color: var(--text); }
</style>

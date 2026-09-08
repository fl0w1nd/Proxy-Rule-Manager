<script lang="ts">
  interface Props {
    label: string;
    checked?: boolean;
    disabled?: boolean;
    onchange?: (checked: boolean) => void;
  }
  let { label, checked = $bindable(false), disabled = false, onchange }: Props = $props();
</script>

<button class="pixel-switch" type="button" role="switch" aria-checked={checked} {disabled}
  onclick={() => { checked = !checked; onchange?.(checked); }}>
  <span class="track" aria-hidden="true"><span class="thumb"></span></span>
  <span>{label}</span>
</button>

<style>
  .pixel-switch { display: inline-flex; align-items: center; gap: 10px; min-height: 32px; padding: 0; border: 0; background: transparent; color: var(--text); font: 400 12px/20px var(--font-ui); cursor: pointer; }
  .track { display: inline-flex; align-items: center; width: 36px; height: 20px; padding: 2px; border: 1px solid var(--border-vis); border-radius: 3px; background: var(--surface-3); box-shadow: var(--edge-inset); transition: background-color 80ms linear; }
  .thumb { width: 14px; height: 14px; border: 1px solid var(--border-vis); background: var(--surface); box-shadow: var(--edge-raised); }
  [aria-checked='true'] .track { background: var(--accent); }
  [aria-checked='true'] .thumb { margin-left: auto; }
  button:hover:not(:disabled)[aria-checked='true'] .track { background: var(--accent-hover); }
  button:disabled { opacity: .45; cursor: not-allowed; }
</style>

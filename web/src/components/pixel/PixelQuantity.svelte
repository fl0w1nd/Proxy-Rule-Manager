<script lang="ts">
  import { joinQuantity, prettyNumber, quantityUnitOptions, splitQuantity, type QuantityKind } from '../../utils/quantity';
  import PixelSelect from './PixelSelect.svelte';

  interface Props {
    id: string;
    label: string;
    kind: QuantityKind;
    units: readonly string[];
    value: string;
    invalid?: boolean;
    describedby?: string;
    disabled?: boolean;
    onchange?: (value: string) => void;
  }

  let { id, label, kind, units, value = $bindable(), invalid = false, describedby, disabled = false, onchange }: Props = $props();
  const options = $derived(quantityUnitOptions(kind, units));
  let amount = $state('');
  let unit = $state('');
  let origin = $state('');

  $effect.pre(() => {
    const next = String(value);
    if (next === origin) return;
    const parsed = splitQuantity(kind, next, units);
    amount = parsed.amount;
    unit = parsed.unit;
    origin = next;
  });

  function emit(nextAmount: string, nextUnit: string) {
    amount = nextAmount;
    unit = nextUnit;
    const next = joinQuantity(kind, nextAmount, nextUnit);
    origin = next;
    value = next;
    onchange?.(next);
  }

  function onAmountInput(event: Event) {
    const raw = (event.currentTarget as HTMLInputElement).value;
    if (raw && !/^\d*\.?\d*$/.test(raw)) {
      (event.currentTarget as HTMLInputElement).value = amount;
      return;
    }
    emit(raw, unit);
  }

  function onAmountBlur() {
    const n = Number(amount);
    if (!amount.trim() || !Number.isFinite(n)) return;
    emit(prettyNumber(n), unit);
  }
</script>

<div class="pixel-quantity" class:is-invalid={invalid}>
  <input {id} class="pixel-input amount" inputmode="decimal" autocomplete="off" {disabled}
    value={amount} aria-invalid={invalid} aria-describedby={describedby}
    oninput={onAmountInput} onblur={onAmountBlur} />
  <div class="unit">
    <PixelSelect id="{id}-unit" label="{label}单位" {options} {disabled}
      value={unit} onchange={(next) => emit(amount, next)} />
  </div>
</div>

<style>
  .pixel-quantity { display: flex; min-width: 0; }
  .amount { flex: 1; min-width: 0; border-radius: 3px 0 0 3px; font-variant-numeric: tabular-nums; }
  .amount:focus { position: relative; z-index: 1; }
  .unit { flex: 0 0 76px; width: 76px; }
  .unit :global(button) { border-left: 0; border-radius: 0 3px 3px 0; }
  .is-invalid .unit :global(button) { border-color: var(--error-border); }
</style>

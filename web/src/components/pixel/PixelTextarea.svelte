<script lang="ts">
  interface Props {
    value?: string;
    placeholder?: string;
    disabled?: boolean;
    error?: boolean;
    rows?: number;
    id?: string;
    class?: string;
    spellcheck?: boolean | 'true' | 'false';
    'aria-label'?: string;
    oninput?: (value: string, event: Event) => void;
  }

  let {
    value = $bindable(),
    placeholder,
    disabled = false,
    error = false,
    rows = 3,
    id,
    class: className = '',
    spellcheck,
    'aria-label': ariaLabel,
    oninput,
  }: Props = $props();
</script>

<textarea
  {id}
  class="pixel-field {className}"
  class:error
  {placeholder}
  {disabled}
  {rows}
  {spellcheck}
  aria-label={ariaLabel}
  aria-invalid={error || undefined}
  bind:value
  oninput={(event) => oninput?.(event.currentTarget.value, event)}
></textarea>

<style>
  .pixel-field {
    width: 100%;
    min-width: 0;
    box-sizing: border-box;
    padding: 6px 8px;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    box-shadow: var(--edge-inset);
    background: var(--surface);
    color: var(--text);
    font: 400 13px/20px var(--font-code);
    outline: none;
    resize: vertical;
    transition: background-color 80ms linear;
  }
  /* Focus ring comes from the global textarea:focus-visible rule (2px --focus, 3px offset). */
  .pixel-field::placeholder {
    color: var(--sec);
  }
  .pixel-field.error {
    border-color: var(--error-border);
  }
  .pixel-field:disabled {
    opacity: 0.45;
  }
</style>

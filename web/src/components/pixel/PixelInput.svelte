<script lang="ts">
  interface Props {
    value?: string | number;
    type?: string;
    placeholder?: string;
    disabled?: boolean;
    error?: boolean;
    id?: string;
    class?: string;
    spellcheck?: boolean | 'true' | 'false';
    inputmode?: 'none' | 'text' | 'decimal' | 'numeric' | 'tel' | 'search' | 'email' | 'url';
    'aria-label'?: string;
    'aria-describedby'?: string;
    oninput?: (value: string, event: Event) => void;
  }

  let {
    value = $bindable(),
    type = 'text',
    placeholder,
    disabled = false,
    error = false,
    id,
    class: className = '',
    spellcheck,
    inputmode,
    'aria-label': ariaLabel,
    'aria-describedby': ariaDescribedby,
    oninput,
  }: Props = $props();
</script>

<input
  {id}
  {type}
  class="pixel-field {className}"
  class:error
  {placeholder}
  {disabled}
  {spellcheck}
  {inputmode}
  aria-label={ariaLabel}
  aria-invalid={error || undefined}
  aria-describedby={ariaDescribedby}
  bind:value
  oninput={(event) => oninput?.(event.currentTarget.value, event)}
/>

<style>
  .pixel-field {
    width: 100%;
    min-width: 0;
    min-height: 32px;
    box-sizing: border-box;
    padding: 4px 8px;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    box-shadow: var(--edge-inset);
    background: var(--surface);
    color: var(--text);
    font: 400 13px/20px var(--font-code);
    outline: none;
    transition: background-color 80ms linear;
  }
  /* Focus ring comes from the global input:focus-visible rule (2px --focus, 3px offset). */
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

<script lang="ts">
  import PixelIcon from './PixelIcon.svelte';

  interface Props {
    value?: string[];
    placeholder?: string;
    disabled?: boolean;
    error?: boolean;
    id?: string;
    class?: string;
    'aria-label'?: string;
    'aria-describedby'?: string;
    allowDuplicates?: boolean;
    onchange?: (tags: string[]) => void;
  }

  let {
    value = $bindable([]),
    placeholder,
    disabled = false,
    error = false,
    id,
    class: className = '',
    'aria-label': ariaLabel,
    'aria-describedby': ariaDescribedby,
    allowDuplicates = false,
    onchange,
  }: Props = $props();

  let inputValue = $state('');
  let isFocused = $state(false);
  let inputRef: HTMLInputElement | undefined = $state();

  const tags = $derived(value ?? []);
  const currentPlaceholder = $derived(
    tags.length === 0
      ? (placeholder ?? '输入标签后按回车或逗号…')
      : (placeholder ? '' : '添加标签…')
  );

  function addTags(rawTags: string[]) {
    if (disabled) return;
    const cleaned = rawTags
      .map((t) => t.trim())
      .filter((t) => t.length > 0);

    if (!cleaned.length) return;

    let next: string[];
    if (allowDuplicates) {
      next = [...tags, ...cleaned];
    } else {
      const existing = new Set(tags);
      const toAdd = cleaned.filter((t) => !existing.has(t));
      if (!toAdd.length) return;
      next = [...tags, ...toAdd];
    }

    value = next;
    onchange?.(next);
  }

  function removeTag(index: number) {
    if (disabled) return;
    const next = tags.filter((_, i) => i !== index);
    value = next;
    onchange?.(next);
  }

  function commitInput() {
    const raw = inputValue.trim();
    if (raw) {
      addTags(raw.split(/[,，]/));
      inputValue = '';
    }
  }

  function handleKeyDown(event: KeyboardEvent) {
    if (disabled) return;

    if (event.key === 'Enter') {
      event.preventDefault();
      commitInput();
    } else if (event.key === ',' || event.key === '，') {
      event.preventDefault();
      commitInput();
    } else if (event.key === 'Backspace') {
      if (!inputValue && tags.length > 0) {
        removeTag(tags.length - 1);
      }
    } else if (event.key === 'Escape') {
      if (inputValue) {
        event.stopPropagation();
        inputValue = '';
      }
    }
  }

  function handleInput(event: Event) {
    const target = event.currentTarget as HTMLInputElement;
    const val = target.value;
    if (val.includes(',') || val.includes('，')) {
      const parts = val.split(/[,，]/);
      const toAdd = parts.slice(0, -1);
      const remainder = parts[parts.length - 1];
      if (toAdd.length > 0) {
        addTags(toAdd);
      }
      inputValue = remainder.trimStart();
    }
  }

  function handlePaste(event: ClipboardEvent) {
    if (disabled) return;
    const text = event.clipboardData?.getData('text') ?? '';
    if (/[,，\n\r\t]/.test(text)) {
      event.preventDefault();
      const parts = text.split(/[,，\n\r\t]+/).map((s) => s.trim()).filter(Boolean);
      if (parts.length > 0) {
        addTags(parts);
      }
    }
  }

  function handleBlur() {
    isFocused = false;
    commitInput();
  }

  function handleContainerClick(event: MouseEvent) {
    if (disabled) return;
    const target = event.target as HTMLElement | null;
    if (target?.closest('.tag-remove')) return;
    if (target === inputRef) return;
    inputRef?.focus();
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
  class="pixel-tag-input {className}"
  class:focused={isFocused}
  class:error
  class:disabled
  onclick={handleContainerClick}
  role="presentation"
>
  {#each tags as tag, index (index + ':' + tag)}
    <span class="tag-chip" title={tag}>
      <span class="tag-text">{tag}</span>
      {#if !disabled}
        <button
          type="button"
          class="tag-remove"
          aria-label="删除标签 {tag}"
          tabindex="-1"
          onclick={(e) => {
            e.stopPropagation();
            removeTag(index);
          }}
        >
          <PixelIcon name="cross" size={10} />
        </button>
      {/if}
    </span>
  {/each}

  <input
    bind:this={inputRef}
    {id}
    type="text"
    class="tag-input-field"
    placeholder={currentPlaceholder}
    {disabled}
    aria-label={ariaLabel ?? '标签输入'}
    aria-invalid={error || undefined}
    aria-describedby={ariaDescribedby}
    spellcheck="false"
    bind:value={inputValue}
    onfocus={() => { isFocused = true; }}
    onblur={handleBlur}
    onkeydown={handleKeyDown}
    oninput={handleInput}
    onpaste={handlePaste}
  />
</div>

<style>
  .pixel-tag-input {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 5px;
    width: 100%;
    min-width: 0;
    min-height: 34px;
    box-sizing: border-box;
    padding: 3px 6px;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    box-shadow: var(--edge-inset);
    background: var(--surface);
    color: var(--text);
    cursor: text;
    transition: background-color 80ms linear, border-color 80ms linear;
  }

  .pixel-tag-input.focused {
    outline: 2px solid var(--focus);
    outline-offset: 1px;
  }

  .pixel-tag-input.error {
    border-color: var(--error-border);
  }

  .pixel-tag-input.disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .tag-chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    height: 22px;
    box-sizing: border-box;
    padding: 0 5px 0 7px;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    background: var(--surface-2);
    color: var(--text);
    font: 400 12px/20px var(--font-code);
    user-select: none;
    max-width: 100%;
    box-shadow: var(--highlight-top);
    outline: none;
  }

  .tag-text {
    max-width: 160px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .tag-remove {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 14px;
    height: 14px;
    padding: 0;
    margin: 0;
    border: none;
    border-radius: 2px;
    background: transparent;
    color: var(--sec);
    cursor: pointer;
    line-height: 1;
    transition: color 80ms ease, background-color 80ms ease;
    outline: none;
  }

  .tag-remove:focus {
    outline: none;
  }

  .tag-remove:focus-visible {
    outline: 1px solid var(--focus);
    outline-offset: 1px;
  }

  .tag-remove:hover {
    color: var(--diff-remove);
    background: var(--surface-3);
  }

  .tag-input-field {
    flex: 1 1 80px;
    min-width: 60px;
    height: 24px;
    border: none;
    outline: none;
    background: transparent;
    color: var(--text);
    font: 400 13px/24px var(--font-code);
    padding: 0 4px;
    margin: 0;
  }

  .tag-input-field::placeholder {
    color: var(--sec);
  }

  .tag-input-field:disabled {
    cursor: not-allowed;
  }
</style>

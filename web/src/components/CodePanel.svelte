<script lang="ts">
  import type { Snippet } from 'svelte';

  let { filename = '', mode = 'preview', stat, fill = false, children }: {
    filename?: string;
    mode?: 'preview' | 'editor';
    stat: string;
    fill?: boolean;
    children: Snippet;
  } = $props();
</script>

<div class="code-panel" class:fill>
  <header>
    <span class="mode"><i aria-hidden="true"></i> FILE {mode === 'editor' ? 'EDITOR' : 'PREVIEW'}</span>
    <span class="filename" title={filename}>{filename || 'NO FILE'}</span>
    <span class="stat">{stat}</span>
  </header>
  <div class="content">{@render children()}</div>
</div>

<style>
  .code-panel {
    min-width: 0;
    overflow: hidden;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    background: var(--terminal-bg);
    transition: border-color 80ms linear;
  }
  /* 编辑器聚焦不画外框，通过面板边框、标题栏分隔线与指示点变色提示 */
  .code-panel:focus-within { border-color: var(--focus); }
  .code-panel:focus-within header { border-bottom-color: var(--focus); }
  .code-panel:focus-within .mode i { background: var(--focus); }
  .fill { display: flex; flex: 1; flex-direction: column; min-height: 0; height: 100%; }
  header {
    display: grid;
    flex-shrink: 0;
    grid-template-columns: 160px minmax(0, 1fr) auto;
    gap: 12px;
    padding: 8px 12px;
    background: var(--surface-2);
    border-bottom: 1px solid var(--border-vis);
    color: var(--terminal-muted);
    font: 400 12px/20px var(--font-ui);
    transition: border-color 80ms linear;
  }
  .mode { color: var(--terminal-text); white-space: nowrap; }
  i { display: inline-block; width: 6px; height: 6px; margin-right: 6px; background: var(--diff-add); }
  .filename { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .stat { color: var(--sec); white-space: nowrap; }
  .content { min-width: 0; }
  .fill .content { display: flex; flex: 1; flex-direction: column; min-height: 0; overflow: hidden; }
  @media (max-width: 640px) {
    header { grid-template-columns: minmax(0, 1fr) auto; }
    .filename { grid-column: 1 / -1; grid-row: 2; }
  }
</style>

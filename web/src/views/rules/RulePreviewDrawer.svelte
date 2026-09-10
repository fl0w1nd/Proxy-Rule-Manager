<script lang="ts">
  import PixelButton from '../../components/pixel/PixelButton.svelte';
  import PixelDrawer from '../../components/pixel/PixelDrawer.svelte';
  import RulePreviewPanel from '../../components/RulePreviewPanel.svelte';
  import type { RulePreview } from '../../api/client';
  import rulesIcon from '../../assets/icons/nav/rules.svg';

  interface Props {
    open: boolean;
    name: string;
    previewing: boolean;
    preview: RulePreview | null;
    error: string;
  }

  let {
    open = $bindable(false),
    name,
    previewing,
    preview,
    error,
  }: Props = $props();
</script>

<PixelDrawer bind:open title={`预览 · ${name}`} icon={rulesIcon} width="760px">
  <div class="editor-form">
    {#if previewing}<p role="status" class="preview-meta">正在抓取来源并编译…</p>{/if}
    {#if error}<div class="notice error" role="status">{error}</div>{/if}
    {#if preview}
      {#key preview}<RulePreviewPanel {preview} />{/key}
    {/if}
  </div>
  {#snippet footer()}
    <PixelButton onclick={() => { open = false; }}>关闭</PixelButton>
  {/snippet}
</PixelDrawer>

<style>
  .editor-form { display: flex; flex-direction: column; gap: 18px; min-width: 0; padding-bottom: 36px; }
  .preview-meta { margin: 0; color: var(--sec); }
  .notice { display: flex; align-items: center; gap: 10px; padding: 10px 14px; background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 4px; color: var(--text); overflow-wrap: anywhere; }
  .notice.error { background: var(--status-error); }
</style>

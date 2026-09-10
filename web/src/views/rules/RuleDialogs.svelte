<script lang="ts">
  import PixelDialog from '../../components/pixel/PixelDialog.svelte';
  import { retroScroll } from '../../utils/scrollbars';

  interface Props {
    leaveOpen: boolean;
    discardOpen: boolean;
    deleteOpen: boolean;
    refConflictOpen: boolean;
    deletingName: string;
    refConflictList: string[];
    onConfirmLeave: () => void;
    onCancelLeave: () => void;
    onConfirmDiscard: () => void;
    onConfirmDelete: () => void;
  }

  let {
    leaveOpen = $bindable(false),
    discardOpen = $bindable(false),
    deleteOpen = $bindable(false),
    refConflictOpen = $bindable(false),
    deletingName,
    refConflictList,
    onConfirmLeave,
    onCancelLeave,
    onConfirmDiscard,
    onConfirmDelete,
  }: Props = $props();
</script>

<PixelDialog bind:open={leaveOpen} title="切换规则页面？" confirmLabel="放弃修改并切换" cancelLabel="继续编辑" danger
  oncancel={onCancelLeave} onconfirm={onConfirmLeave}>
  当前内容尚未保存，切换后将丢弃这些修改。
</PixelDialog>
<PixelDialog bind:open={discardOpen} title="放弃未保存的修改？" confirmLabel="放弃修改" cancelLabel="继续编辑" danger onconfirm={onConfirmDiscard}>
  当前修改尚未保存，关闭后将丢弃这些修改。
</PixelDialog>
<PixelDialog bind:open={deleteOpen} title="删除规则？" confirmLabel="删除" danger onconfirm={onConfirmDelete}>
  删除 {deletingName} 的规则配置。
</PixelDialog>
<PixelDialog bind:open={refConflictOpen} title="规则引用详情" confirmLabel="知道了" showCancel={false} onconfirm={() => { refConflictOpen = false; }}>
  <div class="ref-conflict-content">
    <p>以下规则正在引用该规则，请先解除引用后再删除：</p>
    <div class="ref-conflict-list" use:retroScroll>
      {#each refConflictList as item}<div class="ref-conflict-item"><code>{item}</code></div>{/each}
    </div>
  </div>
</PixelDialog>

<style>
  .ref-conflict-content { display: grid; gap: 12px; }
  .ref-conflict-list { max-height: 240px; overflow-y: auto; background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 3px; padding: 8px 12px; display: grid; gap: 4px; }
  .ref-conflict-item { padding: 3px 0; border-bottom: 1px dashed var(--border); }
  .ref-conflict-item:last-child { border-bottom: none; }
  code { font: 12px/20px var(--font-code); color: var(--sec); overflow-wrap: anywhere; }
</style>

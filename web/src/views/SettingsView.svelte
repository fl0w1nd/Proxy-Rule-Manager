<script lang="ts">
  import PixelTabs from '../components/pixel/PixelTabs.svelte';
  import PixelDialog from '../components/pixel/PixelDialog.svelte';
  import RuntimeSettingsView from './RuntimeSettingsView.svelte';
  import BackupView from './BackupView.svelte';
  import { settingTabs } from './settings';

  let { onstatechange }: { onstatechange?: (dirty: boolean, saving: boolean) => void } = $props();
  let active = $state('runtime');
  let dirty = $state(false);
  let busy = $state(false);
  let pending = $state('');
  let leaveDialog = $state(false);
  let imported = $state('');

  function stateChanged(nextDirty: boolean, nextBusy: boolean) {
    dirty = nextDirty;
    busy = nextBusy;
    onstatechange?.(dirty, busy);
  }
  function leave(next: string) {
    stateChanged(false, false);
    if (next !== 'config') imported = '';
    active = next;
  }
  function switchTab(next: string) {
    if (next === active || busy) return;
    if (dirty) { pending = next; leaveDialog = true; return; }
    leave(next);
  }
</script>

<div class="settings-view">
  <PixelTabs id="settings" label="系统设置" items={settingTabs.map(item => ({ ...item, disabled: busy }))}
    value={active} onchange={switchTab} />
  {#if active === 'runtime'}
    <RuntimeSettingsView onstatechange={stateChanged} />
  {:else if active === 'config'}
    <div role="tabpanel" id="settings-panel-config" aria-labelledby="settings-tab-config">
      {#await import('./ConfigEditorView.svelte')}
        <p role="status">正在加载编辑器…</p>
      {:then component}
        <component.default onstatechange={stateChanged} draft={imported} />
      {:catch}
        <p role="alert">编辑器加载失败，请刷新页面后重试。</p>
      {/await}
    </div>
  {:else if active === 'backup'}
    <div role="tabpanel" id="settings-panel-backup" aria-labelledby="settings-tab-backup">
      <BackupView onstatechange={stateChanged} onimport={(yaml) => { imported = yaml; leave('config'); }} />
    </div>
  {/if}
</div>
<PixelDialog bind:open={leaveDialog} title="切换设置页面？" confirmLabel="放弃修改并切换" cancelLabel="继续编辑" danger
  oncancel={() => { pending = ''; }} onconfirm={() => { leave(pending); pending = ''; }}>
  当前修改尚未保存，切换后将丢弃这些修改。
</PixelDialog>

<style>
  .settings-view { display: flex; flex-direction: column; gap: 20px; max-width: 1000px; min-width: 0; }
</style>

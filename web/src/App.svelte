<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type UpdateDetail } from './api/client';
  import AdminLayout from './layouts/AdminLayout.svelte';
  import DashboardView from './views/DashboardView.svelte';
  import RulesView from './views/RulesView.svelte';
  import ChangesView from './views/ChangesView.svelte';
  import UpdatesView from './views/UpdatesView.svelte';
  import ClientsView from './views/ClientsView.svelte';
  import SettingsView from './views/SettingsView.svelte';
  import PixelDialog from './components/pixel/PixelDialog.svelte';
  import GeositeView from './views/GeositeView.svelte';
  import PixelToast from './components/pixel/PixelToast.svelte';
  import { finishSummary } from './updateLabels';

  type TabType = 'dashboard' | 'rules' | 'changes' | 'updates' | 'geosite' | 'settings' | 'clients';

  let currentTab = $state<TabType>('dashboard');
  let settingsDirty = $state(false);
  let settingsSaving = $state(false);
  let clientsDirty = $state(false);
  let clientsSaving = $state(false);
  let geositeDirty = $state(false);
  let geositeSaving = $state(false);
  let rulesDirty = $state(false);
  let rulesSaving = $state(false);
  let leaveDialog = $state(false);
  let pendingTab = $state<TabType | null>(null);
  let activeJob = $state<string | null>(null);
  let isUpdating = $derived(!!activeJob);
  let activeRuleId = $state<string | null>(null);
  let currentProcessingRuleId = $state<string | null>(null);
  let toastRef: any = null;

  // View references for refresh
  let dashboardRef = $state<any>(null);
  let rulesRef = $state<any>(null);
  let changesRef = $state<any>(null);
  let updatesRef = $state<any>(null);
  let geositeRef = $state<any>(null);

  function getTabFromHash(): TabType | null {
    try {
      const hash = location.hash.replace(/^#\/?/, '').trim() as TabType;
      if (['dashboard', 'rules', 'changes', 'updates', 'geosite', 'settings', 'clients'].includes(hash)) {
        return hash;
      }
    } catch {}
    return null;
  }

  onMount(() => {
    // Read tab from hash, fallback to sessionStorage
    const hashTab = getTabFromHash();
    if (hashTab) {
      currentTab = hashTab;
    } else {
      try {
        const saved = sessionStorage.getItem('prm-admin-tab') as TabType | null;
        if (saved && ['dashboard', 'rules', 'changes', 'updates', 'geosite', 'settings', 'clients'].includes(saved)) {
          currentTab = saved;
        }
      } catch {}
    }

    const onHashChange = () => {
      const t = getTabFromHash();
      if (t && t !== currentTab) {
        if (currentTab === 'settings' && (settingsDirty || settingsSaving)) {
          history.replaceState(null, '', '#settings');
        }
        if (currentTab === 'rules' && (rulesDirty || rulesSaving)) {
          history.replaceState(null, '', '#rules');
        }
        if (currentTab === 'clients' && (clientsDirty || clientsSaving)) history.replaceState(null, '', '#clients');
        if (currentTab === 'geosite' && (geositeDirty || geositeSaving)) history.replaceState(null, '', '#geosite');
        handleTabChange(t);
      }
    };
    window.addEventListener('hashchange', onHashChange);

    // Check for ongoing job
    api.getCurrentUpdate().then((curr) => {
      if (curr && ['running', 'cancelling'].includes(curr.status)) {
        activeJob = curr.id;
      }
    }).catch(() => {});

    return () => {
      window.removeEventListener('hashchange', onHashChange);
    };
  });

  function handleTabChange(tab: TabType) {
    if (tab === currentTab) return;
    if (settingsSaving || rulesSaving || clientsSaving || geositeSaving) {
      toastRef?.show('正在保存，请稍候', 'info');
      return;
    }
    if (currentTab === 'settings' && settingsDirty) {
      pendingTab = tab;
      leaveDialog = true;
      return;
    }
    if (currentTab === 'rules' && rulesDirty) {
      pendingTab = tab;
      leaveDialog = true;
      return;
    }
    if (currentTab === 'clients' && clientsDirty) {
      pendingTab = tab; leaveDialog = true; return;
    }
    if (currentTab === 'geosite' && geositeDirty) {
      pendingTab = tab; leaveDialog = true; return;
    }
    currentTab = tab;
    settingsDirty = false;
    rulesDirty = false;
    clientsDirty = false;
    geositeDirty = false;
    try {
      location.hash = tab;
      sessionStorage.setItem('prm-admin-tab', tab);
    } catch {}
  }

  async function handleStartUpdate(scope: 'all' | 'rules', ruleIds?: string[]) {
    if (activeJob) return;

    if (scope === 'rules' && ruleIds && ruleIds.length > 0) {
      activeRuleId = ruleIds[0];
    } else {
      activeRuleId = null;
    }

    try {
      const summary = await api.startUpdate({ scope, rule_ids: ruleIds });
      activeJob = summary.id;
      toastRef?.show(
        scope === 'all' ? '已触发全量更新任务' : `已触发规则 [${ruleIds?.join(', ')}] 更新`,
        'info'
      );
    } catch (e: any) {
      activeRuleId = null;
      if (e.status === 409 && e.payload?.error?.details?.current_update_id) {
        activeJob = e.payload.error.details.current_update_id;
        toastRef?.show('已有正在执行的更新任务，已自动连接控制台', 'info');
      } else {
        toastRef?.show(`启动更新失败: ${e.message}`, 'error');
      }
    }
  }

  function handleJobFinish(detail?: UpdateDetail) {
    activeJob = null;
    activeRuleId = null;
    currentProcessingRuleId = null;

    if (detail) {
      if (detail.status === 'completed') {
        toastRef?.show(finishSummary(detail), 'success');
      } else if (detail.status === 'cancelled') {
        toastRef?.show('更新已取消', 'info');
      } else {
        toastRef?.show(finishSummary(detail), detail.status === 'completed_with_warnings' ? 'info' : 'error');
      }
    }

    // Refresh views data
    dashboardRef?.loadData();
    rulesRef?.loadRules();
    changesRef?.loadChanges();
    updatesRef?.loadUpdates();
    geositeRef?.loadGeosite();
  }
</script>

<AdminLayout
  activeTab={currentTab}
  onTabChange={handleTabChange}
  onStartUpdate={handleStartUpdate}
  {activeJob}
  {isUpdating}
  onJobFinish={handleJobFinish}
  onProgressRule={(ruleId) => { currentProcessingRuleId = ruleId; }}
  onErrorToast={(msg) => toastRef?.show(msg, 'error')}
>
  {#if currentTab === 'dashboard'}
    <DashboardView
      bind:this={dashboardRef}
      onViewUpdates={() => handleTabChange('updates')}
    />
  {:else if currentTab === 'rules'}
    <RulesView
      bind:this={rulesRef}
      onStartUpdate={handleStartUpdate}
      {activeRuleId}
      {isUpdating}
      {currentProcessingRuleId}
      onstatechange={(dirty, saving) => { rulesDirty = dirty; rulesSaving = saving; }}
    />
  {:else if currentTab === 'clients'}
    <ClientsView onstatechange={(dirty, saving) => { clientsDirty = dirty; clientsSaving = saving; }} />
  {:else if currentTab === 'changes'}
    <ChangesView bind:this={changesRef} />
  {:else if currentTab === 'updates'}
    <UpdatesView bind:this={updatesRef} />
  {:else if currentTab === 'settings'}
    <SettingsView onstatechange={(dirty, saving) => { settingsDirty = dirty; settingsSaving = saving; }} />
  {:else if currentTab === 'geosite'}
    <GeositeView bind:this={geositeRef} onstatechange={(dirty, saving) => { geositeDirty = dirty; geositeSaving = saving; }} />
  {/if}
</AdminLayout>

<PixelToast bind:this={toastRef} />

<PixelDialog bind:open={leaveDialog} title={currentTab === 'clients' ? '离开客户端管理？' : currentTab === 'geosite' ? '离开 Geosite？' : currentTab === 'rules' ? '离开规则管理？' : '离开系统设置？'}
  confirmLabel="放弃修改并离开" cancelLabel="继续编辑" danger
  oncancel={() => { pendingTab = null; }}
  onconfirm={() => {
    const tab = pendingTab;
    pendingTab = null;
    settingsDirty = false;
    rulesDirty = false;
    clientsDirty = false;
    geositeDirty = false;
    if (tab) handleTabChange(tab);
  }}>
  {currentTab === 'rules' ? '当前文件尚未保存，离开后将丢弃这些修改。' : '当前修改尚未保存，离开后将丢弃这些修改。'}
</PixelDialog>

<script lang="ts">
  import { onMount } from 'svelte';
  import { api, APIRequestError, type UpdateDetail, type UpdateScope } from './api/client';
  import AdminLayout from './layouts/AdminLayout.svelte';
  import DashboardView from './views/DashboardView.svelte';
  import RulesView from './views/RulesView.svelte';
  import ChangesView from './views/ChangesView.svelte';
  import UpdatesView from './views/UpdatesView.svelte';
  import ClientsView from './views/ClientsView.svelte';
  import SettingsView from './views/SettingsView.svelte';
  import PixelDialog from './components/pixel/PixelDialog.svelte';
  import GeoDataView from './views/GeoDataView.svelte';
  import PixelToast from './components/pixel/PixelToast.svelte';
  import { finishSummary, scopeText } from './updateLabels';

  type TabType = 'dashboard' | 'rules' | 'changes' | 'updates' | 'geodata' | 'settings' | 'clients';

  let currentTab = $state<TabType>('dashboard');
  let settingsDirty = $state(false);
  let settingsSaving = $state(false);
  let clientsDirty = $state(false);
  let clientsSaving = $state(false);
  let geoDirty = $state(false);
  let geoSaving = $state(false);
  let rulesDirty = $state(false);
  let rulesSaving = $state(false);
  let leaveDialog = $state(false);
  let pendingTab = $state<TabType | null>(null);
  let activeJob = $state<string | null>(null);
  let startingUpdate = $state(false);
  let isUpdating = $derived(startingUpdate || !!activeJob);
  let activeRuleId = $state<string | null>(null);
  let currentProcessingRuleId = $state<string | null>(null);
  let toastRef: any = null;

  // View references for refresh
  let dashboardRef = $state<any>(null);
  let rulesRef = $state<any>(null);
  let changesRef = $state<any>(null);
  let updatesRef = $state<any>(null);
  let geoRef = $state<any>(null);

  function getTabFromHash(): TabType | null {
    try {
      let hash = location.hash.replace(/^#\/?/, '').trim();
      if (hash === 'geosite' || hash === 'geoip') hash = 'geodata';
      if (['dashboard', 'rules', 'changes', 'updates', 'geodata', 'settings', 'clients'].includes(hash)) {
        return hash as TabType;
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
        let saved = sessionStorage.getItem('prm-admin-tab');
        if (saved === 'geosite' || saved === 'geoip') saved = 'geodata';
        if (saved && ['dashboard', 'rules', 'changes', 'updates', 'geodata', 'settings', 'clients'].includes(saved)) {
          currentTab = saved as TabType;
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
        if (currentTab === 'geodata' && (geoDirty || geoSaving)) history.replaceState(null, '', '#geodata');
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
    if (settingsSaving || rulesSaving || clientsSaving || geoSaving) {
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
    if (currentTab === 'geodata' && geoDirty) {
      pendingTab = tab; leaveDialog = true; return;
    }
    currentTab = tab;
    settingsDirty = false;
    rulesDirty = false;
    clientsDirty = false;
    geoDirty = false;
    try {
      location.hash = tab;
      sessionStorage.setItem('prm-admin-tab', tab);
    } catch {}
  }

  async function handleStartUpdate(scope: UpdateScope, ruleIds?: string[]) {
    if (isUpdating) return;
    startingUpdate = true;

    if (scope === 'rules' && ruleIds?.length === 1) {
      activeRuleId = ruleIds[0];
    } else {
      activeRuleId = null;
    }

    try {
      const summary = await api.startUpdate({ scope, rule_ids: ruleIds });
      activeJob = summary.id;
      toastRef?.show(
        scope === 'rules' ? `已启动 ${summary.requested_rule_ids?.length || 0} 条规则更新` : `已启动${scopeText(summary)}任务`,
        'info'
      );
    } catch (e: any) {
      activeRuleId = null;
      if (e instanceof APIRequestError && e.status === 409 && e.details.current_update_id) {
        activeJob = e.details.current_update_id;
        toastRef?.show('已有正在执行的更新任务，已自动连接控制台', 'info');
      } else {
        toastRef?.show(`启动更新失败: ${e.message}`, 'error');
      }
    } finally {
      startingUpdate = false;
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
    geoRef?.loadProviders();
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
  hasUnsavedChanges={settingsDirty || rulesDirty || clientsDirty || geoDirty}
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
  {:else if currentTab === 'geodata'}
    <GeoDataView bind:this={geoRef} onStartUpdate={handleStartUpdate} {isUpdating} onstatechange={(dirty, saving) => { geoDirty = dirty; geoSaving = saving; }} />
  {/if}
</AdminLayout>

<PixelToast bind:this={toastRef} />

<PixelDialog bind:open={leaveDialog} title={currentTab === 'clients' ? '离开客户端管理？' : currentTab === 'geodata' ? '离开 Geo 数据？' : currentTab === 'rules' ? '离开规则管理？' : '离开系统设置？'}
  confirmLabel="放弃修改并离开" cancelLabel="继续编辑" danger
  oncancel={() => { pendingTab = null; }}
  onconfirm={() => {
    const tab = pendingTab;
    pendingTab = null;
    settingsDirty = false;
    rulesDirty = false;
    clientsDirty = false;
    geoDirty = false;
    if (tab) handleTabChange(tab);
  }}>
  {currentTab === 'rules' ? '当前文件尚未保存，离开后将丢弃这些修改。' : '当前修改尚未保存，离开后将丢弃这些修改。'}
</PixelDialog>

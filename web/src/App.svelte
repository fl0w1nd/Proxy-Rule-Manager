<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type UpdateItem, type UpdateDetail } from './api/client';
  import AdminLayout from './layouts/AdminLayout.svelte';
  import DashboardView from './views/DashboardView.svelte';
  import RulesView from './views/RulesView.svelte';
  import ChangesView from './views/ChangesView.svelte';
  import UpdatesView from './views/UpdatesView.svelte';
  import GeositeView from './views/GeositeView.svelte';
  import PixelToast from './components/pixel/PixelToast.svelte';

  type TabType = 'dashboard' | 'rules' | 'changes' | 'updates' | 'geosite';

  let currentTab = $state<TabType>('dashboard');
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
      if (['dashboard', 'rules', 'changes', 'updates', 'geosite'].includes(hash)) {
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
        if (saved && ['dashboard', 'rules', 'changes', 'updates', 'geosite'].includes(saved)) {
          currentTab = saved;
        }
      } catch {}
    }

    const onHashChange = () => {
      const t = getTabFromHash();
      if (t && t !== currentTab) {
        currentTab = t;
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
    currentTab = tab;
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
        toastRef?.show(`更新任务圆满完成 (成功 ${detail.rules_succeeded} / ${detail.rules_total})`, 'success');
      } else if (detail.status === 'cancelled') {
        toastRef?.show('更新任务已取消', 'info');
      } else {
        toastRef?.show(`更新任务结束，状态：${detail.status}`, 'error');
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
      onStartUpdate={handleStartUpdate}
      onViewUpdates={() => handleTabChange('updates')}
    />
  {:else if currentTab === 'rules'}
    <RulesView
      bind:this={rulesRef}
      onStartUpdate={handleStartUpdate}
      {activeRuleId}
      {isUpdating}
      {currentProcessingRuleId}
    />
  {:else if currentTab === 'changes'}
    <ChangesView bind:this={changesRef} />
  {:else if currentTab === 'updates'}
    <UpdatesView bind:this={updatesRef} />
  {:else if currentTab === 'geosite'}
    <GeositeView bind:this={geositeRef} />
  {/if}
</AdminLayout>

<PixelToast bind:this={toastRef} />

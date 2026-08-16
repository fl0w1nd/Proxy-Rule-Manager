<script lang="ts">
  import type { Snippet } from 'svelte';
  import { onMount } from 'svelte';
  import { api, type UpdateDetail } from '../api/client';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import PixelDrawer from '../components/pixel/PixelDrawer.svelte';
  import UpdateConsole from '../components/UpdateConsole.svelte';

  interface Props {
    activeTab: 'dashboard' | 'rules' | 'changes' | 'updates' | 'geosite';
    onTabChange: (tab: 'dashboard' | 'rules' | 'changes' | 'updates' | 'geosite') => void;
    onStartUpdate: (scope: 'all' | 'rules', ruleIds?: string[]) => void;
    activeJob: string | null;
    isUpdating: boolean;
    onJobFinish: (detail?: UpdateDetail) => void;
    onProgressRule?: (ruleId: string) => void;
    onErrorToast?: (msg: string) => void;
    children?: Snippet;
  }

  let {
    activeTab = 'dashboard',
    onTabChange,
    onStartUpdate,
    activeJob,
    isUpdating,
    onJobFinish,
    onProgressRule,
    onErrorToast,
    children,
  }: Props = $props();

  let sidebarOpen = $state(false);
  let drawerOpen = $state(false);
  let theme = $state<'dark' | 'light'>('dark');
  let configDirty = $state(false);
  let isReloadingConfig = $state(false);
  let dirtyInterval: any = null;
  let runtimeVersion = $state('…');

  onMount(() => {
    // Read initial theme
    const stored = localStorage.getItem('prm-theme') as 'dark' | 'light' | null;
    if (stored === 'light' || stored === 'dark') {
      theme = stored;
      document.documentElement.setAttribute('data-theme', stored);
    }

    // Fetch runtime version for sidebar footer
    api.getStatus().then((s) => {
      runtimeVersion = `v${s.version}`;
    }).catch(() => {});

    // Config dirty check loop
    checkDirty();
    dirtyInterval = setInterval(checkDirty, 10000);

    return () => {
      if (dirtyInterval) clearInterval(dirtyInterval);
    };
  });

  async function checkDirty() {
    try {
      const res = await api.checkConfigDirty();
      configDirty = !!res.changed;
    } catch {
      // ignore
    }
  }

  async function handleReloadConfig() {
    isReloadingConfig = true;
    try {
      await api.reloadConfig();
      configDirty = false;
      location.reload();
    } catch (e: any) {
      if (onErrorToast) {
        onErrorToast(`重载失败: ${e.message}`);
      } else {
        console.error('Config reload failed', e);
      }
    } finally {
      isReloadingConfig = false;
    }
  }

  function toggleTheme() {
    const next = theme === 'dark' ? 'light' : 'dark';
    theme = next;
    document.documentElement.classList.add('theme-switching');
    document.documentElement.setAttribute('data-theme', next);
    try {
      localStorage.setItem('prm-theme', next);
    } catch {}
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        document.documentElement.classList.remove('theme-switching');
      });
    });
  }

  const navItems = [
    { id: 'dashboard', label: '仪表盘', icon: 'dashboard' },
    { id: 'rules', label: '规则状态', icon: 'rules' },
    { id: 'changes', label: '变更对比', icon: 'changes' },
    { id: 'updates', label: '更新日志', icon: 'updates' },
    { id: 'geosite', label: 'Geosite', icon: 'globe' },
  ] as const;

  const currentTitle = $derived(
    navItems.find((n) => n.id === activeTab)?.label || '管理系统'
  );

  // Auto open console drawer when updating starts
  $effect(() => {
    if (activeJob) {
      drawerOpen = true;
    }
  });
</script>

<div class="admin-shell">
  <!-- Sidebar -->
  <aside class="admin-sidebar {sidebarOpen ? 'mobile-open' : ''}">
    <div class="sidebar-brand">
      <img src="/static/icons/prm.svg" class="brand-icon" width="32" height="32" alt="PRM" />
      <div>
        <div class="brand-title">PROXY RULE</div>
        <div class="brand-sub">MANAGER ADMIN</div>
      </div>
    </div>

    <nav class="sidebar-nav">
      {#each navItems as item}
        <button
          class="nav-btn {activeTab === item.id ? 'active' : ''}"
          onclick={() => {
            onTabChange(item.id);
            sidebarOpen = false;
          }}
          type="button"
        >
          <PixelIcon name={item.icon} size={14} />
          <span>{item.label}</span>
          {#if activeTab === item.id}
            <span class="active-indicator"></span>
          {/if}
        </button>
      {/each}
    </nav>

    <div class="sidebar-foot">
      <a href="/" class="nav-btn back-btn" target="_self">
        <PixelIcon name="external" size={12} />
        <span>[ 返回规则站 ]</span>
      </a>
      <div class="sidebar-meta">
        <span class="meta-dot"></span>
        <span class="meta-text">PRM {runtimeVersion}</span>
      </div>
    </div>
  </aside>

  <!-- Mobile Backdrop -->
  {#if sidebarOpen}
    <div class="sidebar-backdrop" onclick={() => (sidebarOpen = false)} role="presentation"></div>
  {/if}

  <!-- Main View Column -->
  <div class="admin-main">
    <!-- Topbar -->
    <header class="admin-topbar">
      <div class="topbar-left">
        <button class="mobile-toggle" onclick={() => (sidebarOpen = !sidebarOpen)} type="button" aria-label="切换菜单">
          ☰
        </button>
        <div class="breadcrumb">
          <span class="bc-root">PRM</span>
          <span class="bc-sep">/</span>
          <span class="bc-cur">{currentTitle}</span>
        </div>
      </div>

      <div class="topbar-right">
        <!-- Live Task Capsule -->
        <button
          class="task-capsule {isUpdating ? 'running' : ''}"
          onclick={() => (drawerOpen = true)}
          type="button"
          title="点击查看实时终端"
        >
          <span class="capsule-led"></span>
          <span class="capsule-text">{isUpdating ? '更新进行中…' : '终端日志'}</span>
        </button>

        <PixelButton variant="primary" size="sm" onclick={() => onStartUpdate('all')}>
          <PixelIcon name="refresh" size={12} color="#ffffff" />
          <span>全部更新</span>
        </PixelButton>

        <PixelButton size="sm" onclick={toggleTheme}>
          [ {theme === 'dark' ? '亮色' : '暗色'} ]
        </PixelButton>
      </div>
    </header>

    <!-- Config Dirty Alert Banner -->
    {#if configDirty}
      <div class="dirty-banner" role="alert">
        <div class="dirty-text">
          <PixelIcon name="warn" size={14} color="var(--orange)" />
          <span>检测到配置文件在外部已被修改，是否立即重新加载？</span>
        </div>
        <div class="dirty-acts">
          <PixelButton size="sm" variant="primary" disabled={isReloadingConfig} onclick={handleReloadConfig}>
            {isReloadingConfig ? '重载中…' : '立即重载'}
          </PixelButton>
          <PixelButton size="sm" variant="ghost" onclick={() => { configDirty = false; }}>
            稍后
          </PixelButton>
        </div>
      </div>
    {/if}

    <!-- Content Workspace -->
    <main class="admin-content">
      {#if children}
        {@render children()}
      {/if}
    </main>
  </div>
</div>

<!-- Global Console Drawer -->
<PixelDrawer
  bind:open={drawerOpen}
  title="实时更新控制台"
  width="540px"
>
  <UpdateConsole
    jobId={activeJob}
    onfinish={onJobFinish}
    onprogressrule={onProgressRule}
    onclose={() => (drawerOpen = false)}
  />
</PixelDrawer>

<style>
  .admin-shell {
    display: flex;
    min-height: 100vh;
    width: 100%;
    background: var(--bg);
  }

  /* Sidebar */
  .admin-sidebar {
    width: 240px;
    background: var(--surface);
    border-right: 2px solid var(--border-vis);
    box-shadow: 2px 0 0 var(--shadow);
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    z-index: 40;
  }

  .sidebar-brand {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 20px 18px;
    border-bottom: 2px dashed var(--border-vis);
    background: var(--surface-2);
  }
  .brand-icon {
    image-rendering: pixelated;
  }
  .brand-title {
    font-family: "Doto", "Space Mono", monospace;
    font-size: 15px;
    font-weight: 800;
    letter-spacing: 0.1em;
    color: var(--display);
    line-height: 1.1;
  }
  .brand-sub {
    font-family: "Space Mono", monospace;
    font-size: 9px;
    color: var(--dim);
    letter-spacing: 0.08em;
    margin-top: 2px;
  }

  .sidebar-nav {
    flex: 1;
    padding: 14px 10px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .nav-btn {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 10px 14px;
    background: transparent;
    border: 1px solid transparent;
    color: var(--sec);
    font-family: "Space Mono", monospace;
    font-size: 13px;
    font-weight: 700;
    cursor: pointer;
    text-align: left;
    text-decoration: none;
    transition: all 80ms steps(2, end);
    position: relative;
  }
  .nav-btn:hover {
    background: var(--surface-2);
    color: var(--display);
    border-color: var(--border);
  }
  .nav-btn.active {
    background: var(--surface-2);
    color: var(--orange);
    border-color: var(--orange);
    box-shadow: 2px 2px 0 var(--shadow);
  }
  .active-indicator {
    width: 6px;
    height: 6px;
    background: var(--orange);
    margin-left: auto;
  }

  .sidebar-foot {
    padding: 14px 10px 18px;
    border-top: 2px dashed var(--border-vis);
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .back-btn {
    font-size: 11px;
    color: var(--dim);
  }
  .sidebar-meta {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 14px;
    font-family: "Space Mono", monospace;
    font-size: 10px;
    color: var(--dim);
  }
  .meta-dot {
    width: 6px;
    height: 6px;
    background: var(--green);
  }

  /* Main Column */
  .admin-main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  /* Topbar */
  .admin-topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 28px;
    background: var(--surface);
    border-bottom: 2px solid var(--border-vis);
    box-shadow: 0 2px 0 var(--shadow);
    gap: 16px;
    flex-wrap: wrap;
  }

  .topbar-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .mobile-toggle {
    display: none;
    background: var(--surface-2);
    border: 1px solid var(--border-vis);
    color: var(--display);
    font-size: 16px;
    padding: 4px 8px;
    cursor: pointer;
  }

  .breadcrumb {
    display: flex;
    align-items: center;
    gap: 8px;
    font-family: "Space Mono", monospace;
    font-size: 13px;
    font-weight: 700;
  }
  .bc-root {
    color: var(--dim);
  }
  .bc-sep {
    color: var(--border-vis);
  }
  .bc-cur {
    color: var(--display);
  }

  .topbar-right {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .task-capsule {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    background: var(--surface-2);
    border: 2px solid var(--border-vis);
    padding: 5px 12px;
    font-family: "Space Mono", monospace;
    font-size: 11px;
    font-weight: 700;
    color: var(--sec);
    cursor: pointer;
    box-shadow: 2px 2px 0 var(--shadow);
    transition: all 80ms steps(2, end);
  }
  .task-capsule:hover {
    border-color: var(--orange);
    color: var(--display);
  }
  .capsule-led {
    width: 6px;
    height: 6px;
    background: var(--dim);
  }
  .task-capsule.running {
    border-color: var(--orange);
    color: var(--orange);
  }
  .task-capsule.running .capsule-led {
    background: var(--orange);
    animation: pixel-signal 600ms steps(2, end) infinite;
  }

  /* Dirty Banner */
  .dirty-banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    padding: 10px 28px;
    background: var(--banner-bg);
    border-bottom: 2px solid var(--banner-border);
    color: var(--banner-text);
    font-family: "Space Mono", monospace;
    font-size: 12px;
    font-weight: 700;
    flex-wrap: wrap;
  }
  .dirty-text {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .dirty-acts {
    display: flex;
    gap: 8px;
  }

  /* Content Workspace */
  .admin-content {
    flex: 1;
    padding: 24px 28px 40px;
    max-width: 1440px;
    width: 100%;
    margin: 0 auto;
  }

  /* Mobile Responsive */
  @media (max-width: 768px) {
    .mobile-toggle {
      display: block;
    }
    .admin-sidebar {
      position: fixed;
      top: 0;
      bottom: 0;
      left: 0;
      transform: translateX(-100%);
      transition: transform 120ms steps(3, end);
    }
    .admin-sidebar.mobile-open {
      transform: translateX(0);
    }
    .sidebar-backdrop {
      position: fixed;
      inset: 0;
      z-index: 35;
      background: var(--backdrop);
    }
    .admin-topbar {
      padding: 12px 16px;
    }
    .admin-content {
      padding: 16px 16px 32px;
    }
  }
</style>

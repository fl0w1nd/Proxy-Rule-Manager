<script lang="ts">
  import type { Snippet } from 'svelte';
  import { onMount } from 'svelte';
  import { api, type UpdateDetail } from '../api/client';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import PixelDrawer from '../components/pixel/PixelDrawer.svelte';
  import UpdateConsole from '../components/UpdateConsole.svelte';
  import iconDashboard from '../assets/icons/nav/dashboard.svg';
  import iconClients from '../assets/icons/nav/clients.svg';
  import iconRules from '../assets/icons/nav/rules.svg';
  import iconChanges from '../assets/icons/nav/changes.svg';
  import iconUpdates from '../assets/icons/nav/updates.svg';
  import iconSettings from '../assets/icons/nav/settings.svg';
  import iconGeosite from '../assets/icons/nav/geosite.svg';
  import iconGeoIP from '../assets/icons/nav/geoip.svg';
  import prmBrandIcon from '../assets/icons/brand/prm.svg';

  interface Props {
    activeTab: 'dashboard' | 'rules' | 'changes' | 'updates' | 'geosite' | 'geoip' | 'settings' | 'clients';
    onTabChange: (tab: 'dashboard' | 'rules' | 'changes' | 'updates' | 'geosite' | 'geoip' | 'settings' | 'clients') => void;
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
    { id: 'dashboard', label: '仪表盘', icon: iconDashboard },
    { id: 'rules', label: '规则管理', icon: iconRules },
    { id: 'clients', label: '客户端', icon: iconClients },
    { id: 'changes', label: 'Diff', icon: iconChanges },
    { id: 'updates', label: '更新日志', icon: iconUpdates },
    { id: 'geosite', label: 'Geosite', icon: iconGeosite },
    { id: 'geoip', label: 'GeoIP', icon: iconGeoIP },
    { id: 'settings', label: '系统设置', icon: iconSettings },
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

  // Lock body overflow when mobile sidebar is open
  $effect(() => {
    if (sidebarOpen) {
      const prev = document.body.style.overflow;
      document.body.style.overflow = 'hidden';
      return () => {
        document.body.style.overflow = prev;
      };
    }
  });
</script>

<div class="admin-shell">
  <!-- Sidebar -->
  <aside class="admin-sidebar {sidebarOpen ? 'mobile-open' : ''}">
    <div class="sidebar-brand">
      <img src={prmBrandIcon} class="brand-icon" width="32" height="32" alt="PRM" />
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
          <img src={item.icon} class="nav-pixel-icon" width="24" height="24" alt="" />
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
        <span>[ 返回首页 ]</span>
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
        <h1 class="topbar-title">{currentTitle}</h1>
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
          <PixelIcon name="refresh" size={12} />
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
          <PixelIcon name="warn" size={16} />
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

  .admin-sidebar {
    width: 240px;
    height: 100vh;
    max-height: 100dvh;
    align-self: flex-start;
    position: sticky;
    top: 0;
    background: var(--surface);
    border-right: 1px solid var(--border-vis);
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    overflow: hidden;
    z-index: 40;
  }

  .sidebar-brand {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 20px 18px;
    border-bottom: 1px solid var(--border-vis);
    background: var(--surface-2);
  }
  .brand-icon {
    image-rendering: pixelated;
  }
  .brand-title {
    font: 400 24px/28px var(--font-display);
    letter-spacing: 0.3px;
    color: var(--display);
    text-shadow: none;
  }
  .brand-sub {
    margin-top: 2px;
    color: var(--dim);
    font: 400 12px/20px var(--font-ui);
  }

  .sidebar-nav {
    flex: 1;
    min-height: 0;
    padding: 14px 10px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    overflow-y: auto;
  }

  .nav-btn {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    padding: 8px 12px;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--sec);
    font: 400 12px/20px var(--font-ui);
    cursor: pointer;
    text-align: left;
    text-decoration: none;
    transition: background-color 80ms linear;
  }
  .nav-pixel-icon {
    width: 24px;
    height: 24px;
    image-rendering: pixelated;
    image-rendering: crisp-edges;
    flex-shrink: 0;
    display: block;
  }
  .nav-btn:hover {
    background: var(--surface-2);
    color: var(--text);
  }
  .nav-btn.active {
    background: var(--selected);
    color: var(--selected-text);
    border-color: transparent;
  }
  .active-indicator {
    width: 6px;
    height: 6px;
    background: currentColor;
    margin-left: auto;
  }

  .sidebar-foot {
    flex-shrink: 0;
    padding: 14px 10px 18px;
    border-top: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .back-btn {
    color: var(--dim);
  }
  .sidebar-meta {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 14px;
    color: var(--dim);
    font: 400 12px/20px var(--font-ui);
  }
  .meta-dot {
    width: 6px;
    height: 6px;
    background: var(--status-success);
  }

  .admin-main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .admin-topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 28px;
    background: var(--surface);
    border-bottom: 1px solid var(--border-vis);
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
    border-radius: 4px;
    box-shadow: var(--edge-raised);
    color: var(--display);
    font-size: 16px;
    padding: 4px 8px;
    cursor: pointer;
  }

  .topbar-title {
    font: 400 20px/24px var(--font-ui);
    color: var(--display);
    letter-spacing: 0;
    text-shadow: none;
    margin: 0;
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
    min-height: 28px;
    background: var(--surface-2);
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    padding: 0 12px;
    font: 400 12px/20px var(--font-ui);
    color: var(--sec);
    cursor: pointer;
    box-shadow: var(--edge-raised);
    transition: background-color 80ms linear;
  }
  .task-capsule:hover {
    background: var(--surface);
    color: var(--text);
  }
  .task-capsule:active {
    box-shadow: var(--edge-pressed);
    transform: translateY(1px);
  }
  .capsule-led {
    width: 6px;
    height: 6px;
    background: var(--dim);
  }
  .task-capsule.running {
    background: var(--status-info);
    color: var(--text);
  }
  .task-capsule.running .capsule-led {
    background: currentColor;
    animation: pixel-signal 600ms steps(2, end) infinite;
  }

  .dirty-banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    padding: 10px 28px;
    background: var(--status-warning);
    border-bottom: 1px solid var(--border-vis);
    color: var(--text);
    font: 400 12px/20px var(--font-ui);
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

  .admin-content {
    flex: 1;
    padding: 24px 28px 40px;
    max-width: 1440px;
    width: 100%;
    margin: 0 auto;
  }

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
      transition: transform 140ms cubic-bezier(.2, .8, .2, 1);
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

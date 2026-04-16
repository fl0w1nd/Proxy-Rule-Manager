"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { formatTimestamp, formatBytes, formatRelativeTime } from "@/lib/utils";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { LoadingState } from "@/components/loading-state";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  RefreshCw,
  Settings,
  FileText,
  Activity,
  CheckCircle,
  XCircle,
  Trash2,
  Loader2,
  AlertTriangle,
  Zap,
  LayoutDashboard,
  Monitor,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  History,
  Code2,
  Shield,
  ImageIcon,
  Globe,
  Clock,
  ArrowUpRight,
  type LucideIcon,
} from "lucide-react";
import { useAuth } from "./auth-provider";
import {
  getStatus,
  executeFullSync,
  executeInit,
  getClients,
  clearActivityRecords,
  StatusResponse,
  ClientConfig,
  ChangeRecordSummary,
  FailureRecord,
  ActivityList,
  ChangeRecordCategory,
  getChangeRecords,
  getChangeDiff,
  getFailureRecords,
  getActivityDates,
} from "@/lib/api-client";
import { RulesManager } from "./rules";
import { ConfigEditor } from "./config";
import { TransformersManager } from "./transformers";
import { ClientsManager } from "./clients";
import { WafManager } from "./waf";
import { IconSetManager } from "./iconset";
import { GeositeManager } from "./geosite";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { AppSidebar } from "@/components/app-sidebar";
import { DiffViewer } from "@/components/diff-viewer";
import { cn } from "@/lib/utils";

const TAB_META: Record<string, { label: string; icon: LucideIcon }> = {
  overview: { label: "概览", icon: LayoutDashboard },
  rules: { label: "规则管理", icon: FileText },
  activity: { label: "活动日志", icon: History },
  transformers: { label: "转换器", icon: Code2 },
  clients: { label: "客户端", icon: Monitor },
  security: { label: "安全防护", icon: Shield },
  iconset: { label: "图标资源", icon: ImageIcon },
  geosite: { label: "Geosite", icon: Globe },
  config: { label: "系统配置", icon: Settings },
};

function getChangeLabel(changeType: ChangeRecordSummary["changeType"]) {
  if (changeType === "created") return "新增";
  if (changeType === "deleted") return "删除";
  return "更新";
}

function getChangeBadgeVariant(changeType: ChangeRecordSummary["changeType"]) {
  if (changeType === "created") return "emerald" as const;
  if (changeType === "deleted") return "rose" as const;
  return "blue" as const;
}

interface ActivityFeedProps {
  compact?: boolean;
  items: ChangeRecordSummary[];
  onViewDiff: (change: ChangeRecordSummary) => void;
  getClientDisplayName: (id: string) => string;
  emptyTitle?: string;
}

function ActivityFeed({
  compact = false,
  items,
  onViewDiff,
  getClientDisplayName,
  emptyTitle = "暂无活动记录",
}: ActivityFeedProps) {
  return (
    <div className="space-y-3">
      {items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <Activity className="w-12 h-12 text-muted-foreground/30 mb-4" />
          <p className="text-sm font-medium text-muted-foreground">{emptyTitle}</p>
        </div>
      ) : (
        items.map((change) => (
          <button
            type="button"
            key={change.id}
            onClick={() => onViewDiff(change)}
            className="group flex w-full cursor-pointer items-start justify-between gap-3 rounded-xl p-3 text-left transition-colors hover:bg-accent"
          >
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 mb-1.5">
                <span className="font-medium text-sm truncate" title={change.ruleName}>
                  {change.ruleName}
                </span>
                <Badge variant={getChangeBadgeVariant(change.changeType)} className="text-[10px] shrink-0">
                  {getChangeLabel(change.changeType)}
                </Badge>
              </div>
              <div className="text-xs text-muted-foreground flex items-center gap-3">
                {change.client && (
                  <span className="flex items-center gap-1">
                    <Monitor className="w-3 h-3" />
                    {getClientDisplayName(change.client)}
                  </span>
                )}
                <span>{formatBytes(change.sizeBytes)}</span>
                <span className="ml-auto font-mono">{formatTimestamp(change.timestamp).split(" ")[0]}</span>
              </div>
            </div>
            {!compact && (
              <span
                className="h-6 w-6 flex items-center justify-center rounded-md opacity-0 group-hover:opacity-100 transition-opacity hover:bg-accent"
                aria-hidden="true"
              >
                <FileText className="w-3 h-3" />
              </span>
            )}
          </button>
        ))
      )}
    </div>
  );
}

interface DashboardProps {
  onBack?: () => void;
}

export function Dashboard({ onBack }: DashboardProps) {
  const { logout, authRequired } = useAuth();
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [clients, setClients] = useState<ClientConfig[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isSyncing, setIsSyncing] = useState(false);
  const [activeTab, setActiveTab] = useState("overview");
  const [needsInit, setNeedsInit] = useState(false);
  const [isInitializing, setIsInitializing] = useState(false);
  const [needsFirstSync, setNeedsFirstSync] = useState(false);
  const [activityDate, setActivityDate] = useState<string>("all");
  const [activityClient, setActivityClient] = useState<string>("all");
  const [isClearingActivity, setIsClearingActivity] = useState(false);
  const [createdChangePage, setCreatedChangePage] = useState(1);
  const [updatedChangePage, setUpdatedChangePage] = useState(1);
  const [failurePage, setFailurePage] = useState(1);
  const [createdChangeData, setCreatedChangeData] = useState<ActivityList<ChangeRecordSummary> | null>(null);
  const [updatedChangeData, setUpdatedChangeData] = useState<ActivityList<ChangeRecordSummary> | null>(null);
  const [failureData, setFailureData] = useState<ActivityList<FailureRecord> | null>(null);
  const [selectedChange, setSelectedChange] = useState<ChangeRecordSummary | null>(null);
  const [diffContent, setDiffContent] = useState("");
  const [isDiffLoading, setIsDiffLoading] = useState(false);
  const activityPageSize = 20;
  const [activityDates, setActivityDates] = useState<string[]>([]);
  const [activityTab, setActivityTab] = useState<ChangeRecordCategory | "failures">("created");
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [isClearActivityDialogOpen, setIsClearActivityDialogOpen] = useState(false);

  const fetchStatus = async () => {
    try {
      const [data, { clients: clientList }] = await Promise.all([
        getStatus(),
        getClients(),
      ]);
      setStatus(data);
      setClients(clientList);
      setNeedsInit((data.needsInit ?? false) || (data.rulesCount + data.geositeRulesCount) === 0);
      const hasNeverSynced = !data.lastSync?.lastFullSyncAt && !data.lastSync?.lastSuccessfulSyncAt;
      setNeedsFirstSync((data.rulesCount + data.geositeRulesCount) > 0 && hasNeverSynced);
    } catch (error) {
      console.error("Failed to fetch status:", error);
      toast.error("获取状态失败");
    } finally {
      setIsLoading(false);
    }
  };

  const getClientDisplayName = (clientId: string): string => {
    const client = clients.find(c => c.id === clientId);
    return client?.displayName || clientId;
  };

  const handleInit = async () => {
    setIsInitializing(true);
    try {
      const result = await executeInit();
      if (result.success) {
        toast.success(`初始化成功！已添加 ${result.rulesCount} 条规则`);
        setNeedsInit(false);
        setNeedsFirstSync(true);
        await fetchStatus();
      } else {
        toast.error(result.message);
      }
    } catch (error) {
      toast.error("初始化失败: " + String(error));
    } finally {
      setIsInitializing(false);
    }
  };

  useEffect(() => {
    fetchStatus();
    let interval = setInterval(fetchStatus, 30000);
    const handleVisibility = () => {
      if (document.hidden) {
        clearInterval(interval);
      } else {
        fetchStatus();
        interval = setInterval(fetchStatus, 30000);
      }
    };
    document.addEventListener("visibilitychange", handleVisibility);
    return () => {
      clearInterval(interval);
      document.removeEventListener("visibilitychange", handleVisibility);
    };
  }, []);

  useEffect(() => {
    const handleResize = () => {
      if (window.innerWidth < 768) {
        setSidebarCollapsed(true);
      }
    };
    handleResize();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  const handleFullSync = async () => {
    setIsSyncing(true);
    try {
      const result = await executeFullSync();
      if (result.success) {
        if (result.changedRules.length > 0) {
          toast.success(`同步成功！${result.changedRules.length} 条规则已更新`);
        } else {
          toast.success("同步成功！本次无规则变更");
        }
        setNeedsFirstSync(false);
      } else {
        toast.warning(`同步完成，但有 ${result.failedRules.length} 条规则失败`);
      }
      await fetchStatus();
    } catch (error) {
      toast.error("同步失败: " + String(error));
    } finally {
      setIsSyncing(false);
    }
  };

  const fetchActivity = useCallback(async () => {
    // Only fetch for relevant tabs
    if (activeTab !== "activity" && activeTab !== "overview") return;

    try {
      const dates = await getActivityDates();
      setActivityDates(dates.dates);

      if (activeTab === "overview") {
        // Overview 获取最近 7 天的记录，最多显示 8 条
        const [createdChanges, updatedChanges, failures] = await Promise.all([
          getChangeRecords(undefined, 1, 8, undefined, 7, "created"),
          getChangeRecords(undefined, 1, 8, undefined, 7, "updated"),
          getFailureRecords(undefined, 1, 8, undefined, 7),
        ]);
        setCreatedChangeData(createdChanges);
        setUpdatedChangeData(updatedChanges);
        setFailureData(failures);
      } else {
        const dateParam = activityDate === "all" ? undefined : activityDate;
        const clientParam = activityClient === "all" ? undefined : activityClient;
        const [createdChanges, updatedChanges, failures] = await Promise.all([
          getChangeRecords(dateParam, createdChangePage, activityPageSize, clientParam, undefined, "created"),
          getChangeRecords(dateParam, updatedChangePage, activityPageSize, clientParam, undefined, "updated"),
          getFailureRecords(dateParam, failurePage, activityPageSize, clientParam),
        ]);
        setCreatedChangeData(createdChanges);
        setUpdatedChangeData(updatedChanges);
        setFailureData(failures);
      }
    } catch (error) {
      console.error("Failed to fetch activity:", error);
      if (activeTab === "activity") toast.error("获取活动记录失败");
    }
  }, [activeTab, activityDate, activityClient, createdChangePage, updatedChangePage, failurePage, activityPageSize]);

  useEffect(() => {
    fetchActivity();
  }, [fetchActivity]);

  useEffect(() => {
    if (activityDate !== "all" && !activityDates.includes(activityDate)) {
      setActivityDate("all");
    }
  }, [activityDate, activityDates]);

  useEffect(() => {
    if (activityClient !== "all" && !clients.some((client) => client.id === activityClient)) {
      setActivityClient("all");
    }
  }, [activityClient, clients]);

  const openChangeDiff = async (change: ChangeRecordSummary) => {
    setSelectedChange(change);
    setDiffContent("");
    setIsDiffLoading(true);
    try {
      const result = await getChangeDiff(change.date, change.fileName);
      setDiffContent(result.diff);
    } catch (error) {
      console.error("Failed to fetch diff:", error);
      setDiffContent("diff 已过期或不可用");
    } finally {
      setIsDiffLoading(false);
    }
  };

  const handleClearActivity = async () => {
    setIsClearingActivity(true);
    try {
      await clearActivityRecords();
      toast.success("活动记录已清空");
      setCreatedChangePage(1);
      setUpdatedChangePage(1);
      setFailurePage(1);
      setActivityDate("all");
      setActivityClient("all");
      await fetchActivity();
    } catch (error) {
      toast.error("清空活动记录失败: " + String(error));
    } finally {
      setIsClearingActivity(false);
    }
  };

  const recentDateOptions = activityDates;
  const createdChangeItems = createdChangeData?.items || [];
  const updatedChangeItems = updatedChangeData?.items || [];
  const failureItems = failureData?.items || [];
  const overviewFailureItems = failureItems.slice(0, 8);
  const createdChangeTotalPages = Math.max(1, Math.ceil((createdChangeData?.total || 0) / (createdChangeData?.pageSize || activityPageSize)));
  const updatedChangeTotalPages = Math.max(1, Math.ceil((updatedChangeData?.total || 0) / (updatedChangeData?.pageSize || activityPageSize)));
  const failureTotalPages = Math.max(1, Math.ceil((failureData?.total || 0) / (failureData?.pageSize || activityPageSize)));
  const currentActivityPage = activityTab === "created"
    ? createdChangePage
    : activityTab === "updated"
      ? updatedChangePage
      : failurePage;
  const currentActivityTotalPages = activityTab === "created"
    ? createdChangeTotalPages
    : activityTab === "updated"
      ? updatedChangeTotalPages
      : failureTotalPages;

  // 客户端覆盖分布（规则和 Geosite 分开统计）
  const clientDistribution = useMemo(() => {
    if (!status || clients.length === 0) return [];
    const normalRules = status.rules || [];
    const geositeRulesList = status.geositeRules || [];
    return clients.map((client) => ({
      id: client.id,
      displayName: client.displayName,
      ruleCount: normalRules.filter((r) => r.clients.includes(client.id)).length,
      geositeCount: geositeRulesList.filter((r) => r.clients.includes(client.id)).length,
    }));
  }, [status, clients]);

  // 最近更新的规则（按 lastUpdated 降序，取前 6 条）
  const recentlyUpdatedRules = useMemo(() => {
    if (!status?.rules) return [];
    return [...status.rules]
      .filter((r) => r.lastUpdated)
      .sort((a, b) => new Date(b.lastUpdated!).getTime() - new Date(a.lastUpdated!).getTime())
      .slice(0, 6);
  }, [status?.rules]);

  // 同步健康状态
  const syncHealthStatus = useMemo(() => {
    if (!status?.lastSync) return "unknown" as const;
    const { lastSuccessfulSyncAt, failedRulesCount } = status.lastSync;
    if (!lastSuccessfulSyncAt) return "never" as const;
    const hoursSince = (Date.now() - new Date(lastSuccessfulSyncAt).getTime()) / (1000 * 60 * 60);
    if (failedRulesCount > 0) return "partial" as const;
    if (hoursSince > 48) return "stale" as const;
    return "healthy" as const;
  }, [status?.lastSync]);

  if (isLoading) {
    return (
      <div className="h-screen flex items-center justify-center bg-background">
        <LoadingState />
      </div>
    );
  }

  return (
    <div className="flex h-screen bg-background overflow-hidden relative">
      {/* Sidebar */}
      <AppSidebar
        activeTab={activeTab}
        onTabChange={setActiveTab}
        onLogout={authRequired ? logout : undefined}
        onHome={onBack}
        version={status?.version}
        isCollapsed={sidebarCollapsed}
        onToggle={() => setSidebarCollapsed(!sidebarCollapsed)}
      />

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {/* Header */}
        <header className="h-16 shrink-0 border-b border-border flex items-center justify-between px-6 bg-background sticky top-0 z-20">
          <div className="flex items-center gap-4">
            <h2 className="text-lg font-semibold tracking-tight flex items-center gap-2">
              {(() => {
                const meta = TAB_META[activeTab];
                const Icon = meta?.icon ?? LayoutDashboard;
                return (
                  <>
                    <Icon className="w-5 h-5 text-muted-foreground" />
                    {meta?.label ?? activeTab}
                  </>
                );
              })()}
            </h2>
          </div>
          <div className="flex items-center gap-3">
            <Button
              variant="default"
              onClick={handleFullSync}
              disabled={isSyncing}
            >
              {isSyncing ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
              同步规则
            </Button>
          </div>
        </header>

        {/* Scrollable Content */}
        <main className={cn("flex-1 p-6 scroll-smooth bg-muted/5", activeTab === "geosite" ? "overflow-hidden" : "overflow-y-auto")}>
          <div className={cn("max-w-screen-2xl mx-auto space-y-6", activeTab === "geosite" && "h-full")}>
            {/* Urgent Alerts */}
            {(needsInit || (needsFirstSync && !needsInit)) && (
              <div className="grid gap-4">
                {needsInit && (
                  <Card className="border-destructive/20 bg-destructive/5">
                    <CardContent className="py-4 flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <AlertTriangle className="w-5 h-5 text-destructive" />
                        <div>
                          <h3 className="font-medium text-foreground">系统未初始化</h3>
                          <p className="text-xs text-muted-foreground">检测到暂无规则配置，建议立即初始化。</p>
                        </div>
                      </div>
                      <Button variant="success" size="sm" onClick={handleInit} disabled={isInitializing}>
                        {isInitializing ? <Loader2 className="w-3 h-3 animate-spin mr-1" /> : <Zap className="w-3 h-3 mr-1" />} 初始化
                      </Button>
                    </CardContent>
                  </Card>
                )}
                {needsFirstSync && !needsInit && (
                  <Card className="border-primary/20 bg-primary/5">
                    <CardContent className="py-4 flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <RefreshCw className="w-5 h-5 text-primary" />
                        <div>
                          <h3 className="font-medium text-foreground">待首次同步</h3>
                          <p className="text-xs text-muted-foreground">规则已就绪，请同步以生成配置文件。</p>
                        </div>
                      </div>
                      <Button size="sm" onClick={handleFullSync} disabled={isSyncing}>
                        立即同步
                      </Button>
                    </CardContent>
                  </Card>
                )}
              </div>
            )}

            {/* Overview Tab Content */}
            {activeTab === 'overview' && (
              <div className="space-y-6">
                {/* Row 1: Sync Health Card + KPIs */}
                <div className="grid grid-cols-1 lg:grid-cols-6 gap-4 sm:gap-6">
                  {/* Sync Health - Main Card */}
                  <Card className="lg:col-span-2 p-5">
                    <div className="flex items-center justify-between mb-4">
                      <p className="text-xs text-muted-foreground uppercase tracking-wider font-semibold">同步状态</p>
                      <Badge variant={
                        syncHealthStatus === "healthy" ? "emerald" :
                        syncHealthStatus === "partial" ? "amber" :
                        syncHealthStatus === "stale" ? "amber" :
                        "secondary"
                      }>
                        {syncHealthStatus === "healthy" ? "正常" :
                         syncHealthStatus === "partial" ? "部分失败" :
                         syncHealthStatus === "stale" ? "长时间未同步" :
                         syncHealthStatus === "never" ? "从未同步" : "未知"}
                      </Badge>
                    </div>
                    <div className="space-y-3">
                      <div className="flex items-center justify-between">
                        <span className="text-xs text-muted-foreground">最近成功同步</span>
                        <span className="text-xs font-mono" title={status?.lastSync?.lastSuccessfulSyncAt ? formatTimestamp(status.lastSync.lastSuccessfulSyncAt) : undefined}>
                          {status?.lastSync?.lastSuccessfulSyncAt ? formatRelativeTime(status.lastSync.lastSuccessfulSyncAt) : '—'}
                        </span>
                      </div>
                      <div className="flex items-center justify-between">
                        <span className="text-xs text-muted-foreground">最近全量同步</span>
                        <span className="text-xs font-mono" title={status?.lastSync?.lastFullSyncAt ? formatTimestamp(status.lastSync.lastFullSyncAt) : undefined}>
                          {status?.lastSync?.lastFullSyncAt ? formatRelativeTime(status.lastSync.lastFullSyncAt) : '—'}
                        </span>
                      </div>
                      <div className="h-px bg-border my-1" />
                      <div className="flex items-center gap-4 text-xs">
                        <span className="text-muted-foreground">最近同步结果</span>
                        <div className="flex items-center gap-3 ml-auto">
                          <span title="处理规则数">处理 <strong className="font-mono">{status?.lastSync?.totalRulesCount || 0}</strong></span>
                          <span title="变更规则数">变更 <strong className="font-mono">{status?.lastSync?.changedRulesCount || 0}</strong></span>
                          <span className={cn((status?.lastSync?.failedRulesCount || 0) > 0 && "text-destructive")} title="失败规则数">
                            失败 <strong className="font-mono">{status?.lastSync?.failedRulesCount || 0}</strong>
                          </span>
                        </div>
                      </div>
                    </div>
                  </Card>

                  {/* KPI Cards */}
                  <Card className="p-5 flex flex-col justify-center gap-1">
                    <p className="text-xs text-muted-foreground uppercase tracking-wider font-semibold">规则数量</p>
                    <div className="text-3xl font-mono font-bold tracking-tight">{status?.rulesCount || 0}</div>
                    <p className="text-[11px] text-muted-foreground">输出文件 {status?.ruleFilesCount || 0}</p>
                  </Card>
                  <Card className="p-5 flex flex-col justify-center gap-1">
                    <p className="text-xs text-muted-foreground uppercase tracking-wider font-semibold">Geosite 数量</p>
                    <div className="text-3xl font-mono font-bold tracking-tight">{status?.geositeRulesCount || 0}</div>
                    <p className="text-[11px] text-muted-foreground">输出文件 {status?.geositeRuleFilesCount || 0}</p>
                  </Card>
                  <Card className="p-5 flex flex-col justify-center gap-1">
                    <p className="text-xs text-muted-foreground uppercase tracking-wider font-semibold">今日新增</p>
                    <div className="text-3xl font-mono font-bold tracking-tight text-emerald-600 dark:text-emerald-400">
                      {status?.todayStats?.createdRecords || 0}
                    </div>
                    <p className="text-[11px] text-muted-foreground">新增活动记录</p>
                  </Card>
                  <Card className="p-5 flex flex-col justify-center gap-1">
                    <p className="text-xs text-muted-foreground uppercase tracking-wider font-semibold">今日更新</p>
                    <div className="text-3xl font-mono font-bold tracking-tight text-sky-600 dark:text-sky-400">
                      {status?.todayStats?.updatedRecords || 0}
                    </div>
                    <p className="text-[11px] text-muted-foreground">更新与删除记录</p>
                  </Card>
                  <Card className="p-5 flex flex-col justify-center gap-1">
                    <p className="text-xs text-muted-foreground uppercase tracking-wider font-semibold">今日异常</p>
                    <div className={cn("text-3xl font-mono font-bold tracking-tight", (status?.todayStats?.failureRecords || 0) > 0 ? "text-destructive" : "")}>
                      {status?.todayStats?.failureRecords || 0}
                    </div>
                    <p className="text-[11px] text-muted-foreground">失败源 {status?.todayStats?.failedSources || 0} 个</p>
                  </Card>
                </div>

                {/* Row 2: Client Distribution + Recent Activity */}
                <div className="grid grid-cols-1 items-stretch lg:grid-cols-3 gap-4 lg:gap-6">
                  {/* Client Distribution + Recently Updated Rules */}
                  <div className="space-y-4 lg:space-y-6 lg:h-[640px] lg:flex lg:flex-col">
                    {/* Client Rules */}
                    <Card className="p-5">
                      <div className="flex items-center justify-between mb-4">
                        <p className="text-xs text-muted-foreground uppercase tracking-wider font-semibold">客户端规则</p>
                        <div className="flex items-center gap-3 text-xs text-muted-foreground font-mono">
                          <span>普通 {status?.rulesCount || 0}</span>
                          <span className="text-border">|</span>
                          <span>Geosite {status?.geositeRulesCount || 0}</span>
                        </div>
                      </div>
                      <div className="space-y-3">
                        {clientDistribution.map((client) => {
                          const normalTotal = Math.max(status?.rulesCount || 0, 1);
                          const geositeTotal = Math.max(status?.geositeRulesCount || 0, 1);
                          const rulePct = Math.round((client.ruleCount / normalTotal) * 100);
                          const geositePct = Math.round((client.geositeCount / geositeTotal) * 100);
                          return (
                            <div key={client.id}>
                              <div className="flex items-center justify-between mb-1.5">
                                <span className="text-sm font-medium">{client.displayName}</span>
                                <div className="flex items-center gap-2 text-xs text-muted-foreground font-mono">
                                  <span>{client.ruleCount} 普通</span>
                                  <span>{client.geositeCount} Geosite</span>
                                </div>
                              </div>
                              <div className="space-y-1">
                                <div className="h-1.5 rounded-full bg-muted overflow-hidden">
                                  <div
                                    className="h-full rounded-full bg-primary transition-all duration-500"
                                    style={{ width: `${rulePct}%` }}
                                  />
                                </div>
                                <div className="h-1.5 rounded-full bg-muted overflow-hidden">
                                  <div
                                    className="h-full rounded-full bg-emerald-500 transition-all duration-500"
                                    style={{ width: `${geositePct}%` }}
                                  />
                                </div>
                                <div className="flex items-center justify-between text-[10px] text-muted-foreground font-mono">
                                  <span>普通 {client.ruleCount}/{status?.rulesCount || 0}</span>
                                  <span>Geosite {client.geositeCount}/{status?.geositeRulesCount || 0}</span>
                                </div>
                              </div>
                            </div>
                          );
                        })}
                        {clientDistribution.length === 0 && (
                          <p className="text-xs text-muted-foreground text-center py-4">暂无客户端配置</p>
                        )}
                      </div>
                    </Card>

                    {/* Recently Updated Rules */}
                    <Card className="p-5 lg:flex-1 lg:min-h-0">
                      <div className="flex items-center justify-between mb-4">
                        <p className="text-xs text-muted-foreground uppercase tracking-wider font-semibold">最近更新规则</p>
                        <Button variant="ghost" size="sm" className="h-7 text-xs text-muted-foreground" onClick={() => setActiveTab('rules')}>
                          查看全部 <ArrowUpRight className="w-3 h-3 ml-1" />
                        </Button>
                      </div>
                      <div className="space-y-2 lg:h-[calc(100%-2.75rem)] lg:overflow-y-auto">
                        {recentlyUpdatedRules.map((rule) => (
                          <div key={rule.name} className="flex items-center justify-between rounded-lg p-2 transition-colors hover:bg-accent">
                            <div className="min-w-0 flex-1">
                              <p className="text-sm font-medium truncate">{rule.name}</p>
                              <p className="text-[11px] text-muted-foreground truncate">{rule.description || "暂无说明"}</p>
                            </div>
                            <div className="flex items-center gap-2 shrink-0 ml-3">
                              <span className="text-[11px] text-muted-foreground font-mono flex items-center gap-1">
                                <Clock className="w-3 h-3" />
                                {rule.lastUpdated ? formatRelativeTime(rule.lastUpdated) : '—'}
                              </span>
                            </div>
                          </div>
                        ))}
                        {recentlyUpdatedRules.length === 0 && (
                          <p className="text-xs text-muted-foreground text-center py-6">暂无更新记录</p>
                        )}
                      </div>
                    </Card>
                  </div>

                  {/* Recent Activity - Unified Card */}
                  <Card className="flex flex-col min-h-[400px] max-h-[640px] lg:col-span-2 lg:h-[640px]">
                    <CardHeader className="flex-row items-center justify-between border-b border-border shrink-0">
                      <CardTitle className="flex items-center gap-2">
                        <Activity className="w-4 h-4 text-muted-foreground" />
                        最近活动
                      </CardTitle>
                      <Button variant="ghost" size="sm" className="h-7 text-xs text-muted-foreground" onClick={() => setActiveTab('activity')}>
                        查看全部 <ArrowUpRight className="w-3 h-3 ml-1" />
                      </Button>
                    </CardHeader>
                    <div className="flex-1 flex flex-col min-h-0">
                      <Tabs defaultValue="created" className="flex-1 flex flex-col min-h-0">
                        <div className="px-4 pt-3 shrink-0">
                          <TabsList className="w-fit">
                            <TabsTrigger value="created">
                              新增记录
                              {(status?.todayStats?.createdRecords || 0) > 0 && (
                                <span className="ml-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-emerald-600 px-1 text-[10px] font-bold text-white">
                                  {status?.todayStats?.createdRecords}
                                </span>
                              )}
                            </TabsTrigger>
                            <TabsTrigger value="updated">
                              更新记录
                              {(status?.todayStats?.updatedRecords || 0) > 0 && (
                                <span className="ml-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-sky-600 px-1 text-[10px] font-bold text-white">
                                  {status?.todayStats?.updatedRecords}
                                </span>
                              )}
                            </TabsTrigger>
                            <TabsTrigger value="failures">
                              同步失败
                              {(status?.todayStats?.failureRecords || 0) > 0 && (
                                <span className="ml-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold text-white">
                                  {status?.todayStats?.failureRecords}
                                </span>
                              )}
                            </TabsTrigger>
                          </TabsList>
                        </div>
                        <TabsContent value="created" className="flex-1 overflow-y-auto px-2 pb-2 mt-0">
                          <ActivityFeed
                            compact
                            items={createdChangeItems}
                            onViewDiff={openChangeDiff}
                            getClientDisplayName={getClientDisplayName}
                            emptyTitle="暂无新增记录"
                          />
                        </TabsContent>
                        <TabsContent value="updated" className="flex-1 overflow-y-auto px-2 pb-2 mt-0">
                          <ActivityFeed
                            compact
                            items={updatedChangeItems}
                            onViewDiff={openChangeDiff}
                            getClientDisplayName={getClientDisplayName}
                            emptyTitle="暂无更新记录"
                          />
                        </TabsContent>
                        <TabsContent value="failures" className="flex-1 overflow-y-auto px-4 pb-4 mt-0">
                          {overviewFailureItems.length === 0 ? (
                            <div className="flex flex-col items-center justify-center py-12 text-center text-muted-foreground">
                              <CheckCircle className="mb-2 w-8 h-8 text-success/60" />
                              <p className="text-sm font-medium">运行正常</p>
                              <p className="text-xs mt-1">暂无失败记录</p>
                            </div>
                          ) : (
                            <div className="space-y-3">
                              {overviewFailureItems.map((f) => (
                                <div key={f.id} className="p-3 bg-destructive/5 rounded-xl border border-destructive/10 text-xs hover:bg-destructive/10 transition-colors">
                                  <div className="flex items-center justify-between mb-1">
                                    <p className="font-semibold text-destructive truncate">{f.ruleName}</p>
                                    <span className="text-[10px] text-muted-foreground font-mono">{formatRelativeTime(f.timestamp)}</span>
                                  </div>
                                  {f.stage && (
                                    <p className="text-[10px] text-muted-foreground mb-1 font-mono">阶段: {f.stage}</p>
                                  )}
                                  <p className="text-muted-foreground line-clamp-2">{f.message}</p>
                                </div>
                              ))}
                            </div>
                          )}
                        </TabsContent>
                      </Tabs>
                    </div>
                  </Card>
                </div>
              </div>
            )}

            {/* Other Tabs */}
            {activeTab === 'rules' && <RulesManager onRefresh={fetchStatus} />}
            {activeTab === 'transformers' && <TransformersManager />}
            {activeTab === 'clients' && <ClientsManager />}
            {activeTab === 'security' && <WafManager />}
            {activeTab === 'iconset' && <IconSetManager />}
            {activeTab === 'geosite' && <GeositeManager onRefresh={fetchStatus} />}
            {activeTab === 'config' && <ConfigEditor onSave={fetchStatus} />}

            {/* Activity Full View */}
            {/* Activity Full View */}
            {activeTab === 'activity' && (
              <Card className="flex flex-col min-h-[600px]">
                <CardHeader className="flex-row items-center justify-between border-b border-border">
                  <div className="space-y-1">
                    <CardTitle className="text-lg flex items-center gap-2">
                      <Activity className="w-5 h-5 text-muted-foreground" />
                      活动日志
                      </CardTitle>
                      <CardDescription />
                  </div>
                  <div className="flex flex-wrap items-center gap-2 sm:gap-3">
                    {/* Pagination Controls */}
                    <div className="flex items-center gap-1 rounded-full border border-border bg-surface-subtle p-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 rounded-full hover:bg-background"
                        disabled={currentActivityPage <= 1}
                        onClick={() => {
                          if (activityTab === "created") {
                            setCreatedChangePage((page) => Math.max(1, page - 1));
                            return;
                          }
                          if (activityTab === "updated") {
                            setUpdatedChangePage((page) => Math.max(1, page - 1));
                            return;
                          }
                          setFailurePage((page) => Math.max(1, page - 1));
                        }}
                      >
                        <ChevronLeft className="w-4 h-4 text-muted-foreground" />
                      </Button>
                      <span className="text-xs font-medium text-muted-foreground min-w-[3rem] text-center font-mono">
                        {`${currentActivityPage} / ${currentActivityTotalPages}`}
                      </span>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 rounded-full hover:bg-background"
                        disabled={currentActivityPage >= currentActivityTotalPages}
                        onClick={() => {
                          if (activityTab === "created") {
                            setCreatedChangePage((page) => Math.min(createdChangeTotalPages, page + 1));
                            return;
                          }
                          if (activityTab === "updated") {
                            setUpdatedChangePage((page) => Math.min(updatedChangeTotalPages, page + 1));
                            return;
                          }
                          setFailurePage((page) => Math.min(failureTotalPages, page + 1));
                        }}
                      >
                        <ChevronRight className="w-4 h-4 text-muted-foreground" />
                      </Button>
                    </div>

                    <div className="h-4 w-px bg-border mx-1" />

                    <Select value={activityClient} onValueChange={(value) => {
                      setActivityClient(value);
                      setCreatedChangePage(1);
                      setUpdatedChangePage(1);
                      setFailurePage(1);
                    }}>
                      <SelectTrigger className="w-full sm:w-[160px] h-9 bg-background">
                        <div className="flex items-center gap-2 text-muted-foreground">
                          <Monitor className="w-4 h-4" />
                          <SelectValue placeholder="客户端" />
                        </div>
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">全部客户端</SelectItem>
                        {clients.map((client) => (
                          <SelectItem key={client.id} value={client.id}>
                            {client.displayName}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>

                    <div className="h-4 w-px bg-border mx-1" />

                    <Select value={activityDate} onValueChange={(value) => {
                      setActivityDate(value);
                      setCreatedChangePage(1);
                      setUpdatedChangePage(1);
                      setFailurePage(1);
                    }}>
                      <SelectTrigger className="w-full sm:w-[140px] h-9 bg-background">
                        <div className="flex items-center gap-2 text-muted-foreground">
                          <CalendarDays className="w-4 h-4" />
                          <SelectValue placeholder="日期范围" />
                        </div>
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">全部日期</SelectItem>
                        {recentDateOptions.map(d => <SelectItem key={d} value={d}>{d}</SelectItem>)}
                      </SelectContent>
                    </Select>

                    <div className="h-4 w-px bg-border mx-1" />

                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setIsClearActivityDialogOpen(true)}
                      disabled={isClearingActivity}
                      className="h-9"
                    >
                      <>
                        {isClearingActivity ? (
                          <Loader2 className="w-3.5 h-3.5 animate-spin" />
                        ) : (
                          <Trash2 className="w-3.5 h-3.5" />
                        )}
                        <span className="hidden sm:inline ml-2">清空记录</span>
                      </>
                    </Button>
                  </div>
                </CardHeader>
                <div className="flex-1 p-6">
                  <Tabs
                    defaultValue="created"
                    value={activityTab}
                    onValueChange={(value) => setActivityTab(value as ChangeRecordCategory | "failures")}
                    className="w-full h-full flex flex-col"
                  >
                    <TabsList className="w-full justify-start mb-6">
                      <TabsTrigger value="created">
                        新增记录
                      </TabsTrigger>
                      <TabsTrigger value="updated">
                        更新记录
                      </TabsTrigger>
                      <TabsTrigger
                        value="failures"
                      >
                        失败日志
                      </TabsTrigger>
                    </TabsList>

                    <TabsContent value="created" className="flex-1 mt-0 outline-none">
                      <div className="border border-border rounded-2xl bg-card overflow-hidden">
                        <ActivityFeed
                          items={createdChangeItems}
                          onViewDiff={openChangeDiff}
                          getClientDisplayName={getClientDisplayName}
                          emptyTitle="暂无新增记录"
                        />
                      </div>
                    </TabsContent>
                    <TabsContent value="updated" className="flex-1 mt-0 outline-none">
                      <div className="border border-border rounded-2xl bg-card overflow-hidden">
                        <ActivityFeed
                          items={updatedChangeItems}
                          onViewDiff={openChangeDiff}
                          getClientDisplayName={getClientDisplayName}
                          emptyTitle="暂无更新记录"
                        />
                      </div>
                    </TabsContent>
                    <TabsContent value="failures" className="flex-1 mt-0 outline-none">
                      <div className="border border-border rounded-2xl bg-card overflow-hidden divide-y divide-border">
                        {failureItems.map(f => (
                          <div key={f.id} className="p-4 hover:bg-muted/10 transition-colors flex items-start justify-between gap-4 group">
                            <div className="space-y-1">
                              <div className="flex items-center gap-2">
                                <span className="font-medium text-destructive flex items-center gap-1.5">
                                  <XCircle className="w-4 h-4" />
                                  {f.ruleName}
                                </span>
                                {f.client && (
                                  <Badge variant="secondary" className="text-[10px] font-normal">
                                    {getClientDisplayName(f.client)}
                                  </Badge>
                                )}
                                <Badge variant="outline" className="text-[10px] font-mono font-normal text-muted-foreground">
                                  {formatTimestamp(f.timestamp)}
                                </Badge>
                              </div>
                              <p className="text-sm text-muted-foreground leading-relaxed">{f.message}</p>
                            </div>
                          </div>
                        ))}
                        {failureItems.length === 0 && (
                          <div className="p-12 text-center">
                            <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-success-soft">
                              <CheckCircle className="w-7 h-7 text-success" />
                            </div>
                            <p className="text-sm font-medium text-foreground">运行正常</p>
                            <p className="text-xs text-muted-foreground mt-1">暂无失败记录</p>
                          </div>
                        )}
                      </div>
                    </TabsContent>
                  </Tabs>
                </div>
              </Card>
            )}
          </div>
        </main>
      </div>

      {/* Diff Dialog */}
      <Dialog open={!!selectedChange} onOpenChange={(open) => !open && setSelectedChange(null)}>
        <DialogContent className="max-w-4xl h-[80vh] flex flex-col">
          <DialogHeader>
            <DialogTitle>{selectedChange ? `${getChangeLabel(selectedChange.changeType)}详情 - ${selectedChange.ruleName}` : "记录详情"}</DialogTitle>
            <DialogDescription className="text-xs text-muted-foreground font-mono">
              {selectedChange && (
                <>
                  {formatTimestamp(selectedChange.timestamp)} • {formatBytes(selectedChange.sizeBytes)} • {selectedChange.fileName}
                </>
              )}
            </DialogDescription>
          </DialogHeader>
          <div className="flex-1 overflow-y-auto overflow-hidden rounded-xl border border-border bg-surface-subtle p-0 font-mono text-sm text-card-foreground">
            {isDiffLoading ? (
              <div className="h-full flex items-center justify-center">
                <Loader2 className="animate-spin w-8 h-8 text-muted-foreground" />
              </div>
            ) : (
              <DiffViewer content={diffContent || "无变更内容或无法加载"} className="border-0 rounded-none h-full" />
            )}
          </div>
        </DialogContent>
      </Dialog>

      {/* Clear Activity Confirm Dialog */}
      <Dialog open={isClearActivityDialogOpen} onOpenChange={setIsClearActivityDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Trash2 className="w-5 h-5 text-destructive" />
              确认清空活动记录
            </DialogTitle>
            <DialogDescription>
              确定要清空所有活动记录吗？
              <span className="block mt-1 text-destructive">此操作不可恢复。</span>
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-3 mt-4">
            <Button
              variant="outline"
              onClick={() => setIsClearActivityDialogOpen(false)}
              disabled={isClearingActivity}
            >
              取消
            </Button>
            <Button
              variant="destructive"
              disabled={isClearingActivity}
              onClick={async () => {
                setIsClearActivityDialogOpen(false);
                await handleClearActivity();
              }}
            >
              {isClearingActivity ? (
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
              ) : (
                <Trash2 className="w-4 h-4 mr-2" />
              )}
              确认清空
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

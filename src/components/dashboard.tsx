"use client";

import { useState, useEffect, useCallback } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
  Plus,
  AlertTriangle,
  Zap,
  LayoutDashboard,
  Monitor,
  CalendarDays,
  ChevronLeft,
  ChevronRight
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
  getChangeRecords,
  getChangeDiff,
  getFailureRecords,
  getActivityDates,
} from "@/lib/api-client";
import { RulesManager } from "./rules-manager";
import { ConfigEditor } from "./config-editor";
import { TransformersManager } from "./transformers-manager";
import { ClientsManager } from "./clients-manager";
import { WafManager } from "./waf-manager";
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
  const [changePage, setChangePage] = useState(1);
  const [failurePage, setFailurePage] = useState(1);
  const [changeData, setChangeData] = useState<ActivityList<ChangeRecordSummary> | null>(null);
  const [failureData, setFailureData] = useState<ActivityList<FailureRecord> | null>(null);
  const [selectedChange, setSelectedChange] = useState<ChangeRecordSummary | null>(null);
  const [diffContent, setDiffContent] = useState("");
  const [isDiffLoading, setIsDiffLoading] = useState(false);
  const activityPageSize = 20;
  const [activityDates, setActivityDates] = useState<string[]>([]);
  const [activityTab, setActivityTab] = useState("changes");
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  const fetchStatus = async () => {
    try {
      const [data, { clients: clientList }] = await Promise.all([
        getStatus(),
        getClients(),
      ]);
      setStatus(data);
      setClients(clientList);
      setNeedsInit((data.needsInit ?? false) || data.rulesCount === 0);
      const hasNeverSynced = !data.lastSync?.lastFullSyncAt && !data.lastSync?.lastSuccessfulSyncAt;
      setNeedsFirstSync(data.rulesCount > 0 && hasNeverSynced);
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

  const formatTimestamp = (value: string) =>
    new Date(value).toLocaleString("zh-CN");

  const formatBytes = (value?: number): string => {
    if (!value && value !== 0) return "-";
    if (value < 1024) return `${value} B`;
    if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
    return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  };

  const getChangeLabel = (changeType: ChangeRecordSummary["changeType"]) => {
    if (changeType === "created") return "新增";
    if (changeType === "deleted") return "删除";
    return "更新";
  };

  const getChangeBadgeClass = (changeType: ChangeRecordSummary["changeType"]) => {
    if (changeType === "created") {
      return "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400";
    }
    if (changeType === "deleted") {
      return "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400";
    }
    return "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400";
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
    const interval = setInterval(fetchStatus, 30000);
    return () => clearInterval(interval);
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
      const dateParam = activityDate === "all" ? undefined : activityDate;
      const clientParam = activityClient === "all" ? undefined : activityClient;

      const [changes, failures, dates] = await Promise.all([
        getChangeRecords(dateParam, changePage, activityPageSize, clientParam),
        getFailureRecords(dateParam, failurePage, activityPageSize, clientParam),
        getActivityDates(),
      ]);
      setChangeData(changes);
      setFailureData(failures);
      setActivityDates(dates.dates);
    } catch (error) {
      console.error("Failed to fetch activity:", error);
      if (activeTab === "activity") toast.error("获取活动记录失败");
    }
  }, [activeTab, activityDate, activityClient, changePage, failurePage, activityPageSize]);

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
    if (!confirm("确定要清空所有活动记录吗？此操作不可恢复。")) {
      return;
    }
    setIsClearingActivity(true);
    try {
      await clearActivityRecords();
      toast.success("活动记录已清空");
      setChangePage(1);
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
  const changeItems = changeData?.items || [];
  const failureItems = failureData?.items || [];
  const changeTotalPages = Math.max(1, Math.ceil((changeData?.total || 0) / (changeData?.pageSize || activityPageSize)));
  const failureTotalPages = Math.max(1, Math.ceil((failureData?.total || 0) / (failureData?.pageSize || activityPageSize)));

  if (isLoading) {
    return (
      <div className="h-screen flex items-center justify-center bg-background">
        <Loader2 className="w-8 h-8 animate-spin text-primary" />
      </div>
    );
  }

  // Activity Feed Component
  const ActivityFeed = ({
    compact = false,
    items = changeItems.slice(0, 5),
    onViewDiff = openChangeDiff
  }: {
    compact?: boolean,
    items?: ChangeRecordSummary[],
    onViewDiff?: (c: ChangeRecordSummary) => void
  }) => (
    <div className="space-y-3">
      {items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <Activity className="w-12 h-12 text-muted-foreground/30 mb-4" />
          <p className="text-sm font-medium text-muted-foreground">暂无活动记录</p>
          <p className="text-xs text-muted-foreground/70 mt-1">同步操作后将在此显示变更日志</p>
        </div>
      ) : (
        items.map((change) => (
          <div
            key={change.id}
            onClick={() => onViewDiff(change)}
            className="flex items-start justify-between gap-3 p-3 rounded-lg border border-border/40 hover:border-border bg-card/50 hover:bg-accent/10 transition-all cursor-pointer group shadow-sm"
          >
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2 mb-1">
                <span className="font-medium text-sm truncate" title={change.ruleName}>
                  {change.ruleName}
                </span>
                <Badge variant="outline" className={`${getChangeBadgeClass(change.changeType)} text-[10px] border-0 shrink-0`}>
                  {getChangeLabel(change.changeType)}
                </Badge>
                {change.client && (
                  <Badge variant="secondary" className="text-[10px] font-normal">
                    {getClientDisplayName(change.client)}
                  </Badge>
                )}
              </div>
              <p className="text-xs text-muted-foreground flex items-center justify-between">
                <span>{formatBytes(change.sizeBytes)}</span>
                <span className="ml-2 font-mono">{formatTimestamp(change.timestamp).split(' ')[0]}</span>
              </p>
            </div>
            {!compact && (
              <Button variant="ghost" size="icon" className="h-6 w-6 opacity-0 group-hover:opacity-100 transition-opacity" onClick={(e) => { e.stopPropagation(); onViewDiff(change); }}>
                <FileText className="w-3 h-3" />
              </Button>
            )}
          </div>
        ))
      )}
    </div>
  );

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
        <header className="h-16 shrink-0 border-b flex items-center justify-between px-6 bg-background/80 backdrop-blur-md sticky top-0 z-20">
          <div className="flex items-center gap-4">
            <h2 className="text-lg font-semibold capitalize tracking-tight flex items-center gap-2">
              <LayoutDashboard className="w-5 h-5 text-muted-foreground" />
              {activeTab === 'overview' ? 'Dashboard' : activeTab}
            </h2>
          </div>
          <div className="flex items-center gap-3">
            <Button
              variant="outline"
              size="sm"
              onClick={handleFullSync}
              disabled={isSyncing}
              className="shadow-sm"
            >
              {isSyncing ? <Loader2 className="w-3.5 h-3.5 mr-2 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5 mr-2" />}
              同步规则
            </Button>
          </div>
        </header>

        {/* Scrollable Content */}
        <main className="flex-1 overflow-y-auto p-6 scroll-smooth bg-muted/5">
          <div className="max-w-screen-2xl mx-auto space-y-6">
            {/* Urgent Alerts */}
            {(needsInit || (needsFirstSync && !needsInit)) && (
              <div className="grid gap-4">
                {needsInit && (
                  <Card className="border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/10">
                    <CardContent className="py-4 flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <AlertTriangle className="w-5 h-5 text-amber-600" />
                        <div>
                          <h3 className="font-medium text-amber-900 dark:text-amber-100">系统未初始化</h3>
                          <p className="text-xs text-amber-700 dark:text-amber-300">检测到暂无规则配置，建议立即初始化。</p>
                        </div>
                      </div>
                      <Button size="sm" onClick={handleInit} disabled={isInitializing} className="bg-amber-600 text-white hover:bg-amber-700">
                        {isInitializing ? <Loader2 className="w-3 h-3 animate-spin mr-1" /> : <Zap className="w-3 h-3 mr-1" />} 初始化
                      </Button>
                    </CardContent>
                  </Card>
                )}
                {needsFirstSync && !needsInit && (
                  <Card className="border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/10">
                    <CardContent className="py-4 flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <RefreshCw className="w-5 h-5 text-blue-600" />
                        <div>
                          <h3 className="font-medium text-blue-900 dark:text-blue-100">待首次同步</h3>
                          <p className="text-xs text-blue-700 dark:text-blue-300">规则已就绪，请同步以生成配置文件。</p>
                        </div>
                      </div>
                      <Button size="sm" onClick={handleFullSync} disabled={isSyncing} className="bg-blue-600 text-white hover:bg-blue-700">
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
                {/* Top Row: Stats & System Status */}
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-5 gap-4 sm:gap-6">
                  <Card className="shadow-sm border-border bg-card hover:bg-accent/5 transition-colors">
                    <CardContent className="p-4 flex flex-col justify-center h-full">
                      <p className="text-[10px] text-muted-foreground uppercase tracking-wider font-semibold mb-1">规则总数</p>
                      <div className="text-2xl font-mono font-bold">{status?.rulesCount || 0}</div>
                    </CardContent>
                  </Card>
                  <Card className="shadow-sm border-border bg-card hover:bg-accent/5 transition-colors">
                    <CardContent className="p-4 flex flex-col justify-center h-full">
                      <p className="text-[10px] text-muted-foreground uppercase tracking-wider font-semibold mb-1">今日变更</p>
                      <div className="text-2xl font-mono font-bold text-primary">{status?.todayStats?.ruleFilesChanged || 0}</div>
                    </CardContent>
                  </Card>
                  <Card className="shadow-sm border-border bg-card hover:bg-accent/5 transition-colors">
                    <CardContent className="p-4 flex flex-col justify-center h-full">
                      <p className="text-[10px] text-muted-foreground uppercase tracking-wider font-semibold mb-1">生成文件</p>
                      <div className="text-2xl font-mono font-bold text-orange-500">{status?.ruleFilesCount || 0}</div>
                    </CardContent>
                  </Card>
                  <Card className="shadow-sm border-border bg-card hover:bg-accent/5 transition-colors">
                    <CardContent className="p-4 flex flex-col justify-center h-full">
                      <p className="text-[10px] text-muted-foreground uppercase tracking-wider font-semibold mb-1">异常记录</p>
                      <div className="text-2xl font-mono font-bold text-destructive">{status?.todayStats?.failureRecords || 0}</div>
                    </CardContent>
                  </Card>
                  <Card className="shadow-sm border-border bg-card hover:bg-accent/5 transition-colors">
                    <CardContent className="p-4 flex flex-col justify-center h-full space-y-2">
                      <div>
                        <p className="text-[10px] text-muted-foreground uppercase tracking-wider font-semibold">上次全量同步</p>
                        <p className="text-xs font-mono truncate" title={status?.lastSync?.lastFullSyncAt ? formatTimestamp(status.lastSync.lastFullSyncAt) : 'N/A'}>
                          {status?.lastSync?.lastFullSyncAt ? formatTimestamp(status.lastSync.lastFullSyncAt) : 'N/A'}
                        </p>
                      </div>
                      <div>
                        <p className="text-[10px] text-muted-foreground uppercase tracking-wider font-semibold">上次成功</p>
                        <p className="text-xs font-mono text-green-600 dark:text-green-400 truncate" title={status?.lastSync?.lastSuccessfulSyncAt ? formatTimestamp(status.lastSync.lastSuccessfulSyncAt) : 'N/A'}>
                          {status?.lastSync?.lastSuccessfulSyncAt ? formatTimestamp(status.lastSync.lastSuccessfulSyncAt) : 'N/A'}
                        </p>
                      </div>
                    </CardContent>
                  </Card>
                </div>

                {/* Content Columns: Active Rules & Activity Feed */}
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 lg:gap-6">
                  {/* Rules List */}
                  <Card className="shadow-sm border-border bg-card flex flex-col h-[600px]">
                    <CardHeader className="flex flex-row items-center justify-between pb-2 border-b border-border/40 shrink-0">
                      <div className="space-y-1">
                        <CardTitle className="text-base font-medium flex items-center gap-2">
                          <FileText className="w-4 h-4 text-primary" />
                          活跃规则
                        </CardTitle>
                        <CardDescription className="text-xs">
                          当前系统中配置的所有规则状态
                        </CardDescription>
                      </div>
                      <div className="flex items-center gap-2">
                        <Button variant="ghost" size="sm" onClick={() => setActiveTab('rules')} className="h-8">
                          <Settings className="w-4 h-4 mr-1" /> 管理
                        </Button>
                        <Button size="sm" onClick={() => setActiveTab('rules')} className="h-8">
                          <Plus className="w-4 h-4 mr-1" /> 新建
                        </Button>
                      </div>
                    </CardHeader>
                    <CardContent className="flex-1 overflow-y-auto p-4">
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                        {status?.rules?.map((rule) => (
                          <div key={rule.name} className="flex items-center justify-between p-3 rounded-lg border bg-muted/10 hover:bg-muted/30 transition-colors">
                            <div className="flex items-center gap-3 min-w-0">
                              <div className={`w-2 h-2 rounded-full ${rule.hasError ? 'bg-destructive' : 'bg-green-500'}`} />
                              <div className="min-w-0 flex-1">
                                <p className="text-sm font-medium truncate">{rule.name}</p>
                                <p className="text-xs text-muted-foreground truncate">{rule.description || "No description"}</p>
                              </div>
                            </div>
                            <div className="flex -space-x-1 shrink-0 ml-2">
                              {rule.clients.slice(0, 3).map((client) => (
                                <div key={client} className="w-5 h-5 rounded-full bg-background border flex items-center justify-center text-[8px] uppercase ring-1 ring-background" title={getClientDisplayName(client)}>
                                  {client.charAt(0)}
                                </div>
                              ))}
                              {rule.clients.length > 3 && (
                                <div className="w-5 h-5 rounded-full bg-background border flex items-center justify-center text-[8px] uppercase ring-1 ring-background text-muted-foreground">
                                  +{rule.clients.length - 3}
                                </div>
                              )}
                            </div>
                          </div>
                        ))}
                      </div>
                    </CardContent>
                  </Card>

                  {/* Recent Activity Feed */}
                  <Card className="shadow-sm border-border bg-card flex flex-col h-[600px]">
                    <CardHeader className="pb-2 border-b border-border/40 shrink-0">
                      <CardTitle className="text-base font-medium flex items-center gap-2">
                        <Activity className="w-4 h-4 text-blue-500" />
                        动态流
                      </CardTitle>
                    </CardHeader>
                    <CardContent className="flex-1 overflow-y-auto p-4">
                      <ActivityFeed compact items={changeItems} onViewDiff={openChangeDiff} />
                      <div className="mt-4 pt-4 border-t border-dashed">
                        <div className="text-xs font-semibold text-muted-foreground mb-2">最近失败</div>
                        {failureItems.length === 0 ? <p className="text-xs text-muted-foreground">无异常记录</p> : (
                          <div className="space-y-2">
                            {failureItems.slice(0, 3).map(f => (
                              <div key={f.id} className="p-2 bg-red-50 dark:bg-red-900/10 rounded border border-red-100 dark:border-red-800/30 text-xs">
                                <p className="font-medium text-destructive truncate">{f.ruleName}</p>
                                <p className="text-muted-foreground truncate">{f.message}</p>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    </CardContent>
                  </Card>
                </div>
              </div>
            )}

            {/* Other Tabs */}
            {activeTab === 'rules' && <RulesManager onRefresh={fetchStatus} />}
            {activeTab === 'transformers' && <TransformersManager />}
            {activeTab === 'clients' && <ClientsManager />}
            {activeTab === 'security' && <WafManager />}
            {activeTab === 'config' && <ConfigEditor onSave={fetchStatus} />}

            {/* Activity Full View */}
            {/* Activity Full View */}
            {activeTab === 'activity' && (
              <Card className="shadow-sm border-border bg-card flex flex-col min-h-[600px]">
                <CardHeader className="flex-row items-center justify-between space-y-0 pb-4 border-b border-border/40">
                  <div className="space-y-1">
                    <CardTitle className="text-lg font-semibold flex items-center gap-2">
                      <Activity className="w-5 h-5 text-primary" />
                      系统活动记录
                    </CardTitle>
                    <CardDescription>
                      查看详细的历史变更与同步日志
                    </CardDescription>
                  </div>
                  <div className="flex flex-wrap items-center gap-2 sm:gap-3">
                    {/* Pagination Controls */}
                    <div className="flex items-center gap-1 bg-muted/30 p-1 rounded-md border border-border/50">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 rounded-sm hover:bg-background hover:shadow-sm"
                        disabled={activityTab === "changes" ? changePage <= 1 : failurePage <= 1}
                        onClick={() => activityTab === "changes" ? setChangePage(p => Math.max(1, p - 1)) : setFailurePage(p => Math.max(1, p - 1))}
                      >
                        <ChevronLeft className="w-4 h-4 text-muted-foreground" />
                      </Button>
                      <span className="text-xs font-medium text-muted-foreground min-w-[3rem] text-center font-mono">
                        {activityTab === "changes" ? `${changePage} / ${changeTotalPages}` : `${failurePage} / ${failureTotalPages}`}
                      </span>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 rounded-sm hover:bg-background hover:shadow-sm"
                        disabled={activityTab === "changes" ? changePage >= changeTotalPages : failurePage >= failureTotalPages}
                        onClick={() => activityTab === "changes" ? setChangePage(p => Math.min(changeTotalPages, p + 1)) : setFailurePage(p => Math.min(failureTotalPages, p + 1))}
                      >
                        <ChevronRight className="w-4 h-4 text-muted-foreground" />
                      </Button>
                    </div>

                    <div className="h-4 w-px bg-border/60 mx-1" />

                    <Select value={activityClient} onValueChange={(value) => { setActivityClient(value); setChangePage(1); setFailurePage(1); }}>
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

                    <div className="h-4 w-px bg-border/60 mx-1" />

                    <Select value={activityDate} onValueChange={setActivityDate}>
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

                    <div className="h-4 w-px bg-border/60 mx-1" />

                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleClearActivity}
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
                <CardContent className="flex-1 p-6">
                  <Tabs defaultValue="changes" value={activityTab} onValueChange={setActivityTab} className="w-full h-full flex flex-col">
                    <TabsList className="w-full justify-start border-b rounded-none bg-transparent p-0 mb-6 h-auto">
                      <TabsTrigger
                        value="changes"
                        className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:bg-transparent px-4 py-2 text-muted-foreground data-[state=active]:text-foreground transition-none shadow-none"
                      >
                        变更记录
                      </TabsTrigger>
                      <TabsTrigger
                        value="failures"
                        className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:bg-transparent px-4 py-2 text-muted-foreground data-[state=active]:text-foreground transition-none shadow-none"
                      >
                        失败日志
                      </TabsTrigger>
                    </TabsList>

                    <TabsContent value="changes" className="flex-1 mt-0 outline-none">
                      <div className="border rounded-lg bg-background/50 overflow-hidden">
                        <ActivityFeed items={changeItems} onViewDiff={openChangeDiff} />
                      </div>
                    </TabsContent>
                    <TabsContent value="failures" className="flex-1 mt-0 outline-none">
                      <div className="border rounded-lg bg-background/50 overflow-hidden divide-y">
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
                                <Badge variant="outline" className="text-[10px] font-mono font-normal text-muted-foreground border-border/50 bg-muted/20">
                                  {formatTimestamp(f.timestamp)}
                                </Badge>
                              </div>
                              <p className="text-sm text-muted-foreground leading-relaxed">{f.message}</p>
                            </div>
                          </div>
                        ))}
                        {failureItems.length === 0 && (
                          <div className="p-12 text-center">
                            <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-gradient-to-br from-green-100/50 to-green-50 dark:from-green-900/20 dark:to-green-800/10 flex items-center justify-center">
                              <CheckCircle className="w-8 h-8 text-green-500/60" />
                            </div>
                            <p className="text-sm font-medium text-foreground">运行正常</p>
                            <p className="text-xs text-muted-foreground mt-1">暂无失败记录</p>
                          </div>
                        )}
                      </div>
                    </TabsContent>
                  </Tabs>
                </CardContent>
              </Card>
            )}
          </div>
        </main>
      </div>

      {/* Diff Dialog */}
      <Dialog open={!!selectedChange} onOpenChange={(open) => !open && setSelectedChange(null)}>
        <DialogContent className="max-w-4xl h-[80vh] flex flex-col">
          <DialogHeader>
            <DialogTitle>变更详情 - {selectedChange?.ruleName}</DialogTitle>
            <DialogDescription className="text-xs text-muted-foreground font-mono">
              {selectedChange && (
                <>
                  {formatTimestamp(selectedChange.timestamp)} • {formatBytes(selectedChange.sizeBytes)} • {selectedChange.fileName}
                </>
              )}
            </DialogDescription>
          </DialogHeader>
          <div className="flex-1 overflow-hidden rounded border bg-card text-card-foreground p-0 font-mono text-sm overflow-y-auto shadow-inner">
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
    </div>
  );
}

"use client";

import { useState, useEffect, useCallback, useRef, startTransition } from "react";
import {
  formatTimestamp,
  formatBytes,
  formatRelativeTime,
  formatTimeUntil,
  formatDurationMs,
} from "@/lib/utils";
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
  HardDrive,
  Calendar,
  AlertCircle,
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
  getChangeRecords,
  getChangeDiff,
  getFailureRecords,
  getActivityDates,
  getFailingSources,
  getDiskUsage,
  getWafStats,
  getSyncProgress,
  cancelSync,
  type FailingSource,
  type DiskUsageResponse,
  type DiskUsageBucket,
  type WafStats,
  type SyncProgress,
} from "@/lib/api-client";
import { SyncProgressPill } from "./sync-progress-pill";
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

// Failure records use a special "geosite:{provider}" rule name to mark a
// whole-provider geosite outage (the per-list `geosite_*` failures stay
// hidden because they'd drown out the regular rule feed). This renders that
// distinction so admins immediately recognise it as infrastructure-level,
// not a per-rule problem.
function FailureRuleLabel({ ruleName, className }: { ruleName: string; className?: string }) {
  if (ruleName.startsWith("geosite-stale:")) {
    const provider = ruleName.slice("geosite-stale:".length) || "unknown";
    return (
      <span className={cn("inline-flex items-center gap-1.5", className)}>
        <Globe className="w-3.5 h-3.5 shrink-0" />
        <span className="truncate">{provider}</span>
        <Badge variant="destructive" className="text-[10px] shrink-0 font-normal">
          Geosite 列表已删除
        </Badge>
      </span>
    );
  }
  if (ruleName.startsWith("geosite:")) {
    const provider = ruleName.slice("geosite:".length) || "unknown";
    return (
      <span className={cn("inline-flex items-center gap-1.5", className)}>
        <Globe className="w-3.5 h-3.5 shrink-0" />
        <span className="truncate">{provider}</span>
        <Badge variant="amber" className="text-[10px] shrink-0 font-normal">
          Geosite 源
        </Badge>
      </span>
    );
  }
  return <span className={cn("truncate", className)}>{ruleName}</span>;
}

// SyncHealthCard is the dashboard hero — answers "is everything OK?" in one
// glance. Status badge + dataset summary on top, last/next sync timeline in
// the middle, last sync result counts at the bottom. We keep all the secondary
// info inline so admins don't have to scan a column of stat cards.
function SyncHealthCard({
  status,
  health,
  diskTotal,
  clientCount,
}: {
  status: StatusResponse | null;
  health: "healthy" | "partial" | "stale" | "never" | "unknown";
  diskTotal?: number;
  clientCount: number;
}) {
  const lastSync = status?.lastSync;
  const lastSuccess = lastSync?.lastSuccessfulSyncAt ?? null;
  const lastFull = lastSync?.lastFullSyncAt ?? null;
  const failed = lastSync?.failedRulesCount ?? 0;
  const changed = lastSync?.changedRulesCount ?? 0;
  const total = lastSync?.totalRulesCount ?? 0;
  const duration = lastSync?.lastSyncDurationMs ?? null;
  const next = status?.nextSyncAt ?? "";
  const scheduleMode = status?.scheduleMode;

  const badge = {
    healthy: { label: "正常", variant: "emerald" as const },
    partial: { label: "部分失败", variant: "amber" as const },
    stale: { label: "长时间未同步", variant: "amber" as const },
    never: { label: "从未同步", variant: "secondary" as const },
    unknown: { label: "未知", variant: "secondary" as const },
  }[health];

  return (
    <Card className="p-4">
      <div className="flex flex-wrap items-center gap-3 mb-3">
        <Badge variant={badge.variant} className="text-xs px-2 py-0.5">
          {badge.label}
        </Badge>
        <span className="text-sm text-muted-foreground">
          {(status?.rulesCount ?? 0).toLocaleString()} 普通 ·{" "}
          {(status?.geositeRulesCount ?? 0).toLocaleString()} Geosite ·{" "}
          {clientCount} 个客户端
        </span>
        <span className="ml-auto inline-flex items-center gap-x-5 gap-y-1 text-xs flex-wrap justify-end">
          <span className="text-muted-foreground">
            处理 <strong className="font-mono text-foreground">{total}</strong>
          </span>
          <span className="text-muted-foreground">
            变更 <strong className="font-mono text-foreground">{changed}</strong>
          </span>
          <span className={cn("text-muted-foreground", failed > 0 && "text-destructive")}>
            失败 <strong className="font-mono">{failed}</strong>
          </span>
          {typeof diskTotal === "number" && diskTotal > 0 && (
            <span className="text-muted-foreground font-mono inline-flex items-center gap-1.5">
              <HardDrive className="w-3.5 h-3.5" />
              {formatBytes(diskTotal)}
            </span>
          )}
        </span>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 sm:gap-5 pt-3 border-t border-border">
        <SyncTimelineSlot
          icon={<Clock className="w-3.5 h-3.5" />}
          label="最近成功同步"
          tooltip={lastSuccess ? formatTimestamp(lastSuccess) : undefined}
          value={lastSuccess ? formatRelativeTime(lastSuccess) : "—"}
        />
        <SyncTimelineSlot
          icon={<RefreshCw className="w-3.5 h-3.5" />}
          label="最近全量同步"
          tooltip={lastFull ? formatTimestamp(lastFull) : undefined}
          value={lastFull ? formatRelativeTime(lastFull) : "—"}
          subtitle={duration != null ? `耗时 ${formatDurationMs(duration)}` : undefined}
        />
        <SyncTimelineSlot
          icon={<Calendar className="w-3.5 h-3.5" />}
          label="下次计划同步"
          tooltip={next ? formatTimestamp(next) : undefined}
          value={next ? formatTimeUntil(next) : "未配置"}
          subtitle={scheduleMode === "cron" ? "cron 计划" : scheduleMode === "interval" ? "间隔模式" : undefined}
        />
      </div>
    </Card>
  );
}

function SyncTimelineSlot({
  icon,
  label,
  value,
  tooltip,
  subtitle,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  tooltip?: string;
  subtitle?: string;
}) {
  return (
    <div className="min-w-0 flex items-baseline gap-3 sm:flex-col sm:gap-0.5 sm:items-start">
      <p className="text-[11px] text-muted-foreground uppercase tracking-wider font-medium inline-flex items-center gap-1.5 shrink-0">
        {icon}
        {label}
      </p>
      <div className="flex items-baseline gap-2 min-w-0">
        <p className="text-sm font-mono text-foreground truncate" title={tooltip}>
          {value}
        </p>
        {subtitle && <p className="text-[10px] text-muted-foreground font-mono shrink-0">{subtitle}</p>}
      </div>
    </div>
  );
}

const DISK_BUCKET_LABELS: Record<DiskUsageBucket["key"], string> = {
  rules: "Rules 输出",
  geosite: "Geosite 缓存",
  sources: "本地源",
  iconset: "图标资源",
  client: "客户端文件",
  db: "SQLite DB",
};
const DISK_BUCKET_COLORS: Record<DiskUsageBucket["key"], string> = {
  rules: "bg-primary",
  geosite: "bg-emerald-500",
  sources: "bg-sky-500",
  iconset: "bg-fuchsia-500",
  client: "bg-orange-500",
  db: "bg-amber-500",
};

function DiskUsageCard({ usage, className }: { usage: DiskUsageResponse | null; className?: string }) {
  if (!usage || usage.total === 0) {
    return (
      <Card className={cn("p-4", className)}>
        <div className="flex items-center justify-between">
          <p className="text-xs text-muted-foreground uppercase tracking-wider font-semibold inline-flex items-center gap-1.5">
            <HardDrive className="w-3.5 h-3.5" />
            磁盘占用
          </p>
          <span className="text-xs text-muted-foreground">尚未生成数据</span>
        </div>
      </Card>
    );
  }
  const visible = usage.buckets.filter((b) => b.bytes > 0);
  return (
    <Card className={cn("p-4 flex flex-col", className)}>
      <div className="flex items-center justify-between mb-2">
        <p className="text-xs text-muted-foreground uppercase tracking-wider font-semibold inline-flex items-center gap-1.5">
          <HardDrive className="w-3.5 h-3.5" />
          磁盘占用
        </p>
        <span className="text-sm font-mono font-semibold text-foreground">{formatBytes(usage.total)}</span>
      </div>

      {/* Single horizontal stacked bar — proportional to bytes. Tiny buckets
          (< 0.5% of total) are skipped so the bar stays visually scannable. */}
      <div className="h-1.5 rounded-full overflow-hidden flex bg-muted mb-2.5">
        {visible.map((b) => {
          const pct = (b.bytes / usage.total) * 100;
          if (pct < 0.5) return null;
          return (
            <div
              key={b.key}
              className={cn(DISK_BUCKET_COLORS[b.key], "transition-all duration-500")}
              style={{ width: `${pct}%` }}
              title={`${DISK_BUCKET_LABELS[b.key]} ${formatBytes(b.bytes)}`}
            />
          );
        })}
      </div>

      <ul className="grid grid-cols-2 sm:grid-cols-3 gap-x-5 gap-y-1 text-xs">
        {visible.map((b) => (
          <li key={b.key} className="flex items-center justify-between gap-2 min-w-0">
            <span className="inline-flex items-center gap-1.5 min-w-0 truncate">
              <span className={cn(DISK_BUCKET_COLORS[b.key], "size-2 rounded-full shrink-0")} />
              <span className="truncate text-muted-foreground">{DISK_BUCKET_LABELS[b.key]}</span>
            </span>
            <span className="font-mono text-foreground shrink-0">{formatBytes(b.bytes)}</span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

function WafSummaryCard({ stats, onJump }: { stats: WafStats | null; onJump: () => void }) {
  const total = stats?.bans.total ?? 0;
  const permanent = stats?.bans.permanent ?? 0;
  const blocked = stats?.temporary.currentlyBlocked ?? 0;
  const tracked = stats?.temporary.totalTracked ?? 0;
  return (
    <Card className="p-4 flex flex-col">
      <div className="flex items-center justify-between mb-2">
        <p className="text-xs text-muted-foreground uppercase tracking-wider font-semibold inline-flex items-center gap-1.5">
          <Shield className="w-3.5 h-3.5" />
          WAF 防护
        </p>
        <Button variant="ghost" size="sm" className="h-6 px-2 text-xs text-muted-foreground" onClick={onJump}>
          详情 <ArrowUpRight className="w-3 h-3 ml-1" />
        </Button>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <div className={cn("text-xl font-mono font-semibold tracking-tight leading-tight", total > 0 ? "text-foreground" : "text-muted-foreground")}>
            {total}
          </div>
          <p className="text-[11px] text-muted-foreground mt-0.5">活动封禁{permanent > 0 ? `（永久 ${permanent}）` : ""}</p>
        </div>
        <div>
          <div className={cn("text-xl font-mono font-semibold tracking-tight leading-tight", blocked > 0 ? "text-destructive" : "text-muted-foreground")}>
            {blocked}
          </div>
          <p className="text-[11px] text-muted-foreground mt-0.5">当前限速{tracked > 0 ? ` · 跟踪 ${tracked}` : ""}</p>
        </div>
      </div>
    </Card>
  );
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
}

function ActivityFeed({ compact = false, items, onViewDiff, getClientDisplayName }: ActivityFeedProps) {
  return (
    <div className="space-y-3">
      {items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <Activity className="w-12 h-12 text-muted-foreground/30 mb-4" />
          <p className="text-sm font-medium text-muted-foreground">暂无活动记录</p>
          <p className="text-xs text-muted-foreground/70 mt-1">同步操作后将在此显示变更日志</p>
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
  // syncProgress is kept fresh by polling; when running=true the top-right
  // action switches to SyncProgressPill.
  // syncKickoff stays true briefly after POST /sync/full until polling sees
  // running=true, which prevents duplicate clicks.
  const [syncProgress, setSyncProgress] = useState<SyncProgress | null>(null);
  const [syncKickoff, setSyncKickoff] = useState(false);
  const [isCancellingSync, setIsCancellingSync] = useState(false);
  // lastShownJobIdRef deduplicates completion snapshots that have already
  // triggered a toast.
  const lastShownJobIdRef = useRef<string | null>(null);
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
  const [isClearActivityDialogOpen, setIsClearActivityDialogOpen] = useState(false);
  const [failingSources, setFailingSources] = useState<FailingSource[]>([]);
  const [diskUsage, setDiskUsage] = useState<DiskUsageResponse | null>(null);
  const [wafStats, setWafStats] = useState<WafStats | null>(null);
  const diffRequestRef = useRef(0);
  // Stable timestamp for sync health calculation, avoiding Date.now() during render
  const [now, setNow] = useState(() => Date.now());

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

  // Aux data shown only on the overview tab (failing sources, disk, WAF).
  // Refreshed alongside status; the WAF and disk endpoints are cheap so we
  // don't bother debouncing — and any error here just leaves the panels
  // empty, never crashes the dashboard.
  const fetchOverviewAux = useCallback(async () => {
    if (activeTab !== "overview") return;
    const [sourcesRes, diskRes, wafRes] = await Promise.allSettled([
      getFailingSources(7, 5),
      getDiskUsage(),
      getWafStats(),
    ]);
    if (sourcesRes.status === "fulfilled") setFailingSources(sourcesRes.value.sources || []);
    if (diskRes.status === "fulfilled") setDiskUsage(diskRes.value);
    if (wafRes.status === "fulfilled") setWafStats(wafRes.value);
  }, [activeTab]);

  useEffect(() => {
    startTransition(() => { fetchOverviewAux(); });
  }, [fetchOverviewAux]);

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
    startTransition(() => { fetchStatus(); });
    let interval = setInterval(() => { startTransition(() => { fetchStatus(); }); }, 30000);
    const handleVisibility = () => {
      if (document.hidden) {
        clearInterval(interval);
      } else {
        startTransition(() => { fetchStatus(); });
        interval = setInterval(() => { startTransition(() => { fetchStatus(); }); }, 30000);
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
        startTransition(() => { setSidebarCollapsed(true); });
      }
    };
    handleResize();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  // Full sync runs asynchronously: the trigger returns an ack immediately and
  // the effect below pulls real progress. Only kickoff failures (409 or
  // network errors) surface in the catch block.
  const handleFullSync = async () => {
    if (syncKickoff || syncProgress?.running) return;
    setSyncKickoff(true);
    try {
      await executeFullSync();
      // Probe progress immediately so the button does not feel idle for 1.5s.
      try {
        const progress = await getSyncProgress();
        setSyncProgress(progress);
      } catch {
        // Silent fallback: the next poll will pick it up.
      }
    } catch (error) {
      const msg = String(error);
      if (msg.includes("SYNC_ALREADY_RUNNING") || msg.includes("409")) {
        toast.info("已有同步正在进行，可在右上角查看进度");
        try {
          setSyncProgress(await getSyncProgress());
        } catch {
          // Silent fallback here as well.
        }
      } else {
        toast.error("同步触发失败: " + msg);
      }
    } finally {
      setSyncKickoff(false);
    }
  };

  // Sync cancel only requests server-side cancellation; polling owns the
  // actual state transition.
  const handleCancelSync = async () => {
    if (isCancellingSync) return;
    setIsCancellingSync(true);
    try {
      await cancelSync();
      toast.message("已请求取消同步，正在等待引擎收尾");
    } catch (error) {
      const msg = String(error);
      if (msg.includes("404")) {
        // The sync has already finished; the next poll will sync state.
        setSyncProgress((prev) => (prev ? { ...prev, running: false } : prev));
      } else {
        toast.error("取消失败: " + msg);
      }
    } finally {
      setIsCancellingSync(false);
    }
  };

  // Polling behavior:
  //   - Default to 5s polls to cover syncs started from another window or a schedule
  //   - Switch to 1.5s polls while running=true for near-real-time feedback
  //   - When the sync finishes, use the last snapshot for a toast and refresh
  //     the overview status
  useEffect(() => {
    let cancelled = false;
    const tick = async () => {
      try {
        const progress = await getSyncProgress();
        if (cancelled) return;
        setSyncProgress(() => {
          // Detect the latest completion snapshot; the first poll can hit too.
          const snap = progress.last;
          if (!progress.running && snap && snap.jobId !== lastShownJobIdRef.current) {
            const isFirstPoll = lastShownJobIdRef.current === null;
            lastShownJobIdRef.current = snap.jobId;
            // On first poll after mount, only record the jobId so we don't
            // re-announce a sync that completed before the page loaded.
            if (!isFirstPoll) {
              queueMicrotask(() => {
                if (snap.cancelled) {
                  toast.warning("同步已取消");
                } else if (snap.success) {
                  if (snap.changedCount > 0) {
                    toast.success(`同步成功！${snap.changedCount} 条规则已更新`);
                  } else {
                    toast.success("同步成功！本次无规则变更");
                  }
                  setNeedsFirstSync(false);
                } else if (snap.error) {
                  toast.error("同步失败: " + snap.error);
                } else {
                  toast.warning(`同步完成，但有 ${snap.failedCount} 条规则失败`);
                }
                fetchStatus();
              });
            }
          }
          return progress;
        });
      } catch {
        // Network hiccups and unauthenticated responses land here. Keep the
        // previous state and try again on the next poll.
      }
    };
    tick();
    const intervalMs = syncProgress?.running ? 1500 : 5000;
    const id = window.setInterval(tick, intervalMs);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [syncProgress?.running]);

  // Derived state for the top-right control. kickoff counts as syncing so
  // the button enters loading state immediately and resists duplicate clicks.
  const isSyncing = syncKickoff || (syncProgress?.running ?? false);

  const fetchActivity = useCallback(async () => {
    // Only fetch for relevant tabs
    if (activeTab !== "activity" && activeTab !== "overview") return;

    try {
      const dates = await getActivityDates();
      setActivityDates(dates.dates);

      if (activeTab === "overview") {
        // Overview pulls the last 7 days of records and shows at most 8 rows.
        const [changes, failures] = await Promise.all([
          getChangeRecords(undefined, 1, 8, undefined, 7),
          getFailureRecords(undefined, 1, 8, undefined, 7),
        ]);
        setChangeData(changes);
        setFailureData(failures);
      } else {
        const dateParam = activityDate === "all" ? undefined : activityDate;
        const clientParam = activityClient === "all" ? undefined : activityClient;
        const [changes, failures] = await Promise.all([
          getChangeRecords(dateParam, changePage, activityPageSize, clientParam),
          getFailureRecords(dateParam, failurePage, activityPageSize, clientParam),
        ]);
        setChangeData(changes);
        setFailureData(failures);
      }
    } catch (error) {
      console.error("Failed to fetch activity:", error);
      if (activeTab === "activity") toast.error("获取活动记录失败");
    }
  }, [activeTab, activityDate, activityClient, changePage, failurePage, activityPageSize]);

  useEffect(() => {
    startTransition(() => { fetchActivity(); });
  }, [fetchActivity]);

  useEffect(() => {
    if (activityDate !== "all" && !activityDates.includes(activityDate)) {
      startTransition(() => { setActivityDate("all"); });
    }
  }, [activityDate, activityDates]);

  useEffect(() => {
    if (activityClient !== "all" && !clients.some((client) => client.id === activityClient)) {
      startTransition(() => { setActivityClient("all"); });
    }
  }, [activityClient, clients]);

  const openChangeDiff = async (change: ChangeRecordSummary) => {
    const requestId = diffRequestRef.current + 1;
    diffRequestRef.current = requestId;
    setSelectedChange(change);
    setDiffContent("");
    setIsDiffLoading(true);
    try {
      const result = await getChangeDiff(change.date, change.fileName);
      if (diffRequestRef.current !== requestId) return;
      setDiffContent(result.diff);
    } catch (error) {
      if (diffRequestRef.current !== requestId) return;
      console.error("Failed to fetch diff:", error);
      setDiffContent("diff 已过期或不可用");
    } finally {
      if (diffRequestRef.current !== requestId) return;
      setIsDiffLoading(false);
    }
  };

  const handleClearActivity = async () => {
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
  const filteredChangeItems = changeItems;
  const filteredFailureItems = failureItems;
  const changeTotalPages = Math.max(1, Math.ceil((changeData?.total || 0) / (changeData?.pageSize || activityPageSize)));
  const failureTotalPages = Math.max(1, Math.ceil((failureData?.total || 0) / (failureData?.pageSize || activityPageSize)));

  // Sync health combines the latest successful sync time and the failure count.
  // - never: no successful sync yet
  // - partial: the latest sync had failed rules
  // - stale: the last successful sync is older than 48 hours
  // - healthy: neither condition applies
  const syncHealthStatus: "healthy" | "partial" | "stale" | "never" | "unknown" =
    !status?.lastSync
      ? "unknown"
      : !status.lastSync.lastSuccessfulSyncAt
        ? "never"
        : status.lastSync.failedRulesCount > 0
          ? "partial"
          : (now - new Date(status.lastSync.lastSuccessfulSyncAt).getTime()) / 3_600_000 > 48
            ? "stale"
            : "healthy";

  // Keep `now` fresh whenever status changes so staleness check is accurate
  useEffect(() => {
    if (status) {
      startTransition(() => { setNow(Date.now()); });
    }
  }, [status]);

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
            {syncProgress?.running ? (
              <SyncProgressPill
                progress={syncProgress}
                onCancel={handleCancelSync}
                isCancelling={isCancellingSync}
              />
            ) : (
              <Button
                variant="default"
                onClick={handleFullSync}
                disabled={isSyncing}
              >
                {isSyncing ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
                同步规则
              </Button>
            )}
          </div>
        </header>

        {/* Scrollable Content. On overview/geosite at lg+ we lock main to
            the viewport (overflow-hidden) so the dashboard's flex-1 chain
            has a definite height to divide; below lg the content scrolls
            naturally. Other tabs always scroll. */}
        <main
          className={cn(
            "flex-1 p-6 scroll-smooth bg-muted/5",
            activeTab === "geosite"
              ? "overflow-hidden"
              : activeTab === "overview"
                ? "overflow-y-auto lg:overflow-hidden"
                : "overflow-y-auto",
          )}
        >
          {/* Wrapper on overview: only `h-full` at lg+ (with min-h-0 to break
              the default min-content floor); below lg fall back to min-h-full
              so the page can grow and the outer scroll works. */}
          <div
            className={cn(
              "max-w-screen-2xl mx-auto",
              activeTab === "geosite" && "h-full",
              activeTab === "overview"
                ? "flex flex-col gap-4 min-h-full lg:h-full lg:min-h-0"
                : "space-y-6",
            )}
          >
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
              <div className="flex flex-col gap-4 lg:flex-1 lg:min-h-0">
                <SyncHealthCard
                  status={status}
                  health={syncHealthStatus}
                  diskTotal={diskUsage?.total}
                  clientCount={clients.length}
                />

                {/* Disk + WAF — middle row, natural height */}
                <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
                  <DiskUsageCard usage={diskUsage} className="lg:col-span-2" />
                  <WafSummaryCard stats={wafStats} onJump={() => setActiveTab('security')} />
                </div>

                {/* Failing sources + Recent activity — bottom row, claims
                    remaining viewport height. lg:min-h-0 is critical: a
                    `min-h-[…]` floor would reintroduce the flex `min-content`
                    behaviour and let inner lists push the grid past flex-1.
                    Inner scroll containers (TabsContent, ul) handle overflow. */}
                <div className="grid grid-cols-1 lg:grid-cols-5 gap-4 lg:gap-6 lg:flex-1 lg:min-h-0">
                  <Card className="lg:col-span-2 p-4 flex flex-col min-h-0 overflow-hidden">
                    <div className="flex items-center justify-between mb-3 shrink-0">
                      <p className="text-xs text-muted-foreground uppercase tracking-wider font-semibold flex items-center gap-1.5">
                        <AlertCircle className="w-3.5 h-3.5" />
                        本周失败源
                      </p>
                      <span className="text-[10px] text-muted-foreground font-mono">近 7 天</span>
                    </div>
                    {failingSources.length === 0 ? (
                      <div className="flex-1 flex flex-col items-center justify-center text-center">
                        <CheckCircle className="w-6 h-6 text-success/60 mb-1.5" />
                        <p className="text-xs text-muted-foreground">7 天内无失败</p>
                      </div>
                    ) : (
                      <ul className="space-y-1 overflow-y-auto flex-1">
                        {failingSources.map((source) => (
                          <li
                            key={source.ruleName}
                            className="flex items-start gap-3 rounded-md px-2 py-1.5 hover:bg-accent transition-colors cursor-pointer"
                            onClick={() => setActiveTab('activity')}
                            role="button"
                          >
                            <div className="min-w-0 flex-1">
                              <FailureRuleLabel ruleName={source.ruleName} className="text-sm font-medium" />
                              <p className="text-[11px] text-muted-foreground truncate mt-0.5" title={source.lastMessage}>
                                {source.lastMessage || "(no message)"}
                              </p>
                            </div>
                            <div className="text-right shrink-0">
                              <div className="text-sm font-mono font-semibold text-destructive">{source.count} 次</div>
                              <div className="text-[10px] text-muted-foreground font-mono">{formatRelativeTime(source.lastTimestamp)}</div>
                            </div>
                          </li>
                        ))}
                      </ul>
                    )}
                  </Card>

                  <Card className="lg:col-span-3 flex flex-col min-h-0 overflow-hidden">
                    <CardHeader className="flex-row items-center justify-between border-b border-border shrink-0 py-3">
                      <CardTitle className="flex items-center gap-2 text-sm">
                        <Activity className="w-4 h-4 text-muted-foreground" />
                        最近活动
                      </CardTitle>
                      <Button variant="ghost" size="sm" className="h-7 text-xs text-muted-foreground" onClick={() => setActiveTab('activity')}>
                        查看全部 <ArrowUpRight className="w-3 h-3 ml-1" />
                      </Button>
                    </CardHeader>
                    <div className="flex-1 flex flex-col min-h-0">
                      <Tabs defaultValue="changes" className="flex-1 flex flex-col min-h-0">
                        <div className="px-4 pt-3 shrink-0">
                          <TabsList className="w-fit">
                            <TabsTrigger value="changes">最近变更</TabsTrigger>
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
                        <TabsContent value="changes" className="flex-1 min-h-0 overflow-y-auto px-2 pb-2 mt-0">
                          <ActivityFeed compact items={filteredChangeItems.slice(0, 8)} onViewDiff={openChangeDiff} getClientDisplayName={getClientDisplayName} />
                        </TabsContent>
                        <TabsContent value="failures" className="flex-1 min-h-0 overflow-y-auto px-4 pb-4 mt-0">
                          {filteredFailureItems.length === 0 ? (
                            <div className="flex flex-col items-center justify-center py-12 text-center text-muted-foreground">
                              <CheckCircle className="mb-2 w-8 h-8 text-success/60" />
                              <p className="text-sm font-medium">运行正常</p>
                              <p className="text-xs mt-1">暂无失败记录</p>
                            </div>
                          ) : (
                            <div className="space-y-3">
                              {filteredFailureItems.slice(0, 8).map(f => (
                                <div key={f.id} className="p-3 bg-destructive/5 rounded-xl border border-destructive/10 text-xs hover:bg-destructive/10 transition-colors">
                                  <div className="flex items-center justify-between gap-2 mb-1 min-w-0">
                                    <FailureRuleLabel ruleName={f.ruleName} className="font-semibold text-destructive min-w-0" />
                                    <span className="text-[10px] text-muted-foreground font-mono shrink-0">{formatRelativeTime(f.timestamp)}</span>
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
                      <CardDescription>
                      查看历史变更与同步失败记录
                    </CardDescription>
                  </div>
                  <div className="flex flex-wrap items-center gap-2 sm:gap-3">
                    {/* Pagination Controls */}
                    <div className="flex items-center gap-1 rounded-full border border-border bg-surface-subtle p-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 rounded-full hover:bg-background"
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
                        className="h-7 w-7 rounded-full hover:bg-background"
                        disabled={activityTab === "changes" ? changePage >= changeTotalPages : failurePage >= failureTotalPages}
                        onClick={() => activityTab === "changes" ? setChangePage(p => Math.min(changeTotalPages, p + 1)) : setFailurePage(p => Math.min(failureTotalPages, p + 1))}
                      >
                        <ChevronRight className="w-4 h-4 text-muted-foreground" />
                      </Button>
                    </div>

                    <div className="h-4 w-px bg-border mx-1" />

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

                    <div className="h-4 w-px bg-border mx-1" />

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
                  <Tabs defaultValue="changes" value={activityTab} onValueChange={setActivityTab} className="w-full h-full flex flex-col">
                    <TabsList className="w-full justify-start mb-6">
                      <TabsTrigger
                        value="changes"
                      >
                        变更记录
                      </TabsTrigger>
                      <TabsTrigger
                        value="failures"
                      >
                        失败日志
                      </TabsTrigger>
                    </TabsList>

                    <TabsContent value="changes" className="flex-1 mt-0 outline-none">
                      <div className="border border-border rounded-2xl bg-card overflow-hidden">
                        <ActivityFeed items={changeItems} onViewDiff={openChangeDiff} getClientDisplayName={getClientDisplayName} />
                      </div>
                    </TabsContent>
                    <TabsContent value="failures" className="flex-1 mt-0 outline-none">
                      <div className="border border-border rounded-2xl bg-card overflow-hidden divide-y divide-border">
                        {failureItems.map(f => (
                          <div key={f.id} className="p-4 hover:bg-muted/10 transition-colors flex items-start justify-between gap-4 group">
                            <div className="space-y-1 min-w-0">
                              <div className="flex items-center gap-2 min-w-0">
                                <span className="font-medium text-destructive flex items-center gap-1.5 min-w-0">
                                  <XCircle className="w-4 h-4 shrink-0" />
                                  <FailureRuleLabel ruleName={f.ruleName} />
                                </span>
                                {f.client && (
                                  <Badge variant="secondary" className="text-[10px] font-normal shrink-0">
                                    {getClientDisplayName(f.client)}
                                  </Badge>
                                )}
                                <Badge variant="outline" className="text-[10px] font-mono font-normal text-muted-foreground shrink-0">
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
            <DialogTitle>变更详情 - {selectedChange?.ruleName}</DialogTitle>
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

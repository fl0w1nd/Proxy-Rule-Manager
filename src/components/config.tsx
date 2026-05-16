"use client";

import { useState, useEffect, useMemo, useRef, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Loader2,
  Download,
  Upload,
  Database,
  Clock,
  Settings,
  Layers,
  Globe,
  Shield,
  Activity,
  Sliders,
  RotateCcw,
  CheckCircle2,
  AlertTriangle,
  HardDrive,
  Network,
  Cpu,
} from "lucide-react";
import {
  backupDatabase,
  getCdnSettings,
  getClients,
  getConfigRaw,
  getGeositeProviders,
  getStatus,
  getSyncSchedule,
  getSystemSettings,
  getWafStats,
  restoreDatabase,
  updateSyncSchedule,
  updateSystemSettings,
  type CdnSettings,
  type ClientConfig,
  type GeositeProviderStatus,
  type StatusResponse,
  type SystemSettingsResponse,
  type WafStats,
} from "@/lib/api-client";
import { DEFAULT_SYSTEM_SETTINGS, RulesConfig, SystemSettings } from "@/lib/schema";
import { isGeositeRule } from "@/lib/rule-classification";
import { formatRelativeTime } from "@/lib/utils";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

interface ConfigEditorProps {
  onSave: () => void;
}

export function ConfigEditor({ onSave }: ConfigEditorProps) {
  const [config, setConfig] = useState<RulesConfig | null>(null);
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [clients, setClients] = useState<ClientConfig[]>([]);
  const [geositeProviders, setGeositeProviders] = useState<GeositeProviderStatus[]>([]);
  const [cdnSettings, setCdnSettings] = useState<CdnSettings | null>(null);
  const [wafStats, setWafStats] = useState<WafStats | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const [isBackingUp, setIsBackingUp] = useState(false);
  const [isRestoring, setIsRestoring] = useState(false);

  // Schedule state
  const [isScheduleLoading, setIsScheduleLoading] = useState(true);
  const [isScheduleUpdating, setIsScheduleUpdating] = useState(false);
  const [scheduleMode, setScheduleMode] = useState<"interval" | "cron">("interval");
  const [intervalHours, setIntervalHours] = useState(24);
  const [cronExpression, setCronExpression] = useState("0 0 * * *");
  const [syncScheduleNextAt, setSyncScheduleNextAt] = useState<string | null>(null);

  // System settings state
  const [systemSettings, setSystemSettings] = useState<SystemSettings | null>(null);
  const [systemDefaults, setSystemDefaults] = useState<SystemSettings>(DEFAULT_SYSTEM_SETTINGS);
  const [isSystemLoading, setIsSystemLoading] = useState(true);
  const [isSystemSaving, setIsSystemSaving] = useState(false);

  const restoreDbRef = useRef<HTMLInputElement | null>(null);

  const fetchAll = useCallback(async () => {
    try {
      const [configRes, statusRes, clientsRes, providersRes, cdnRes, wafRes] =
        await Promise.allSettled([
          getConfigRaw(),
          getStatus(),
          getClients(),
          getGeositeProviders(),
          getCdnSettings(),
          getWafStats(),
        ]);
      if (configRes.status === "fulfilled") setConfig(configRes.value.config);
      if (statusRes.status === "fulfilled") setStatus(statusRes.value);
      if (clientsRes.status === "fulfilled") setClients(clientsRes.value.clients);
      if (providersRes.status === "fulfilled") setGeositeProviders(providersRes.value.providers);
      if (cdnRes.status === "fulfilled") setCdnSettings(cdnRes.value.settings);
      if (wafRes.status === "fulfilled") setWafStats(wafRes.value);
      if (configRes.status === "rejected") toast.error("获取配置失败");
    } finally {
      setIsLoading(false);
    }
  }, []);

  const fetchSchedule = useCallback(async () => {
    try {
      const { schedule } = await getSyncSchedule();
      setScheduleMode(schedule.mode || "interval");
      setIntervalHours(schedule.intervalHours || 24);
      setCronExpression(schedule.cronExpression || "0 0 * * *");
      setSyncScheduleNextAt(schedule.nextSyncAt || null);
    } catch (error) {
      console.error("Failed to fetch schedule:", error);
      toast.error("获取定时同步配置失败");
    } finally {
      setIsScheduleLoading(false);
    }
  }, []);

  const fetchSystem = useCallback(async () => {
    setIsSystemLoading(true);
    try {
      const { settings, defaults }: SystemSettingsResponse = await getSystemSettings();
      setSystemSettings(settings);
      setSystemDefaults(defaults);
    } catch (error) {
      console.error("Failed to fetch system settings:", error);
      toast.error("获取系统参数失败");
      setSystemSettings(DEFAULT_SYSTEM_SETTINGS);
    } finally {
      setIsSystemLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchAll();
    fetchSchedule();
    fetchSystem();
  }, [fetchAll, fetchSchedule, fetchSystem]);

  const handleSaveSchedule = async () => {
    setIsScheduleUpdating(true);
    try {
      const payload =
        scheduleMode === "interval"
          ? { mode: "interval" as const, intervalHours }
          : { mode: "cron" as const, cronExpression: cronExpression.trim() };
      const result = await updateSyncSchedule(payload);
      setScheduleMode(result.schedule.mode || "interval");
      setIntervalHours(result.schedule.intervalHours || 24);
      setCronExpression(result.schedule.cronExpression || "0 0 * * *");
      setSyncScheduleNextAt(result.schedule.nextSyncAt || null);
      toast.success("定时同步配置已更新");
    } catch (error) {
      toast.error("更新定时同步失败: " + String(error));
    } finally {
      setIsScheduleUpdating(false);
    }
  };

  const handleSaveSystem = async () => {
    if (!systemSettings) return;
    setIsSystemSaving(true);
    try {
      const result = await updateSystemSettings(systemSettings);
      setSystemSettings(result.settings);
      toast.success("系统参数已更新");
    } catch (error) {
      toast.error("更新系统参数失败: " + String(error));
    } finally {
      setIsSystemSaving(false);
    }
  };

  const handleResetSystem = () => {
    setSystemSettings(systemDefaults);
    toast.message("已重置为默认值，记得点击保存");
  };

  const handleBackup = async () => {
    setIsBackingUp(true);
    try {
      const blob = await backupDatabase();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      const dateTag = new Date().toISOString().split("T")[0];
      link.href = url;
      link.download = `proxy-rule-manager-backup-${dateTag}.zip`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
      toast.success("数据库备份成功");
    } catch (error) {
      toast.error("数据库备份失败: " + String(error));
    } finally {
      setIsBackingUp(false);
    }
  };

  const handleRestore = async (file: File) => {
    setIsRestoring(true);
    try {
      await restoreDatabase(file);
      toast.success("数据库恢复成功");
      await Promise.all([fetchAll(), fetchSchedule(), fetchSystem()]);
      onSave();
    } catch (error) {
      toast.error("数据库恢复失败: " + String(error));
    } finally {
      setIsRestoring(false);
    }
  };

  // ----- Overview derivations (no charts; counts only) -----
  const overview = useMemo(() => {
    if (!config) return null;
    const rules = config.rules || [];
    const normalRules = rules.filter((r) => !isGeositeRule(r));
    const geositeRules = rules.filter((r) => isGeositeRule(r));
    const sources = rules.reduce((acc, r) => acc + (r.sources?.length || 0), 0);
    const refLinks = rules.reduce(
      (acc, r) => acc + (r.sources?.filter((s) => s.type === "ref").length || 0),
      0,
    );
    const tags = new Set<string>();
    for (const r of rules) for (const t of r.tags || []) tags.add(t);
    return {
      version: config.version,
      ruleCount: normalRules.length,
      geositeCount: geositeRules.length,
      transformerCount: Object.keys(config.transformers || {}).length,
      sources,
      refLinks,
      tagCount: tags.size,
    };
  }, [config]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin text-primary" />
      </div>
    );
  }

  if (!config) {
    return (
      <Card className="p-6 text-center text-muted-foreground">无法加载配置。</Card>
    );
  }

  const lastSuccessfulSyncAt = status?.lastSync?.lastSuccessfulSyncAt || null;
  const ruleFilesCount =
    (status?.ruleFilesCount || 0) + (status?.geositeRuleFilesCount || 0);

  return (
    <div className="space-y-8 max-w-5xl">
      <input
        ref={restoreDbRef}
        type="file"
        accept=".zip"
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) handleRestore(file);
          e.currentTarget.value = "";
        }}
      />

      {/* ============== Clean Global Header ============== */}
      <div className="flex flex-col sm:flex-row sm:items-end justify-between gap-4 pb-4 border-b border-border/40">
        <div className="flex items-center gap-4">
          <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary shadow-sm border border-primary/20">
            <Settings className="w-6 h-6" />
          </div>
          <div>
            <div className="flex items-center gap-3">
              <h2 className="text-2xl font-bold tracking-tight text-foreground">系统配置</h2>
              <Badge variant="outline" className="font-mono text-xs bg-background shadow-sm">
                v{overview?.version ?? 1}
              </Badge>
            </div>
            <p className="text-sm text-muted-foreground mt-1.5 flex items-center gap-2">
              {lastSuccessfulSyncAt ? (
                <>
                  <span className="relative flex h-2 w-2">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-success opacity-75"></span>
                    <span className="relative inline-flex rounded-full h-2 w-2 bg-success"></span>
                  </span>
                  最近同步于 {formatRelativeTime(lastSuccessfulSyncAt)}
                </>
              ) : (
                <>
                  <span className="h-2 w-2 rounded-full bg-muted-foreground/50"></span>
                  尚未执行过同步
                </>
              )}
            </p>
          </div>
        </div>
      </div>

      <Tabs defaultValue="overview" className="w-full">
        <TabsList className="bg-surface-subtle mb-6 p-1">
          <TabsTrigger value="overview" className="rounded-sm px-4">
            <Activity className="w-4 h-4 mr-2" />
            概览
          </TabsTrigger>
          <TabsTrigger value="schedule" className="rounded-sm px-4">
            <Clock className="w-4 h-4 mr-2" />
            定时同步
          </TabsTrigger>
          <TabsTrigger value="system" className="rounded-sm px-4">
            <Sliders className="w-4 h-4 mr-2" />
            系统参数
          </TabsTrigger>
          <TabsTrigger value="database" className="rounded-sm px-4">
            <Database className="w-4 h-4 mr-2" />
            数据库
          </TabsTrigger>
        </TabsList>

        {/* ====================== Overview tab ====================== */}
        <TabsContent value="overview" className="space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Asset Stats */}
            <Card className="p-6 flex flex-col gap-6 shadow-sm border-border/60">
              <div className="flex items-center gap-2">
                <Layers className="w-5 h-5 text-primary" />
                <h3 className="font-semibold text-foreground">核心资产</h3>
              </div>
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-y-6 gap-x-4">
                {[
                  { label: "配置产物", value: ruleFilesCount },
                  { label: "普通规则", value: overview?.ruleCount ?? 0 },
                  { label: "Geosite", value: overview?.geositeCount ?? 0 },
                  { label: "客户端", value: clients.length },
                  { label: "转换器", value: overview?.transformerCount ?? 0 },
                  { label: "数据源", value: overview?.sources ?? 0 },
                  { label: "标签", value: overview?.tagCount ?? 0 },
                  { label: "内部引用", value: overview?.refLinks ?? 0 },
                ].map((m) => (
                  <div key={m.label} className="flex flex-col gap-1">
                    <p className="text-xs font-medium text-muted-foreground">
                      {m.label}
                    </p>
                    <p className="text-2xl font-mono font-bold tracking-tight text-foreground">
                      {m.value}
                    </p>
                  </div>
                ))}
              </div>
            </Card>

            {/* System Health */}
            <Card className="p-6 flex flex-col gap-6 shadow-sm border-border/60">
              <div className="flex items-center gap-2">
                <Activity className="w-5 h-5 text-primary" />
                <h3 className="font-semibold text-foreground">组件健康度</h3>
              </div>
              <div className="space-y-4 flex-1">
                {/* CDN */}
                <div className="flex items-center justify-between p-3 rounded-lg bg-surface-subtle border border-border/40">
                  <div className="flex items-center gap-3">
                    <div className="bg-background p-2 rounded-md shadow-sm border border-border/50">
                      <Network className="w-4 h-4 text-foreground" />
                    </div>
                    <div>
                      <p className="text-sm font-medium text-foreground">CDN 响应</p>
                      <p className="text-[11px] text-muted-foreground mt-0.5">
                        {cdnSettings?.enabled
                          ? `${cdnSettings.cacheMode} · 容灾缓存 ${cdnSettings.staleIfErrorSeconds}s`
                          : "未启用 CDN 优化"}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    {cdnSettings?.enabled ? (
                      <Badge variant="emerald" className="text-[10px] uppercase">Active</Badge>
                    ) : (
                      <Badge variant="outline" className="text-[10px] uppercase text-muted-foreground">Disabled</Badge>
                    )}
                  </div>
                </div>

                {/* WAF */}
                <div className="flex items-center justify-between p-3 rounded-lg bg-surface-subtle border border-border/40">
                  <div className="flex items-center gap-3">
                    <div className="bg-background p-2 rounded-md shadow-sm border border-border/50">
                      <Shield className="w-4 h-4 text-foreground" />
                    </div>
                    <div>
                      <p className="text-sm font-medium text-foreground">WAF 防护</p>
                      <p className="text-[11px] text-muted-foreground mt-0.5">
                        封禁 {wafStats?.bans?.total ?? 0} 个来源 (永久 {wafStats?.bans?.permanent ?? 0})
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                     {(wafStats?.bans?.total ?? 0) > 0 ? (
                       <Badge variant="amber" className="text-[10px] uppercase">Protecting</Badge>
                     ) : (
                       <Badge variant="outline" className="text-[10px] uppercase text-success border-success/30 bg-success/5">Standby</Badge>
                     )}
                  </div>
                </div>

                {/* SQLite */}
                <div className="flex items-center justify-between p-3 rounded-lg bg-surface-subtle border border-border/40">
                  <div className="flex items-center gap-3">
                    <div className="bg-background p-2 rounded-md shadow-sm border border-border/50">
                      <HardDrive className="w-4 h-4 text-foreground" />
                    </div>
                    <div>
                      <p className="text-sm font-medium text-foreground">本地存储</p>
                      <p className="text-[11px] text-muted-foreground mt-0.5 font-mono">
                        SQLite Database
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                     <Badge variant="emerald" className="text-[10px] uppercase">Healthy</Badge>
                  </div>
                </div>
              </div>
            </Card>
          </div>

          <Card className="p-6 shadow-sm border-border/60">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2">
                <Globe className="w-5 h-5 text-primary" />
                <h3 className="font-semibold text-foreground">Geosite 提供商</h3>
              </div>
              <Badge variant="secondary" className="font-mono text-xs">
                {geositeProviders.length} Total
              </Badge>
            </div>
            
            {geositeProviders.length > 0 ? (
              <div className="border border-border/60 rounded-lg overflow-hidden max-h-[300px] flex flex-col">
                <div className="overflow-y-auto">
                  <ul className="divide-y divide-border/60">
                    {geositeProviders.map((p) => (
                      <li
                        key={p.provider}
                        className="flex items-center gap-4 p-3 text-sm hover:bg-surface-subtle/50 transition-colors"
                      >
                        <div className="flex items-center justify-center w-8 shrink-0">
                          {p.ready ? (
                            <CheckCircle2 className="w-4 h-4 text-success" />
                          ) : (
                            <Clock className="w-4 h-4 text-muted-foreground/50" />
                          )}
                        </div>
                        <span className="font-mono font-medium text-foreground min-w-[8rem]">
                          {p.provider}
                        </span>
                        <span className="text-xs text-muted-foreground flex-1 truncate">
                          {p.fetchedAt
                            ? `${p.catalogCount} 个列表 · 更新于 ${formatRelativeTime(p.fetchedAt)}`
                            : "等待首次抓取..."}
                        </span>
                        <div className="shrink-0 pr-2">
                          {p.ready ? (
                            <Badge variant="emerald" className="text-[10px] bg-success/10 text-success border-success/20">
                              就绪
                            </Badge>
                          ) : (
                            <Badge variant="outline" className="text-[10px]">
                              未就绪
                            </Badge>
                          )}
                        </div>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center py-10 bg-surface-subtle/30 rounded-lg border border-dashed border-border/60">
                <Globe className="w-8 h-8 text-muted-foreground/30 mb-2" />
                <p className="text-sm text-muted-foreground">未配置任何 Geosite 提供商</p>
              </div>
            )}
          </Card>
        </TabsContent>

        {/* ====================== Schedule tab ====================== */}
        <TabsContent value="schedule">
          <Card className="shadow-sm border-border/60 overflow-hidden">
            <div className="bg-surface-subtle px-6 py-5 border-b border-border/40">
              <h3 className="text-lg font-semibold text-foreground flex items-center gap-2">
                <Clock className="w-5 h-5 text-primary" />
                自动同步策略
              </h3>
              <p className="text-sm text-muted-foreground mt-1">
                配置系统在后台拉取更新并生成规则产物的频率。
              </p>
            </div>
            
            <div className="p-6 space-y-8">
              {isScheduleLoading ? (
                <div className="flex items-center justify-center py-8 text-muted-foreground">
                  <Loader2 className="w-6 h-6 animate-spin text-primary" />
                </div>
              ) : (
                <>
                  {/* Mode Selector */}
                  <div className="space-y-3">
                    <Label className="text-sm font-medium">执行模式</Label>
                    <div className="flex p-1 bg-surface-subtle rounded-lg w-fit">
                      <button
                        onClick={() => setScheduleMode("interval")}
                        className={cn(
                          "px-6 py-2 text-sm font-medium rounded-md transition-all",
                          scheduleMode === "interval" 
                            ? "bg-background text-foreground shadow-sm" 
                            : "text-muted-foreground hover:text-foreground hover:bg-background/50"
                        )}
                      >
                        固定间隔 (Interval)
                      </button>
                      <button
                        onClick={() => setScheduleMode("cron")}
                        className={cn(
                          "px-6 py-2 text-sm font-medium rounded-md transition-all",
                          scheduleMode === "cron" 
                            ? "bg-background text-foreground shadow-sm" 
                            : "text-muted-foreground hover:text-foreground hover:bg-background/50"
                        )}
                      >
                        Cron 表达式
                      </button>
                    </div>
                  </div>

                  {/* Settings Area */}
                  <div className="p-5 bg-background rounded-xl border border-border/60">
                    {scheduleMode === "interval" ? (
                      <div className="space-y-4 max-w-sm">
                        <Label className="text-sm font-medium text-foreground">同步间隔</Label>
                        <Select
                          value={String(intervalHours)}
                          onValueChange={(value) => setIntervalHours(parseInt(value, 10))}
                        >
                          <SelectTrigger className="h-11">
                            <SelectValue placeholder="选择间隔" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="1">每 1 小时执行一次</SelectItem>
                            <SelectItem value="2">每 2 小时执行一次</SelectItem>
                            <SelectItem value="6">每 6 小时执行一次</SelectItem>
                            <SelectItem value="12">每 12 小时执行一次</SelectItem>
                            <SelectItem value="24">每 24 小时执行一次 (每天)</SelectItem>
                            <SelectItem value="48">每 48 小时执行一次 (每两天)</SelectItem>
                            <SelectItem value="72">每 72 小时执行一次 (每三天)</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                    ) : (
                      <div className="space-y-4 max-w-md">
                        <div>
                          <Label className="text-sm font-medium text-foreground">Cron 表达式</Label>
                          <p className="text-xs text-muted-foreground mt-1 mb-2">支持 5 段或 6 段格式，使用 UTC 时间。</p>
                        </div>
                        <Input
                          value={cronExpression}
                          onChange={(e) => setCronExpression(e.target.value)}
                          className="font-mono h-11 text-base bg-surface-subtle"
                          placeholder="0 0 * * *"
                        />
                        <div className="bg-primary/5 border border-primary/20 rounded-md p-3">
                          <p className="text-xs text-primary-foreground/80 leading-relaxed">
                            示例: <br/>
                            <code className="font-mono font-semibold">0 0 * * *</code> = 每天 UTC 00:00<br/>
                            <code className="font-mono font-semibold">0 */6 * * *</code> = 每 6 小时
                          </p>
                        </div>
                      </div>
                    )}
                  </div>

                  {/* Next Sync Preview & Action */}
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pt-4 border-t border-border/40">
                    <div className="flex flex-col">
                      <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">下一次执行</span>
                      {syncScheduleNextAt ? (
                        <span className="text-sm font-mono font-medium text-foreground mt-1">
                          {new Date(syncScheduleNextAt).toLocaleString()} (本地时间)
                        </span>
                      ) : (
                        <span className="text-sm text-muted-foreground mt-1">待计算...</span>
                      )}
                    </div>
                    
                    <Button
                      size="lg"
                      onClick={handleSaveSchedule}
                      disabled={
                        isScheduleUpdating ||
                        (scheduleMode === "cron" && !cronExpression.trim())
                      }
                      className="min-w-[120px]"
                    >
                      {isScheduleUpdating ? (
                        <Loader2 className="w-5 h-5 animate-spin" />
                      ) : (
                        "应用配置"
                      )}
                    </Button>
                  </div>
                </>
              )}
            </div>
          </Card>
        </TabsContent>

        {/* ====================== System settings tab ====================== */}
        <TabsContent value="system">
          {isSystemLoading || !systemSettings ? (
            <Card className="p-12 flex items-center justify-center">
              <div className="flex flex-col items-center gap-3 text-muted-foreground">
                <Loader2 className="w-6 h-6 animate-spin text-primary" />
                <span className="text-sm">加载核心参数...</span>
              </div>
            </Card>
          ) : (
            <SystemSettingsForm
              value={systemSettings}
              defaults={systemDefaults}
              isSaving={isSystemSaving}
              onChange={setSystemSettings}
              onReset={handleResetSystem}
              onSave={handleSaveSystem}
              onReload={fetchSystem}
            />
          )}
        </TabsContent>

        {/* ====================== Database tab ====================== */}
        <TabsContent value="database">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Card className="p-6 shadow-sm border-border/60 flex flex-col justify-between">
              <div>
                <div className="flex items-center gap-2 mb-2">
                  <div className="p-2 bg-primary/10 rounded-md">
                    <Download className="w-5 h-5 text-primary" />
                  </div>
                  <h3 className="text-lg font-semibold text-foreground">备份数据库</h3>
                </div>
                <p className="text-sm text-muted-foreground mb-6 leading-relaxed">
                  将当前系统的所有配置（规则、客户端、数据源、系统参数等）打包为 Zip 文件下载。建议在重大变更前执行此操作。
                </p>
              </div>
              <Button
                className="w-full sm:w-auto"
                onClick={handleBackup}
                disabled={isBackingUp}
              >
                {isBackingUp ? (
                  <Loader2 className="w-4 h-4 animate-spin mr-2" />
                ) : (
                  <Download className="w-4 h-4 mr-2" />
                )}
                生成并下载备份
              </Button>
            </Card>

            <Card className="p-6 shadow-sm border-destructive/20 bg-destructive/5 flex flex-col justify-between">
              <div>
                <div className="flex items-center gap-2 mb-2">
                  <div className="p-2 bg-destructive/10 rounded-md">
                    <Upload className="w-5 h-5 text-destructive" />
                  </div>
                  <h3 className="text-lg font-semibold text-destructive">恢复数据库</h3>
                </div>
                <p className="text-sm text-destructive/80 mb-6 leading-relaxed">
                  上传 Zip 备份文件。<b>此操作将彻底覆盖当前所有数据！</b> 
                  恢复后系统会自动重新加载参数。
                </p>
              </div>
              <Button
                variant="destructive"
                className="w-full sm:w-auto"
                onClick={() => restoreDbRef.current?.click()}
                disabled={isRestoring}
              >
                {isRestoring ? (
                  <Loader2 className="w-4 h-4 animate-spin mr-2" />
                ) : (
                  <Upload className="w-4 h-4 mr-2" />
                )}
                选择备份并覆盖恢复
              </Button>
            </Card>
          </div>

          <div className="mt-6 p-4 rounded-lg bg-warning/10 border border-warning/20 flex gap-3">
            <AlertTriangle className="w-5 h-5 text-warning shrink-0" />
            <div className="text-sm text-warning-foreground">
              <p className="font-semibold mb-1">迁移提示</p>
              <ul className="list-disc list-inside space-y-1 ml-1 opacity-90 text-xs">
                <li>从旧版 (TS &le; 0.3.x) 升级时，请先用 <code>scripts/migrate-legacy-backup.sh</code> 转换备份格式。</li>
                <li>WAF 封禁列表不会从旧版自动迁移，需要在新版重新配置。</li>
              </ul>
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}

// ===========================================================================
// SystemSettingsForm
// ===========================================================================

interface SystemSettingsFormProps {
  value: SystemSettings;
  defaults: SystemSettings;
  isSaving: boolean;
  onChange: (v: SystemSettings) => void;
  onReset: () => void;
  onSave: () => void;
  onReload: () => void;
}

function SystemSettingsForm({
  value,
  defaults,
  isSaving,
  onChange,
  onReset,
  onSave,
}: SystemSettingsFormProps) {
  const update = <
    K1 extends keyof SystemSettings,
    K2 extends keyof SystemSettings[K1],
  >(
    section: K1,
    field: K2,
    next: SystemSettings[K1][K2],
  ) => {
    onChange({
      ...value,
      [section]: {
        ...value[section],
        [field]: next,
      },
    });
  };

  return (
    <div className="space-y-6 pb-20">
      <SettingsCard 
        title="抓取器 (Fetcher)" 
        description="控制系统从远程数据源下载规则和资源的请求行为"
        icon={<Network className="w-5 h-5" />}
      >
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <SettingItem
            label="超时时间"
            description="单次 HTTP 请求的最长等待时间"
            unit="秒"
            min={1} max={600}
            defaultValue={defaults.fetch.timeoutSeconds}
            value={value.fetch.timeoutSeconds}
            onChange={(v) => update("fetch", "timeoutSeconds", v)}
          />
          <SettingItem
            label="最大下载体积"
            description="超过将记为失败，避免恶意大文件耗尽内存"
            unit="MB"
            min={1} max={256}
            defaultValue={defaults.fetch.maxDownloadMB}
            value={value.fetch.maxDownloadMB}
            onChange={(v) => update("fetch", "maxDownloadMB", v)}
          />
          <SettingItem
            label="同主机并发"
            description="同时请求同一域名的最大连接数"
            unit="连接"
            min={1} max={64}
            defaultValue={defaults.fetch.perHostConcurrency}
            value={value.fetch.perHostConcurrency}
            onChange={(v) => update("fetch", "perHostConcurrency", v)}
          />
          <div className="md:col-span-2">
            <SettingTextItem
              label="User-Agent"
              description="发往上游数据源的 HTTP 请求头"
              defaultValue={defaults.fetch.userAgent}
              value={value.fetch.userAgent}
              onChange={(v) => update("fetch", "userAgent", v)}
            />
          </div>
        </div>
      </SettingsCard>

      <SettingsCard 
        title="JS 转换器沙箱 (Sandbox)" 
        description="自定义脚本 (goja) 运行时的安全边界配置"
        icon={<Cpu className="w-5 h-5" />}
      >
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <SettingItem
            label="单次执行超时"
            description="超过将强制中断脚本并退回原始内容"
            unit="ms"
            min={100} max={60000}
            defaultValue={defaults.transformer.timeoutMs}
            value={value.transformer.timeoutMs}
            onChange={(v) => update("transformer", "timeoutMs", v)}
          />
          <SettingItem
            label="输出体积上限"
            description="脚本返回结果的最大限制，超出将截断"
            unit="MB"
            min={1} max={256}
            defaultValue={defaults.transformer.maxOutputMB}
            value={value.transformer.maxOutputMB}
            onChange={(v) => update("transformer", "maxOutputMB", v)}
          />
        </div>
      </SettingsCard>

      <SettingsCard 
        title="管理后台速率限制 (Rate Limit)" 
        description="控制登录失败的指数退避防护，此限制仅对管理接口生效"
        icon={<Shield className="w-5 h-5" />}
      >
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <SettingItem
            label="基础延迟"
            description="首次失败后的冷却起点时长"
            unit="秒"
            min={1} max={600}
            defaultValue={defaults.rateLimit.baseDelaySeconds}
            value={value.rateLimit.baseDelaySeconds}
            onChange={(v) => update("rateLimit", "baseDelaySeconds", v)}
          />
          <SettingItem
            label="最长封锁时长"
            description="指数退避惩罚的封顶时长"
            unit="秒"
            min={60} max={86400}
            defaultValue={defaults.rateLimit.maxBlockSeconds}
            value={value.rateLimit.maxBlockSeconds}
            onChange={(v) => update("rateLimit", "maxBlockSeconds", v)}
          />
          <SettingItem
            label="永久封禁阈值"
            description="累计失败达此次数后，写入 SQLite 实行永久封禁"
            unit="次"
            min={1} max={1000}
            defaultValue={defaults.rateLimit.permanentBanLimit}
            value={value.rateLimit.permanentBanLimit}
            onChange={(v) => update("rateLimit", "permanentBanLimit", v)}
          />
          <SettingItem
            label="记录保留时长"
            description="达到此时长未再发生失败，则历史计数清零"
            unit="小时"
            min={1} max={720}
            defaultValue={defaults.rateLimit.recordMaxAgeHours}
            value={value.rateLimit.recordMaxAgeHours}
            onChange={(v) => update("rateLimit", "recordMaxAgeHours", v)}
          />
        </div>
      </SettingsCard>

      {/* Sticky Action Bar */}
      <div className="fixed bottom-6 right-6 left-6 md:left-[calc(16rem+1.5rem)] lg:left-[calc(16rem+1.5rem)] max-w-5xl z-10 flex justify-end gap-3 p-4 bg-surface-elevated/90 backdrop-blur-md border border-border/60 rounded-2xl shadow-xl">
        <div className="flex items-center gap-4 w-full justify-between sm:justify-end">
          <span className="text-xs text-muted-foreground hidden sm:inline-block">修改保存后将立即全局生效</span>
          <div className="flex gap-3 w-full sm:w-auto">
            <Button 
              variant="outline" 
              onClick={onReset} 
              disabled={isSaving}
              className="flex-1 sm:flex-none bg-background"
            >
              <RotateCcw className="w-4 h-4 mr-2" />
              恢复默认
            </Button>
            <Button 
              onClick={onSave} 
              disabled={isSaving}
              className="flex-1 sm:flex-none shadow-md"
            >
              {isSaving ? (
                <Loader2 className="w-4 h-4 animate-spin mr-2" />
              ) : (
                <CheckCircle2 className="w-4 h-4 mr-2" />
              )}
              保存参数
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

function SettingsCard({ title, description, icon, children }: { title: string, description: string, icon: React.ReactNode, children: React.ReactNode }) {
  return (
    <Card className="shadow-sm border-border/60 overflow-hidden">
      <div className="bg-surface-subtle/50 px-6 py-4 border-b border-border/40 flex flex-col gap-1">
        <h3 className="text-base font-semibold text-foreground flex items-center gap-2">
          <span className="text-primary">{icon}</span>
          {title}
        </h3>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>
      <div className="p-6">
        {children}
      </div>
    </Card>
  )
}

function SettingItem({
  label,
  description,
  unit,
  value,
  defaultValue,
  min,
  max,
  onChange,
}: {
  label: string;
  description: string;
  unit: string;
  value: number;
  defaultValue: number;
  min: number;
  max: number;
  onChange: (v: number) => void;
}) {
  const isCustom = value !== defaultValue;
  const isOutOfRange = value < min || value > max;

  return (
    <div className="flex flex-col gap-2">
      <div className="flex justify-between items-baseline">
        <Label className="text-sm font-medium text-foreground">{label}</Label>
        {isCustom && (
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                onClick={() => onChange(defaultValue)}
                className="text-[10px] flex items-center gap-1 text-primary hover:underline"
              >
                <RotateCcw className="w-3 h-3" /> 重置为 {defaultValue}
              </button>
            </TooltipTrigger>
            <TooltipContent>恢复默认值</TooltipContent>
          </Tooltip>
        )}
      </div>
      
      <div className="relative">
        <Input
          type="text"
          inputMode="numeric"
          value={String(value)}
          onChange={(e) => {
            const raw = e.target.value.replace(/[^\d-]/g, "");
            if (raw === "" || raw === "-") {
              onChange(defaultValue);
              return;
            }
            const n = parseInt(raw, 10);
            if (Number.isFinite(n)) onChange(n);
          }}
          aria-invalid={isOutOfRange}
          className={cn(
            "pr-12 font-mono h-10",
            isOutOfRange && "border-destructive focus-visible:ring-destructive"
          )}
        />
        <div className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground pointer-events-none select-none">
          {unit}
        </div>
      </div>
      
      <div className="flex justify-between items-start mt-0.5">
        <p className="text-xs text-muted-foreground leading-relaxed">{description}</p>
        <span className="text-[10px] text-muted-foreground/60 font-mono shrink-0 ml-2">
          {min}-{max}
        </span>
      </div>
    </div>
  );
}

function SettingTextItem({
  label,
  description,
  value,
  defaultValue,
  onChange,
}: {
  label: string;
  description: string;
  value: string;
  defaultValue: string;
  onChange: (v: string) => void;
}) {
  const isCustom = value !== defaultValue;

  return (
    <div className="flex flex-col gap-2">
      <div className="flex justify-between items-baseline">
        <Label className="text-sm font-medium text-foreground">{label}</Label>
        {isCustom && (
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                onClick={() => onChange(defaultValue)}
                className="text-[10px] flex items-center gap-1 text-primary hover:underline"
              >
                <RotateCcw className="w-3 h-3" /> 重置
              </button>
            </TooltipTrigger>
            <TooltipContent>恢复默认值</TooltipContent>
          </Tooltip>
        )}
      </div>
      
      <Input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="font-mono h-10"
      />
      
      <p className="text-xs text-muted-foreground leading-relaxed mt-0.5">{description}</p>
    </div>
  );
}

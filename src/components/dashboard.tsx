"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  RefreshCw,
  Settings,
  FileText,
  Activity,
  Clock,
  CheckCircle,
  XCircle,
  Loader2,
  Plus,
  LogOut,
  ArrowLeft,
  Sun,
  Moon,
  Code2,
} from "lucide-react";
import { useAuth } from "./auth-provider";
import { useTheme } from "./theme-provider";
import { getStatus, executeFullSync, executeInit, getClients, getSyncSchedule, updateSyncSchedule, StatusResponse, ClientConfig, SyncSchedule } from "@/lib/api-client";
import { RulesManager } from "./rules-manager";
import { ConfigEditor } from "./config-editor";
import { TransformersManager } from "./transformers-manager";
import { ClientsManager } from "./clients-manager";
import { toast } from "sonner";
import { AlertTriangle, Zap, Monitor } from "lucide-react";

interface DashboardProps {
  onBack?: () => void;
}

export function Dashboard({ onBack }: DashboardProps) {
  const { logout } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [clients, setClients] = useState<ClientConfig[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isSyncing, setIsSyncing] = useState(false);
  const [activeTab, setActiveTab] = useState("overview");
  const [needsInit, setNeedsInit] = useState(false);
  const [isInitializing, setIsInitializing] = useState(false);
  const [syncSchedule, setSyncSchedule] = useState<SyncSchedule | null>(null);
  const [isUpdatingSchedule, setIsUpdatingSchedule] = useState(false);
  const [needsFirstSync, setNeedsFirstSync] = useState(false); // 初始化后未同步提醒

  const fetchStatus = async () => {
    try {
      const [data, { clients: clientList }, { schedule }] = await Promise.all([
        getStatus(),
        getClients(),
        getSyncSchedule(),
      ]);
      setStatus(data);
      setClients(clientList);
      setSyncSchedule(schedule);
      // 检查是否需要初始化
      if (data.rulesCount === 0) {
        setNeedsInit(true);
      }
      // 检查是否需要首次同步（有规则但从未全量同步成功）
      const hasNeverSynced = !data.lastSync?.lastFullSyncAt && !data.lastSync?.lastSuccessfulSyncAt;
      if (data.rulesCount > 0 && hasNeverSynced) {
        setNeedsFirstSync(true);
      } else {
        setNeedsFirstSync(false);
      }
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
        setNeedsFirstSync(true); // 初始化后需要同步
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

  const handleFullSync = async () => {
    setIsSyncing(true);
    try {
      const result = await executeFullSync();
      if (result.success) {
        toast.success(`同步成功！${result.changedRules.length} 条规则已更新`);
        setNeedsFirstSync(false); // 同步成功后清除提醒
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

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-slate-900">
        <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-gray-100 dark:from-slate-900 dark:via-slate-800 dark:to-slate-900 transition-colors">
      {/* Header */}
      <header className="border-b bg-white/80 dark:bg-slate-900/80 backdrop-blur-sm sticky top-0 z-50">
        <div className="container mx-auto px-4 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            {onBack && (
              <Button
                variant="ghost"
                size="icon"
                onClick={onBack}
                className="text-gray-500"
              >
                <ArrowLeft className="w-5 h-5" />
              </Button>
            )}
            <div className="w-10 h-10 bg-gradient-to-br from-blue-500 to-cyan-500 rounded-xl flex items-center justify-center">
              <Settings className="w-5 h-5 text-white" />
            </div>
            <div>
              <h1 className="text-xl font-bold text-gray-900 dark:text-white">管理后台</h1>
              <p className="text-xs text-gray-500 dark:text-gray-400">Proxy Rule Manager</p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="icon"
              onClick={toggleTheme}
              className="text-gray-600 dark:text-gray-300"
            >
              {theme === "light" ? (
                <Moon className="w-5 h-5" />
              ) : (
                <Sun className="w-5 h-5" />
              )}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleFullSync}
              disabled={isSyncing}
              className="border-gray-300 dark:border-slate-600"
            >
              {isSyncing ? (
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
              ) : (
                <RefreshCw className="w-4 h-4 mr-2" />
              )}
              同步规则
            </Button>
            <Button
              variant="ghost"
              size="icon"
              onClick={logout}
              className="text-gray-500"
            >
              <LogOut className="w-5 h-5" />
            </Button>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="container mx-auto px-4 py-6">
        {/* 初始化提示 */}
        {needsInit && (
          <Card className="mb-6 border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20">
            <CardContent className="py-6">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className="w-12 h-12 rounded-xl bg-amber-100 dark:bg-amber-800/30 flex items-center justify-center">
                    <AlertTriangle className="w-6 h-6 text-amber-600 dark:text-amber-400" />
                  </div>
                  <div>
                    <h3 className="font-semibold text-amber-900 dark:text-amber-100">系统未初始化</h3>
                    <p className="text-sm text-amber-700 dark:text-amber-300">
                      检测到暂无规则配置，点击按钮导入初始规则模板（基于 ACL4SSR）
                    </p>
                  </div>
                </div>
                <Button
                  onClick={handleInit}
                  disabled={isInitializing}
                  className="bg-amber-600 hover:bg-amber-700 text-white"
                >
                  {isInitializing ? (
                    <>
                      <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                      初始化中...
                    </>
                  ) : (
                    <>
                      <Zap className="w-4 h-4 mr-2" />
                      一键初始化
                    </>
                  )}
                </Button>
              </div>
            </CardContent>
          </Card>
        )}

        {/* 首次同步提醒 */}
        {needsFirstSync && !needsInit && (
          <Card className="mb-6 border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/20">
            <CardContent className="py-6">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className="w-12 h-12 rounded-xl bg-blue-100 dark:bg-blue-800/30 flex items-center justify-center">
                    <RefreshCw className="w-6 h-6 text-blue-600 dark:text-blue-400" />
                  </div>
                  <div>
                    <h3 className="font-semibold text-blue-900 dark:text-blue-100">请执行首次同步</h3>
                    <p className="text-sm text-blue-700 dark:text-blue-300">
                      规则已导入，但尚未同步。点击按钮立即同步以生成规则文件。
                    </p>
                  </div>
                </div>
                <Button
                  onClick={handleFullSync}
                  disabled={isSyncing}
                  className="bg-blue-600 hover:bg-blue-700 text-white"
                >
                  {isSyncing ? (
                    <>
                      <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                      同步中...
                    </>
                  ) : (
                    <>
                      <RefreshCw className="w-4 h-4 mr-2" />
                      立即同步
                    </>
                  )}
                </Button>
              </div>
            </CardContent>
          </Card>
        )}

        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList className="bg-white dark:bg-slate-800 border shadow-sm">
            <TabsTrigger value="overview" className="data-[state=active]:bg-blue-500 data-[state=active]:text-white">
              <Activity className="w-4 h-4 mr-2" />
              概览
            </TabsTrigger>
            <TabsTrigger value="rules" className="data-[state=active]:bg-blue-500 data-[state=active]:text-white">
              <FileText className="w-4 h-4 mr-2" />
              规则管理
            </TabsTrigger>
            <TabsTrigger value="transformers" className="data-[state=active]:bg-blue-500 data-[state=active]:text-white">
              <Code2 className="w-4 h-4 mr-2" />
              转换器
            </TabsTrigger>
            <TabsTrigger value="clients" className="data-[state=active]:bg-blue-500 data-[state=active]:text-white">
              <Monitor className="w-4 h-4 mr-2" />
              客户端
            </TabsTrigger>
            <TabsTrigger value="config" className="data-[state=active]:bg-blue-500 data-[state=active]:text-white">
              <Settings className="w-4 h-4 mr-2" />
              配置编辑
            </TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="mt-6">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
              <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
                <CardHeader className="pb-2">
                  <CardDescription className="text-gray-500 dark:text-gray-400">规则总数</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="text-3xl font-bold text-gray-900 dark:text-white">
                    {status?.rulesCount || 0}
                  </div>
                </CardContent>
              </Card>

              <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
                <CardHeader className="pb-2">
                  <CardDescription className="text-gray-500 dark:text-gray-400">今日更新</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="text-3xl font-bold text-blue-500">
                    {status?.todayStats?.rulesChanged || 0}
                  </div>
                </CardContent>
              </Card>

              <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
                <CardHeader className="pb-2">
                  <CardDescription className="text-gray-500 dark:text-gray-400">Blob 写入</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="text-3xl font-bold text-amber-500">
                    {status?.todayStats?.blobWriteCount || 0}
                  </div>
                </CardContent>
              </Card>

              <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
                <CardHeader className="pb-2">
                  <CardDescription className="text-gray-500 dark:text-gray-400">失败来源</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="text-3xl font-bold text-red-500">
                    {status?.todayStats?.failedSources || 0}
                  </div>
                </CardContent>
              </Card>
            </div>

            {/* Last Sync Info */}
            <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700 mb-6">
              <CardHeader>
                <CardTitle className="text-gray-900 dark:text-white flex items-center gap-2">
                  <Clock className="w-5 h-5 text-blue-500" />
                  同步状态
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <div>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mb-1">上次同步</p>
                    <p className="text-gray-900 dark:text-white">
                      {status?.lastSync?.lastFullSyncAt
                        ? new Date(status.lastSync.lastFullSyncAt).toLocaleString("zh-CN")
                        : "从未同步"}
                    </p>
                  </div>
                  <div>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mb-1">上次单规则刷新</p>
                    <p className="text-gray-900 dark:text-white">
                      {status?.lastSync?.lastPartialSyncAt
                        ? new Date(status.lastSync.lastPartialSyncAt).toLocaleString("zh-CN")
                        : "从未同步"}
                    </p>
                  </div>
                  <div>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mb-1">上次成功同步</p>
                    <p className="text-gray-900 dark:text-white">
                      {status?.lastSync?.lastSuccessfulSyncAt
                        ? new Date(status.lastSync.lastSuccessfulSyncAt).toLocaleString("zh-CN")
                        : "从未成功"}
                    </p>
                  </div>
                </div>

                {/* 定时同步设置 */}
                <div className="mt-4 pt-4 border-t border-gray-200 dark:border-slate-700">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium text-gray-900 dark:text-white">定时同步</p>
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        系统将每隔指定时间自动同步所有规则
                      </p>
                      {syncSchedule?.nextSyncAt && (
                        <p className="text-xs text-blue-500 mt-1">
                          下次同步: {new Date(syncSchedule.nextSyncAt).toLocaleString("zh-CN")}
                        </p>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <select
                        value={syncSchedule?.intervalHours || 24}
                        onChange={async (e) => {
                          const hours = parseInt(e.target.value, 10);
                          setIsUpdatingSchedule(true);
                          try {
                            const result = await updateSyncSchedule(hours);
                            setSyncSchedule(result.schedule);
                            toast.success(`定时同步已设置为每 ${hours} 小时`);
                          } catch (error) {
                            toast.error("更新失败: " + String(error));
                          } finally {
                            setIsUpdatingSchedule(false);
                          }
                        }}
                        disabled={isUpdatingSchedule}
                        className="px-3 py-2 rounded-md border border-gray-300 dark:border-slate-600 bg-white dark:bg-slate-900 text-gray-900 dark:text-white text-sm"
                      >
                        <option value={1}>1 小时</option>
                        <option value={2}>2 小时</option>
                        <option value={6}>6 小时</option>
                        <option value={12}>12 小时</option>
                        <option value={24}>24 小时</option>
                        <option value={48}>48 小时</option>
                        <option value={72}>72 小时</option>
                      </select>
                      {isUpdatingSchedule && (
                        <Loader2 className="w-4 h-4 animate-spin text-blue-500" />
                      )}
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Rules Overview */}
            <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
              <CardHeader>
                <CardTitle className="text-gray-900 dark:text-white flex items-center justify-between">
                  <span className="flex items-center gap-2">
                    <FileText className="w-5 h-5 text-blue-500" />
                    规则列表
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setActiveTab("rules")}
                    className="text-blue-500"
                  >
                    <Plus className="w-4 h-4 mr-1" />
                    管理规则
                  </Button>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2 max-h-96 overflow-y-auto">
                  {status?.rules?.map((rule) => (
                    <div
                      key={rule.name}
                      className="flex items-center justify-between p-3 rounded-lg bg-gray-50 dark:bg-slate-900 hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors"
                    >
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-blue-100 to-cyan-100 dark:from-blue-900/30 dark:to-cyan-900/30 flex items-center justify-center">
                          <FileText className="w-4 h-4 text-blue-500" />
                        </div>
                        <div>
                          <p className="font-medium text-gray-900 dark:text-white">{rule.name}</p>
                          <p className="text-xs text-gray-500 dark:text-gray-400">
                            {rule.description || "无描述"}
                          </p>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        {rule.clients.map((client: string) => (
                          <Badge
                            key={client}
                            variant="secondary"
                            className="bg-gray-200 dark:bg-slate-700 text-gray-700 dark:text-gray-300 whitespace-nowrap"
                          >
                            {getClientDisplayName(client)}
                          </Badge>
                        ))}
                        {rule.hasError ? (
                          <XCircle className="w-5 h-5 text-red-500" />
                        ) : (
                          <CheckCircle className="w-5 h-5 text-green-500" />
                        )}
                      </div>
                    </div>
                  ))}
                  {(!status?.rules || status.rules.length === 0) && (
                    <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                      暂无规则，请先添加规则配置
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="rules" className="mt-6">
            <RulesManager onRefresh={fetchStatus} />
          </TabsContent>

          <TabsContent value="transformers" className="mt-6">
            <TransformersManager onRefresh={fetchStatus} />
          </TabsContent>

          <TabsContent value="clients" className="mt-6">
            <ClientsManager onRefresh={fetchStatus} />
          </TabsContent>

          <TabsContent value="config" className="mt-6">
            <ConfigEditor onSave={fetchStatus} />
          </TabsContent>
        </Tabs>
      </main>
    </div>
  );
}

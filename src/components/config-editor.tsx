"use client";

import { useState, useEffect, useRef } from "react";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Code, Loader2, Maximize2, X, Download, Upload, Database, FileText, Clock } from "lucide-react";
import {
  backupDatabase,
  exportConfigTemplate,
  getConfigRaw,
  getSyncSchedule,
  importConfigTemplate,
  restoreDatabase,
  updateSyncSchedule,
} from "@/lib/api-client";
import { RulesConfig } from "@/lib/schema";
import { toast } from "sonner";
import Editor from "@monaco-editor/react";
import YAML from "yaml";
import { useTheme } from "./theme-provider";

interface ConfigEditorProps {
  onSave: () => void;
}

export function ConfigEditor({ onSave }: ConfigEditorProps) {
  const { theme } = useTheme();
  const [config, setConfig] = useState<RulesConfig | null>(null);
  const [yamlContent, setYamlContent] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isExportingTemplate, setIsExportingTemplate] = useState(false);
  const [isImportingTemplate, setIsImportingTemplate] = useState(false);
  const [isBackingUp, setIsBackingUp] = useState(false);
  const [isRestoring, setIsRestoring] = useState(false);
  const [isScheduleLoading, setIsScheduleLoading] = useState(true);
  const [isScheduleUpdating, setIsScheduleUpdating] = useState(false);
  const [scheduleMode, setScheduleMode] = useState<"interval" | "cron">("interval");
  const [intervalHours, setIntervalHours] = useState(24);
  const [cronExpression, setCronExpression] = useState("0 0 * * *");
  const [syncScheduleNextAt, setSyncScheduleNextAt] = useState<string | null>(null);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const importTemplateRef = useRef<HTMLInputElement | null>(null);
  const restoreDbRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    fetchConfig();
    fetchSchedule();
  }, []);

  // ESC key exits fullscreen
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && isFullscreen) {
        setIsFullscreen(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isFullscreen]);

  const fetchConfig = async () => {
    try {
      const { config } = await getConfigRaw();
      setConfig(config);
      setYamlContent(YAML.stringify(config, { indent: 2 }));
    } catch (error) {
      console.error("Failed to fetch config:", error);
      toast.error("获取配置失败");
    } finally {
      setIsLoading(false);
    }
  };

  const handleExportTemplate = async () => {
    setIsExportingTemplate(true);
    try {
      const blob = await exportConfigTemplate();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      const dateTag = new Date().toISOString().split("T")[0];
      link.href = url;
      link.download = `proxy-rule-template-${dateTag}.json`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
      toast.success("模板导出成功");
    } catch (error) {
      toast.error("模板导出失败: " + String(error));
    } finally {
      setIsExportingTemplate(false);
    }
  };

  const handleImportTemplate = async (file: File) => {
    setIsImportingTemplate(true);
    try {
      await importConfigTemplate(file);
      toast.success("模板导入成功，数据库已重置");
      await fetchConfig();
      onSave();
    } catch (error) {
      toast.error("模板导入失败: " + String(error));
    } finally {
      setIsImportingTemplate(false);
    }
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
      await fetchConfig();
      onSave();
    } catch (error) {
      toast.error("数据库恢复失败: " + String(error));
    } finally {
      setIsRestoring(false);
    }
  };

  const fetchSchedule = async () => {
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
  };

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

  const editorTheme = theme === "dark" ? "vs-dark" : "light";

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
      </div>
    );
  }

  // Fullscreen mode
  if (isFullscreen) {
    return (
      <div className="fixed inset-0 z-50 bg-white dark:bg-slate-900 flex flex-col">
        <input
          ref={importTemplateRef}
          type="file"
          accept=".json,.yaml,.yml"
          className="hidden"
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) {
              handleImportTemplate(file);
            }
            e.currentTarget.value = "";
          }}
        />
        <input
          ref={restoreDbRef}
          type="file"
          accept=".zip"
          className="hidden"
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) {
              handleRestore(file);
            }
            e.currentTarget.value = "";
          }}
        />
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-slate-700 bg-gray-50 dark:bg-slate-800">
          <div className="flex items-center gap-3">
            <Code className="w-5 h-5 text-blue-500" />
            <span className="font-semibold text-gray-900 dark:text-white">YAML 配置编辑器</span>
            <Badge variant="outline" className="border-gray-300 text-gray-500">
              只读
            </Badge>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setIsFullscreen(false)}
              className="ml-2"
            >
              <X className="w-5 h-5" />
            </Button>
          </div>
        </div>

        <div className="flex-1">
          <Editor
            height="100%"
            defaultLanguage="yaml"
            value={yamlContent}
            theme={editorTheme}
            options={{
              readOnly: true,
              minimap: { enabled: true },
              fontSize: 14,
              lineNumbers: "on",
              scrollBeyondLastLine: false,
              automaticLayout: true,
              tabSize: 2,
              wordWrap: "off",
              padding: { top: 16 },
              scrollbar: {
                horizontal: "visible",
                vertical: "visible",
                horizontalScrollbarSize: 12,
                verticalScrollbarSize: 12,
              },
            }}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <input
        ref={importTemplateRef}
        type="file"
        accept=".json,.yaml,.yml"
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) {
            handleImportTemplate(file);
          }
          e.currentTarget.value = "";
        }}
      />
      <input
        ref={restoreDbRef}
        type="file"
        accept=".zip"
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) {
            handleRestore(file);
          }
          e.currentTarget.value = "";
        }}
      />
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
          <CardHeader className="pb-2">
            <CardDescription className="text-gray-500 dark:text-gray-400">配置版本</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-gray-900 dark:text-white">v{config?.version || 1}</div>
          </CardContent>
        </Card>
        <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
          <CardHeader className="pb-2">
            <CardDescription className="text-gray-500 dark:text-gray-400">规则数量</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-500">{config?.rules?.length || 0}</div>
          </CardContent>
        </Card>
        <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
          <CardHeader className="pb-2">
            <CardDescription className="text-gray-500 dark:text-gray-400">预定义转换器</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-amber-500">
              {Object.keys(config?.transformers || {}).length}
            </div>
          </CardContent>
        </Card>
      </div>

      <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
        <CardHeader>
          <CardTitle className="text-gray-900 dark:text-white flex items-center gap-2">
            <Clock className="w-5 h-5 text-blue-500" />
            定时同步
          </CardTitle>
          <CardDescription className="text-gray-500 dark:text-gray-400">
            支持 interval 与 cron 两种模式，系统将按配置自动同步规则。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {isScheduleLoading ? (
            <div className="flex items-center gap-2 text-gray-500">
              <Loader2 className="w-4 h-4 animate-spin text-blue-500" />
              正在加载定时配置...
            </div>
          ) : (
            <>
              <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div className="flex items-center gap-3">
                  <span className="text-sm text-gray-600 dark:text-gray-300">模式</span>
                  <Select
                    value={scheduleMode}
                    onValueChange={(value) => setScheduleMode(value as "interval" | "cron")}
                  >
                    <SelectTrigger className="w-40">
                      <SelectValue placeholder="选择模式" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="interval">Interval</SelectItem>
                      <SelectItem value="cron">Cron</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                {syncScheduleNextAt && (
                  <span className="text-xs text-blue-500">
                    下次同步: {new Date(syncScheduleNextAt).toLocaleString("zh-CN")}
                  </span>
                )}
              </div>

              {scheduleMode === "interval" ? (
                <div className="flex flex-col gap-2 md:flex-row md:items-center">
                  <span className="text-sm text-gray-600 dark:text-gray-300">同步间隔</span>
                  <Select
                    value={String(intervalHours)}
                    onValueChange={(value) => setIntervalHours(parseInt(value, 10))}
                  >
                    <SelectTrigger className="w-40">
                      <SelectValue placeholder="选择间隔" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="1">1 小时</SelectItem>
                      <SelectItem value="2">2 小时</SelectItem>
                      <SelectItem value="6">6 小时</SelectItem>
                      <SelectItem value="12">12 小时</SelectItem>
                      <SelectItem value="24">24 小时</SelectItem>
                      <SelectItem value="48">48 小时</SelectItem>
                      <SelectItem value="72">72 小时</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              ) : (
                <div className="space-y-1">
                  <label className="text-sm text-gray-600 dark:text-gray-300">
                    Cron 表达式（支持 5/6 段）
                  </label>
                  <input
                    value={cronExpression}
                    onChange={(e) => setCronExpression(e.target.value)}
                    className="w-full px-3 py-2 rounded-md border border-gray-300 dark:border-slate-600 bg-white dark:bg-slate-900 text-gray-900 dark:text-white text-sm font-mono"
                    placeholder="0 0 * * *"
                  />
                  <p className="text-xs text-gray-500 dark:text-gray-400">
                    示例: <span className="font-mono">0 */6 * * *</span> 表示每 6 小时执行一次
                  </p>
                </div>
              )}

              <div className="flex items-center gap-3">
                <Button
                  variant="outline"
                  onClick={handleSaveSchedule}
                  disabled={
                    isScheduleUpdating ||
                    (scheduleMode === "cron" && !cronExpression.trim())
                  }
                >
                  {isScheduleUpdating ? (
                    <Loader2 className="w-4 h-4 animate-spin" />
                  ) : (
                    "保存定时配置"
                  )}
                </Button>
                <span className="text-xs text-gray-500 dark:text-gray-400">
                  Cron 模式变更后将重新计算下次同步时间。
                </span>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <CardTitle className="text-gray-900 dark:text-white flex items-center gap-2">
                <Code className="w-5 h-5 text-blue-500" />
                YAML 配置编辑器
              </CardTitle>
              <Badge variant="outline" className="border-gray-300 text-gray-500">
                只读
              </Badge>
            </div>
          </div>
          <CardDescription className="text-gray-500 dark:text-gray-400">
            只读查看当前配置模版。请使用下方导入模版或数据库恢复来变更配置。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="relative border border-gray-200 dark:border-slate-700 rounded-lg overflow-hidden">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setIsFullscreen(true)}
              className="absolute top-2 right-2 z-10 bg-white/80 dark:bg-slate-800/80 hover:bg-white dark:hover:bg-slate-700 shadow-sm"
              title="全屏查看 (ESC 退出)"
            >
              <Maximize2 className="w-4 h-4" />
            </Button>
            <Editor
              height="600px"
              defaultLanguage="yaml"
              value={yamlContent}
              theme={editorTheme}
              options={{
                readOnly: true,
                minimap: { enabled: false },
                fontSize: 13,
                lineNumbers: "on",
                scrollBeyondLastLine: false,
                automaticLayout: true,
                tabSize: 2,
                wordWrap: "off",
                padding: { top: 16, bottom: 16 },
                scrollbar: {
                  horizontal: "visible",
                  vertical: "visible",
                  horizontalScrollbarSize: 10,
                  verticalScrollbarSize: 10,
                },
              }}
            />
          </div>
          <div className="text-sm text-gray-500 dark:text-gray-400">
            提示: 配置编辑为只读，需通过导入模版或恢复数据库来变更。
          </div>
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
          <CardHeader>
            <CardTitle className="text-gray-900 dark:text-white flex items-center gap-2">
              <FileText className="w-5 h-5 text-blue-500" />
              配置模板
            </CardTitle>
            <CardDescription className="text-gray-500 dark:text-gray-400">
              分享/导入规则模板，仅包含配置内容，不含运行元数据。
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-3">
            <Button
              variant="outline"
              onClick={() => importTemplateRef.current?.click()}
              disabled={isImportingTemplate}
            >
              {isImportingTemplate ? <Loader2 className="w-4 h-4 animate-spin" /> : <Upload className="w-4 h-4 mr-2" />}
              导入模板
            </Button>
            <Button
              variant="outline"
              onClick={handleExportTemplate}
              disabled={isExportingTemplate}
            >
              {isExportingTemplate ? <Loader2 className="w-4 h-4 animate-spin" /> : <Download className="w-4 h-4 mr-2" />}
              导出模板
            </Button>
          </CardContent>
        </Card>

        <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
          <CardHeader>
            <CardTitle className="text-gray-900 dark:text-white flex items-center gap-2">
              <Database className="w-5 h-5 text-amber-500" />
              数据库备份与恢复
            </CardTitle>
            <CardDescription className="text-gray-500 dark:text-gray-400">
              备份/恢复完整数据库（含元数据）。恢复将覆盖当前数据。
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-3">
            <Button
              variant="outline"
              onClick={() => restoreDbRef.current?.click()}
              disabled={isRestoring}
            >
              {isRestoring ? <Loader2 className="w-4 h-4 animate-spin" /> : <Upload className="w-4 h-4 mr-2" />}
              恢复数据库
            </Button>
            <Button
              variant="outline"
              onClick={handleBackup}
              disabled={isBackingUp}
            >
              {isBackingUp ? <Loader2 className="w-4 h-4 animate-spin" /> : <Download className="w-4 h-4 mr-2" />}
              备份数据库
            </Button>
          </CardContent>
        </Card>
      </div>

      <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
        <CardHeader>
          <CardTitle className="text-gray-900 dark:text-white text-lg">配置说明</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4 text-gray-700 dark:text-gray-300 text-sm">
          <div>
            <h4 className="font-medium text-gray-900 dark:text-white mb-2">规则结构</h4>
            <pre className="p-3 rounded bg-gray-100 dark:bg-slate-900 text-xs font-mono overflow-x-auto text-gray-800 dark:text-gray-200">
              {`rules:
  - name: YouTube          # 规则名称（必填）
    description: YouTube 规则  # 描述（可选）
    sources:               # 数据来源
      - type: url
        url: https://example.com/rules.list
      - type: ref
        ref: OtherRule     # 引用其他规则
      - type: local
        content: |         # 本地内容
          DOMAIN,example.com
    transforms:            # 后处理转换（可选）
      - type: replace
        pattern: "old"
        replacement: "new"
    merge:                 # 合并配置
      strategy: concat     # concat/union/intersect
      dedupe: true         # 是否去重
    output:
      clients:             # 输出客户端（多选）
        - clash_meta
        - shadowrocket
      client_overrides:    # 客户端差异化配置
        shadowrocket:
          transforms:
            - type: replace
              pattern: "..."
              replacement: "..."`}
            </pre>
          </div>

          <div>
            <h4 className="font-medium text-gray-900 dark:text-white mb-2">支持的转换类型</h4>
            <ul className="space-y-1 list-disc list-inside text-gray-600 dark:text-gray-400">
              <li><code className="text-blue-600 dark:text-blue-400">replace</code> - 正则替换</li>
              <li><code className="text-blue-600 dark:text-blue-400">remove_lines</code> - 删除匹配行</li>
              <li><code className="text-blue-600 dark:text-blue-400">regex_extract</code> - 正则提取并模板化</li>
              <li><code className="text-blue-600 dark:text-blue-400">dedupe</code> - 去重</li>
              <li><code className="text-blue-600 dark:text-blue-400">sort</code> - 排序</li>
              <li><code className="text-blue-600 dark:text-blue-400">trim</code> - 去空白</li>
              <li><code className="text-blue-600 dark:text-blue-400">normalize_eol</code> - 规范换行符</li>
            </ul>
          </div>

          <div>
            <h4 className="font-medium text-gray-900 dark:text-white mb-2">聚合规则</h4>
            <pre className="p-3 rounded bg-gray-100 dark:bg-slate-900 text-xs font-mono overflow-x-auto text-gray-800 dark:text-gray-200">
              {`rules:
  - name: ProxyMedia
    compose_from:         # 聚合其他规则
      - YouTube
      - Netflix
      - Spotify
    merge:
      strategy: union
    output:
      clients: [clash_meta, shadowrocket]`}
            </pre>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

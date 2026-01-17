"use client";

import { useState, useEffect, useRef } from "react";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Code, Save, Loader2, AlertCircle, CheckCircle, Maximize2, X, Download, Upload } from "lucide-react";
import { exportConfigBundle, getConfigRaw, importConfigBundle, saveConfig } from "@/lib/api-client";
import { RulesConfig, validateConfig } from "@/lib/schema";
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
  const [isSaving, setIsSaving] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [isImporting, setIsImporting] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [isDirty, setIsDirty] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const importInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    fetchConfig();
  }, []);

  // ESC 键退出全屏
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
      setIsDirty(false);
    } catch (error) {
      console.error("Failed to fetch config:", error);
      toast.error("获取配置失败");
    } finally {
      setIsLoading(false);
    }
  };

  const handleYamlChange = (value: string | undefined) => {
    const newValue = value || "";
    setYamlContent(newValue);
    setIsDirty(true);

    try {
      const parsed = YAML.parse(newValue);
      validateConfig(parsed);
      setValidationError(null);
    } catch (error) {
      if (error instanceof Error) {
        setValidationError(error.message);
      }
    }
  };

  const handleSave = async () => {
    try {
      const parsed = YAML.parse(yamlContent);
      const validated = validateConfig(parsed);

      setIsSaving(true);
      await saveConfig(validated);
      setConfig(validated);
      setIsDirty(false);
      toast.success("配置保存成功");
      onSave();
    } catch (error) {
      toast.error("保存失败: " + String(error));
    } finally {
      setIsSaving(false);
    }
  };

  const handleExport = async () => {
    setIsExporting(true);
    try {
      const blob = await exportConfigBundle();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      const dateTag = new Date().toISOString().split("T")[0];
      link.href = url;
      link.download = `proxy-rule-manager-${dateTag}.zip`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
      toast.success("导出成功");
    } catch (error) {
      toast.error("导出失败: " + String(error));
    } finally {
      setIsExporting(false);
    }
  };

  const handleImport = async (file: File) => {
    setIsImporting(true);
    try {
      await importConfigBundle(file);
      toast.success("导入成功");
      await fetchConfig();
      onSave();
    } catch (error) {
      toast.error("导入失败: " + String(error));
    } finally {
      setIsImporting(false);
    }
  };

  const handleReset = () => {
    if (config) {
      setYamlContent(YAML.stringify(config, { indent: 2 }));
      setIsDirty(false);
      setValidationError(null);
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

  // 全屏模式
  if (isFullscreen) {
    return (
      <div className="fixed inset-0 z-50 bg-white dark:bg-slate-900 flex flex-col">
        <input
          ref={importInputRef}
          type="file"
          accept=".zip"
          className="hidden"
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) {
              handleImport(file);
            }
            e.currentTarget.value = "";
          }}
        />
        {/* 顶部工具栏 */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-slate-700 bg-gray-50 dark:bg-slate-800">
          <div className="flex items-center gap-3">
            <Code className="w-5 h-5 text-blue-500" />
            <span className="font-semibold text-gray-900 dark:text-white">YAML 配置编辑器</span>
            {isDirty && (
              <Badge variant="outline" className="border-amber-500 text-amber-500">
                未保存
              </Badge>
            )}
            {validationError ? (
              <Badge variant="destructive" className="flex items-center gap-1">
                <AlertCircle className="w-3 h-3" />
                格式错误
              </Badge>
            ) : (
              <Badge className="bg-green-100 dark:bg-green-500/20 text-green-700 dark:text-green-400 flex items-center gap-1">
                <CheckCircle className="w-3 h-3" />
                格式正确
              </Badge>
            )}
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => importInputRef.current?.click()}
              disabled={isImporting}
            >
              {isImporting ? <Loader2 className="w-4 h-4 animate-spin" /> : <Upload className="w-4 h-4 mr-1" />}
              导入
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleExport}
              disabled={isExporting}
            >
              {isExporting ? <Loader2 className="w-4 h-4 animate-spin" /> : <Download className="w-4 h-4 mr-1" />}
              导出
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleReset}
              disabled={!isDirty}
            >
              重置
            </Button>
            <Button
              size="sm"
              onClick={handleSave}
              disabled={isSaving || !!validationError}
              className="bg-gradient-to-r from-blue-500 to-cyan-500 hover:from-blue-600 hover:to-cyan-600 text-white"
            >
              {isSaving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4 mr-1" />}
              保存
            </Button>
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

        {/* 错误提示 */}
        {validationError && (
          <div className="px-4 py-2 bg-red-50 dark:bg-red-500/10 border-b border-red-200 dark:border-red-500/30 text-red-700 dark:text-red-400 text-sm">
            <span className="font-medium">验证错误: </span>
            <span className="font-mono text-xs">{validationError}</span>
          </div>
        )}

        {/* 编辑器 */}
        <div className="flex-1">
          <Editor
            height="100%"
            defaultLanguage="yaml"
            value={yamlContent}
            onChange={handleYamlChange}
            theme={editorTheme}
            options={{
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
        ref={importInputRef}
        type="file"
        accept=".zip"
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) {
            handleImport(file);
          }
          e.currentTarget.value = "";
        }}
      />
      {/* Info Cards */}
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

      {/* Editor */}
      <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <CardTitle className="text-gray-900 dark:text-white flex items-center gap-2">
                <Code className="w-5 h-5 text-blue-500" />
                YAML 配置编辑器
              </CardTitle>
              {isDirty && (
                <Badge variant="outline" className="border-amber-500 text-amber-500">
                  未保存
                </Badge>
              )}
            </div>
            {validationError ? (
              <Badge variant="destructive" className="flex items-center gap-1">
                <AlertCircle className="w-3 h-3" />
                格式错误
              </Badge>
            ) : (
              <Badge className="bg-green-100 dark:bg-green-500/20 text-green-700 dark:text-green-400 flex items-center gap-1">
                <CheckCircle className="w-3 h-3" />
                格式正确
              </Badge>
            )}
          </div>
          <CardDescription className="text-gray-500 dark:text-gray-400">
            直接编辑 YAML 配置文件。支持语法高亮和实时验证。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {validationError && (
            <div className="p-3 rounded-lg bg-red-50 dark:bg-red-500/10 border border-red-200 dark:border-red-500/30 text-red-700 dark:text-red-400 text-sm">
              <p className="font-medium">验证错误:</p>
              <p className="mt-1 font-mono text-xs">{validationError}</p>
            </div>
          )}

          {/* 编辑器容器 - 相对定位用于放置全屏按钮 */}
          <div className="relative border border-gray-200 dark:border-slate-700 rounded-lg overflow-hidden">
            {/* 全屏按钮 - 编辑器右上角 */}
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setIsFullscreen(true)}
              className="absolute top-2 right-2 z-10 bg-white/80 dark:bg-slate-800/80 hover:bg-white dark:hover:bg-slate-700 shadow-sm"
              title="全屏编辑 (ESC 退出)"
            >
              <Maximize2 className="w-4 h-4" />
            </Button>
            <Editor
              height="600px"
              defaultLanguage="yaml"
              value={yamlContent}
              onChange={handleYamlChange}
              theme={editorTheme}
              options={{
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

          <div className="flex items-center justify-between">
            <div className="text-sm text-gray-500 dark:text-gray-400">
              提示: 保存后需要手动刷新规则才能生效
            </div>
            <div className="flex items-center gap-3">
              <Button
                variant="outline"
                onClick={() => importInputRef.current?.click()}
                disabled={isImporting}
                className="border-gray-300 dark:border-slate-600"
              >
                {isImporting ? (
                  <>
                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                    导入中
                  </>
                ) : (
                  <>
                    <Upload className="w-4 h-4 mr-2" />
                    导入配置
                  </>
                )}
              </Button>
              <Button
                variant="outline"
                onClick={handleExport}
                disabled={isExporting}
                className="border-gray-300 dark:border-slate-600"
              >
                {isExporting ? (
                  <>
                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                    导出中
                  </>
                ) : (
                  <>
                    <Download className="w-4 h-4 mr-2" />
                    导出配置
                  </>
                )}
              </Button>
              <Button
                variant="outline"
                onClick={handleReset}
                disabled={!isDirty}
                className="border-gray-300 dark:border-slate-600"
              >
                重置
              </Button>
              <Button
                onClick={handleSave}
                disabled={isSaving || !!validationError}
                className="bg-gradient-to-r from-blue-500 to-cyan-500 hover:from-blue-600 hover:to-cyan-600 text-white"
              >
                {isSaving ? (
                  <>
                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                    保存中...
                  </>
                ) : (
                  <>
                    <Save className="w-4 h-4 mr-2" />
                    保存配置
                  </>
                )}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Help Section */}
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

"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  RefreshCw,
  Eye,
  MoreVertical,
  Plus,
  Search,
  Copy,
  Loader2,
  FileText,
  CheckCircle,
  XCircle,
  Maximize2,
  X,
  Trash2,
  AlertTriangle,
} from "lucide-react";
import { getConfig, refreshRule, previewRule, deleteRule, getClients, PreviewResponse, ClientConfig } from "@/lib/api-client";
import { RulesConfig, RuleConfig, ClientType } from "@/lib/schema";
import { RuleEditor } from "./rule-editor";
import { toast } from "sonner";

interface RulesManagerProps {
  onRefresh: () => void;
}

export function RulesManager({ onRefresh }: RulesManagerProps) {
  const [config, setConfig] = useState<RulesConfig | null>(null);
  const [clients, setClients] = useState<ClientConfig[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [refreshingRules, setRefreshingRules] = useState<Set<string>>(new Set());
  const [previewData, setPreviewData] = useState<PreviewResponse | null>(null);
  const [previewingRule, setPreviewingRule] = useState<string | null>(null);
  const [previewClient, setPreviewClient] = useState<ClientType>("clash_meta");
  const [editingRule, setEditingRule] = useState<RuleConfig | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [isPreviewFullscreen, setIsPreviewFullscreen] = useState(false);
  const [deletingRule, setDeletingRule] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const fetchConfig = async () => {
    try {
      const [{ config }, { clients: clientList }] = await Promise.all([
        getConfig(),
        getClients(),
      ]);
      setConfig(config);
      setClients(clientList);
    } catch (error) {
      console.error("Failed to fetch data:", error);
      toast.error("获取配置失败");
    } finally {
      setIsLoading(false);
    }
  };

  const getClientDisplayName = (clientId: string): string => {
    const client = clients.find(c => c.id === clientId);
    return client?.displayName || clientId;
  };

  const getClientPathName = (clientId: string): string => {
    const client = clients.find(c => c.id === clientId);
    return client?.pathName || clientId;
  };

  useEffect(() => {
    fetchConfig();
  }, []);

  // ESC 键退出全屏
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && isPreviewFullscreen) {
        setIsPreviewFullscreen(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isPreviewFullscreen]);

  const handleRefreshRule = async (ruleName: string) => {
    setRefreshingRules((prev) => new Set(prev).add(ruleName));
    try {
      const result = await refreshRule(ruleName);
      if (result.success) {
        toast.success(`规则 "${ruleName}" 刷新成功`);
      } else {
        toast.warning(`规则刷新完成，但有错误: ${result.failedRules.map((r) => r.error).join(", ")}`);
      }
      onRefresh();
    } catch (error) {
      toast.error("刷新失败: " + String(error));
    } finally {
      setRefreshingRules((prev) => {
        const next = new Set(prev);
        next.delete(ruleName);
        return next;
      });
    }
  };

  const handlePreviewRule = async (ruleName: string, clients: ClientType[]) => {
    setPreviewingRule(ruleName);
    setPreviewClient(clients[0] || "clash_meta");
    setPreviewData(null);
    try {
      const result = await previewRule(ruleName);
      setPreviewData(result);
    } catch (error) {
      toast.error("预览失败: " + String(error));
      setPreviewingRule(null);
    }
  };

  const copyRuleUrl = (ruleName: string, client: ClientType) => {
    const clientPath = getClientPathName(client);
    const url = `${window.location.origin}/Rules/${clientPath}/${ruleName}.list`;
    navigator.clipboard.writeText(url);
    toast.success("已复制规则 URL");
  };

  const handleDeleteRule = async (ruleName: string) => {
    setIsDeleting(true);
    try {
      const result = await deleteRule(ruleName);
      if (result.success) {
        toast.success(`规则 "${ruleName}" 已删除`);
        await fetchConfig();
        onRefresh();
      }
    } catch (error) {
      toast.error("删除失败: " + String(error));
    } finally {
      setIsDeleting(false);
      setDeletingRule(null);
    }
  };

  const closePreview = () => {
    setPreviewingRule(null);
    setIsPreviewFullscreen(false);
  };

  const filteredRules = config?.rules.filter(
    (rule) =>
      rule.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      rule.displayName?.toLowerCase().includes(searchQuery.toLowerCase()) ||
      rule.description?.toLowerCase().includes(searchQuery.toLowerCase())
  );

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
      </div>
    );
  }

  // 全屏预览模式
  if (isPreviewFullscreen && previewingRule && previewData) {
    return (
      <div className="fixed inset-0 z-50 bg-white dark:bg-slate-900 flex flex-col">
        {/* 顶部工具栏 */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-slate-700 bg-gray-50 dark:bg-slate-800">
          <div className="flex items-center gap-3">
            <FileText className="w-5 h-5 text-blue-500" />
            <span className="font-semibold text-gray-900 dark:text-white">预览: {previewingRule}</span>
            {previewData.diagnostics.truncated && (
              <Badge variant="outline" className="border-amber-500 text-amber-500">
                内容已截断（共 {previewData.diagnostics.totalLines} 行）
              </Badge>
            )}
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                const content = previewData.contents[previewClient];
                if (content) {
                  navigator.clipboard.writeText(content);
                  toast.success("已复制内容");
                }
              }}
            >
              <Copy className="w-4 h-4 mr-1" />
              复制
            </Button>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setIsPreviewFullscreen(false)}
            >
              <X className="w-5 h-5" />
            </Button>
          </div>
        </div>

        {/* 数据源状态 */}
        {previewData.diagnostics.sourceResults.length > 0 && (
          <div className="px-4 py-2 bg-gray-50 dark:bg-slate-800/50 border-b border-gray-200 dark:border-slate-700">
            <div className="flex flex-wrap gap-4">
              {previewData.diagnostics.sourceResults.map((source, i) => (
                <div key={i} className="flex items-center gap-2 text-sm">
                  {source.success ? (
                    <CheckCircle className="w-4 h-4 text-green-500" />
                  ) : (
                    <XCircle className="w-4 h-4 text-red-500" />
                  )}
                  <span className="text-xs font-medium text-gray-500 dark:text-gray-400">#{i + 1}</span>
                  <span className="text-gray-700 dark:text-gray-300 truncate max-w-md">{source.url}</span>
                  {source.size !== undefined && source.size > 0 && (
                    <span className="text-gray-500">({(source.size / 1024).toFixed(1)} KB)</span>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* 客户端标签 */}
        <Tabs value={previewClient} onValueChange={(v) => setPreviewClient(v as ClientType)} className="flex-1 flex flex-col min-h-0 overflow-hidden">
          <div className="px-4 py-2 border-b border-gray-200 dark:border-slate-700 shrink-0">
            <TabsList className="bg-gray-100 dark:bg-slate-800">
              {Object.keys(previewData.contents).map((client) => (
                <TabsTrigger key={client} value={client} className="data-[state=active]:bg-white dark:data-[state=active]:bg-slate-700">
                  {getClientDisplayName(client)}
                </TabsTrigger>
              ))}
            </TabsList>
          </div>
          {Object.entries(previewData.contents).map(([client, content]) => (
            <TabsContent key={client} value={client} className="flex-1 m-0 min-h-0 overflow-auto">
              <pre className="p-4 text-sm font-mono text-gray-800 dark:text-gray-200 whitespace-pre">
                {content || "暂无内容"}
              </pre>
            </TabsContent>
          ))}
        </Tabs>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Search and Actions */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center gap-4">
        <div className="relative flex-1 w-full sm:max-w-md">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
          <Input
            placeholder="搜索规则..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-10 bg-white dark:bg-slate-800"
          />
        </div>
        <Button
          onClick={() => {
            setEditingRule(null);
            setIsEditorOpen(true);
          }}
          className="bg-gradient-to-r from-blue-500 to-cyan-500 hover:from-blue-600 hover:to-cyan-600 text-white"
        >
          <Plus className="w-4 h-4 mr-2" />
          添加规则
        </Button>
      </div>

      {/* Rules Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {filteredRules?.map((rule) => (
          <Card key={rule.name} className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700 hover:shadow-lg transition-shadow">
            <CardHeader className="pb-2">
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-blue-100 to-cyan-100 dark:from-blue-900/30 dark:to-cyan-900/30 flex items-center justify-center">
                    <FileText className="w-5 h-5 text-blue-500" />
                  </div>
                  <div>
                    <CardTitle className="text-gray-900 dark:text-white text-lg">{rule.displayName || rule.name}</CardTitle>
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                      {rule.description || `ID: ${rule.name}`}
                    </p>
                  </div>
                </div>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon" className="text-gray-500">
                      <MoreVertical className="w-4 h-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
                    <DropdownMenuItem
                      onClick={() => {
                        setEditingRule(rule);
                        setIsEditorOpen(true);
                      }}
                    >
                      编辑规则
                    </DropdownMenuItem>
                    {rule.output.clients.map((client) => (
                      <DropdownMenuItem
                        key={client}
                        onClick={() => copyRuleUrl(rule.name, client)}
                      >
                        <Copy className="w-4 h-4 mr-2" />
                        复制 {getClientDisplayName(client)} URL
                      </DropdownMenuItem>
                    ))}
                    <DropdownMenuItem
                      onClick={() => setDeletingRule(rule.name)}
                      className="text-red-600 focus:text-red-600 focus:bg-red-50 dark:focus:bg-red-900/20"
                    >
                      <Trash2 className="w-4 h-4 mr-2" />
                      删除规则
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </CardHeader>
            <CardContent>
              <div className="flex items-center gap-2 mb-3 flex-wrap">
                {rule.output.clients.map((client) => (
                  <Badge
                    key={client}
                    variant="secondary"
                    className="bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-gray-300"
                  >
                    {getClientDisplayName(client)}
                  </Badge>
                ))}
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handlePreviewRule(rule.name, rule.output.clients)}
                  className="flex-1"
                >
                  <Eye className="w-4 h-4 mr-1" />
                  预览
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleRefreshRule(rule.name)}
                  disabled={refreshingRules.has(rule.name)}
                  className="flex-1"
                >
                  {refreshingRules.has(rule.name) ? (
                    <Loader2 className="w-4 h-4 mr-1 animate-spin" />
                  ) : (
                    <RefreshCw className="w-4 h-4 mr-1" />
                  )}
                  刷新
                </Button>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {filteredRules?.length === 0 && (
        <div className="text-center py-12 text-gray-500 dark:text-gray-400">
          {searchQuery ? "未找到匹配的规则" : "暂无规则，请添加规则配置"}
        </div>
      )}

      {/* Preview Dialog */}
      <Dialog open={!!previewingRule && !isPreviewFullscreen} onOpenChange={(open) => !open && closePreview()}>
        <DialogContent className="max-w-5xl w-[90vw] h-[80vh] bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700 flex flex-col p-0">
          <DialogHeader className="px-6 pt-6 pb-4 border-b border-gray-200 dark:border-slate-700">
            <DialogTitle className="text-gray-900 dark:text-white flex items-center gap-2">
              <FileText className="w-5 h-5 text-blue-500" />
              预览: {previewingRule}
            </DialogTitle>
            {previewData?.diagnostics.truncated && (
              <DialogDescription className="text-amber-500">
                内容已截断（共 {previewData.diagnostics.totalLines} 行）
              </DialogDescription>
            )}
          </DialogHeader>

          {previewData ? (
            <div className="flex-1 flex flex-col min-h-0 overflow-hidden">
              {/* Source Results */}
              {previewData.diagnostics.sourceResults.length > 0 && (
                <div className="px-6 py-3 bg-gray-50 dark:bg-slate-900 border-b border-gray-200 dark:border-slate-700">
                  <p className="text-sm text-gray-500 dark:text-gray-400 mb-2">数据源状态:</p>
                  <div className="flex flex-wrap gap-4">
                    {previewData.diagnostics.sourceResults.map((source, i) => (
                      <div key={i} className="flex items-center gap-2 text-sm">
                        {source.success ? (
                          <CheckCircle className="w-4 h-4 text-green-500" />
                        ) : (
                          <XCircle className="w-4 h-4 text-red-500" />
                        )}
                        <span className="text-xs font-medium text-gray-500 dark:text-gray-400">#{i + 1}</span>
                        <span className="text-gray-700 dark:text-gray-300 truncate max-w-xs">{source.url}</span>
                        {source.size !== undefined && source.size > 0 && (
                          <span className="text-gray-500">({(source.size / 1024).toFixed(1)} KB)</span>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Content Tabs */}
              <Tabs value={previewClient} onValueChange={(v) => setPreviewClient(v as ClientType)} className="flex-1 flex flex-col min-h-0">
                <div className="px-6 py-3 border-b border-gray-200 dark:border-slate-700 flex items-center justify-between">
                  <TabsList className="bg-gray-100 dark:bg-slate-900">
                    {Object.keys(previewData.contents).map((client) => (
                      <TabsTrigger key={client} value={client} className="data-[state=active]:bg-white dark:data-[state=active]:bg-slate-800">
                        {getClientDisplayName(client)}
                      </TabsTrigger>
                    ))}
                  </TabsList>
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    {previewData.contents[previewClient]?.split('\n').length || 0} 行
                  </span>
                </div>
                {Object.entries(previewData.contents).map(([client, content]) => (
                  <TabsContent key={client} value={client} className="flex-1 m-0 relative min-h-0 overflow-hidden">
                    {/* 全屏和复制按钮 - 内容区域右上角 */}
                    <div className="absolute top-2 right-2 z-10 flex gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => {
                          navigator.clipboard.writeText(content);
                          toast.success("已复制内容");
                        }}
                        className="bg-white/80 dark:bg-slate-800/80 hover:bg-white dark:hover:bg-slate-700 shadow-sm"
                        title="复制内容"
                      >
                        <Copy className="w-4 h-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setIsPreviewFullscreen(true)}
                        className="bg-white/80 dark:bg-slate-800/80 hover:bg-white dark:hover:bg-slate-700 shadow-sm"
                        title="全屏预览 (ESC 退出)"
                      >
                        <Maximize2 className="w-4 h-4" />
                      </Button>
                    </div>
                    <div className="h-full overflow-auto bg-gray-50 dark:bg-slate-900">
                      <pre className="p-4 text-sm font-mono text-gray-800 dark:text-gray-200 whitespace-pre min-w-max">
                        {content || "暂无内容"}
                      </pre>
                    </div>
                  </TabsContent>
                ))}
              </Tabs>
            </div>
          ) : (
            <div className="flex-1 flex items-center justify-center">
              <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* Rule Editor Dialog */}
      {/* Rule Editor Dialog */}
      <Dialog open={isEditorOpen} onOpenChange={setIsEditorOpen}>
        <DialogContent className="max-w-4xl h-[90vh] p-0 flex flex-col bg-background border-border overflow-hidden gap-0">
          <DialogHeader className="p-6 pb-2 border-b shrink-0 hidden"> {/* Hidden because custom header in Editor */}
            <DialogTitle className="text-foreground">
              {editingRule ? `编辑规则: ${editingRule.name}` : "添加新规则"}
            </DialogTitle>
          </DialogHeader>
          <RuleEditor
            rule={editingRule}
            config={config}
            onSave={async () => {
              setIsEditorOpen(false);
              await fetchConfig();
              onRefresh();
            }}
            onCancel={() => setIsEditorOpen(false)}
          />
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={!!deletingRule} onOpenChange={(open) => !open && setDeletingRule(null)}>
        <DialogContent className="max-w-md bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
          <DialogHeader>
            <DialogTitle className="text-gray-900 dark:text-white flex items-center gap-2">
              <AlertTriangle className="w-5 h-5 text-red-500" />
              确认删除规则
            </DialogTitle>
            <DialogDescription className="text-gray-500 dark:text-gray-400">
              确定要删除规则 <strong className="text-gray-900 dark:text-white">{deletingRule}</strong> 吗？
              <br />
              <span className="text-red-500">此操作将同时删除所有客户端的规则文件，且无法恢复。</span>
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-3 mt-4">
            <Button
              variant="outline"
              onClick={() => setDeletingRule(null)}
              disabled={isDeleting}
            >
              取消
            </Button>
            <Button
              variant="destructive"
              onClick={() => deletingRule && handleDeleteRule(deletingRule)}
              disabled={isDeleting}
              className="bg-red-500 hover:bg-red-600"
            >
              {isDeleting ? (
                <>
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                  删除中...
                </>
              ) : (
                <>
                  <Trash2 className="w-4 h-4 mr-2" />
                  确认删除
                </>
              )}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

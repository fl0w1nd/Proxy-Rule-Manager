"use client";

import { useState, useEffect, useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
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
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
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
  Tag,
  Pencil,
  CopyPlus,
} from "lucide-react";
import { getConfig, refreshRule, previewRule, deleteRule, getClients, PreviewResponse, ClientConfig } from "@/lib/api-client";
import { RulesConfig, RuleConfig, ClientType } from "@/lib/schema";
import { RuleEditor } from "./editor";
import { toast } from "sonner";
import { RuleIcon } from "./icon-picker";

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
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [expandedCard, setExpandedCard] = useState<string | null>(null);

  const handleDuplicateRule = (rule: RuleConfig) => {
    const existingNames = new Set(config?.rules.map((r) => r.name) || []);
    let newName = `${rule.name}-copy`;
    let i = 2;
    while (existingNames.has(newName)) {
      newName = `${rule.name}-copy-${i}`;
      i++;
    }
    const duplicated: RuleConfig = JSON.parse(JSON.stringify(rule));
    duplicated.name = newName;
    if (duplicated.displayName) {
      duplicated.displayName = `${duplicated.displayName} (副本)`;
    }
    setEditingRule(duplicated);
    setIsEditorOpen(true);
  };

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
      // Ensure previewClient matches an actual key in the result.
      // The pre-set value (from rule.output.clients) may not appear in contents
      // if that client failed or wasn't included in this preview run.
      const availableClients = Object.keys(result.contents);
      if (availableClients.length > 0 && !availableClients.includes(clients[0])) {
        setPreviewClient(availableClients[0] as ClientType);
      }
    } catch (error) {
      toast.error("预览失败: " + String(error));
      setPreviewingRule(null);
    }
  };

  const copyRuleUrl = async (ruleName: string, client: ClientType) => {
    const clientPath = getClientPathName(client);
    const url = `${window.location.origin}/Rules/${encodeURIComponent(clientPath)}/${encodeURIComponent(ruleName)}.list`;
    try {
      await navigator.clipboard.writeText(url);
      toast.success("已复制规则 URL");
    } catch {
      toast.error("复制失败");
    }
  };

  const handleDeleteRule = async (ruleName: string) => {
    setIsDeleting(true);
    try {
      await deleteRule(ruleName);
      toast.success(`规则 "${ruleName}" 已删除`);
      await fetchConfig();
      onRefresh();
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

  // 提取所有唯一标签
  const allTags = useMemo(() => {
    return Array.from(
      new Set(config?.rules.flatMap((rule) => rule.tags || []) || [])
    ).sort();
  }, [config?.rules]);

  // 标签切换
  const toggleTag = (tag: string) => {
    setSelectedTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]
    );
  };

  const filteredRules = useMemo(() => {
    return config?.rules.filter((rule) => {
      const matchesSearch =
        rule.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        rule.displayName?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        rule.description?.toLowerCase().includes(searchQuery.toLowerCase());

      const matchesTags =
        selectedTags.length === 0 ||
        selectedTags.some((tag) => rule.tags?.includes(tag));

      return matchesSearch && matchesTags;
    });
  }, [config?.rules, searchQuery, selectedTags]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin text-primary" />
      </div>
    );
  }

  // 全屏预览模式
  if (isPreviewFullscreen && previewingRule && previewData) {
    return (
      <div className="fixed inset-0 z-50 bg-background flex flex-col">
        {/* 顶部工具栏 */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-border bg-muted/50">
          <div className="flex items-center gap-3">
            {(() => {
              const rule = config?.rules.find(r => r.name === previewingRule);
              return rule?.icon ? (
                <RuleIcon icon={rule.icon} className="w-6 h-6 text-muted-foreground" />
              ) : (
                <FileText className="w-5 h-5 text-muted-foreground" />
              );
            })()}
            <span className="font-semibold text-foreground">预览: {previewingRule}</span>
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
              onClick={async () => {
                const content = previewData.contents[previewClient];
                if (content) {
                  try {
                    await navigator.clipboard.writeText(content);
                    toast.success("已复制内容");
                  } catch {
                    toast.error("复制失败");
                  }
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
          <div className="px-4 py-2 bg-muted/50 border-b border-border">
            <div className="flex flex-wrap gap-4">
              {previewData.diagnostics.sourceResults.map((source, i) => (
                <div key={i} className="flex items-center gap-2 text-sm">
                  {source.success ? (
                    <CheckCircle className="w-4 h-4 text-green-500" />
                  ) : (
                    <XCircle className="w-4 h-4 text-red-500" />
                  )}
                  <span className="text-xs font-medium text-muted-foreground">#{i + 1}</span>
                  <span className="text-foreground/80 truncate max-w-md">{source.url}</span>
                  {source.size !== undefined && source.size > 0 && (
                    <span className="text-muted-foreground">({(source.size / 1024).toFixed(1)} KB)</span>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* 客户端标签 */}
        <Tabs value={previewClient} onValueChange={(v) => setPreviewClient(v as ClientType)} className="flex-1 flex flex-col min-h-0 overflow-hidden">
          <div className="px-4 py-2 border-b border-border shrink-0">
            <TabsList className="bg-muted">
              {Object.keys(previewData.contents).map((client) => (
                <TabsTrigger key={client} value={client} className="data-[state=active]:bg-white dark:data-[state=active]:bg-slate-700">
                  {getClientDisplayName(client)}
                </TabsTrigger>
              ))}
            </TabsList>
          </div>
          {Object.entries(previewData.contents).map(([client, content]) => (
            <TabsContent key={client} value={client} className="flex-1 m-0 min-h-0 overflow-auto">
              <pre className="p-4 text-sm font-mono text-foreground/80 whitespace-pre">
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
      <div className="flex flex-col sm:flex-row items-start sm:items-center gap-4 flex-wrap">
        <div className="relative w-full sm:w-auto sm:flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input
            placeholder="搜索规则..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-10 bg-background"
          />
        </div>
        <Button
          variant="success"
          onClick={() => {
            setEditingRule(null);
            setIsEditorOpen(true);
          }}
        >
          <Plus className="w-4 h-4 mr-2" />
          添加规则
        </Button>
      </div>

      {/* 标签筛选器 */}
      {allTags.length > 0 && (
        <div className="flex items-center gap-2 overflow-x-auto pb-2">
          <div className="flex items-center gap-1 text-sm text-muted-foreground flex-shrink-0">
            <Tag className="w-4 h-4" />
            <span>标签筛选:</span>
          </div>
          {allTags.map((tag) => (
            <Badge
              key={tag}
              variant={selectedTags.includes(tag) ? "default" : "outline"}
              role="button"
              tabIndex={0}
              aria-pressed={selectedTags.includes(tag)}
              className={`cursor-pointer transition-colors flex-shrink-0 ${selectedTags.includes(tag)
                ? "bg-primary hover:bg-primary/90 text-primary-foreground"
                : "hover:bg-accent"
                }`}
              onClick={() => toggleTag(tag)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  toggleTag(tag);
                }
              }}
            >
              {tag}
            </Badge>
          ))}
          {selectedTags.length > 0 && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSelectedTags([])}
              className="h-6 px-2 text-xs text-muted-foreground hover:text-foreground flex-shrink-0"
            >
              清除筛选
            </Button>
          )}
        </div>
      )}

      {/* Rules Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-5">
        {filteredRules?.map((rule, index) => {
          const isExpanded = expandedCard === rule.name;
          return (
            <Card
              key={rule.name}
              className={`p-5 animate-slide-up opacity-0 flex flex-col relative transition-[z-index] ${isExpanded ? "z-50" : "z-0"
                }`}
              style={{ animationDelay: `${index * 50}ms` }}
            >
              {/* Trigger button – top right */}
              <button
                onClick={() => setExpandedCard(isExpanded ? null : rule.name)}
                className={`fab-trigger w-8 h-8 flex items-center justify-center rounded-xl transition-all duration-250 ${isExpanded
                  ? "bg-foreground text-background shadow-md"
                  : "text-muted-foreground/40 hover:text-muted-foreground hover:bg-accent"
                  }`}
              >
                {isExpanded ? (
                  <X className="w-4 h-4" />
                ) : (
                  <MoreVertical className="w-4 h-4" />
                )}
              </button>

              {/* Floating toolbar – slides down from trigger */}
              <div className={`fab-toolbar ${isExpanded ? "open" : ""}`}>
                {/* Edit */}
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      className="fab-item text-muted-foreground"
                      style={{ "--fab-i": 0 } as React.CSSProperties}
                      onClick={() => {
                        setEditingRule(rule);
                        setIsEditorOpen(true);
                        setExpandedCard(null);
                      }}
                    >
                      <Pencil className="w-[15px] h-[15px]" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent side="left">编辑</TooltipContent>
                </Tooltip>

                {/* Preview */}
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      className="fab-item text-muted-foreground"
                      style={{ "--fab-i": 1 } as React.CSSProperties}
                      onClick={() => {
                        handlePreviewRule(rule.name, rule.output.clients);
                        setExpandedCard(null);
                      }}
                    >
                      <Eye className="w-[15px] h-[15px]" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent side="left">预览</TooltipContent>
                </Tooltip>

                {/* Refresh */}
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      className="fab-item text-muted-foreground"
                      style={{ "--fab-i": 2 } as React.CSSProperties}
                      onClick={() => {
                        handleRefreshRule(rule.name);
                        setExpandedCard(null);
                      }}
                      disabled={refreshingRules.has(rule.name)}
                    >
                      {refreshingRules.has(rule.name) ? (
                        <Loader2 className="w-[15px] h-[15px] animate-spin" />
                      ) : (
                        <RefreshCw className="w-[15px] h-[15px]" />
                      )}
                    </button>
                  </TooltipTrigger>
                  <TooltipContent side="left">刷新</TooltipContent>
                </Tooltip>

                {/* Copy URL */}
                {rule.output.clients.length === 1 ? (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button
                        className="fab-item text-muted-foreground"
                        style={{ "--fab-i": 3 } as React.CSSProperties}
                        onClick={() => {
                          copyRuleUrl(rule.name, rule.output.clients[0]);
                          setExpandedCard(null);
                        }}
                      >
                        <Copy className="w-[15px] h-[15px]" />
                      </button>
                    </TooltipTrigger>
                    <TooltipContent side="left">复制 URL</TooltipContent>
                  </Tooltip>
                ) : (
                  <DropdownMenu>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <DropdownMenuTrigger asChild>
                          <button
                            className="fab-item text-muted-foreground"
                            style={{ "--fab-i": 3 } as React.CSSProperties}
                          >
                            <Copy className="w-[15px] h-[15px]" />
                          </button>
                        </DropdownMenuTrigger>
                      </TooltipTrigger>
                      <TooltipContent side="left">复制 URL</TooltipContent>
                    </Tooltip>
                    <DropdownMenuContent align="end" side="left" className="bg-background border-border">
                      {rule.output.clients.map((client) => (
                        <DropdownMenuItem
                          key={client}
                          onClick={() => {
                            copyRuleUrl(rule.name, client);
                            setExpandedCard(null);
                          }}
                        >
                          {getClientDisplayName(client)}
                        </DropdownMenuItem>
                      ))}
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}

                {/* Duplicate */}
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      className="fab-item text-muted-foreground"
                      style={{ "--fab-i": 4 } as React.CSSProperties}
                      onClick={() => {
                        handleDuplicateRule(rule);
                        setExpandedCard(null);
                      }}
                    >
                      <CopyPlus className="w-[15px] h-[15px]" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent side="left">复制规则</TooltipContent>
                </Tooltip>

                {/* Divider */}
                <div className="fab-divider" />

                {/* Delete */}
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      className="fab-item text-red-500/70 dark:text-red-400/60 hover:!bg-red-50 dark:hover:!bg-red-900/20 hover:text-red-600 dark:hover:text-red-400"
                      style={{ "--fab-i": 5 } as React.CSSProperties}
                      onClick={() => {
                        setDeletingRule(rule.name);
                        setExpandedCard(null);
                      }}
                    >
                      <Trash2 className="w-[15px] h-[15px]" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent side="left">删除</TooltipContent>
                </Tooltip>
              </div>

              {/* Header: Icon + Name + Description */}
              <div className="flex items-start gap-3 pr-8">
                <div className="neu-icon flex-shrink-0">
                  {rule.icon ? (
                    <RuleIcon icon={rule.icon} className="w-5 h-5 text-muted-foreground" />
                  ) : (
                    <FileText className="w-[18px] h-[18px] text-muted-foreground" />
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <h3 className="text-[15px] font-semibold text-foreground truncate leading-tight">
                    {rule.displayName || rule.name}
                  </h3>
                  <p className="text-xs text-muted-foreground mt-1 line-clamp-2 leading-relaxed">
                    {rule.description || `ID: ${rule.name}`}
                  </p>
                </div>
              </div>

              {/* Tags */}
              <div className="flex flex-wrap items-center gap-1.5 mt-3 min-h-[22px]">
                {rule.tags && rule.tags.length > 0 ? (
                  rule.tags.map((tag) => (
                    <Badge key={tag} variant="violet" className="text-[10px]">
                      {tag}
                    </Badge>
                  ))
                ) : (
                  <span className="text-[10px] text-muted-foreground/40 italic">无标签</span>
                )}
              </div>

              {/* Client badges */}
              <div className="flex items-start gap-2 mt-3 pt-3 border-t border-border/40">
                <span className="text-[10px] text-muted-foreground flex-shrink-0 mt-0.5">输出:</span>
                <div className="flex flex-wrap gap-1.5 flex-1">
                  {rule.output.clients.map((client) => (
                    <Badge key={client} variant="blue" className="text-[10px]">
                      {getClientDisplayName(client)}
                    </Badge>
                  ))}
                </div>
              </div>
            </Card>
          );
        })}
      </div>

      {filteredRules?.length === 0 && (
        <div className="text-center py-20">
          <div className="w-20 h-20 mx-auto mb-6 rounded-2xl bg-gradient-to-br from-muted/50 to-muted flex items-center justify-center">
            <FileText className="w-10 h-10 text-muted-foreground/40" />
          </div>
          <p className="text-lg font-medium text-foreground">
            {searchQuery || selectedTags.length > 0 ? "未找到匹配的规则" : "暂无规则"}
          </p>
          <p className="text-sm text-muted-foreground mt-2 max-w-sm mx-auto">
            {searchQuery || selectedTags.length > 0
              ? "尝试调整搜索条件或清除筛选标签"
              : "点击右上角「添加规则」按钮创建第一条规则"}
          </p>
          {!searchQuery && selectedTags.length === 0 && (
            <Button
              onClick={() => { setEditingRule(null); setIsEditorOpen(true); }}
              className="mt-6"
            >
              <Plus className="w-4 h-4 mr-2" />
              添加规则
            </Button>
          )}
        </div>
      )}

      {/* Preview Dialog */}
      <Dialog open={!!previewingRule && !isPreviewFullscreen} onOpenChange={(open) => !open && closePreview()}>
        <DialogContent className="max-w-5xl w-[95vw] sm:w-[90vw] h-[80vh] flex flex-col p-0">
          <DialogHeader className="px-6 pt-6 pb-4 border-b border-border">
            <DialogTitle className="flex items-center gap-2">
              {(() => {
                const rule = config?.rules.find(r => r.name === previewingRule);
                return rule?.icon ? (
                  <RuleIcon icon={rule.icon} className="w-6 h-6 text-muted-foreground" />
                ) : (
                  <FileText className="w-5 h-5 text-muted-foreground" />
                );
              })()}
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
                <div className="px-6 py-3 bg-muted/50 border-b border-border">
                  <p className="text-sm text-muted-foreground mb-2">数据源状态:</p>
                  <div className="flex flex-wrap gap-4">
                    {previewData.diagnostics.sourceResults.map((source, i) => (
                      <div key={i} className="flex items-center gap-2 text-sm">
                        {source.success ? (
                          <CheckCircle className="w-4 h-4 text-green-500" />
                        ) : (
                          <XCircle className="w-4 h-4 text-red-500" />
                        )}
                        <span className="text-xs font-medium text-muted-foreground">#{i + 1}</span>
                        <span className="text-foreground/80 truncate max-w-xs">{source.url}</span>
                        {source.size !== undefined && source.size > 0 && (
                          <span className="text-muted-foreground">({(source.size / 1024).toFixed(1)} KB)</span>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Content Tabs */}
              <Tabs value={previewClient} onValueChange={(v) => setPreviewClient(v as ClientType)} className="flex-1 flex flex-col min-h-0">
                <div className="px-6 py-3 border-b border-border flex items-center justify-between">
                  <TabsList className="bg-muted">
                    {Object.keys(previewData.contents).map((client) => (
                      <TabsTrigger key={client} value={client} className="data-[state=active]:bg-white dark:data-[state=active]:bg-slate-800">
                        {getClientDisplayName(client)}
                      </TabsTrigger>
                    ))}
                  </TabsList>
                  <span className="text-sm text-muted-foreground">
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
                        onClick={async () => {
                          try {
                            await navigator.clipboard.writeText(content);
                            toast.success("已复制内容");
                          } catch {
                            toast.error("复制失败");
                          }
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
                    <div className="h-full overflow-auto bg-muted/50">
                      <pre className="p-4 text-sm font-mono text-foreground/80 whitespace-pre min-w-max">
                        {content || "暂无内容"}
                      </pre>
                    </div>
                  </TabsContent>
                ))}
              </Tabs>
            </div>
          ) : (
            <div className="flex-1 flex items-center justify-center">
              <Loader2 className="w-8 h-8 animate-spin text-primary" />
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* Rule Editor Dialog */}
      <Dialog open={isEditorOpen} onOpenChange={setIsEditorOpen}>
        <DialogContent className="max-w-4xl h-[90vh] p-0 flex flex-col bg-background border-border overflow-hidden gap-0">
          <DialogHeader className="p-6 pb-2 border-b shrink-0 hidden"> {/* Hidden because custom header in Editor */}
            <DialogTitle className="text-foreground">
              {editingRule ? `编辑规则: ${editingRule.name}` : "添加新规则"}
            </DialogTitle>
            <DialogDescription className="hidden">
              规则编辑界面
            </DialogDescription>
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
        <DialogContent className="max-w-md">
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
              className="bg-destructive hover:bg-destructive/90 text-destructive-foreground"
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

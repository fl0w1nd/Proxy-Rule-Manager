"use client";

import { useState, useEffect, useMemo, useRef, startTransition } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { SearchInput } from "@/components/ui/search-input";
import { EmptyState } from "@/components/ui/empty-state";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { CodeViewer } from "./code-viewer";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
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
  ChevronDown,
  HelpCircle,
} from "lucide-react";
import { getConfig, refreshRule, previewRule, deleteRule, getClients, saveConfig, getStatus, PreviewResponse, ClientConfig } from "@/lib/api-client";
import { RulesConfig, RuleConfig, ClientType, DEFAULT_SYSTEM_SETTINGS } from "@/lib/schema";
import { RuleEditor } from "./editor";
import { toast } from "sonner";
import { RuleIcon } from "./icon-picker";
import { isGeositeRule } from "@/lib/rule-classification";

interface RulesManagerProps {
  onRefresh: () => void;
}

export function RulesManager({ onRefresh }: RulesManagerProps) {
  const [config, setConfig] = useState<RulesConfig | null>(null);
  const [clients, setClients] = useState<ClientConfig[]>([]);
  const [ruleStatusMap, setRuleStatusMap] = useState<Record<string, { hasError: boolean; lastFailureAt: string | null; consecutiveFailures: number }>>({});
  const [failureThreshold, setFailureThreshold] = useState<number>(DEFAULT_SYSTEM_SETTINGS.sync.failureThreshold);
  const [isLoading, setIsLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [refreshingRules, setRefreshingRules] = useState<Set<string>>(new Set());
  const [previewData, setPreviewData] = useState<PreviewResponse | null>(null);
  const [previewingRule, setPreviewingRule] = useState<string | null>(null);
  const [previewClient, setPreviewClient] = useState<ClientType>("clash_meta");
  const [editingRule, setEditingRule] = useState<RuleConfig | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  // Track the editor's saving/dirty state so we can prevent unintended
  // dialog closes that would silently drop in-flight saves or unsaved edits.
  const [isEditorSaving, setIsEditorSaving] = useState(false);
  const [isEditorDirty, setIsEditorDirty] = useState(false);
  const [isPreviewFullscreen, setIsPreviewFullscreen] = useState(false);
  const [deletingRule, setDeletingRule] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [expandedCard, setExpandedCard] = useState<string | null>(null);
  const [selectedRuleNames, setSelectedRuleNames] = useState<string[]>([]);
  const [isMultiSelectMode, setIsMultiSelectMode] = useState(false);
  const [isBatchDialogOpen, setIsBatchDialogOpen] = useState(false);
  const [batchTagInput, setBatchTagInput] = useState("");
  const [batchAddTags, setBatchAddTags] = useState(true);
  const [batchReplaceTags, setBatchReplaceTags] = useState(false);
  const [batchClientIds, setBatchClientIds] = useState<string[]>([]);
  const [batchAddClients, setBatchAddClients] = useState(true);
  const [batchReplaceClients, setBatchReplaceClients] = useState(false);
  const [isBatchSaving, setIsBatchSaving] = useState(false);
  const previewRequestRef = useRef(0);

  const handleDuplicateRule = (rule: RuleConfig) => {
    if (isGeositeRule(rule)) {
      toast.error("Geosite 规则由系统管理，请在 Geosite 页面重新导入");
      return;
    }
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
      const [{ config }, { clients: clientList }, statusResult] = await Promise.all([
        getConfig(),
        getClients(),
        getStatus().catch(() => null),
      ]);
      setConfig(config);
      setClients(clientList);
      if (statusResult && Array.isArray(statusResult.rules)) {
        const map: Record<string, { hasError: boolean; lastFailureAt: string | null; consecutiveFailures: number }> = {};
        for (const r of statusResult.rules) {
          map[r.name] = {
            hasError: !!r.hasError,
            lastFailureAt: r.lastFailureAt ?? null,
            consecutiveFailures: r.consecutiveFailures ?? 0,
          };
        }
        setRuleStatusMap(map);
        if (typeof statusResult.failureThreshold === "number" && statusResult.failureThreshold > 0) {
          setFailureThreshold(statusResult.failureThreshold);
        }
      }
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

  useEffect(() => {
    startTransition(() => { fetchConfig(); });
  }, []);

  useEffect(() => {
    startTransition(() => { setSelectedRuleNames((current) => current.filter((name) => config?.rules.some((rule) => rule.name === name))); });
  }, [config?.rules]);

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
    const requestId = previewRequestRef.current + 1;
    previewRequestRef.current = requestId;
    setPreviewingRule(ruleName);
    setPreviewClient(clients[0] || "clash_meta");
    setPreviewData(null);
    try {
      const result = await previewRule(ruleName, undefined, 10000);
      if (previewRequestRef.current !== requestId) return;
      setPreviewData(result);
      // Ensure previewClient matches an actual key in the result.
      // The pre-set value (from rule.output.clients) may not appear in contents
      // if that client failed or wasn't included in this preview run.
      const availableClients = Object.keys(result.contents);
      if (availableClients.length > 0 && !availableClients.includes(clients[0])) {
        setPreviewClient(availableClients[0] as ClientType);
      }
    } catch (error) {
      if (previewRequestRef.current !== requestId) return;
      toast.error("预览失败: " + String(error));
      setPreviewingRule(null);
    }
  };

  const copyRuleUrl = async (ruleName: string, client: ClientType) => {
    const url = `${window.location.origin}/Rules/${encodeURIComponent(client)}/${encodeURIComponent(ruleName)}.list`;
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

  const toggleRuleSelection = (ruleName: string) => {
    setSelectedRuleNames((current) =>
      current.includes(ruleName) ? current.filter((name) => name !== ruleName) : [...current, ruleName]
    );
  };

  const toggleMultiSelectMode = () => {
    setIsMultiSelectMode((current) => {
      if (current) {
        setSelectedRuleNames([]);
      }
      return !current;
    });
    setExpandedCard(null);
  };

  const openBatchDialog = () => {
    setBatchTagInput("");
    setBatchAddTags(true);
    setBatchReplaceTags(false);
    setBatchClientIds([]);
    setBatchAddClients(true);
    setBatchReplaceClients(false);
    setIsBatchDialogOpen(true);
  };

  const toggleBatchClient = (clientId: string) => {
    setBatchClientIds((current) =>
      current.includes(clientId) ? current.filter((id) => id !== clientId) : [...current, clientId]
    );
  };

  const updateBatchTagMode = (mode: "add" | "replace", checked: boolean) => {
    if (mode === "add") {
      setBatchAddTags(checked);
      if (checked) {
        setBatchReplaceTags(false);
      }
      return;
    }
    setBatchReplaceTags(checked);
    if (checked) {
      setBatchAddTags(false);
    }
  };

  const updateBatchClientMode = (mode: "add" | "replace", checked: boolean) => {
    if (mode === "add") {
      setBatchAddClients(checked);
      if (checked) {
        setBatchReplaceClients(false);
      }
      return;
    }
    setBatchReplaceClients(checked);
    if (checked) {
      setBatchAddClients(false);
    }
  };

  const handleBatchSave = async () => {
    if (!config || selectedRuleNames.length === 0) return;

    const nextTags = Array.from(
      new Set(
        batchTagInput
          .split(/[\n,，]/)
          .map((item) => item.trim())
          .filter(Boolean)
      )
    );

    if (nextTags.length > 0 && !batchAddTags && !batchReplaceTags) {
      toast.error("请选择 TAG 处理方式");
      return;
    }

    if (batchClientIds.length > 0 && !batchAddClients && !batchReplaceClients) {
      toast.error("请选择客户端处理方式");
      return;
    }

    const shouldUpdateTags = nextTags.length > 0 && (batchAddTags || batchReplaceTags);
    const shouldUpdateClients = batchClientIds.length > 0 && (batchAddClients || batchReplaceClients);

    if (!shouldUpdateTags && !shouldUpdateClients) {
      toast.error("请至少设置一项");
      return;
    }

    setIsBatchSaving(true);
    try {
      const { config: latestConfig, rev } = await getConfig();
      const selectedSet = new Set(selectedRuleNames);
      const updatedRules = latestConfig.rules.map((rule) => {
        if (!selectedSet.has(rule.name) || isGeositeRule(rule)) {
          return rule;
        }

        const mergedTags = shouldUpdateTags
          ? (batchReplaceTags
            ? nextTags
            : Array.from(new Set([...(rule.tags || []), ...nextTags])))
          : rule.tags;

        const mergedClients = shouldUpdateClients
          ? (batchReplaceClients
            ? batchClientIds
            : Array.from(new Set([...rule.output.clients, ...batchClientIds])))
          : rule.output.clients;

        return {
          ...rule,
          tags: mergedTags,
          output: {
            ...rule.output,
            clients: mergedClients,
          },
        };
      });

      await saveConfig({
        ...latestConfig,
        rules: updatedRules,
      }, rev);

      let refreshFailed = 0;
      if (shouldUpdateClients) {
        const results = await Promise.allSettled(selectedRuleNames.map((name) => refreshRule(name)));
        refreshFailed = results.filter((result) => result.status === "rejected").length;
      }

      setIsBatchDialogOpen(false);
      setIsMultiSelectMode(false);
      setSelectedRuleNames([]);
      await fetchConfig();
      onRefresh();

      if (refreshFailed > 0) {
        toast.warning(`已处理 ${selectedRuleNames.length} 条规则，${refreshFailed} 条刷新失败`);
      } else {
        toast.success(`已处理 ${selectedRuleNames.length} 条规则`);
      }
    } catch (error) {
      toast.error("批量处理失败: " + String(error));
    } finally {
      setIsBatchSaving(false);
    }
  };

  // 提取所有唯一标签
  const allTags = useMemo(() => {
    return Array.from(
      new Set(config?.rules.filter((rule) => !isGeositeRule(rule)).flatMap((rule) => rule.tags || []) || [])
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
      if (isGeositeRule(rule)) {
        return false;
      }
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

  const allVisibleSelected = !!filteredRules?.length && filteredRules.every((rule) => selectedRuleNames.includes(rule.name));

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
        <div className="flex items-center justify-between border-b border-border bg-background px-4 py-3">
          <div className="flex items-center gap-3">
            {(() => {
              const rule = config?.rules.find(r => r.name === previewingRule);
              return rule?.icon ? (
                <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary-soft">
                  <RuleIcon icon={rule.icon} className="w-5 h-5 text-primary" />
                </div>
              ) : (
                <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary-soft">
                  <FileText className="w-5 h-5 text-primary" />
                </div>
              );
            })()}
            <span className="font-semibold text-foreground">预览: {previewingRule}</span>
            {previewData.diagnostics.truncated && (
              <Badge variant="outline" className="border-warning/25 bg-warning-soft text-warning">
                内容已截断（共 {previewData.diagnostics.totalLines} 行）
              </Badge>
            )}
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="secondary"
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
              variant="secondary"
              size="icon"
              className="rounded-full"
              onClick={() => setIsPreviewFullscreen(false)}
            >
              <X className="w-5 h-5" />
            </Button>
          </div>
        </div>

        {/* 数据源状态 */}
        {previewData.diagnostics.sourceResults.length > 0 && (
          <div className="border-b border-border bg-surface-subtle px-4 py-2">
            <div className="flex flex-wrap gap-4">
              {previewData.diagnostics.sourceResults.map((source, i) => (
                <div key={i} className="flex items-center gap-2 text-sm">
                  {source.success ? (
                    <CheckCircle className="w-4 h-4 text-success" />
                  ) : (
                    <XCircle className="w-4 h-4 text-destructive" />
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
          <div className="flex items-center justify-between border-b border-border px-4 py-2.5 shrink-0">
            <TabsList>
              {Object.keys(previewData.contents).map((client) => (
                <TabsTrigger key={client} value={client}>
                  {getClientDisplayName(client)}
                </TabsTrigger>
              ))}
            </TabsList>
            <span className="text-xs font-mono text-muted-foreground">
              {previewData.contents[previewClient]?.split("\n").length || 0} 行
            </span>
          </div>
          {Object.entries(previewData.contents).map(([client, content]) => (
            <TabsContent key={client} value={client} className="flex-1 m-0 min-h-0 overflow-hidden">
              <CodeViewer
                content={content}
                emptyText="暂无内容"
                showLineNumbers={false}
                className="h-full rounded-none border-none"
                height="100%"
              />
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
        <SearchInput
          placeholder="搜索规则..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          fullWidth
        />
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
        {filteredRules && filteredRules.length > 0 ? (
          <Button variant={isMultiSelectMode ? "default" : "outline"} size="sm" onClick={toggleMultiSelectMode}>
            {isMultiSelectMode ? "取消多选" : "多选"}
          </Button>
        ) : null}
        {isMultiSelectMode && filteredRules && filteredRules.length > 0 ? (
          <Button variant="outline" size="sm" onClick={() => {
            const allNames = filteredRules.map((rule) => rule.name);
            setSelectedRuleNames(allVisibleSelected ? [] : allNames);
          }}>
            {allVisibleSelected ? "取消全选" : "全选"}
          </Button>
        ) : null}
        {isMultiSelectMode && selectedRuleNames.length > 0 ? (
          <Button variant="outline" size="sm" onClick={openBatchDialog}>
            批量处理 ({selectedRuleNames.length})
          </Button>
        ) : null}
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
                ? "bg-primary text-primary-foreground hover:bg-primary/90"
                : "bg-surface-subtle/70 hover:bg-accent"
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
          const isSelected = selectedRuleNames.includes(rule.name);
          return (
            <Card
              key={rule.name}
              onClick={() => {
                if (isMultiSelectMode) {
                  toggleRuleSelection(rule.name);
                }
              }}
              className={`group p-5 animate-slide-up opacity-0 flex flex-col relative transition-[z-index,box-shadow,border-color,background-color,transform] ${isMultiSelectMode ? "cursor-pointer hover:border-primary/25 hover:shadow-[var(--shadow-md)]" : ""} ${isSelected ? "border-primary/80 bg-primary-soft/40 before:pointer-events-none before:absolute before:inset-0 before:rounded-[inherit] before:border-2 before:border-primary/75 before:shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--primary)_40%,transparent)] after:pointer-events-none after:absolute after:inset-[1px] after:rounded-[calc(var(--radius-2xl)-1px)] after:bg-primary/8 dark:after:bg-primary/12 shadow-[0_0_0_1px_color-mix(in_srgb,var(--primary)_55%,transparent),0_0_0_4px_color-mix(in_srgb,var(--primary)_18%,transparent),0_16px_36px_-16px_color-mix(in_srgb,var(--primary)_45%,transparent)] dark:shadow-[0_0_0_1px_color-mix(in_srgb,var(--primary)_85%,transparent),0_0_0_4px_color-mix(in_srgb,var(--primary)_30%,transparent),0_0_28px_color-mix(in_srgb,var(--primary)_28%,transparent)]" : ""} ${isExpanded ? "z-50" : "z-0"
                }`}
              style={{ animationDelay: `${index * 50}ms` }}
            >
              {/* Trigger button – top right */}
              <button
                type="button"
                onClick={(event) => {
                  event.stopPropagation();
                  setExpandedCard(isExpanded ? null : rule.name);
                }}
                aria-expanded={isExpanded}
                aria-label={isExpanded ? `收起 ${rule.displayName || rule.name} 操作` : `展开 ${rule.displayName || rule.name} 操作`}
                className={`fab-trigger flex h-8 w-8 items-center justify-center rounded-xl transition-all duration-250 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-primary/15 ${isExpanded
                  ? "bg-foreground text-background shadow-[var(--shadow-sm)]"
                  : "text-muted-foreground/40 hover:bg-accent hover:text-muted-foreground"
                  }`}
              >
                {isExpanded ? (
                  <X className="w-4 h-4" />
                ) : (
                  <MoreVertical className="w-4 h-4" />
                )}
              </button>

              {/* Floating toolbar – slides down from trigger */}
              <div className={`fab-toolbar ${isExpanded ? "open" : ""}`} onClick={(event) => event.stopPropagation()}>
                {/* Edit */}
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      type="button"
                      className="fab-item text-muted-foreground"
                      aria-label={`编辑 ${rule.displayName || rule.name}`}
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
                      type="button"
                      className="fab-item text-muted-foreground"
                      aria-label={`预览 ${rule.displayName || rule.name}`}
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
                      type="button"
                      className="fab-item text-muted-foreground"
                      aria-label={`刷新 ${rule.displayName || rule.name}`}
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
                        type="button"
                        className="fab-item text-muted-foreground"
                        aria-label={`复制 ${rule.displayName || rule.name} URL`}
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
                            type="button"
                            className="fab-item text-muted-foreground"
                            aria-label={`复制 ${rule.displayName || rule.name} URL`}
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
                      type="button"
                      className="fab-item text-muted-foreground"
                      aria-label={`复制规则 ${rule.displayName || rule.name}`}
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
                      type="button"
                      className="fab-item text-destructive/70 hover:bg-destructive/8 hover:text-destructive"
                      aria-label={`删除 ${rule.displayName || rule.name}`}
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
                <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-primary-soft">
                  {rule.icon ? (
                    <RuleIcon icon={rule.icon} className="w-5 h-5 text-primary" />
                  ) : (
                    <FileText className="w-[18px] h-[18px] text-primary" />
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <h3 className="text-sm font-semibold text-foreground truncate leading-tight">
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
                <RuleStatusBadge status={ruleStatusMap[rule.name]} threshold={failureThreshold} />
              </div>

              {/* Client badges */}
              <div className="flex items-start gap-2 mt-3 pt-3 border-t border-border">
                <span className="text-[10px] text-muted-foreground flex-shrink-0 mt-0.5">输出:</span>
                <div className="flex flex-wrap gap-1.5 flex-1">
                  {rule.output.clients.map((client) => (
                    <Badge key={client} variant="blue" className="text-[10px]">
                      {getClientDisplayName(client)}
                    </Badge>
                  ))}
                </div>
              </div>

              <div className="mt-4 flex gap-2" onClick={(event) => event.stopPropagation()}>
                <Button
                  variant="secondary"
                  size="sm"
                  className="flex-1"
                  onClick={() => handlePreviewRule(rule.name, rule.output.clients)}
                >
                  <Eye className="w-3.5 h-3.5" />
                  预览
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  className="flex-1"
                  onClick={() => {
                    setEditingRule(rule);
                    setIsEditorOpen(true);
                  }}
                >
                  <Pencil className="w-3.5 h-3.5" />
                  编辑
                </Button>
              </div>
            </Card>
          );
        })}
      </div>

      {filteredRules?.length === 0 && (
        <EmptyState
          icon={FileText}
          title={searchQuery || selectedTags.length > 0 ? "未找到匹配的规则" : "暂无规则"}
          description={searchQuery || selectedTags.length > 0 ? "尝试调整搜索条件或清除筛选标签" : "点击右上角「添加规则」按钮创建第一条规则"}
          action={!searchQuery && selectedTags.length === 0 ? (
            <Button onClick={() => { setEditingRule(null); setIsEditorOpen(true); }}>
              <Plus className="w-4 h-4 mr-2" />添加规则
            </Button>
          ) : undefined}
        />
      )}

      {/* Preview Dialog */}
      <Dialog open={!!previewingRule && !isPreviewFullscreen} onOpenChange={(open) => !open && closePreview()}>
        <DialogContent className="max-w-5xl w-[95vw] sm:w-[90vw] h-[80vh] flex flex-col p-0">
          <DialogHeader className="px-6 pt-6 pb-4 border-b border-border">
            <DialogTitle className="flex items-center gap-3">
              {(() => {
                const rule = config?.rules.find(r => r.name === previewingRule);
                return rule?.icon ? (
                  <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary-soft">
                    <RuleIcon icon={rule.icon} className="w-4 h-4 text-primary" />
                  </div>
                ) : (
                  <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary-soft">
                    <FileText className="w-4 h-4 text-primary" />
                  </div>
                );
              })()}
              预览: {previewingRule}
            </DialogTitle>
            {previewData?.diagnostics.truncated && (
              <DialogDescription className="text-warning">
                内容已截断（共 {previewData.diagnostics.totalLines} 行）
              </DialogDescription>
            )}
          </DialogHeader>

          {previewData ? (
            <div className="flex-1 flex flex-col min-h-0 overflow-hidden">
              {/* Source Results */}
              {previewData.diagnostics.sourceResults.length > 0 && (
                <div className="border-b border-border bg-surface-subtle px-6 py-3">
                  <p className="text-sm text-muted-foreground mb-2">数据源状态:</p>
                  <div className="flex flex-wrap gap-4">
                    {previewData.diagnostics.sourceResults.map((source, i) => (
                      <div key={i} className="flex items-center gap-2 text-sm">
                        {source.success ? (
                          <CheckCircle className="w-4 h-4 text-success" />
                        ) : (
                          <XCircle className="w-4 h-4 text-destructive" />
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
                <div className="flex items-center justify-between border-b border-border px-6 py-3">
                  <TabsList>
                    {Object.keys(previewData.contents).map((client) => (
                      <TabsTrigger key={client} value={client}>
                        {getClientDisplayName(client)}
                      </TabsTrigger>
                    ))}
                  </TabsList>
                  <div className="flex items-center gap-2">
                    <span className="text-sm text-muted-foreground">
                      {previewData.contents[previewClient]?.split('\n').length || 0} 行
                    </span>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={async () => {
                        try {
                          await navigator.clipboard.writeText(previewData.contents[previewClient] || "");
                          toast.success("已复制内容");
                        } catch {
                          toast.error("复制失败");
                        }
                      }}
                      className="border border-border/50 bg-background/90 shadow-[var(--shadow-xs)] hover:bg-background"
                      title="复制内容"
                    >
                      <Copy className="w-4 h-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => setIsPreviewFullscreen(true)}
                      className="border border-border/50 bg-background/90 shadow-[var(--shadow-xs)] hover:bg-background"
                      title="全屏预览 (ESC 退出)"
                    >
                      <Maximize2 className="w-4 h-4" />
                    </Button>
                  </div>
                </div>
                {Object.entries(previewData.contents).map(([client, content]) => (
                  <TabsContent key={client} value={client} className="flex-1 m-0 relative min-h-0 overflow-hidden">
                    <CodeViewer
                      content={content}
                      emptyText="暂无内容"
                      showLineNumbers={false}
                      className="h-full rounded-none border-none"
                      height="100%"
                    />
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
      <Dialog
        open={isEditorOpen}
        onOpenChange={(open) => {
          if (open) {
            setIsEditorOpen(true);
            return;
          }
          if (isEditorSaving) {
            return;
          }
          if (isEditorDirty) {
            const ok = typeof window !== "undefined"
              ? window.confirm("有未保存的修改，确定要放弃吗？")
              : true;
            if (!ok) return;
          }
          setIsEditorOpen(false);
        }}
      >
        <DialogContent
          className="max-w-4xl h-[90vh] p-0 flex flex-col bg-background border-border overflow-hidden gap-0"
          onEscapeKeyDown={(e) => {
            if (isEditorSaving) e.preventDefault();
          }}
          onPointerDownOutside={(e) => {
            if (isEditorSaving) e.preventDefault();
          }}
          onInteractOutside={(e) => {
            if (isEditorSaving) e.preventDefault();
          }}
        >
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
            onSavingChange={setIsEditorSaving}
            onDirtyChange={setIsEditorDirty}
            onSave={async () => {
              setIsEditorOpen(false);
              await fetchConfig();
              onRefresh();
            }}
            onCancel={() => setIsEditorOpen(false)}
          />
        </DialogContent>
      </Dialog>

      <Dialog open={isBatchDialogOpen} onOpenChange={setIsBatchDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              批量处理
              <HelpIcon text="TAG 支持新增和覆盖；输出客户端支持新增和覆盖。" />
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-5">
            <div className="space-y-3">
              <Label htmlFor="rules-batch-tags">TAG</Label>
              <Input
                id="rules-batch-tags"
                value={batchTagInput}
                onChange={(event) => setBatchTagInput(event.target.value)}
                placeholder="tag1, tag2"
              />
              <div className="flex flex-wrap items-center gap-4">
                <div className="flex items-center gap-2">
                  <Checkbox
                    id="rules-batch-tag-add"
                    checked={batchAddTags}
                    onCheckedChange={(checked) => updateBatchTagMode("add", checked === true)}
                  />
                  <Label htmlFor="rules-batch-tag-add">新增</Label>
                </div>
                <div className="flex items-center gap-2">
                  <Checkbox
                    id="rules-batch-tag-replace"
                    checked={batchReplaceTags}
                    onCheckedChange={(checked) => updateBatchTagMode("replace", checked === true)}
                  />
                  <Label htmlFor="rules-batch-tag-replace">覆盖</Label>
                </div>
              </div>
            </div>

            <div className="space-y-3">
              <Label>输出客户端</Label>
              <DropdownMenu modal={false}>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    className="flex min-h-11 w-full items-center justify-between gap-3 rounded-xl border border-border bg-card px-3 py-2 text-left shadow-[var(--shadow-xs)] transition-colors hover:border-primary/20"
                  >
                    <div className="flex flex-wrap gap-2">
                      {batchClientIds.length > 0 ? (
                        batchClientIds.map((clientId) => (
                          <Badge key={clientId} variant="blue">
                            {getClientDisplayName(clientId)}
                          </Badge>
                        ))
                      ) : (
                        <span className="text-sm text-muted-foreground">选择客户端</span>
                      )}
                    </div>
                    <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" className="min-w-72">
                  {clients.map((client) => (
                    <DropdownMenuCheckboxItem
                      key={client.id}
                      checked={batchClientIds.includes(client.id)}
                      onCheckedChange={() => toggleBatchClient(client.id)}
                      onSelect={(event) => event.preventDefault()}
                    >
                      {client.displayName}
                    </DropdownMenuCheckboxItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
              <div className="flex flex-wrap items-center gap-4">
                <div className="flex items-center gap-2">
                  <Checkbox
                    id="rules-batch-client-add"
                    checked={batchAddClients}
                    onCheckedChange={(checked) => updateBatchClientMode("add", checked === true)}
                  />
                  <Label htmlFor="rules-batch-client-add">新增</Label>
                </div>
                <div className="flex items-center gap-2">
                  <Checkbox
                    id="rules-batch-client-replace"
                    checked={batchReplaceClients}
                    onCheckedChange={(checked) => updateBatchClientMode("replace", checked === true)}
                  />
                  <Label htmlFor="rules-batch-client-replace">覆盖</Label>
                </div>
              </div>
            </div>
          </div>
          <div className="mt-6 flex justify-end gap-3">
            <Button variant="outline" onClick={() => setIsBatchDialogOpen(false)} disabled={isBatchSaving}>
              取消
            </Button>
            <Button onClick={handleBatchSave} disabled={isBatchSaving}>
              {isBatchSaving ? (
                <>
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                  保存中...
                </>
              ) : (
                "保存"
              )}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={!!deletingRule} onOpenChange={(open) => !open && setDeletingRule(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="text-foreground flex items-center gap-2">
              <AlertTriangle className="w-5 h-5 text-destructive" />
              确认删除规则
            </DialogTitle>
            <DialogDescription className="text-muted-foreground">
              确定要删除规则 <strong className="text-foreground">{deletingRule}</strong> 吗？
              <br />
              <span className="text-destructive">此操作将同时删除所有客户端的规则文件，且无法恢复。</span>
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

function HelpIcon({ text }: { text: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <HelpCircle className="h-4 w-4 cursor-help text-muted-foreground transition-colors hover:text-primary" />
      </TooltipTrigger>
      <TooltipContent side="top" className="max-w-xs">
        <p className="text-sm">{text}</p>
      </TooltipContent>
    </Tooltip>
  );
}

// RuleStatusBadge is the admin-side counterpart of FailureBadge in home.tsx.
// Severity ladder (most → least informative):
//   1. consecutiveFailures >= threshold → "更新失败 ×N" (warning, persistent)
//   2. hasError (last attempt failed, not yet at threshold) → "上次失败"
//   3. otherwise nothing
function RuleStatusBadge({
  status,
  threshold,
}: {
  status?: { hasError: boolean; lastFailureAt: string | null; consecutiveFailures: number };
  threshold: number;
}) {
  if (!status) return null;
  if (status.consecutiveFailures >= threshold && threshold > 0) {
    return (
      <Badge variant="outline" className="border-warning/30 bg-warning-soft text-warning text-[10px]">
        更新失败 ×{status.consecutiveFailures}
      </Badge>
    );
  }
  if (status.hasError) {
    return (
      <Badge variant="destructive" className="text-[10px]">
        上次失败
      </Badge>
    );
  }
  return null;
}

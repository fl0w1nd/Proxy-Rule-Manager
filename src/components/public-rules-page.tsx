"use client";

import { useState, useEffect, useMemo } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Search,
  Copy,
  Eye,
  Sun,
  Moon,
  Settings,
  FileText,
  Globe,
  Loader2,
  ExternalLink,
  CheckCircle,
  Maximize2,
  X,
  Tag,
} from "lucide-react";
import { useTheme } from "./theme-provider";
import { toast } from "sonner";
import { ClientFileMeta } from "@/lib/schema";
import { RuleIcon } from "./icon-picker";

interface RuleInfo {
  name: string;
  displayName?: string;
  description?: string;
  icon?: string;
  tags?: string[];
  clients: string[];
}

interface ClientConfig {
  id: string;
  displayName: string;
  pathName: string;
}

export function PublicRulesPage({ onAdminClick }: { onAdminClick: () => void }) {
  const { theme, toggleTheme } = useTheme();
  const [rules, setRules] = useState<RuleInfo[]>([]);
  const [clients, setClients] = useState<ClientConfig[]>([]);
  const [clientFiles, setClientFiles] = useState<ClientFileMeta[]>([]);
  const [lastSyncAt, setLastSyncAt] = useState<string | null>(null);
  const [version, setVersion] = useState<string>("");
  const [isLoading, setIsLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [activeMainTab, setActiveMainTab] = useState<"rules" | "configs">("rules");
  const [activeClient, setActiveClient] = useState<string>("");
  const [previewItem, setPreviewItem] = useState<{
    type: "rule" | "config";
    name: string;
    clientId: string;
    fileName: string;
    ext?: string;
  } | null>(null);
  const [previewContent, setPreviewContent] = useState<string>("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [copiedRule, setCopiedRule] = useState<string | null>(null);
  const [copiedConfig, setCopiedConfig] = useState<string | null>(null);
  const [isPreviewFullscreen, setIsPreviewFullscreen] = useState(false);

  const [selectedTags, setSelectedTags] = useState<string[]>([]);

  // 切换主标签时清空已选标签
  useEffect(() => {
    setSelectedTags([]);
  }, [activeMainTab]);

  useEffect(() => {
    fetchData();
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

  const fetchData = async () => {
    try {
      // 获取规则状态和客户端列表（从 /api/status 统一获取，无需鉴权）
      const [statusResponse, filesResponse] = await Promise.all([
        fetch("/api/status"),
        fetch("/api/client-files/public"),
      ]);

      if (statusResponse.ok) {
        const data = await statusResponse.json();
        setRules(data.rules || []);
        setLastSyncAt(data.lastSyncAt || null);
        setVersion(data.version || "");
        // 从 status 响应中获取客户端列表
        if (data.clients && data.clients.length > 0) {
          setClients(data.clients);
          // 设置默认激活的客户端（始终设置为第一个，避免闭包问题）
          setActiveClient((prev) => prev || data.clients[0].id);
        }
      }

      if (filesResponse.ok) {
        const data = await filesResponse.json();
        setClientFiles(data.files || []);
      }
    } catch (error) {
      console.error("Failed to fetch data:", error);
    } finally {
      setIsLoading(false);
    }
  };

  const getClientConfig = (clientId: string): ClientConfig | undefined => {
    return clients.find(c => c.id === clientId);
  };

  const getRuleUrl = (ruleName: string, clientId: string) => {
    const client = getClientConfig(clientId);
    const clientPath = client?.pathName || clientId;
    return `${window.location.origin}/Rules/${clientPath}/${ruleName}.list`;
  };

  const getConfigUrl = (clientId: string, name: string, ext: string) => {
    return `${window.location.origin}/${clientId}/${name}.${ext}`;
  };

  const copyRuleUrl = (ruleName: string, clientId?: string) => {
    const target = clientId || activeClient;
    const url = getRuleUrl(ruleName, target);
    navigator.clipboard.writeText(url);
    setCopiedRule(ruleName);
    setTimeout(() => setCopiedRule(null), 2000);
  };

  const getClientDisplayName = (clientId: string) => {
    return getClientConfig(clientId)?.displayName || clientId;
  };

  const copyConfigUrl = (file: ClientFileMeta) => {
    const url = getConfigUrl(file.clientId, file.configId, file.ext);
    navigator.clipboard.writeText(url);
    setCopiedConfig(file.id);
    setTimeout(() => setCopiedConfig(null), 2000);
  };

  const handlePreview = async (item: {
    type: "rule" | "config";
    name: string;
    clientId: string;
    fileName: string;
    ext?: string;
  }) => {
    setPreviewItem(item);
    setPreviewLoading(true);
    setPreviewContent("");

    try {
      const response = item.type === "rule"
        ? await fetch(getRuleUrl(item.name, item.clientId))
        : await fetch(getConfigUrl(item.clientId, item.name, item.ext || ""));
      if (response.ok) {
        const text = await response.text();
        setPreviewContent(text);
      } else {
        setPreviewContent("# 文件暂不可用");
      }
    } catch (error) {
      setPreviewContent("# 加载失败: " + String(error));
    } finally {
      setPreviewLoading(false);
    }
  };

  // 提取所有唯一标签
  const allTags = useMemo(() => {
    return Array.from(
      new Set(rules.flatMap((rule) => rule.tags || []))
    ).sort();
  }, [rules]);

  // 标签切换
  const toggleTag = (tag: string) => {
    setSelectedTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]
    );
  };

  const filteredRules = useMemo(() => {
    return rules.filter((rule) => {
      const matchesSearch =
        rule.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        rule.displayName?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        rule.description?.toLowerCase().includes(searchQuery.toLowerCase());

      const matchesTags =
        selectedTags.length === 0 ||
        selectedTags.some((tag) => rule.tags?.includes(tag));

      return matchesSearch && matchesTags;
    });
  }, [rules, searchQuery, selectedTags]);

  const clientRules = filteredRules.filter((rule) =>
    rule.clients.includes(activeClient)
  );

  const filteredClientFiles = clientFiles.filter((file) => {
    const query = searchQuery.toLowerCase();
    const configId = `${file.configId}.${file.ext}`.toLowerCase();
    const displayName = (file.displayName || "").toLowerCase();
    const description = (file.description || "").toLowerCase();
    return configId.includes(query) || displayName.includes(query) || description.includes(query);
  });

  const clientPublicFiles = filteredClientFiles.filter(
    (file) => file.clientId === activeClient
  );

  const closePreview = () => {
    setPreviewItem(null);
    setIsPreviewFullscreen(false);
  };

  // 全屏预览模式
  if (isPreviewFullscreen && previewItem) {
    return (
      <div className="fixed inset-0 z-50 bg-white dark:bg-slate-900 flex flex-col">
        {/* 顶部工具栏 */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-slate-700 bg-gray-50 dark:bg-slate-800">
          <div className="flex items-center gap-3">
            {(() => {
              const rule = rules.find(r => r.name === previewItem.name);
              return rule?.icon ? (
                <RuleIcon icon={rule.icon} className="w-6 h-6 text-gray-500 dark:text-gray-400" />
              ) : (
                <FileText className="w-5 h-5 text-gray-500 dark:text-gray-400" />
              );
            })()}
            <span className="font-semibold text-gray-900 dark:text-white">{previewItem.fileName}</span>
            <Badge className="bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
              {getClientConfig(previewItem.clientId)?.displayName || previewItem.clientId}
            </Badge>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-sm text-gray-500 dark:text-gray-400">
              {previewContent.split('\n').length} 行
            </span>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                navigator.clipboard.writeText(previewContent);
                toast.success("已复制内容");
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

        {/* 内容 - 带行号 */}
        <div className="flex-1 overflow-auto">
          {previewLoading ? (
            <div className="flex items-center justify-center h-full">
              <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
            </div>
          ) : (
            <div className="flex text-sm font-mono min-w-max">
              {/* 行号 */}
              <div className="py-4 pl-4 pr-3 text-right text-gray-400 dark:text-gray-500 select-none border-r border-gray-200 dark:border-slate-700 bg-gray-100 dark:bg-slate-800 sticky left-0">
                {previewContent.split('\n').map((_, i) => (
                  <div key={i}>{i + 1}</div>
                ))}
              </div>
              {/* 内容 */}
              <pre className="py-4 px-4 text-gray-800 dark:text-gray-200 whitespace-pre">
                {previewContent || "暂无内容"}
              </pre>
            </div>
          )}
        </div>
      </div>
    );
  }

  return (
    <TooltipProvider>
      <div className="min-h-screen bg-gradient-to-br from-gray-50 to-gray-100 dark:from-slate-900 dark:via-slate-800 dark:to-slate-900 transition-colors">
        {/* Header */}
        <header className="sticky top-0 z-50 border-b bg-white/80 dark:bg-slate-900/80 backdrop-blur-sm">
          <div className="container mx-auto px-4 py-3 sm:py-4">
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-2 sm:gap-3 min-w-0">
                <div className="w-9 h-9 sm:w-10 sm:h-10 flex items-center justify-center flex-shrink-0 transition-transform hover:scale-105 duration-300">
                  <img src="/logo.svg" alt="Logo" className="w-8 h-8 sm:w-9 sm:h-9" />
                </div>
                <div className="min-w-0">
                  <h1 className="text-lg sm:text-xl font-bold text-gray-900 dark:text-white truncate">
                    代理规则集
                  </h1>
                  <div className="flex items-center gap-2">
                    <p className="text-xs text-gray-500 dark:text-gray-400 hidden xs:block">
                      Proxy Rule Manager
                    </p>
                    {version && (
                      <span className="text-[11px] px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-500 border border-blue-500/20 font-mono leading-none">
                        v{version}
                      </span>
                    )}
                  </div>
                </div>
              </div>
              <div className="flex items-center gap-1 sm:gap-2 flex-shrink-0">
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={toggleTheme}
                  className="text-gray-600 dark:text-gray-300 min-w-[44px] min-h-[44px]"
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
                  onClick={onAdminClick}
                  className="text-gray-600 dark:text-gray-300 min-h-[44px] px-3"
                >
                  <Settings className="w-4 h-4 sm:mr-1" />
                  <span className="hidden sm:inline">管理</span>
                </Button>
              </div>
            </div>
          </div>
        </header>

        {/* Main Content */}
        <main className="container mx-auto px-4 py-8">
          {/* Main Tabs & Search */}
          <div className="mb-6 space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 sm:gap-4">
              <Tabs value={activeMainTab} onValueChange={(v) => setActiveMainTab(v as "rules" | "configs")}>
                <TabsList className="bg-white dark:bg-slate-800 border shadow-sm">
                  <TabsTrigger value="rules" className="data-[state=active]:bg-blue-500 data-[state=active]:text-white min-h-[44px] px-4">
                    规则
                  </TabsTrigger>
                  <TabsTrigger value="configs" className="data-[state=active]:bg-blue-500 data-[state=active]:text-white min-h-[44px] px-4">
                    配置文件
                  </TabsTrigger>
                </TabsList>
              </Tabs>
              <div className="relative w-full sm:w-auto sm:min-w-[280px]">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
                <Input
                  placeholder={activeMainTab === "rules" ? "搜索规则..." : "搜索配置文件..."}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-10 bg-white dark:bg-slate-800 h-11"
                />
              </div>
            </div>
            <Tabs
              value={activeClient}
              onValueChange={(v) => setActiveClient(v)}
            >
              <TabsList className="bg-white dark:bg-slate-800 border shadow-sm overflow-x-auto scrollbar-hide w-full sm:w-auto flex-nowrap">
                {clients.map((client) => (
                  <TabsTrigger
                    key={client.id}
                    value={client.id}
                    className="data-[state=active]:bg-blue-500 data-[state=active]:text-white min-h-[44px] px-4 flex-shrink-0"
                  >
                    {client.displayName}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>

            {/* 标签筛选器 - 仅在规则标签页且有标签时显示 */}
            {activeMainTab === "rules" && allTags.length > 0 && (
              <div className="flex items-center gap-2 overflow-x-auto pb-2 scrollbar-hide">
                <div className="flex items-center gap-1 text-sm text-gray-500 dark:text-gray-400 flex-shrink-0">
                  <Tag className="w-4 h-4" />
                  <span>标签:</span>
                </div>
                {allTags.map((tag) => (
                  <Badge
                    key={tag}
                    variant={selectedTags.includes(tag) ? "default" : "outline"}
                    role="button"
                    tabIndex={0}
                    aria-pressed={selectedTags.includes(tag)}
                    className={`cursor-pointer transition-colors flex-shrink-0 min-h-[32px] px-3 ${selectedTags.includes(tag)
                      ? "bg-blue-500 hover:bg-blue-600 text-white"
                      : "hover:bg-gray-100 dark:hover:bg-slate-700"
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
                    className="h-8 px-3 text-xs text-gray-500 hover:text-gray-700 flex-shrink-0 min-h-[44px]"
                  >
                    清除筛选
                  </Button>
                )}
              </div>
            )}
          </div>

          {/* Content Grid */}
          {isLoading ? (
            <div className="flex items-center justify-center py-20">
              <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
            </div>
          ) : activeMainTab === "rules" ? (
            clientRules.length === 0 ? (
              <div className="text-center py-20">
                <div className="w-20 h-20 mx-auto mb-6 rounded-2xl bg-gradient-to-br from-blue-100/50 to-blue-50 dark:from-blue-900/20 dark:to-blue-800/10 flex items-center justify-center">
                  <Globe className="w-10 h-10 text-blue-400/50 dark:text-blue-500/30" />
                </div>
                <p className="text-lg font-medium text-gray-900 dark:text-white">
                  {searchQuery || selectedTags.length > 0 ? "未找到匹配的规则" : "暂无规则"}
                </p>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-2 max-w-sm mx-auto">
                  {searchQuery || selectedTags.length > 0
                    ? "尝试调整搜索条件或清除筛选标签"
                    : "该客户端暂无可用规则"}
                </p>
              </div>
            ) : (
              <div className="card-grid-dashed">
                {clientRules.map((rule, index) => (
                  <div
                    key={rule.name}
                    className="card-refined group relative flex flex-col h-full gap-0 py-4 px-0 animate-slide-up opacity-0"
                    style={{ animationDelay: `${index * 30}ms` }}
                  >
                    {/* 右下角浮动按钮组 */}
                    <div className="absolute bottom-3 right-4 flex items-center gap-1.5 opacity-0 group-hover:opacity-100 transition-opacity duration-150">
                      <Tooltip delayDuration={100}>
                        <TooltipTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-7 w-7 rounded-md text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-white/5 transition-colors"
                            onClick={() => handlePreview({
                              type: "rule",
                              name: rule.name,
                              clientId: activeClient,
                              fileName: rule.name,
                            })}
                          >
                            <Eye className="w-3.5 h-3.5" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent side="top" className="text-xs">
                          查看内容
                        </TooltipContent>
                      </Tooltip>
                      <Tooltip delayDuration={100}>
                        <TooltipTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon"
                            className={`h-7 w-7 rounded-md transition-colors ${copiedRule === rule.name
                              ? "bg-green-500 text-white hover:bg-green-600"
                              : "text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-white/5"
                              }`}
                            onClick={() => copyRuleUrl(rule.name)}
                          >
                            {copiedRule === rule.name ? (
                              <CheckCircle className="w-3.5 h-3.5" />
                            ) : (
                              <Copy className="w-3.5 h-3.5" />
                            )}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent side="top" className="text-xs">
                          {copiedRule === rule.name ? "已复制" : "复制 URL"}
                        </TooltipContent>
                      </Tooltip>
                    </div>

                    <div className="pb-3 px-5">
                      <div className="flex items-start gap-3.5">
                        <div className="icon-refined w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0">
                          {rule.icon ? (
                            <RuleIcon icon={rule.icon} className="w-5.5 h-5.5 text-gray-500 dark:text-gray-400" />
                          ) : (
                            <FileText className="w-[18px] h-[18px] text-gray-500 dark:text-gray-400" />
                          )}
                        </div>
                        <div className="min-w-0 flex-1">
                          <h3 className="text-[15px] font-medium text-gray-900 dark:text-white truncate leading-tight">
                            {rule.displayName || rule.name}
                          </h3>
                          <Tooltip delayDuration={300}>
                            <TooltipTrigger asChild>
                              <div className={`mt-1.5 ${rule.description ? "cursor-default" : ""}`}>
                                <p className="text-[13px] text-gray-500 dark:text-gray-400 line-clamp-2 min-h-[2.6em] leading-snug">
                                  {rule.description || "暂无描述"}
                                </p>
                              </div>
                            </TooltipTrigger>
                            {rule.description && (
                              <TooltipContent
                                side="bottom"
                                align="start"
                                className="max-w-[300px] bg-gray-900 text-white dark:bg-white dark:text-gray-900 border-none shadow-lg"
                              >
                                <p className="text-[13px] whitespace-pre-wrap break-words leading-relaxed">
                                  {rule.description}
                                </p>
                              </TooltipContent>
                            )}
                          </Tooltip>
                        </div>
                      </div>
                    </div>
                    <div className="flex-1 flex flex-col justify-end pt-0 pb-0.5 px-5">
                      {/* 标签 */}
                      <div className="flex flex-wrap items-center gap-1.5 pr-16">
                        {rule.tags && rule.tags.length > 0 ? (
                          rule.tags.slice(0, 4).map((tag) => (
                            <span
                              key={tag}
                              className="badge-refined text-[11px] px-2 py-0.5 rounded-md font-normal"
                            >
                              {tag}
                            </span>
                          ))
                        ) : (
                          <span className="text-[11px] text-gray-300 dark:text-gray-600">—</span>
                        )}
                        {rule.tags && rule.tags.length > 4 && (
                          <span className="text-[11px] text-gray-400 dark:text-gray-500">+{rule.tags.length - 4}</span>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )
          ) : (
            clientPublicFiles.length === 0 ? (
              <div className="text-center py-20">
                <div className="w-20 h-20 mx-auto mb-6 rounded-2xl bg-gradient-to-br from-emerald-100/50 to-emerald-50 dark:from-emerald-900/20 dark:to-emerald-800/10 flex items-center justify-center">
                  <FileText className="w-10 h-10 text-emerald-400/50 dark:text-emerald-500/30" />
                </div>
                <p className="text-lg font-medium text-gray-900 dark:text-white">
                  {searchQuery ? "未找到匹配的配置文件" : "暂无公开配置文件"}
                </p>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-2 max-w-sm mx-auto">
                  {searchQuery
                    ? "尝试使用其他关键词搜索"
                    : "该客户端暂无公开的配置文件"}
                </p>
              </div>
            ) : (
              <div className="card-grid-dashed">
                {clientPublicFiles.map((file, index) => {
                  return (
                    <div
                      key={file.id}
                      className="card-refined group py-4 px-5 animate-slide-up opacity-0"
                      style={{ animationDelay: `${index * 30}ms` }}
                    >
                      <div className="flex items-start gap-3.5">
                        <div className="icon-refined w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0">
                          <FileText className="w-4.5 h-4.5 text-gray-500 dark:text-gray-400" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <h3 className="text-[15px] font-medium text-gray-900 dark:text-white truncate leading-tight">
                            {file.displayName || `${file.configId}.${file.ext}`}
                          </h3>
                          <p className="text-[12px] text-gray-400 dark:text-gray-500 mt-0.5 font-mono truncate">
                            {file.configId}.{file.ext}
                          </p>
                          {file.description && (
                            <p className="text-[13px] text-gray-500 dark:text-gray-400 mt-1.5 line-clamp-2 leading-snug">
                              {file.description}
                            </p>
                          )}
                        </div>
                      </div>
                      <div className="flex gap-2.5 mt-4">
                        <Button
                          variant="outline"
                          size="sm"
                          className="flex-1 h-9 text-[13px] border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-white/5"
                          onClick={() => handlePreview({
                            type: "config",
                            name: file.configId,
                            clientId: file.clientId,
                            fileName: `${file.configId}.${file.ext}`,
                            ext: file.ext,
                          })}
                        >
                          <Eye className="w-3.5 h-3.5 mr-1.5" />
                          预览
                        </Button>
                        <Button
                          size="sm"
                          className={`flex-1 h-9 text-[13px] transition-colors ${copiedConfig === file.id
                            ? "bg-green-600 hover:bg-green-700"
                            : "bg-gray-900 hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-100"
                            }`}
                          onClick={() => copyConfigUrl(file)}
                        >
                          {copiedConfig === file.id ? (
                            <>
                              <CheckCircle className="w-3.5 h-3.5 mr-1.5" />
                              已复制
                            </>
                          ) : (
                            <>
                              <Copy className="w-3.5 h-3.5 mr-1.5" />
                              复制
                            </>
                          )}
                        </Button>
                      </div>
                    </div>
                  );
                })}
              </div>
            )
          )
          }

          {/* Stats */}
          <div className="mt-8 text-center text-sm text-gray-500 dark:text-gray-400 space-y-1">
            {activeMainTab === "rules" ? (
              <>
                <p>共 {clientRules.length} 条规则</p>
                {lastSyncAt && (
                  <p>上次更新: {new Date(lastSyncAt).toLocaleString("zh-CN")}</p>
                )}
              </>
            ) : (
              <p>共 {clientPublicFiles.length} 个配置文件</p>
            )}
          </div>
        </main>

        {/* Footer */}
        <footer className="border-t bg-white/50 dark:bg-slate-900/50 mt-auto">
          <div className="container mx-auto px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
            <p>Proxy Rule Manager • 代理规则集托管服务</p>
            <p className="mt-1">
              <a
                href="https://github.com/Fl0w1nd/Proxy-Rule-Manager"
                target="_blank"
                rel="noopener noreferrer"
                className="hover:text-blue-500 inline-flex items-center gap-1"
              >
                <ExternalLink className="w-3 h-3" />
                GitHub
              </a>
            </p>
          </div>
        </footer>

        {/* Preview Dialog */}
        <Dialog open={!!previewItem && !isPreviewFullscreen} onOpenChange={(open) => !open && closePreview()}>
          <DialogContent className="max-w-5xl w-[90vw] h-[80vh] bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700 flex flex-col p-0">
            <DialogHeader className="px-6 pt-6 pb-4 border-b border-gray-200 dark:border-slate-700">
              <DialogTitle className="flex items-center gap-2 text-gray-900 dark:text-white">
                {(() => {
                  const rule = rules.find(r => r.name === previewItem?.name);
                  return rule?.icon ? (
                    <RuleIcon icon={rule.icon} className="w-6 h-6 text-gray-500 dark:text-gray-400" />
                  ) : (
                    <FileText className="w-5 h-5 text-gray-500 dark:text-gray-400" />
                  );
                })()}
                {previewItem?.fileName}
                <Badge className="bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400 ml-2">
                  {previewItem ? getClientDisplayName(previewItem.clientId) : getClientDisplayName(activeClient)}
                </Badge>
              </DialogTitle>
            </DialogHeader>
            <div className="flex-1 flex flex-col min-h-0 overflow-hidden relative">
              {previewLoading ? (
                <div className="flex-1 flex items-center justify-center">
                  <Loader2 className="w-6 h-6 animate-spin text-blue-500" />
                </div>
              ) : (
                <>
                  {/* 工具栏 */}
                  <div className="flex items-center justify-between px-6 py-2 border-b border-gray-200 dark:border-slate-700 bg-gray-50 dark:bg-slate-900">
                    <span className="text-sm text-gray-500 dark:text-gray-400">
                      {previewContent.split('\n').length} 行
                    </span>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          navigator.clipboard.writeText(previewContent);
                          toast.success("已复制内容");
                        }}
                        className="h-7 px-2"
                      >
                        <Copy className="w-4 h-4 mr-1" />
                        复制
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setIsPreviewFullscreen(true)}
                        className="h-7 px-2"
                      >
                        <Maximize2 className="w-4 h-4 mr-1" />
                        全屏
                      </Button>
                    </div>
                  </div>
                  {/* 内容区域 - 带行号 */}
                  <div className="flex-1 overflow-auto bg-gray-50 dark:bg-slate-900">
                    <div className="flex text-sm font-mono min-w-max">
                      {/* 行号 */}
                      <div className="py-4 pl-4 pr-3 text-right text-gray-400 dark:text-gray-500 select-none border-r border-gray-200 dark:border-slate-700 bg-gray-100 dark:bg-slate-800 sticky left-0">
                        {previewContent.split('\n').map((_, i) => (
                          <div key={i}>{i + 1}</div>
                        ))}
                      </div>
                      {/* 内容 */}
                      <pre className="py-4 px-4 text-gray-800 dark:text-gray-200 whitespace-pre">
                        {previewContent || "暂无内容"}
                      </pre>
                    </div>
                  </div>
                </>
              )}
            </div>
          </DialogContent>
        </Dialog>
      </div>
    </TooltipProvider>
  );
}



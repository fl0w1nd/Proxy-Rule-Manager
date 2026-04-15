"use client";

import { useState, useEffect, useMemo } from "react";
import NextImage from "next/image";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
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
  CheckCircle,
  Maximize2,
  X,
  Tag,
  Image as ImageIcon,
  Github,
  Clock,
  Download,
} from "lucide-react";
import { useTheme } from "./theme-provider";
import { toast } from "sonner";
import { ClientFileMeta } from "@/lib/schema";
import { RuleIcon } from "./icon-picker";
import {
  listIcons,
  IconMeta,
  ClientConfig,
  PublicRuleInfo,
  getPublicStatus,
  getPublicClientFiles,
} from "@/lib/api-client";
import { AmbientBackground } from "./ambient-background";
import { CodeViewer } from "./code-viewer";
import { cn } from "@/lib/utils";

const MAIN_TABS = [
  { key: "rules", label: "规则" },
  { key: "configs", label: "配置文件" },
  { key: "icons", label: "图标集" },
] as const;

const TAG_BADGE_VARIANTS = ["blue", "rose", "amber", "violet", "teal", "emerald"] as const;

export function PublicRulesPage({ onAdminClick }: { onAdminClick: () => void }) {
  const { theme, toggleTheme } = useTheme();
  const [rules, setRules] = useState<PublicRuleInfo[]>([]);
  const [clients, setClients] = useState<ClientConfig[]>([]);
  const [clientFiles, setClientFiles] = useState<ClientFileMeta[]>([]);
  const [lastSyncAt, setLastSyncAt] = useState<string | null>(null);
  const [version, setVersion] = useState<string>("");
  const [isLoading, setIsLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [activeMainTab, setActiveMainTab] = useState<"rules" | "configs" | "icons">("rules");
  const [icons, setIcons] = useState<IconMeta[]>([]);
  const [copiedIcon, setCopiedIcon] = useState<string | null>(null);
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
  
  const [isPreviewFullscreen, setIsPreviewFullscreen] = useState(false);

  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [isScrolled, setIsScrolled] = useState(false);

  // 切换主标签时清空已选标签
  useEffect(() => {
    setSelectedTags([]);
  }, [activeMainTab]);

  useEffect(() => {
    const handleScroll = () => setIsScrolled(window.scrollY > 10);
    window.addEventListener("scroll", handleScroll, { passive: true });
    return () => window.removeEventListener("scroll", handleScroll);
  }, []);

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

  const copyToClipboard = async (
    text: string,
    onCopied: () => void,
    successMessage?: string
  ) => {
    try {
      await navigator.clipboard.writeText(text);
      onCopied();
      if (successMessage) {
        toast.success(successMessage);
      }
    } catch {
      toast.error("复制失败");
    }
  };

  const fetchData = async () => {
    try {
      const [statusResult, filesResult, iconsResult] = await Promise.all([
        getPublicStatus(),
        getPublicClientFiles(),
        listIcons().catch(() => ({ icons: [] })),
      ]);

      setRules(statusResult.rules || []);
      setLastSyncAt(statusResult.lastSyncAt || null);
      setVersion(statusResult.version || "");
      if (statusResult.clients && statusResult.clients.length > 0) {
        setClients(statusResult.clients);
        setActiveClient((prev) => prev || statusResult.clients[0].id);
      }

      setClientFiles(filesResult.files || []);
      setIcons(iconsResult.icons || []);
    } catch {
      toast.error("加载公开数据失败");
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
    return `${window.location.origin}/Rules/${encodeURIComponent(clientPath)}/${encodeURIComponent(ruleName)}.list`;
  };

  const getConfigUrl = (name: string, ext: string) => {
    return `${window.location.origin}/client/${encodeURIComponent(name)}.${encodeURIComponent(ext)}`;
  };

  const copyRuleUrl = async (ruleName: string, clientId?: string) => {
    const target = clientId || activeClient;
    const url = getRuleUrl(ruleName, target);
    await copyToClipboard(url, () => {
      setCopiedRule(ruleName);
      setTimeout(() => setCopiedRule(null), 2000);
    });
  };

  const getClientDisplayName = (clientId: string) => {
    return getClientConfig(clientId)?.displayName || clientId;
  };

  

  const getIconFullUrl = (icon: IconMeta) => {
    return `${window.location.origin}${icon.url}`;
  };

  const copyIconUrl = async (icon: IconMeta) => {
    await copyToClipboard(getIconFullUrl(icon), () => {
      setCopiedIcon(icon.id);
      setTimeout(() => setCopiedIcon(null), 2000);
    });
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
        : await fetch(getConfigUrl(item.name, item.ext || ""));
      if (response.ok) {
        const text = await response.text();
        setPreviewContent(text);
      } else {
        setPreviewContent("# 文件暂不可用");
      }
    } catch {
      setPreviewContent("# 加载失败，请稍后重试");
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

  const filteredIcons = useMemo(() => {
    if (!searchQuery) return icons;
    const query = searchQuery.toLowerCase();
    return icons.filter((icon) => icon.name.toLowerCase().includes(query));
  }, [icons, searchQuery]
  );

  // 标签颜色循环 - 柔和的多彩色系
  const getTagBadgeVariant = (tag: string) => {
    let hash = 0;
    for (let i = 0; i < tag.length; i++) {
      hash = tag.charCodeAt(i) + ((hash << 5) - hash);
    }
    return TAG_BADGE_VARIANTS[Math.abs(hash) % TAG_BADGE_VARIANTS.length];
  };

  const closePreview = () => {
    setPreviewItem(null);
    setIsPreviewFullscreen(false);
  };

  // 全屏预览模式
  if (isPreviewFullscreen && previewItem) {
    return (
      <div className="fixed inset-0 z-50 flex flex-col bg-background">
        {/* 顶部工具栏 */}
        <div className="flex items-center justify-between border-b border-border bg-background px-6 py-4">
          <div className="flex items-center gap-3">
            {(() => {
              const rule = rules.find(r => r.name === previewItem.name);
              return rule?.icon ? (
                <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary-soft">
                  <RuleIcon icon={rule.icon} className="w-5 h-5 text-primary-foreground dark:text-primary" />
                </div>
              ) : (
                <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary-soft">
                  <FileText className="w-5 h-5 text-primary-foreground dark:text-primary" />
                </div>
              );
            })()}
            <span className="font-semibold text-foreground">{previewItem.fileName}</span>
            <Badge variant="active">
              {getClientConfig(previewItem.clientId)?.displayName || previewItem.clientId}
            </Badge>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-sm text-muted-foreground">
              {previewContent.split('\n').length} 行
            </span>
            <Button
              variant="secondary"
              className="h-10 rounded-full px-4 text-sm"
              onClick={() => {
                void copyToClipboard(previewContent, () => undefined, "已复制内容");
              }}
            >
              <Copy className="w-4 h-4" />
              复制
            </Button>
            <Button
              variant="secondary"
              size="icon-lg"
              className="rounded-full"
              onClick={() => setIsPreviewFullscreen(false)}
            >
              <X className="w-5 h-5" />
            </Button>
          </div>
        </div>

        {/* 内容 - 带行号 */}
        <div className="flex-1 overflow-auto m-4">
          <CodeViewer content={previewContent} loading={previewLoading} />
        </div>
      </div>
    );
  }

  return (
    <TooltipProvider>
      <div className="relative min-h-screen bg-background transition-colors">
        <AmbientBackground />
        {/* Header */}
        <header
          className={cn(
            "sticky top-0 z-50 border-b border-border bg-background/95 backdrop-blur-md transition-shadow",
            isScrolled ? "shadow-[var(--shadow-sm)]" : ""
          )}
        >
          <div className="container mx-auto px-4 sm:px-6 py-4">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-3 min-w-0">
                <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary shadow-[var(--shadow-xs)]">
                  <NextImage src="/logo.svg" alt="Logo" width={22} height={22} className="w-[22px] h-[22px]" />
                </div>
                <div className="min-w-0">
                  <h1 className="text-lg sm:text-xl font-bold text-foreground truncate tracking-tight leading-tight">
                    代理规则集
                  </h1>
                  <div className="flex items-center gap-2">
                    <p className="text-xs text-muted-foreground hidden xs:block">
                      Proxy Rule Manager
                    </p>
                    {version && (
                      <span className="rounded-full bg-primary/10 px-2 py-0.5 font-mono text-[10px] font-semibold text-primary-foreground dark:text-primary">
                        v{version}
                      </span>
                    )}
                  </div>
                </div>
              </div>
              <div className="flex items-center gap-2 flex-shrink-0">
                {lastSyncAt && (
                  <Tooltip delayDuration={100}>
                    <TooltipTrigger asChild>
                      <div className="hidden cursor-default items-center gap-1.5 rounded-full border border-border bg-surface-subtle px-3 py-1.5 text-xs font-medium text-muted-foreground sm:flex">
                        <Clock className="w-3 h-3" />
                        <span>{new Date(lastSyncAt).toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" })}</span>
                      </div>
                    </TooltipTrigger>
                    <TooltipContent side="bottom" className="text-xs">
                      上次同步: {new Date(lastSyncAt).toLocaleString("zh-CN")}
                    </TooltipContent>
                  </Tooltip>
                )}
                <Tooltip delayDuration={100}>
                  <TooltipTrigger asChild>
                    <Button
                      variant="secondary"
                      size="icon"
                      asChild
                    >
                      <a
                        href="https://github.com/Fl0w1nd/Proxy-Rule-Manager"
                        target="_blank"
                        rel="noopener noreferrer"
                        aria-label="GitHub"
                      >
                        <Github className="w-4 h-4" />
                      </a>
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="bottom" className="text-xs">GitHub</TooltipContent>
                </Tooltip>
                <Button
                  variant="secondary"
                  size="icon"
                  onClick={toggleTheme}
                  aria-label={theme === "light" ? "切换到深色模式" : "切换到浅色模式"}
                >
                  {theme === "light" ? (
                    <Moon className="w-4 h-4" />
                  ) : (
                    <Sun className="w-4 h-4" />
                  )}
                </Button>
                <Button
                  variant="default"
                  onClick={onAdminClick}
                  className="text-sm"
                >
                  <Settings className="w-4 h-4" />
                  <span className="hidden sm:inline">管理</span>
                </Button>
              </div>
            </div>
          </div>
        </header>

        {/* Main Content */}
        <main className="container mx-auto px-4 sm:px-6 py-8">
          {/* Main Tabs & Search */}
          <div className="mb-8 space-y-5">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
              <div className="inline-flex w-fit items-center gap-1 rounded-full bg-surface-subtle p-1">
                {MAIN_TABS.map((tab) => (
                <button
                  key={tab.key}
                  onClick={() => setActiveMainTab(tab.key)}
                  className={cn(
                    "rounded-full px-5 py-2 text-sm font-semibold transition-all duration-150 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-primary/25",
                    activeMainTab === tab.key
                      ? "bg-primary text-primary-foreground shadow-[var(--shadow-xs)]"
                      : "text-muted-foreground hover:text-foreground"
                  )}
                >
                  {tab.label}
                </button>
              ))}
              </div>

              <div className="flex w-full items-center gap-2 rounded-full border border-border bg-background px-4 py-2.5 shadow-[var(--shadow-xs)] sm:w-auto sm:min-w-[280px]">
                <Search className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                <input
                  type="search"
                  aria-label={activeMainTab === "rules" ? "搜索规则" : activeMainTab === "configs" ? "搜索配置文件" : "搜索图标"}
                  placeholder={activeMainTab === "rules" ? "搜索规则" : activeMainTab === "configs" ? "搜索配置文件" : "搜索图标"}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="w-full bg-transparent text-sm font-medium text-foreground outline-none placeholder:text-muted-foreground"
                />
              </div>
            </div>

            {/* Client Tabs */}
            {activeMainTab !== "icons" && (
              <div className="inline-flex max-w-full items-center gap-1 overflow-x-auto rounded-full bg-surface-subtle p-1 scrollbar-hide">
                {clients.map((client) => (
                  <button
                    key={client.id}
                    onClick={() => setActiveClient(client.id)}
                    className={cn(
                      "shrink-0 whitespace-nowrap rounded-full px-4 py-2 text-sm font-semibold transition-all duration-150 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-primary/25",
                      activeClient === client.id
                        ? "bg-foreground text-background shadow-[var(--shadow-xs)]"
                        : "text-muted-foreground hover:text-foreground"
                    )}
                  >
                    {client.displayName}
                  </button>
                ))}
              </div>
            )}

            {/* Tag Filter */}
            {activeMainTab === "rules" && allTags.length > 0 && (
              <div className="flex items-center gap-2 overflow-x-auto pb-1 scrollbar-hide">
                <div className="flex shrink-0 items-center gap-1.5 text-sm text-muted-foreground" role="img" aria-label="标签筛选">
                  <Tag className="w-3.5 h-3.5" aria-hidden="true" />
                </div>
                {allTags.map((tag) => (
                  <button
                    key={tag}
                    onClick={() => toggleTag(tag)}
                    className={cn(
                      "shrink-0 rounded-full border px-3 py-1.5 text-xs font-semibold transition-all duration-150 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-primary/25",
                      selectedTags.includes(tag)
                        ? "border-primary/30 bg-primary-soft text-primary-foreground dark:text-primary"
                        : "border-border bg-surface-subtle text-muted-foreground hover:text-foreground"
                    )}
                  >
                    {tag}
                  </button>
                ))}
                {selectedTags.length > 0 && (
                  <button
                    onClick={() => setSelectedTags([])}
                    className="text-xs text-muted-foreground hover:text-foreground flex-shrink-0 ml-1 transition-colors"
                  >
                    清除筛选
                  </button>
                )}
              </div>
            )}
          </div>

          {/* Content Grid */}
          {isLoading ? (
            <div className="flex items-center justify-center py-24">
              <div className="flex flex-col items-center gap-3">
                <Loader2 className="w-6 h-6 animate-spin text-primary" />
                <p className="text-sm font-medium text-muted-foreground">加载中...</p>
              </div>
            </div>
          ) : activeMainTab === "rules" ? (
            clientRules.length === 0 ? (
              <div className="text-center py-24">
                <div className="bg-surface-subtle w-20 h-20 mx-auto mb-6 flex items-center justify-center rounded-2xl">
                  <Globe className="w-8 h-8 text-muted-foreground/40" />
                </div>
                <p className="text-lg font-semibold text-foreground">
                  {searchQuery || selectedTags.length > 0 ? "未找到匹配的规则" : "暂无规则"}
                </p>
                <p className="text-sm text-muted-foreground mt-2 max-w-sm mx-auto">
                  {searchQuery || selectedTags.length > 0
                    ? "尝试调整搜索条件或清除筛选标签"
                    : "该客户端暂无可用规则"}
                </p>
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                {clientRules.map((rule, index) => (
                  <Card
                    key={rule.name}
                    className="group relative p-5 animate-slide-up opacity-0 hover:shadow-[var(--shadow-md)] transition-shadow duration-200"
                    style={{ animationDelay: `${index * 40}ms` }}
                  >
                    {/* Floating action buttons */}
                    <div className="absolute right-4 top-4 flex translate-y-1 items-center gap-1 rounded-full border border-border bg-background p-1 opacity-0 shadow-[var(--shadow-sm)] transition-all duration-200 group-hover:translate-y-0 group-hover:opacity-100 group-focus-within:translate-y-0 group-focus-within:opacity-100">
                      <Tooltip delayDuration={100}>
                        <TooltipTrigger asChild>
                          <button
                            className="flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-primary/15"
                            onClick={() => handlePreview({
                              type: "rule",
                              name: rule.name,
                              clientId: activeClient,
                              fileName: rule.name,
                            })}
                          >
                            <Eye className="w-3.5 h-3.5" />
                          </button>
                        </TooltipTrigger>
                        <TooltipContent side="top" className="text-xs">
                          查看内容
                        </TooltipContent>
                      </Tooltip>
                      <Tooltip delayDuration={100}>
                        <TooltipTrigger asChild>
                          <button
                            className={cn(
                              "flex h-8 w-8 items-center justify-center rounded-lg transition-all duration-200 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-primary/15",
                              copiedRule === rule.name
                                ? "border border-success/20 bg-success-soft text-success"
                                : "text-muted-foreground hover:bg-accent hover:text-foreground"
                            )}
                            onClick={() => {
                              void copyRuleUrl(rule.name);
                            }}
                          >
                            {copiedRule === rule.name ? (
                              <CheckCircle className="w-3.5 h-3.5" />
                            ) : (
                              <Copy className="w-3.5 h-3.5" />
                            )}
                          </button>
                        </TooltipTrigger>
                        <TooltipContent side="top" className="text-xs">
                          {copiedRule === rule.name ? "已复制" : "复制 URL"}
                        </TooltipContent>
                      </Tooltip>
                    </div>

                    <div className="flex items-start gap-3">
                      <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary-soft">
                        {rule.icon ? (
                          <RuleIcon icon={rule.icon} className="w-5 h-5 text-primary-foreground dark:text-primary" />
                        ) : (
                          <FileText className="w-[18px] h-[18px] text-primary-foreground dark:text-primary" />
                        )}
                      </div>
                      <div className="min-w-0 flex-1 pr-16">
                        <h3 className="text-sm font-semibold text-foreground truncate leading-tight">
                          {rule.displayName || rule.name}
                        </h3>
                        {rule.description && (
                          <Tooltip delayDuration={300}>
                            <TooltipTrigger asChild>
                              <div className="mt-1.5 cursor-default">
                                <p className="text-xs text-muted-foreground line-clamp-2 leading-relaxed">
                                  {rule.description}
                                </p>
                              </div>
                            </TooltipTrigger>
                            <TooltipContent
                              side="bottom"
                              align="start"
                              showArrow={false}
                              className="max-w-[300px] bg-foreground text-background shadow-[var(--shadow-lg)] rounded-lg"
                            >
                              <p className="text-xs whitespace-pre-wrap break-words leading-relaxed">
                                {rule.description}
                              </p>
                            </TooltipContent>
                          </Tooltip>
                        )}
                      </div>
                    </div>

                    {/* Tags */}
                    <div className="flex flex-wrap items-center gap-1.5 mt-4">
                      {rule.tags && rule.tags.length > 0 ? (
                        rule.tags.slice(0, 4).map((tag) => (
                          <Badge
                            key={tag}
                            variant={getTagBadgeVariant(tag)}
                            className="text-[10px]"
                          >
                            {tag}
                          </Badge>
                        ))
                      ) : (
                        <span className="text-[11px] text-muted-foreground/70">—</span>
                      )}
                      {rule.tags && rule.tags.length > 4 && (
                        <span className="text-[11px] text-muted-foreground">+{rule.tags.length - 4}</span>
                      )}
                    </div>
                  </Card>
                ))}
              </div>
            )
          ) : activeMainTab === "configs" ? (
            clientPublicFiles.length === 0 ? (
              <div className="text-center py-24">
                <div className="bg-surface-subtle w-20 h-20 mx-auto mb-6 flex items-center justify-center rounded-2xl">
                  <FileText className="w-8 h-8 text-muted-foreground/40" />
                </div>
                <p className="text-lg font-semibold text-foreground">
                  {searchQuery ? "未找到匹配的配置文件" : "暂无公开配置文件"}
                </p>
                <p className="text-sm text-muted-foreground mt-2 max-w-sm mx-auto">
                  {searchQuery
                    ? "尝试使用其他关键词搜索"
                    : "该客户端暂无公开的配置文件"}
                </p>
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                {clientPublicFiles.map((file, index) => {
                  return (
                    <Card
                      key={file.id}
                      className="p-5 animate-slide-up opacity-0"
                      style={{ animationDelay: `${index * 40}ms` }}
                    >
                      <div className="flex items-start gap-3">
                        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-success-soft">
                          <FileText className="w-[18px] h-[18px] text-success" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <h3 className="text-sm font-semibold text-foreground truncate leading-tight">
                            {file.displayName || `${file.configId}.${file.ext}`}
                          </h3>
                          <p className="text-[11px] text-muted-foreground mt-0.5 font-mono truncate">
                            {file.configId}.{file.ext}
                          </p>
                          {file.description && (
                            <p className="text-xs text-muted-foreground mt-1.5 line-clamp-2 leading-relaxed">
                              {file.description}
                            </p>
                          )}
                        </div>
                      </div>
                      <div className="flex gap-2 mt-4">
                        <Button
                          variant="secondary"
                          size="sm"
                          className="flex-1 flex items-center justify-center gap-1.5"
                          onClick={() => handlePreview({
                            type: "config",
                            name: file.configId,
                            clientId: file.clientId,
                            fileName: `${file.configId}.${file.ext}`,
                            ext: file.ext,
                          })}
                        >
                          <Eye className="w-3.5 h-3.5" />
                          预览
                        </Button>
                        <Button
                          variant="default"
                          size="sm"
                          className="flex-1 flex items-center justify-center gap-1.5"
                          asChild
                        >
                          <a
                            href={getConfigUrl(file.configId, file.ext)}
                            download={`${file.configId}.${file.ext}`}
                          >
                            <Download className="w-3.5 h-3.5" />
                            下载
                          </a>
                        </Button>
                      </div>
                    </Card>
                  );
                })}
              </div>
            )
          ) : (
            /* Icons Tab */
            filteredIcons.length === 0 ? (
              <div className="text-center py-24">
                <div className="bg-surface-subtle w-20 h-20 mx-auto mb-6 flex items-center justify-center rounded-2xl">
                  <ImageIcon className="w-8 h-8 text-muted-foreground/40" />
                </div>
                <p className="text-lg font-semibold text-foreground">
                  {searchQuery ? "未找到匹配的图标" : "暂无图标"}
                </p>
                <p className="text-sm text-muted-foreground mt-2 max-w-sm mx-auto">
                  {searchQuery
                    ? "尝试使用其他关键词搜索"
                    : "图标集暂无图标"}
                </p>
              </div>
            ) : (
              <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8 gap-5">
                {filteredIcons.map((icon, index) => (
                  <Card
                    key={icon.id}
                    className="group p-3 animate-slide-up opacity-0"
                    style={{ animationDelay: `${index * 25}ms` }}
                  >
                    <div className="relative mb-2 flex aspect-square w-full items-center justify-center overflow-hidden rounded-xl border border-border bg-surface-subtle p-2">
                      <NextImage
                        src={icon.url}
                        alt={icon.name}
                        fill
                        sizes="(max-width: 640px) 30vw, (max-width: 1024px) 16vw, 10vw"
                        unoptimized
                        className="object-contain"
                      />
                    </div>
                    <p className="text-xs font-medium text-foreground truncate text-center" title={icon.name}>
                      {icon.name}
                    </p>
                    <Button
                      variant={copiedIcon === icon.id ? "success" : "secondary"}
                      size="sm"
                      className="mt-2 h-8 w-full rounded-lg text-[11px] font-medium"
                      onClick={() => {
                        void copyIconUrl(icon);
                      }}
                    >
                      {copiedIcon === icon.id ? (
                        <>
                          <CheckCircle className="w-3 h-3" />
                          已复制
                        </>
                      ) : (
                        <>
                          <Copy className="w-3 h-3" />
                          复制
                        </>
                      )}
                    </Button>
                  </Card>
                ))}
              </div>
            )
          )
          }

          {/* Stats */}
          <div className="mt-12 flex justify-center">
            <div className="rounded-full px-6 py-2 text-center text-xs font-medium text-muted-foreground">
              {activeMainTab === "rules" ? (
                <p>共 {clientRules.length} 条规则</p>
              ) : activeMainTab === "configs" ? (
                <p>共 {clientPublicFiles.length} 个配置文件</p>
              ) : (
                <p>共 {filteredIcons.length} 个图标</p>
              )}
            </div>
          </div>
        </main>

        {/* Preview Dialog */}
        <Dialog open={!!previewItem && !isPreviewFullscreen} onOpenChange={(open) => !open && closePreview()}>
          <DialogContent className="max-w-5xl w-[90vw] h-[80vh] flex flex-col p-0 overflow-hidden">
            <DialogHeader className="px-6 pt-6 pb-4 border-b border-border">
              <DialogTitle className="flex items-center gap-3 text-foreground font-semibold">
                {(() => {
                  const rule = rules.find(r => r.name === previewItem?.name);
                  return rule?.icon ? (
                    <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-soft">
                      <RuleIcon icon={rule.icon} className="w-4 h-4 text-primary-foreground dark:text-primary" />
                    </div>
                  ) : (
                    <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-soft">
                      <FileText className="w-4 h-4 text-primary-foreground dark:text-primary" />
                    </div>
                  );
                })()}
                {previewItem?.fileName}
                <Badge variant="active" className="ml-1">
                  {previewItem ? getClientDisplayName(previewItem.clientId) : getClientDisplayName(activeClient)}
                </Badge>
              </DialogTitle>
            </DialogHeader>
            <div className="flex-1 flex flex-col min-h-0 overflow-hidden relative mx-4 mb-4">
              {previewLoading ? (
                <div className="flex-1 flex items-center justify-center">
                  <Loader2 className="w-6 h-6 animate-spin text-primary/50" />
                </div>
              ) : (
                <>
                  {/* 工具栏 */}
                  <div className="flex items-center justify-between px-4 py-2 mb-2">
                    <span className="text-sm text-muted-foreground">
                      {previewContent.split('\n').length} 行
                    </span>
                    <div className="flex items-center gap-2">
                      <Button
                        variant="secondary"
                        size="sm"
                        className="text-xs flex items-center gap-1.5"
                        onClick={() => {
                          void copyToClipboard(previewContent, () => undefined, "已复制内容");
                        }}
                      >
                        <Copy className="w-3.5 h-3.5" />
                        复制
                      </Button>
                      <Button
                        variant="secondary"
                        size="sm"
                        className="text-xs flex items-center gap-1.5"
                        onClick={() => setIsPreviewFullscreen(true)}
                      >
                        <Maximize2 className="w-3.5 h-3.5" />
                        全屏
                      </Button>
                    </div>
                  </div>
                  {/* 内容区域 - 带行号 */}
                  <CodeViewer content={previewContent} className="flex-1" />
                </>
              )}
            </div>
          </DialogContent>
        </Dialog>
      </div>
    </TooltipProvider>
  );
}

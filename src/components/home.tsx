"use client";

import { useState, useEffect, useMemo, useRef, startTransition } from "react";
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
  Clock,
  Download,
} from "lucide-react";
import { Icon } from "@iconify/react";
import { useTheme } from "./theme-provider";
import { toast } from "sonner";
import { ClientFileMeta, DEFAULT_SYSTEM_SETTINGS, resolveOutputExt } from "@/lib/schema";
import { RuleIcon } from "./icon-picker";
import {
  listIcons,
  IconMeta,
  ClientConfig,
  PublicRuleInfo,
  PublicGeositeInfo,
  getPublicStatus,
  getPublicClientFiles,
} from "@/lib/api-client";
import { AmbientBackground } from "./ambient-background";
import { CodeViewer } from "./code-viewer";
import { SearchInput } from "@/components/ui/search-input";
import { EmptyState } from "@/components/ui/empty-state";
import { cn, formatRelativeTime } from "@/lib/utils";

const MAIN_TABS = [
  { key: "rules", label: "规则订阅" },
  { key: "geosite", label: "Geosite" },
  { key: "configs", label: "客户端配置" },
  { key: "icons", label: "图标资源" },
] as const;

const TAG_BADGE_VARIANTS = ["blue", "rose", "amber", "violet", "teal", "emerald"] as const;

const DEFAULT_FAILURE_THRESHOLD = DEFAULT_SYSTEM_SETTINGS.sync.failureThreshold;

// FailureBadge renders a single status pill describing the rule's recent sync
// health. We no longer time-window "数据陈旧"; the badge is purely driven by
// actual sync-attempt outcomes:
//   - consecutiveFailures >= threshold → "更新失败 ×N" (warning, persistent)
//   - hasError (last attempt failed but not yet at threshold) → "上次失败"
//   - otherwise nothing
function FailureBadge({
  hasError,
  lastFailureAt,
  consecutiveFailures,
  threshold,
}: {
  hasError: boolean;
  lastFailureAt: string | null;
  consecutiveFailures: number;
  threshold: number;
}) {
  if (consecutiveFailures >= threshold && threshold > 0) {
    return (
      <Tooltip delayDuration={300}>
        <TooltipTrigger asChild>
          <Badge variant="outline" className="border-warning/30 bg-warning-soft text-warning text-[10px]">
            更新失败 ×{consecutiveFailures}
          </Badge>
        </TooltipTrigger>
        <TooltipContent side="top" align="start" showArrow={false}>
          <p className="text-xs">
            已连续 {consecutiveFailures} 次同步失败
            {lastFailureAt ? `，最近一次：${formatRelativeTime(lastFailureAt)}` : ""}
          </p>
        </TooltipContent>
      </Tooltip>
    );
  }
  if (hasError) {
    return (
      <Tooltip delayDuration={300}>
        <TooltipTrigger asChild>
          <Badge variant="destructive" className="text-[10px]">
            上次失败
          </Badge>
        </TooltipTrigger>
        <TooltipContent side="top" align="start" showArrow={false}>
          <p className="text-xs">
            {lastFailureAt ? `失败时间：${formatRelativeTime(lastFailureAt)}` : "最近一次同步失败"}
          </p>
        </TooltipContent>
      </Tooltip>
    );
  }
  return null;
}

export function PublicRulesPage({ onAdminClick }: { onAdminClick: () => void }) {
  const { mode, toggleMode } = useTheme();
  const [rules, setRules] = useState<PublicRuleInfo[]>([]);
  const [geositeRules, setGeositeRules] = useState<PublicGeositeInfo[]>([]);
  const [clients, setClients] = useState<ClientConfig[]>([]);
  const [clientFiles, setClientFiles] = useState<ClientFileMeta[]>([]);
  const [lastSyncAt, setLastSyncAt] = useState<string | null>(null);
  const [failureThreshold, setFailureThreshold] = useState<number>(DEFAULT_FAILURE_THRESHOLD);
  const [version, setVersion] = useState<string>("");
  const [isLoading, setIsLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [activeMainTab, setActiveMainTab] = useState<"rules" | "geosite" | "configs" | "icons">("rules");
  const [icons, setIcons] = useState<IconMeta[]>([]);
  const [copiedIcon, setCopiedIcon] = useState<string | null>(null);
  const [activeClient, setActiveClient] = useState<string>("");
  const [previewItem, setPreviewItem] = useState<{
    type: "rule" | "geosite" | "config";
    name: string;
    clientId: string;
    fileName: string;
    ext?: string;
    provider?: string;
  } | null>(null);
  const [previewContent, setPreviewContent] = useState<string>("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [copiedRule, setCopiedRule] = useState<string | null>(null);
  
  const [isPreviewFullscreen, setIsPreviewFullscreen] = useState(false);

  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [selectedGeositeProvider, setSelectedGeositeProvider] = useState<string>("all");
  const [isScrolled, setIsScrolled] = useState(false);
  const previewRequestRef = useRef(0);

  // 切换主标签时清空已选标签和来源
  useEffect(() => {
    startTransition(() => {
      setSelectedTags([]);
      setSelectedGeositeProvider("all");
    });
  }, [activeMainTab]);

  useEffect(() => {
    const handleScroll = () => setIsScrolled(window.scrollY > 10);
    window.addEventListener("scroll", handleScroll, { passive: true });
    return () => window.removeEventListener("scroll", handleScroll);
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
      setGeositeRules(statusResult.geositeRules || []);
      setLastSyncAt(statusResult.lastSyncAt || null);
      if (typeof statusResult.failureThreshold === "number" && statusResult.failureThreshold > 0) {
        setFailureThreshold(statusResult.failureThreshold);
      }
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

  useEffect(() => {
    startTransition(() => { fetchData(); });
  }, []);

  const getClientConfig = (clientId: string): ClientConfig | undefined => {
    return clients.find(c => c.id === clientId);
  };

  // 客户端可以自定义产出文件后缀；缺省值回退到 .list 以兼容历史 URL。
  const getClientExt = (clientId: string) => resolveOutputExt(getClientConfig(clientId)?.outputExt);

  const getRuleUrl = (ruleName: string, clientId: string) => {
    return `${window.location.origin}/Rules/${encodeURIComponent(clientId)}/${encodeURIComponent(ruleName)}.${getClientExt(clientId)}`;
  };

  const getGeositeUrl = (provider: string, outputName: string, clientId: string) => {
    return `${window.location.origin}/Rules/${encodeURIComponent(clientId)}/geosite/${encodeURIComponent(provider)}/${encodeURIComponent(outputName)}.${getClientExt(clientId)}`;
  };

  const getConfigUrl = (clientId: string, name: string, ext: string) => {
    return `${window.location.origin}/client/${encodeURIComponent(clientId)}/${encodeURIComponent(name)}.${encodeURIComponent(ext)}`;
  };

  const copyRuleUrl = async (ruleName: string, clientId?: string) => {
    const target = clientId || activeClient;
    const url = getRuleUrl(ruleName, target);
    await copyToClipboard(url, () => {
      setCopiedRule(ruleName);
      setTimeout(() => setCopiedRule(null), 2000);
    });
  };

  const copyGeositeUrl = async (provider: string, outputName: string, clientId?: string) => {
    const target = clientId || activeClient;
    const url = getGeositeUrl(provider, outputName, target);
    await copyToClipboard(url, () => {
      setCopiedRule(`${provider}/${outputName}`);
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
    type: "rule" | "geosite" | "config";
    name: string;
    clientId: string;
    fileName: string;
    ext?: string;
    provider?: string;
  }) => {
    const requestId = previewRequestRef.current + 1;
    previewRequestRef.current = requestId;
    setPreviewItem(item);
    setPreviewLoading(true);
    setPreviewContent("");

    try {
      const response = item.type === "rule"
        ? await fetch(getRuleUrl(item.name, item.clientId))
        : item.type === "geosite"
          ? await fetch(getGeositeUrl(item.provider || "", item.name, item.clientId))
        : await fetch(getConfigUrl(item.clientId, item.name, item.ext || ""));
      if (response.ok) {
        const text = await response.text();
        if (previewRequestRef.current !== requestId) return;
        setPreviewContent(text);
      } else {
        if (previewRequestRef.current !== requestId) return;
        setPreviewContent("# 文件暂不可用");
      }
    } catch {
      if (previewRequestRef.current !== requestId) return;
      setPreviewContent("# 加载失败，请稍后重试");
    } finally {
      if (previewRequestRef.current !== requestId) return;
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

  const geositeProviders = useMemo(() => {
    const providers = Array.from(new Set(geositeRules.map((r) => r.provider))).sort();
    return providers;
  }, [geositeRules]);

  const filteredGeositeRules = useMemo(() => {
    return geositeRules.filter((rule) => {
      const matchesSearch =
        rule.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        rule.displayName?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        rule.description?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        rule.attrs.join(" ").toLowerCase().includes(searchQuery.toLowerCase()) ||
        rule.provider.toLowerCase().includes(searchQuery.toLowerCase());
      const matchesProvider = selectedGeositeProvider === "all" || rule.provider === selectedGeositeProvider;
      return matchesSearch && matchesProvider && rule.clients.includes(activeClient);
    });
  }, [geositeRules, searchQuery, activeClient, selectedGeositeProvider]);

  const filteredClientFiles = clientFiles.filter((file) => {
    const query = searchQuery.toLowerCase();
    const configPath = `/client/${file.clientId}/${file.configId}.${file.ext}`.toLowerCase();
    const displayName = (file.displayName || "").toLowerCase();
    const description = (file.description || "").toLowerCase();
    return configPath.includes(query) || displayName.includes(query) || description.includes(query);
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
                <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-surface-strong">
                  <RuleIcon icon={rule.icon} className="w-5 h-5" />
                </div>
              ) : (
                <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-surface-strong">
                  <FileText className="w-5 h-5 text-muted-foreground" />
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
        <div className="m-4 flex-1 min-h-0">
          <CodeViewer content={previewContent} loading={previewLoading} className="h-full" height="100%" />
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
            "sticky top-0 z-50 border-b border-transparent bg-background/80 backdrop-blur-xl backdrop-saturate-150 transition-[border-color,box-shadow] duration-200 ease-out",
            isScrolled ? "border-border shadow-[var(--shadow-xs)]" : ""
          )}
        >
          <div className="container mx-auto px-4 sm:px-6 py-4">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-3 min-w-0">
                <div className="flex h-10 w-10 items-center justify-center rounded-2xl border border-border/60 bg-card shadow-[var(--shadow-xs)]">
                  <NextImage src="/logo.svg" alt="Logo" width={22} height={22} className="w-[22px] h-[22px]" />
                </div>
                <div className="min-w-0">
                  <h1 className="text-lg sm:text-xl font-bold text-foreground truncate tracking-tight leading-tight">
                    代理规则集
                  </h1>
                  <div className="flex items-center gap-1.5">
                    <p className="text-[11px] text-muted-foreground hidden xs:block">
                      Proxy Rule Manager
                    </p>
                    {version && (
                      <span className="rounded-full border border-border/60 bg-surface-subtle px-1.5 py-0.5 font-mono text-[10px] font-medium text-muted-foreground leading-none tabular">
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
                      <div className="hidden cursor-default items-center gap-1.5 rounded-full border border-border/70 bg-surface-subtle px-3 py-1.5 text-[11px] font-medium text-muted-foreground tabular sm:flex">
                        <span className="relative flex h-1.5 w-1.5 shrink-0">
                          <span className="absolute inline-flex h-full w-full rounded-full bg-success/40 opacity-60 animate-ping" />
                          <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-success" />
                        </span>
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
                        <Icon icon="simple-icons:github" className="w-4 h-4" />
                      </a>
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="bottom" className="text-xs">GitHub</TooltipContent>
                </Tooltip>
                <Button
                  variant="secondary"
                  size="icon"
                  onClick={toggleMode}
                  aria-label={mode === "light" ? "切换到深色模式" : "切换到浅色模式"}
                >
                  {mode === "light" ? (
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
              <div className="inline-flex w-fit items-center gap-1 rounded-full bg-surface-subtle p-1" role="tablist" aria-label="内容类型">
                {MAIN_TABS.map((tab) => (
                <button
                  key={tab.key}
                  role="tab"
                  aria-selected={activeMainTab === tab.key}
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

              <SearchInput
                aria-label={activeMainTab === "rules" ? "搜索规则" : activeMainTab === "geosite" ? "搜索 Geosite" : activeMainTab === "configs" ? "搜索配置文件" : "搜索图标"}
                placeholder={activeMainTab === "rules" ? "搜索规则" : activeMainTab === "geosite" ? "搜索 Geosite" : activeMainTab === "configs" ? "搜索配置文件" : "搜索图标"}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
            </div>

            {/* Client Tabs */}
            {activeMainTab !== "icons" && (
              <div className="inline-flex max-w-full items-center gap-1 overflow-x-auto rounded-full bg-surface-subtle p-1 scrollbar-hide" role="tablist" aria-label="客户端选择">
                {clients.map((client) => {
                  const count = activeMainTab === "rules"
                    ? rules.filter(r => r.clients.includes(client.id)).length
                    : activeMainTab === "geosite"
                      ? geositeRules.filter(r => r.clients.includes(client.id)).length
                    : clientFiles.filter(f => f.clientId === client.id).length;
                  return (
                    <button
                      key={client.id}
                      role="tab"
                      aria-selected={activeClient === client.id}
                      onClick={() => setActiveClient(client.id)}
                      className={cn(
                        "shrink-0 whitespace-nowrap rounded-full px-4 py-2 text-sm font-semibold transition-all duration-150 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-primary/25",
                        activeClient === client.id
                          ? "bg-foreground text-background shadow-[var(--shadow-xs)]"
                          : "text-muted-foreground hover:text-foreground"
                      )}
                    >
                      {client.displayName}
                      <span className={cn(
                        "ml-1.5 text-xs",
                        activeClient === client.id ? "opacity-70" : "opacity-50"
                      )}>
                        {count}
                      </span>
                    </button>
                  );
                })}
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
                        ? "border-primary/30 bg-primary-soft text-primary"
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

            {/* Geosite Provider Filter */}
            {activeMainTab === "geosite" && geositeProviders.length > 1 && (
              <div className="flex items-center gap-2 overflow-x-auto pb-1 scrollbar-hide">
                <div className="flex shrink-0 items-center gap-1.5 text-sm text-muted-foreground" role="img" aria-label="来源筛选">
                  <Globe className="w-3.5 h-3.5" aria-hidden="true" />
                </div>
                <button
                  onClick={() => setSelectedGeositeProvider("all")}
                  className={cn(
                    "shrink-0 rounded-full border px-3 py-1.5 text-xs font-semibold transition-all duration-150 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-primary/25",
                    selectedGeositeProvider === "all"
                      ? "border-primary/30 bg-primary-soft text-primary"
                      : "border-border bg-surface-subtle text-muted-foreground hover:text-foreground"
                  )}
                >
                  全部
                </button>
                {geositeProviders.map((provider) => (
                  <button
                    key={provider}
                    onClick={() => setSelectedGeositeProvider(provider)}
                    className={cn(
                      "shrink-0 rounded-full border px-3 py-1.5 text-xs font-semibold transition-all duration-150 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-primary/25",
                      selectedGeositeProvider === provider
                        ? "border-primary/30 bg-primary-soft text-primary"
                        : "border-border bg-surface-subtle text-muted-foreground hover:text-foreground"
                    )}
                  >
                    {provider}
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Result Summary Bar */}
          {!isLoading && (
            <div className="flex items-center justify-between text-xs text-muted-foreground mb-2">
              <span>
                {activeMainTab === "rules" ? (
                  <>
                    {getClientDisplayName(activeClient)} · {clientRules.length} 条规则
                    {(searchQuery || selectedTags.length > 0) && ` (共 ${rules.filter(r => r.clients.includes(activeClient)).length} 条)`}
                  </>
                ) : activeMainTab === "geosite" ? (
                  <>{getClientDisplayName(activeClient)} · {filteredGeositeRules.length} 条 Geosite</>
                ) : activeMainTab === "configs" ? (
                  <>{getClientDisplayName(activeClient)} · {clientPublicFiles.length} 个配置</>
                ) : (
                  <>{filteredIcons.length} 个图标</>
                )}
              </span>
              {lastSyncAt && (
                <span className="flex items-center gap-1">
                  <Clock className="w-3 h-3" />
                  同步于 {formatRelativeTime(lastSyncAt)}
                </span>
              )}
            </div>
          )}

          {/* Content Grid */}
          {isLoading ? (
            <div className="flex items-center justify-center py-24">
              <div className="flex flex-col items-center gap-3 text-muted-foreground" aria-live="polite">
                <div className="flex items-center gap-1.5" aria-hidden="true">
                  <span className="block h-1.5 w-1.5 rounded-full bg-primary animate-[bounce_1s_ease-in-out_infinite] [animation-delay:-0.2s]" />
                  <span className="block h-1.5 w-1.5 rounded-full bg-primary animate-[bounce_1s_ease-in-out_infinite] [animation-delay:-0.1s]" />
                  <span className="block h-1.5 w-1.5 rounded-full bg-primary animate-[bounce_1s_ease-in-out_infinite]" />
                </div>
                <p className="text-xs font-medium">正在加载</p>
              </div>
            </div>
          ) : activeMainTab === "rules" ? (
            clientRules.length === 0 ? (
              <EmptyState
                icon={Globe}
                title={searchQuery || selectedTags.length > 0 ? "未找到匹配的规则" : "暂无规则"}
                description={searchQuery || selectedTags.length > 0 ? "尝试调整搜索条件或清除筛选标签" : "该客户端暂无可用规则"}
                className="py-24"
              />
            ) : (
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                {clientRules.map((rule, index) => (
                    <Card
                      key={rule.name}
                      className="group relative h-full p-5 animate-slide-up opacity-0 hover-lift"
                      style={{ animationDelay: `${index * 40}ms` }}
                    >
                    <div className="flex items-start gap-3">
                      <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-surface-strong shrink-0">
                        {rule.icon ? (
                          <RuleIcon icon={rule.icon} className="w-5 h-5" />
                        ) : (
                          <FileText className="w-[18px] h-[18px] text-muted-foreground" />
                        )}
                      </div>
                      <div className="min-w-0 flex-1">
                        <h3 className="text-sm font-semibold text-foreground truncate leading-tight">
                          {rule.displayName || rule.name}
                        </h3>
                        <p className="text-[11px] text-muted-foreground/60 font-mono truncate mt-0.5">{rule.name}</p>
                      </div>
                    </div>

                    <div className="min-h-5 mt-1.5 pl-[3.25rem]">
                      {rule.description && (
                        <Tooltip delayDuration={300}>
                          <TooltipTrigger asChild>
                            <p className="text-xs text-muted-foreground line-clamp-2 leading-relaxed cursor-default">
                              {rule.description}
                            </p>
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

                    <div className="min-h-5 mt-2">
                      {(rule.tags && rule.tags.length > 0) || rule.hasError || (rule.consecutiveFailures ?? 0) >= failureThreshold ? (
                        <div className="flex flex-wrap items-center gap-1.5">
                          {rule.tags?.slice(0, 4).map((tag) => (
                            <Badge
                              key={tag}
                              variant={getTagBadgeVariant(tag)}
                              className="text-[10px]"
                            >
                              {tag}
                            </Badge>
                          ))}
                          {rule.tags && rule.tags.length > 4 && (
                            <span className="text-[11px] text-muted-foreground">+{rule.tags.length - 4}</span>
                          )}
                          <FailureBadge
                            hasError={!!rule.hasError}
                            lastFailureAt={rule.lastFailureAt ?? null}
                            consecutiveFailures={rule.consecutiveFailures ?? 0}
                            threshold={failureThreshold}
                          />
                        </div>
                      ) : null}
                    </div>

                    <div className="flex-1 min-h-2" />

                    <div className="flex items-center gap-2 border-t border-border pt-4">
                      <Button
                        variant="secondary"
                        size="sm"
                        className="flex-1 text-xs"
                        onClick={() => handlePreview({
                          type: "rule",
                          name: rule.name,
                          clientId: activeClient,
                          fileName: rule.name,
                        })}
                      >
                        <Eye className="w-3.5 h-3.5" />
                        预览
                      </Button>
                      <Button
                        variant={copiedRule === rule.name ? "success" : "default"}
                        size="sm"
                        className="flex-1 text-xs"
                        onClick={() => {
                          void copyRuleUrl(rule.name);
                        }}
                      >
                        {copiedRule === rule.name ? (
                          <>
                            <CheckCircle className="w-3.5 h-3.5" />
                            已复制
                          </>
                        ) : (
                          <>
                            <Copy className="w-3.5 h-3.5" />
                            复制订阅
                          </>
                        )}
                      </Button>
                    </div>
                  </Card>
                ))}
              </div>
            )
          ) : activeMainTab === "geosite" ? (
            filteredGeositeRules.length === 0 ? (
              <EmptyState
                icon={Globe}
                title={searchQuery ? "未找到匹配的 Geosite" : "暂无 Geosite"}
                description={searchQuery ? "尝试使用其他关键词搜索" : "该客户端暂无可用 Geosite 规则"}
                className="py-24"
              />
            ) : (
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                {filteredGeositeRules.map((rule, index) => (
                    <Card
                      key={`${rule.provider}-${rule.outputName}`}
                      className="group relative h-full p-5 animate-slide-up opacity-0 hover-lift"
                      style={{ animationDelay: `${index * 40}ms` }}
                    >
                    <div className="flex items-start gap-3">
                      <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-surface-strong shrink-0">
                        <Globe className="w-[18px] h-[18px] text-muted-foreground" />
                      </div>
                      <div className="min-w-0 flex-1">
                        <h3 className="text-sm font-semibold text-foreground truncate leading-tight">
                          {rule.displayName || rule.list}
                        </h3>
                        <p className="text-[11px] text-muted-foreground/60 font-mono truncate mt-0.5">
                          {rule.provider}/{rule.outputName}
                        </p>
                      </div>
                    </div>

                    <div className="min-h-5 mt-1.5 pl-[3.25rem]">
                      {rule.description && (
                        <p className="text-xs text-muted-foreground line-clamp-2 leading-relaxed">
                          {rule.description}
                        </p>
                      )}
                    </div>

                    <div className="min-h-5 mt-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <Badge variant="outline">{rule.provider}</Badge>
                        {rule.attrs.map((attr) => (
                          <Badge key={`${rule.outputName}-${attr}`} variant="secondary">@{attr}</Badge>
                        ))}
                        <FailureBadge
                          hasError={!!rule.hasError}
                          lastFailureAt={rule.lastFailureAt ?? null}
                          consecutiveFailures={rule.consecutiveFailures ?? 0}
                          threshold={failureThreshold}
                        />
                      </div>
                    </div>

                    <div className="flex-1 min-h-2" />

                    <div className="flex items-center gap-2 border-t border-border pt-4">
                      <Button
                        variant="secondary"
                        size="sm"
                        className="flex-1 text-xs"
                        onClick={() => handlePreview({
                          type: "geosite",
                          name: rule.outputName,
                          provider: rule.provider,
                          clientId: activeClient,
                          fileName: `${rule.provider}/${rule.outputName}`,
                        })}
                      >
                        <Eye className="w-3.5 h-3.5" />
                        预览
                      </Button>
                      <Button
                        variant={copiedRule === `${rule.provider}/${rule.outputName}` ? "success" : "default"}
                        size="sm"
                        className="flex-1 text-xs"
                        onClick={() => {
                          void copyGeositeUrl(rule.provider, rule.outputName);
                        }}
                      >
                        {copiedRule === `${rule.provider}/${rule.outputName}` ? (
                          <>
                            <CheckCircle className="w-3.5 h-3.5" />
                            已复制
                          </>
                        ) : (
                          <>
                            <Copy className="w-3.5 h-3.5" />
                            复制订阅
                          </>
                        )}
                      </Button>
                    </div>
                  </Card>
                ))}
              </div>
            )
          ) : activeMainTab === "configs" ? (
            clientPublicFiles.length === 0 ? (
              <EmptyState
                icon={FileText}
                title={searchQuery ? "未找到匹配的配置文件" : "暂无公开配置文件"}
                description={searchQuery ? "尝试使用其他关键词搜索" : "该客户端暂无公开的配置文件"}
                className="py-24"
              />
            ) : (
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                {clientPublicFiles.map((file, index) => {
                  return (
                    <Card
                      key={file.id}
                      className="h-full p-5 animate-slide-up opacity-0 hover-lift"
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
                            /client/{file.clientId}/{file.configId}.{file.ext}
                          </p>
                        </div>
                      </div>
                      <div className="min-h-5 mt-1.5 pl-[3.25rem]">
                        {file.description && (
                          <p className="text-xs text-muted-foreground line-clamp-2 leading-relaxed">
                            {file.description}
                          </p>
                        )}
                      </div>
                      <div className="flex-1 min-h-2" />
                      <div className="flex items-center gap-2 border-t border-border pt-4">
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
                            href={getConfigUrl(file.clientId, file.configId, file.ext)}
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
              <EmptyState
                icon={ImageIcon}
                title={searchQuery ? "未找到匹配的图标" : "暂无图标"}
                description={searchQuery ? "尝试使用其他关键词搜索" : "图标集暂无图标"}
                className="py-24"
              />
            ) : (
              <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8 gap-5">
                {filteredIcons.map((icon, index) => (
                  <Card
                    key={icon.id}
                    className="group p-3 animate-slide-up opacity-0 hover-lift"
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

          <div className="h-12" />
        </main>

        {/* Preview Dialog */}
        <Dialog open={!!previewItem && !isPreviewFullscreen} onOpenChange={(open) => !open && closePreview()}>
          <DialogContent className="max-w-5xl w-[90vw] h-[80vh] flex flex-col p-0 overflow-hidden">
            <DialogHeader className="px-6 pt-6 pb-4 border-b border-border">
              <DialogTitle className="flex items-center gap-3 text-foreground font-semibold">
                {(() => {
                  const rule = rules.find(r => r.name === previewItem?.name);
                  return rule?.icon ? (
                    <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-surface-strong">
                      <RuleIcon icon={rule.icon} className="w-4 h-4" />
                    </div>
                  ) : (
                    <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-surface-strong">
                      <FileText className="w-4 h-4 text-muted-foreground" />
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
                  <CodeViewer content={previewContent} className="flex-1 min-h-0" height="100%" />
                </>
              )}
            </div>
          </DialogContent>
        </Dialog>
      </div>
    </TooltipProvider>
  );
}

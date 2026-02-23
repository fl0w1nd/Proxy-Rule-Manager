"use client";

import { useState, useEffect, useMemo } from "react";
import NextImage from "next/image";
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
  ExternalLink,
  CheckCircle,
  Maximize2,
  X,
  Tag,
  Image as ImageIcon,
  Sparkles,
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
  const [copiedConfig, setCopiedConfig] = useState<string | null>(null);
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
    return `${window.location.origin}/Rules/${clientPath}/${ruleName}.list`;
  };

  const getConfigUrl = (name: string, ext: string) => {
    return `${window.location.origin}/client/${name}.${ext}`;
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

  const copyConfigUrl = async (file: ClientFileMeta) => {
    const url = getConfigUrl(file.configId, file.ext);
    await copyToClipboard(url, () => {
      setCopiedConfig(file.id);
      setTimeout(() => setCopiedConfig(null), 2000);
    });
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

  const filteredIcons = useMemo(() => {
    if (!searchQuery) return icons;
    const query = searchQuery.toLowerCase();
    return icons.filter((icon) => icon.name.toLowerCase().includes(query));
  }, [icons, searchQuery]
  );

  // 标签颜色循环 - 柔和的多彩色系
  const tagColorClasses = [
    "neu-badge-blue",
    "neu-badge-rose",
    "neu-badge-amber",
    "neu-badge-violet",
    "neu-badge-teal",
    "neu-badge-emerald",
  ];

  // 基于标签名生成稳定的颜色索引
  const getTagColorClass = (tag: string) => {
    let hash = 0;
    for (let i = 0; i < tag.length; i++) {
      hash = tag.charCodeAt(i) + ((hash << 5) - hash);
    }
    return tagColorClasses[Math.abs(hash) % tagColorClasses.length];
  };

  const closePreview = () => {
    setPreviewItem(null);
    setIsPreviewFullscreen(false);
  };

  // 全屏预览模式
  if (isPreviewFullscreen && previewItem) {
    return (
      <div className="fixed inset-0 z-50 bg-background flex flex-col">
        {/* 顶部工具栏 */}
        <div className="flex items-center justify-between px-6 py-4 glass-header">
          <div className="flex items-center gap-3">
            {(() => {
              const rule = rules.find(r => r.name === previewItem.name);
              return rule?.icon ? (
                <div className="neu-icon">
                      <RuleIcon icon={rule.icon} className="w-5 h-5 text-primary/60" />
                </div>
              ) : (
                <div className="neu-icon">
                  <FileText className="w-5 h-5 text-primary/60" />
                </div>
              );
            })()}
            <span className="font-semibold text-foreground">{previewItem.fileName}</span>
            <span className="neu-badge neu-badge-active">
              {getClientConfig(previewItem.clientId)?.displayName || previewItem.clientId}
            </span>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-sm text-muted-foreground">
              {previewContent.split('\n').length} 行
            </span>
            <button
              className="neu-btn px-4 py-2 text-sm text-muted-foreground flex items-center gap-1.5"
              onClick={() => {
                void copyToClipboard(previewContent, () => undefined, "已复制内容");
              }}
            >
              <Copy className="w-4 h-4" />
              复制
            </button>
            <button
              className="neu-btn w-10 h-10 flex items-center justify-center text-muted-foreground"
              onClick={() => setIsPreviewFullscreen(false)}
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* 内容 - 带行号 */}
        <div className="flex-1 overflow-auto m-4">
          <div className="neu-inset p-0 overflow-hidden">
            {previewLoading ? (
              <div className="flex items-center justify-center h-64">
                <Loader2 className="w-8 h-8 animate-spin text-primary/50" />
              </div>
            ) : (
              <div className="flex text-sm font-mono min-w-max">
                {/* 行号 */}
                <div className="py-4 pl-4 pr-3 text-right text-muted-foreground select-none sticky left-0 bg-muted/20">
                  {previewContent.split('\n').map((_, i) => (
                    <div key={i}>{i + 1}</div>
                  ))}
                </div>
                {/* 内容 */}
                <pre className="py-4 px-4 text-foreground whitespace-pre">
                  {previewContent || "暂无内容"}
                </pre>
              </div>
            )}
          </div>
        </div>
      </div>
    );
  }

  return (
    <TooltipProvider>
      <div className="neu-surface transition-colors">
        <AmbientBackground />
        {/* Header */}
        <header className={`sticky top-0 z-50 glass-header${isScrolled ? " scrolled" : ""}`}>
          <div className="container mx-auto px-4 sm:px-6 py-4">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-3 min-w-0">
                <div className="neu-icon !w-11 !h-11 !rounded-[16px]">
                  <NextImage src="/logo.svg" alt="Logo" width={24} height={24} className="w-6 h-6" />
                </div>
                <div className="min-w-0">
                  <h1 className="text-lg sm:text-xl font-bold text-foreground truncate tracking-tight">
                    代理规则集
                  </h1>
                  <div className="flex items-center gap-2">
                    <p className="text-xs text-muted-foreground hidden xs:block">
                      Proxy Rule Manager
                    </p>
                    {version && (
                      <span className="neu-badge !text-[10px] !px-2 !py-0 font-mono leading-relaxed">
                        v{version}
                      </span>
                    )}
                  </div>
                </div>
              </div>
              <div className="flex items-center gap-2 flex-shrink-0">
                <button
                  onClick={toggleTheme}
                  className="neu-btn w-11 h-11 flex items-center justify-center text-muted-foreground !rounded-[14px]"
                >
                  {theme === "light" ? (
                    <Moon className="w-[18px] h-[18px]" />
                  ) : (
                    <Sun className="w-[18px] h-[18px]" />
                  )}
                </button>
                <button
                  onClick={onAdminClick}
                  className="neu-btn h-11 px-4 flex items-center gap-2 text-muted-foreground text-sm !rounded-[14px]"
                >
                  <Settings className="w-4 h-4" />
                  <span className="hidden sm:inline">管理</span>
                </button>
              </div>
            </div>
          </div>
        </header>

        {/* Main Content */}
        <main className="container mx-auto px-4 sm:px-6 py-8">
          {/* Main Tabs & Search */}
          <div className="mb-8 space-y-5">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
              {/* Neumorphic Pill Tabs */}
              <div className="neu-pill flex items-center gap-1">
                {(
                  [
                    { key: "rules", label: "规则" },
                    { key: "configs", label: "配置文件" },
                    { key: "icons", label: "图标集" },
                  ] as const
                ).map((tab) => (
                  <button
                    key={tab.key}
                    onClick={() => setActiveMainTab(tab.key)}
                    className={`px-5 py-2.5 text-sm font-medium transition-all duration-200 ${
                      activeMainTab === tab.key
                        ? "neu-pill-active"
                        : "rounded-[50px] text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {tab.label}
                  </button>
                ))}
              </div>

              {/* Search */}
              <div className="neu-search flex items-center gap-2 px-4 py-2.5 w-full sm:w-auto sm:min-w-[280px]">
                <Search className="w-4 h-4 text-muted-foreground flex-shrink-0" />
                <input
                  type="text"
                  placeholder={activeMainTab === "rules" ? "搜索规则..." : activeMainTab === "configs" ? "搜索配置文件..." : "搜索图标..."}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="bg-transparent border-none outline-none text-sm text-foreground placeholder:text-muted-foreground w-full"
                />
              </div>
            </div>

            {/* Client Tabs */}
            {activeMainTab !== "icons" && (
              <div className="neu-pill flex items-center gap-1 overflow-x-auto scrollbar-hide w-fit max-w-full">
                {clients.map((client) => (
                  <button
                    key={client.id}
                    onClick={() => setActiveClient(client.id)}
                    className={`px-4 py-2 text-sm font-medium transition-all duration-200 flex-shrink-0 whitespace-nowrap ${
                      activeClient === client.id
                        ? "neu-pill-active"
                        : "rounded-[50px] text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {client.displayName}
                  </button>
                ))}
              </div>
            )}

            {/* Tag Filter */}
            {activeMainTab === "rules" && allTags.length > 0 && (
              <div className="flex items-center gap-2 overflow-x-auto pb-1 scrollbar-hide">
                <div className="flex items-center gap-1.5 text-sm text-muted-foreground flex-shrink-0">
                  <Tag className="w-3.5 h-3.5" />
                </div>
                {allTags.map((tag) => (
                  <button
                    key={tag}
                    onClick={() => toggleTag(tag)}
                    className={`flex-shrink-0 transition-all duration-200 ${
                      selectedTags.includes(tag)
                        ? "neu-badge neu-badge-active"
                        : "neu-badge hover:text-foreground"
                    }`}
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
              <div className="neu-raised p-8 flex flex-col items-center gap-4">
                <Loader2 className="w-8 h-8 animate-spin text-primary/50" />
                <p className="text-sm text-muted-foreground">加载中...</p>
              </div>
            </div>
          ) : activeMainTab === "rules" ? (
            clientRules.length === 0 ? (
              <div className="text-center py-24">
                <div className="neu-raised w-24 h-24 mx-auto mb-6 flex items-center justify-center !rounded-[28px]">
                  <Globe className="w-10 h-10 text-primary/40" />
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
              <div className="neu-grid">
                {clientRules.map((rule, index) => (
                  <Card
                    key={rule.name}
                    className="group relative p-5 animate-slide-up opacity-0"
                    style={{ animationDelay: `${index * 40}ms` }}
                  >
                    {/* Floating action buttons */}
                    <div className="absolute top-4 right-4 glass-float flex items-center gap-1">
                      <Tooltip delayDuration={100}>
                        <TooltipTrigger asChild>
                          <button
                            className="w-8 h-8 flex items-center justify-center rounded-lg text-muted-foreground hover:text-foreground transition-colors"
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
                            className={`w-8 h-8 flex items-center justify-center rounded-lg transition-all duration-200 ${
                              copiedRule === rule.name
                                ? "bg-green-400/20 text-green-600 dark:text-green-400"
                                : "text-muted-foreground hover:text-foreground"
                            }`}
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

                    <div className="flex items-start gap-3.5">
                      <div className="neu-icon">
                        {rule.icon ? (
                          <RuleIcon icon={rule.icon} className="w-5 h-5 text-primary/60" />
                        ) : (
                          <FileText className="w-[18px] h-[18px] text-primary/60" />
                        )}
                      </div>
                      <div className="min-w-0 flex-1 pr-16">
                        <h3 className="text-[15px] font-semibold text-foreground truncate leading-tight">
                          {rule.displayName || rule.name}
                        </h3>
                        {rule.description && (
                          <Tooltip delayDuration={300}>
                            <TooltipTrigger asChild>
                              <div className="mt-2 cursor-default">
                                <p className="text-[13px] text-muted-foreground line-clamp-2 leading-relaxed">
                                  {rule.description}
                                </p>
                              </div>
                            </TooltipTrigger>
                            <TooltipContent
                              side="bottom"
                              align="start"
                              showArrow={false}
                              className="max-w-[300px] bg-background text-foreground border border-border shadow-lg rounded-xl"
                            >
                              <p className="text-[13px] whitespace-pre-wrap break-words leading-relaxed">
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
                          <span
                            key={tag}
                            className={`neu-badge ${getTagColorClass(tag)}`}
                          >
                            {tag}
                          </span>
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
                <div className="neu-raised w-24 h-24 mx-auto mb-6 flex items-center justify-center !rounded-[28px]">
                  <FileText className="w-10 h-10 text-primary/40" />
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
              <div className="neu-grid">
                {clientPublicFiles.map((file, index) => {
                  return (
                    <Card
                      key={file.id}
                      className="p-5 animate-slide-up opacity-0"
                      style={{ animationDelay: `${index * 40}ms` }}
                    >
                      <div className="flex items-start gap-3.5">
                        <div className="neu-icon-accent">
                          <FileText className="w-[18px] h-[18px] text-green-600/70 dark:text-green-400/70" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <h3 className="text-[15px] font-semibold text-foreground truncate leading-tight">
                            {file.displayName || `${file.configId}.${file.ext}`}
                          </h3>
                          <p className="text-[11px] text-muted-foreground mt-0.5 font-mono truncate">
                            {file.configId}.{file.ext}
                          </p>
                          {file.description && (
                            <p className="text-[13px] text-muted-foreground mt-2 line-clamp-2 leading-relaxed">
                              {file.description}
                            </p>
                          )}
                        </div>
                      </div>
                      <div className="flex gap-3 mt-5">
                        <button
                          className="neu-btn flex-1 h-10 text-[13px] text-muted-foreground flex items-center justify-center gap-1.5"
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
                        </button>
                        <button
                          className={`flex-1 h-10 text-[13px] font-medium flex items-center justify-center gap-1.5 rounded-[14px] transition-all duration-200 ${
                            copiedConfig === file.id
                              ? "bg-green-400/20 text-green-600 dark:text-green-400 shadow-[inset_2px_2px_5px_rgba(0,0,0,0.04),inset_-2px_-2px_5px_rgba(255,255,255,0.4)]"
                              : "neu-pill-active"
                          }`}
                          onClick={() => {
                            void copyConfigUrl(file);
                          }}
                        >
                          {copiedConfig === file.id ? (
                            <>
                              <CheckCircle className="w-3.5 h-3.5" />
                              已复制
                            </>
                          ) : (
                            <>
                              <Copy className="w-3.5 h-3.5" />
                              复制链接
                            </>
                          )}
                        </button>
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
                <div className="neu-raised w-24 h-24 mx-auto mb-6 flex items-center justify-center !rounded-[28px]">
                  <ImageIcon className="w-10 h-10 text-primary/40" />
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
                    <div className="relative aspect-square w-full flex items-center justify-center rounded-xl overflow-hidden mb-2 neu-inset p-2">
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
                    <button
                      className={`w-full mt-2 h-7 text-[11px] font-medium rounded-lg flex items-center justify-center gap-1 transition-all duration-200 ${
                        copiedIcon === icon.id
                          ? "bg-green-400/20 text-green-600 dark:text-green-400"
                          : "neu-pill-active !rounded-lg"
                      }`}
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
                    </button>
                  </Card>
                ))}
              </div>
            )
          )
          }

          {/* Stats */}
          <div className="mt-10 flex justify-center">
            <div className="neu-stats px-6 py-2.5 text-center text-sm text-muted-foreground/80 space-y-0.5">
              {activeMainTab === "rules" ? (
                <>
                  <p>共 {clientRules.length} 条规则</p>
                  {lastSyncAt && (
                    <p className="text-xs opacity-70">上次更新: {new Date(lastSyncAt).toLocaleString("zh-CN")}</p>
                  )}
                </>
              ) : activeMainTab === "configs" ? (
                <p>共 {clientPublicFiles.length} 个配置文件</p>
              ) : (
                <p>共 {filteredIcons.length} 个图标</p>
              )}
            </div>
          </div>
        </main>

        {/* Footer */}
        <footer className="mt-12 pb-8">
          <div className="container mx-auto px-4 sm:px-6 text-center">
            <div className="neu-footer inline-flex flex-col items-center gap-1.5 px-8 py-4">
              <div className="flex items-center gap-2 text-sm text-amber-800/50 dark:text-amber-200/40">
                <Sparkles className="w-3.5 h-3.5" />
                <span>Proxy Rule Manager</span>
              </div>
              <a
                href="https://github.com/Fl0w1nd/Proxy-Rule-Manager"
                target="_blank"
                rel="noopener noreferrer"
                className="text-amber-700/40 dark:text-amber-300/30 hover:text-amber-700/60 dark:hover:text-amber-300/50 inline-flex items-center gap-1 text-xs transition-colors"
              >
                <ExternalLink className="w-3 h-3" />
                GitHub
              </a>
            </div>
          </div>
        </footer>

        {/* Preview Dialog */}
        <Dialog open={!!previewItem && !isPreviewFullscreen} onOpenChange={(open) => !open && closePreview()}>
          <DialogContent className="max-w-5xl w-[90vw] h-[80vh] flex flex-col p-0 !rounded-2xl overflow-hidden border-none bg-[#e7ebf8] dark:bg-[#191d2b]">
            <DialogHeader className="px-6 pt-6 pb-4 glass-header !border-b-0">
              <DialogTitle className="flex items-center gap-3 text-foreground font-semibold">
                {(() => {
                  const rule = rules.find(r => r.name === previewItem?.name);
                  return rule?.icon ? (
                    <div className="neu-icon !w-9 !h-9 !rounded-[10px]">
                      <RuleIcon icon={rule.icon} className="w-4 h-4 text-primary/60" />
                    </div>
                  ) : (
                    <div className="neu-icon !w-9 !h-9 !rounded-[10px]">
                      <FileText className="w-4 h-4 text-primary/60" />
                    </div>
                  );
                })()}
                {previewItem?.fileName}
                <span className="neu-badge neu-badge-active ml-1">
                  {previewItem ? getClientDisplayName(previewItem.clientId) : getClientDisplayName(activeClient)}
                </span>
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
                      <button
                        className="neu-btn px-3 py-1.5 text-xs text-muted-foreground flex items-center gap-1.5 !rounded-lg"
                        onClick={() => {
                          void copyToClipboard(previewContent, () => undefined, "已复制内容");
                        }}
                      >
                        <Copy className="w-3.5 h-3.5" />
                        复制
                      </button>
                      <button
                        className="neu-btn px-3 py-1.5 text-xs text-muted-foreground flex items-center gap-1.5 !rounded-lg"
                        onClick={() => setIsPreviewFullscreen(true)}
                      >
                        <Maximize2 className="w-3.5 h-3.5" />
                        全屏
                      </button>
                    </div>
                  </div>
                  {/* 内容区域 - 带行号 */}
                  <div className="flex-1 overflow-auto neu-inset">
                    <div className="flex text-sm font-mono min-w-max">
                      {/* 行号 */}
                      <div className="py-4 pl-4 pr-3 text-right text-muted-foreground select-none sticky left-0 bg-muted/20">
                        {previewContent.split('\n').map((_, i) => (
                          <div key={i}>{i + 1}</div>
                        ))}
                      </div>
                      {/* 内容 */}
                      <pre className="py-4 px-4 text-foreground whitespace-pre">
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

"use client";

import { useState, useEffect } from "react";
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
} from "lucide-react";
import { useTheme } from "./theme-provider";
import { toast } from "sonner";

interface RuleInfo {
  name: string;
  description?: string;
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
  const [lastSyncAt, setLastSyncAt] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [activeClient, setActiveClient] = useState<string>("");
  const [previewRule, setPreviewRule] = useState<string | null>(null);
  const [previewContent, setPreviewContent] = useState<string>("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [copiedRule, setCopiedRule] = useState<string | null>(null);
  const [isPreviewFullscreen, setIsPreviewFullscreen] = useState(false);

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
      const response = await fetch("/api/status");
      if (response.ok) {
        const data = await response.json();
        setRules(data.rules || []);
        setLastSyncAt(data.lastSyncAt || null);
        // 从 status 响应中获取客户端列表
        if (data.clients && data.clients.length > 0) {
          setClients(data.clients);
          // 设置默认激活的客户端（始终设置为第一个，避免闭包问题）
          setActiveClient((prev) => prev || data.clients[0].id);
        }
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

  const copyRuleUrl = (ruleName: string) => {
    const url = getRuleUrl(ruleName, activeClient);
    navigator.clipboard.writeText(url);
    setCopiedRule(ruleName);
    setTimeout(() => setCopiedRule(null), 2000);
  };

  const handlePreview = async (ruleName: string) => {
    setPreviewRule(ruleName);
    setPreviewLoading(true);
    setPreviewContent("");

    try {
      const client = getClientConfig(activeClient);
      const clientPath = client?.pathName || activeClient;
      const response = await fetch(`/Rules/${clientPath}/${ruleName}.list`);
      if (response.ok) {
        const text = await response.text();
        setPreviewContent(text);
      } else {
        setPreviewContent("# 规则文件暂不可用\n# 请先执行同步操作");
      }
    } catch (error) {
      setPreviewContent("# 加载失败: " + String(error));
    } finally {
      setPreviewLoading(false);
    }
  };

  const filteredRules = rules.filter(
    (rule) =>
      rule.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      rule.description?.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const clientRules = filteredRules.filter((rule) =>
    rule.clients.includes(activeClient)
  );

  const closePreview = () => {
    setPreviewRule(null);
    setIsPreviewFullscreen(false);
  };

  const currentClient = getClientConfig(activeClient);

  // 全屏预览模式
  if (isPreviewFullscreen && previewRule) {
    return (
      <div className="fixed inset-0 z-50 bg-white dark:bg-slate-900 flex flex-col">
        {/* 顶部工具栏 */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-slate-700 bg-gray-50 dark:bg-slate-800">
          <div className="flex items-center gap-3">
            <FileText className="w-5 h-5 text-blue-500" />
            <span className="font-semibold text-gray-900 dark:text-white">{previewRule}</span>
            <Badge className="bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
              {currentClient?.displayName || activeClient}
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
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-gray-100 dark:from-slate-900 dark:via-slate-800 dark:to-slate-900 transition-colors">
      {/* Header */}
      <header className="sticky top-0 z-50 border-b bg-white/80 dark:bg-slate-900/80 backdrop-blur-sm">
        <div className="container mx-auto px-4 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 bg-gradient-to-br from-blue-500 to-cyan-500 rounded-xl flex items-center justify-center">
                <Globe className="w-5 h-5 text-white" />
              </div>
              <div>
                <h1 className="text-xl font-bold text-gray-900 dark:text-white">
                  代理规则集
                </h1>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  Proxy Rule Manager
                </p>
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
                onClick={onAdminClick}
                className="text-gray-600 dark:text-gray-300"
              >
                <Settings className="w-4 h-4 mr-1" />
                管理
              </Button>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="container mx-auto px-4 py-8">
        {/* Client Tabs & Search */}
        <div className="mb-6 space-y-4">
          <Tabs
            value={activeClient}
            onValueChange={(v) => setActiveClient(v)}
          >
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
              <TabsList className="bg-white dark:bg-slate-800 border shadow-sm">
                {clients.map((client) => (
                  <TabsTrigger
                    key={client.id}
                    value={client.id}
                    className="data-[state=active]:bg-blue-500 data-[state=active]:text-white"
                  >
                    {client.displayName}
                  </TabsTrigger>
                ))}
              </TabsList>
              <div className="relative w-full sm:w-72">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
                <Input
                  placeholder="搜索规则..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-10 bg-white dark:bg-slate-800"
                />
              </div>
            </div>
          </Tabs>
        </div>

        {/* Rules Grid */}
        {isLoading ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
          </div>
        ) : clientRules.length === 0 ? (
          <div className="text-center py-20 text-gray-500 dark:text-gray-400">
            {searchQuery ? "未找到匹配的规则" : "暂无规则，请先在管理后台添加"}
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
            {clientRules.map((rule) => (
              <Card
                key={rule.name}
                className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700 hover:shadow-lg transition-shadow"
              >
                <CardHeader className="pb-2">
                  <div className="flex items-start gap-3">
                    <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-blue-100 to-cyan-100 dark:from-blue-900/30 dark:to-cyan-900/30 flex items-center justify-center flex-shrink-0">
                      <FileText className="w-5 h-5 text-blue-600 dark:text-blue-400" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <CardTitle className="text-base text-gray-900 dark:text-white truncate">
                        {rule.name}
                      </CardTitle>
                      <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 line-clamp-2">
                        {rule.description || "无描述"}
                      </p>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="pt-2">
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      className="flex-1"
                      onClick={() => handlePreview(rule.name)}
                    >
                      <Eye className="w-4 h-4 mr-1" />
                      预览
                    </Button>
                    <Button
                      size="sm"
                      className={`flex-1 transition-colors ${copiedRule === rule.name
                        ? "bg-green-500 hover:bg-green-600"
                        : "bg-blue-500 hover:bg-blue-600"
                        }`}
                      onClick={() => copyRuleUrl(rule.name)}
                    >
                      {copiedRule === rule.name ? (
                        <>
                          <CheckCircle className="w-4 h-4 mr-1" />
                          已复制
                        </>
                      ) : (
                        <>
                          <Copy className="w-4 h-4 mr-1" />
                          复制
                        </>
                      )}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}

        {/* Stats */}
        <div className="mt-8 text-center text-sm text-gray-500 dark:text-gray-400 space-y-1">
          <p>共 {clientRules.length} 条规则</p>
          {lastSyncAt && (
            <p>上次更新: {new Date(lastSyncAt).toLocaleString("zh-CN")}</p>
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
      <Dialog open={!!previewRule && !isPreviewFullscreen} onOpenChange={(open) => !open && closePreview()}>
        <DialogContent className="max-w-5xl w-[90vw] h-[80vh] bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700 flex flex-col p-0">
          <DialogHeader className="px-6 pt-6 pb-4 border-b border-gray-200 dark:border-slate-700">
            <DialogTitle className="flex items-center gap-2 text-gray-900 dark:text-white">
              <FileText className="w-5 h-5 text-blue-500" />
              {previewRule}
              <Badge className="bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400 ml-2">
                {currentClient?.displayName || activeClient}
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
  );
}

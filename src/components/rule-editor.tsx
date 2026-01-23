"use client";

import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
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
  Trash2,
  Loader2,
  GripVertical,
  ChevronDown,
  ChevronRight,
  HelpCircle,
  Copy,
  Link2,
  FileText,
  FolderInput,
  Edit3,
  Code2,
  Replace,
  Eraser,
  Settings2,
  Monitor,
  Eye,
  CheckCircle,
  XCircle,
} from "lucide-react";
import {
  RuleConfig,
  RulesConfig,
  ClientType,
  SourceConfig,
  SourceType,
  Transform,
  MergeStrategy,
  ScriptTransformer,
} from "@/lib/schema";
import { saveConfig, renameRule, previewRule, PreviewResponse, getClients, ClientConfig } from "@/lib/api-client";
import { toast } from "sonner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

interface RuleEditorProps {
  rule: RuleConfig | null;
  config: RulesConfig | null;
  onSave: () => void;
  onCancel: () => void;
}

// 帮助文本
const HELP_TEXTS = {
  ruleName: "规则的唯一标识符，用于生成文件路径。例如：YouTube 会生成 /Rules/Clash Meta/YouTube.list",
  description: "对规则的简短描述，方便管理和查找",
  sources: "添加数据来源，支持三种类型：URL（远程规则）、引用（已有规则）、本地（自定义内容）",
  sourceUrl: "从远程 URL 获取规则内容",
  sourceRef: "引用项目中已存在的其他规则",
  sourceLocal: "直接编写本地规则内容",
  transforms: "对来源数据进行处理，可指定处理特定来源或全部",
  transformTarget: "选择要处理的数据来源，可以是特定来源或全部",
  merge: "多个数据来源的合并方式",
  outputClients: "选择要生成规则文件的代理客户端",
  clientOverrides: "为特定客户端添加额外的转换操作",
  useTransformer: "使用预定义的 JS 脚本转换器处理内容",
  replace: "使用正则表达式替换匹配的内容",
  removeLines: "删除匹配正则表达式的行",
};

// 来源类型图标
const SOURCE_TYPE_ICONS = {
  url: Link2,
  ref: FolderInput,
  local: FileText,
};

// 转换旧格式到新格式
function migrateRule(rule: RuleConfig): RuleConfig {
  const newRule = { ...rule };

  // 迁移 compose_from 到 sources
  if (rule.compose_from && rule.compose_from.length > 0 && (!rule.sources || rule.sources.length === 0)) {
    newRule.sources = rule.compose_from.map((ref) => ({
      type: "ref" as SourceType,
      ref,
    }));
    newRule.compose_from = undefined;
  }

  // 确保 sources 中有 type 字段
  if (newRule.sources) {
    newRule.sources = newRule.sources.map((s) => ({
      ...s,
      type: s.type || (s.url ? "url" : s.ref ? "ref" : "local"),
    }));
  }

  return newRule;
}

// 默认规则模板（客户端列表将在组件加载时动态设置）
const DEFAULT_RULE: RuleConfig = {
  name: "",
  displayName: "",
  description: "",
  sources: [],
  transforms: [],
  output: {
    clients: [], // 将在 useEffect 中动态设置
  },
};

// 帮助图标组件
function HelpIcon({ text }: { text: string }) {
  return (
    <TooltipProvider delayDuration={200}>
      <Tooltip>
        <TooltipTrigger asChild>
          <HelpCircle className="w-4 h-4 text-gray-400 hover:text-blue-500 cursor-help inline-flex" />
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-xs">
          <p className="text-sm">{text}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

// 区块标题组件
function SectionHeader({
  title,
  help,
  expanded,
  onToggle,
  badge,
  actions,
}: {
  title: string;
  help?: string;
  expanded: boolean;
  onToggle: () => void;
  badge?: React.ReactNode;
  actions?: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between py-2 px-3 bg-muted/30 rounded-t-lg border-b hover:bg-muted/50 transition-colors">
      <button
        type="button"
        onClick={onToggle}
        className="flex items-center gap-2 flex-1 min-w-0"
      >
        <span className="flex-shrink-0 text-muted-foreground">
          {expanded ? (
            <ChevronDown className="w-4 h-4" />
          ) : (
            <ChevronRight className="w-4 h-4" />
          )}
        </span>
        <span className="font-medium text-sm text-foreground truncate">{title}</span>
        {help && <HelpIcon text={help} />}
        {badge}
      </button>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  );
}

export function RuleEditor({ rule, config, onSave, onCancel }: RuleEditorProps) {
  const [formData, setFormData] = useState<RuleConfig>(() =>
    rule ? migrateRule(rule) : DEFAULT_RULE
  );
  const [isSaving, setIsSaving] = useState(false);
  const [expandedSections, setExpandedSections] = useState<Set<string>>(
    new Set(["basic", "sources", "transforms", "merge", "output"])
  );
  const [draggedIndex, setDraggedIndex] = useState<number | null>(null);
  const [editingLocalContent, setEditingLocalContent] = useState<number | null>(null);
  const [localContentDraft, setLocalContentDraft] = useState("");

  // 预览相关状态
  const [isPreviewOpen, setIsPreviewOpen] = useState(false);
  const [isPreviewLoading, setIsPreviewLoading] = useState(false);
  const [previewData, setPreviewData] = useState<PreviewResponse | null>(null);
  const [previewClient, setPreviewClient] = useState<ClientType>("clash_meta");

  // 动态客户端列表
  const [clientsList, setClientsList] = useState<ClientConfig[]>([]);

  const fetchLatestClients = async (): Promise<ClientConfig[]> => {
    try {
      const { clients } = await getClients();
      setClientsList(clients);
      return clients;
    } catch (err) {
      console.error("Failed to load clients:", err);
      return clientsList;
    }
  };

  // 加载客户端列表
  useEffect(() => {
    getClients()
      .then(({ clients }) => {
        setClientsList(clients);
        if (!rule && formData.output.clients.length === 0 && clients.length > 0) {
          setFormData((prev) => ({
            ...prev,
            output: {
              ...prev.output,
              clients: clients.map((c) => c.id),
            },
          }));
        }
      })
      .catch((err) => {
        console.error("Failed to load clients:", err);
      });
  }, [rule, formData.output.clients.length]);

  const toggleSection = (section: string) => {
    setExpandedSections((prev) => {
      const next = new Set(prev);
      if (next.has(section)) {
        next.delete(section);
      } else {
        next.add(section);
      }
      return next;
    });
  };

  // 保存规则
  const handleSave = async () => {
    if (!formData.name.trim()) {
      toast.error("规则 ID 不能为空");
      return;
    }
    if (formData.output.clients.length === 0) {
      toast.error("请至少选择一个客户端");
      return;
    }

    setIsSaving(true);
    try {
      const oldName = rule?.name;
      const newName = formData.name.trim();
      const nameChanged = oldName && oldName !== newName;

      const latestClients = await fetchLatestClients();
      const validClientIds =
        latestClients.length > 0 ? new Set(latestClients.map((c) => c.id)) : null;

      // 清理空的来源
      const cleanedData = {
        ...formData,
        name: newName,
        output: {
          ...formData.output,
          clients: validClientIds
            ? formData.output.clients.filter((id) => validClientIds.has(id))
            : formData.output.clients,
          client_overrides: validClientIds && formData.output.client_overrides
            ? Object.fromEntries(
              Object.entries(formData.output.client_overrides).filter(([id]) =>
                validClientIds.has(id)
              )
            )
            : formData.output.client_overrides,
        },
        sources: formData.sources?.filter((s) =>
          (s.type === "url" && s.url) ||
          (s.type === "ref" && s.ref) ||
          (s.type === "local" && s.content)
        ),
      };

      if (validClientIds) {
        const removedClients = formData.output.clients.filter((id) => !validClientIds.has(id));
        if (removedClients.length > 0) {
          toast.warning(`客户端列表已更新，已移除无效客户端: ${removedClients.join(", ")}`);
        }
        if (cleanedData.output.clients.length === 0) {
          toast.error("客户端列表已变更，请重新选择输出客户端");
          setIsSaving(false);
          return;
        }
      }

      // 如果规则 ID 改变了，先调用重命名 API
      if (nameChanged) {
        try {
          await renameRule(oldName, newName);
        } catch (err) {
          toast.error("重命名失败: " + String(err));
          setIsSaving(false);
          return;
        }
      }

      // 重新获取最新配置（重命名后可能已更新引用关系），防止覆盖后端的更新
      const { getConfig } = await import("@/lib/api-client");
      const { config: latestConfig } = await getConfig();

      // 基于最新配置应用本地编辑
      const updatedRules = [...latestConfig.rules];
      // 使用 newName 查找（因为如果改名了，后端已经把名字改过来了）
      const existingIndex = updatedRules.findIndex((r) => r.name === newName);

      if (existingIndex >= 0) {
        updatedRules[existingIndex] = cleanedData;
      } else {
        updatedRules.push(cleanedData);
      }

      await saveConfig({
        version: latestConfig.version || 1,
        transformers: latestConfig.transformers || {},
        rules: updatedRules,
      });

      // 保存成功后自动刷新该规则
      try {
        const { refreshRule } = await import("@/lib/api-client");
        await refreshRule(cleanedData.name);
        toast.success("规则保存并刷新成功");
      } catch (refreshErr) {
        // 刷新失败不阻止保存成功
        console.error("Rule refresh failed:", refreshErr);
        toast.success("规则保存成功（刷新失败，请手动刷新）");
      }

      onSave();
    } catch (error) {
      toast.error("保存失败: " + String(error));
    } finally {
      setIsSaving(false);
    }
  };

  // 预览当前编辑的规则
  const handlePreview = async () => {
    if (!formData.name.trim()) {
      toast.error("请先填写规则名称");
      return;
    }
    if (!formData.sources || formData.sources.length === 0) {
      toast.error("请先添加数据来源");
      return;
    }

    setIsPreviewLoading(true);
    setIsPreviewOpen(true);
    setPreviewData(null);

    try {
      const result = await previewRule(undefined, formData);
      setPreviewData(result);
      // 设置默认预览客户端为第一个可用的
      if (result.contents && Object.keys(result.contents).length > 0) {
        setPreviewClient(Object.keys(result.contents)[0] as ClientType);
      }
    } catch (error) {
      toast.error("预览失败: " + String(error));
      setIsPreviewOpen(false);
    } finally {
      setIsPreviewLoading(false);
    }
  };

  // 数据来源管理
  const addSource = (type: SourceType) => {
    const newSource: SourceConfig = { type };
    if (type === "url") newSource.url = "";
    if (type === "ref") newSource.ref = "";
    if (type === "local") newSource.content = "";

    setFormData((prev) => ({
      ...prev,
      sources: [...(prev.sources || []), newSource],
    }));
  };

  const updateSource = (index: number, updates: Partial<SourceConfig>) => {
    setFormData((prev) => ({
      ...prev,
      sources: prev.sources?.map((s, i) => (i === index ? { ...s, ...updates } : s)),
    }));
  };

  const removeSource = (index: number) => {
    setFormData((prev) => ({
      ...prev,
      sources: prev.sources?.filter((_, i) => i !== index),
      // 更新 transforms 中的 target 索引
      transforms: prev.transforms?.map((t) => {
        if (Array.isArray(t.target)) {
          return {
            ...t,
            target: t.target
              .filter((idx) => idx !== index)
              .map((idx) => (idx > index ? idx - 1 : idx)),
          };
        }
        return t;
      }),
    }));
  };

  // 后处理管理
  const addTransform = (type: "use" | "replace" | "remove_lines") => {
    const newTransform: Transform = {
      type,
      target: "all",
    };
    if (type === "replace") {
      newTransform.pattern = "";
      newTransform.replacement = "";
    }
    if (type === "remove_lines") {
      newTransform.pattern = "";
    }

    setFormData((prev) => ({
      ...prev,
      transforms: [...(prev.transforms || []), newTransform],
    }));
  };

  const updateTransform = (index: number, updates: Partial<Transform>) => {
    setFormData((prev) => ({
      ...prev,
      transforms: prev.transforms?.map((t, i) => (i === index ? { ...t, ...updates } : t)),
    }));
  };

  const removeTransform = (index: number) => {
    setFormData((prev) => ({
      ...prev,
      transforms: prev.transforms?.filter((_, i) => i !== index),
    }));
  };

  const duplicateTransform = (index: number) => {
    setFormData((prev) => {
      const transforms = [...(prev.transforms || [])];
      transforms.splice(index + 1, 0, { ...transforms[index] });
      return { ...prev, transforms };
    });
  };

  // 拖动排序
  const handleDragStart = (index: number) => setDraggedIndex(index);
  const handleDragEnd = () => setDraggedIndex(null);
  const handleDragOver = (e: React.DragEvent, index: number) => {
    e.preventDefault();
    if (draggedIndex === null || draggedIndex === index) return;

    setFormData((prev) => {
      const transforms = [...(prev.transforms || [])];
      const [dragged] = transforms.splice(draggedIndex, 1);
      transforms.splice(index, 0, dragged);
      return { ...prev, transforms };
    });
    setDraggedIndex(index);
  };

  // 客户端选择
  const toggleClient = (client: ClientType) => {
    setFormData((prev) => {
      const clients = prev.output.clients.includes(client)
        ? prev.output.clients.filter((c) => c !== client)
        : [...prev.output.clients, client];
      return { ...prev, output: { ...prev.output, clients } };
    });
  };

  // 客户端差异化配置
  const toggleClientOverride = (client: ClientType, enabled: boolean) => {
    setFormData((prev) => {
      const currentOverride = prev.output.client_overrides?.[client];
      return {
        ...prev,
        output: {
          ...prev.output,
          client_overrides: {
            ...prev.output.client_overrides,
            [client]: {
              enabled,
              useGlobalTransforms: currentOverride?.useGlobalTransforms ?? true,
              transforms: currentOverride?.transforms || [],
            },
          },
        },
      };
    });
  };

  const toggleUseGlobalTransforms = (client: ClientType, useGlobal: boolean) => {
    setFormData((prev) => {
      const currentOverride = prev.output.client_overrides?.[client];
      return {
        ...prev,
        output: {
          ...prev.output,
          client_overrides: {
            ...prev.output.client_overrides,
            [client]: {
              enabled: currentOverride?.enabled ?? true,
              useGlobalTransforms: useGlobal,
              transforms: currentOverride?.transforms || [],
            },
          },
        },
      };
    });
  };

  const addClientTransform = (client: ClientType, type: "use" | "replace" | "remove_lines") => {
    const newTransform: Transform = { type, target: "all" };
    if (type === "replace") {
      newTransform.pattern = "";
      newTransform.replacement = "";
    }
    if (type === "remove_lines") {
      newTransform.pattern = "";
    }

    setFormData((prev) => {
      const currentOverride = prev.output.client_overrides?.[client];
      return {
        ...prev,
        output: {
          ...prev.output,
          client_overrides: {
            ...prev.output.client_overrides,
            [client]: {
              enabled: currentOverride?.enabled ?? true,
              useGlobalTransforms: currentOverride?.useGlobalTransforms ?? true,
              transforms: [
                ...(currentOverride?.transforms || []),
                newTransform,
              ],
            },
          },
        },
      };
    });
  };

  const updateClientTransform = (client: ClientType, index: number, updates: Partial<Transform>) => {
    setFormData((prev) => {
      const currentOverride = prev.output.client_overrides?.[client];
      const currentTransforms = currentOverride?.transforms || [];
      return {
        ...prev,
        output: {
          ...prev.output,
          client_overrides: {
            ...prev.output.client_overrides,
            [client]: {
              enabled: currentOverride?.enabled ?? true,
              useGlobalTransforms: currentOverride?.useGlobalTransforms ?? true,
              transforms: currentTransforms.map((t, i) =>
                i === index ? { ...t, ...updates } : t
              ),
            },
          },
        },
      };
    });
  };

  const removeClientTransform = (client: ClientType, index: number) => {
    setFormData((prev) => {
      const currentOverride = prev.output.client_overrides?.[client];
      const currentTransforms = currentOverride?.transforms || [];
      return {
        ...prev,
        output: {
          ...prev.output,
          client_overrides: {
            ...prev.output.client_overrides,
            [client]: {
              enabled: currentOverride?.enabled ?? true,
              useGlobalTransforms: currentOverride?.useGlobalTransforms ?? true,
              transforms: currentTransforms.filter((_, i) => i !== index),
            },
          },
        },
      };
    });
  };

  // 获取可用的其他规则列表
  const availableRules = config?.rules.filter((r) => r.name !== formData.name) || [];
  const transformers = config?.transformers || {};

  return (
    <div className="flex flex-col h-full bg-background">
      {/* Sticky Header */}
      <div className="flex-none flex items-center justify-between px-6 py-4 border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 z-20">
        <div>
          <h2 className="text-lg font-semibold tracking-tight">{formData.name || (rule ? rule.name : "新建规则")}</h2>
          <p className="text-xs text-muted-foreground">配置规则详情与转换逻辑</p>
        </div>
        <div className="flex items-center gap-3">
          <Button variant="outline" onClick={onCancel} disabled={isSaving}>取消</Button>
          <Button variant="outline" onClick={handlePreview} disabled={isSaving}>
            <Eye className="w-4 h-4 mr-1" />
            预览
          </Button>
          <Button onClick={handleSave} disabled={isSaving} className="min-w-[100px] shadow-sm">
            {isSaving ? <Loader2 className="w-4 h-4 animate-spin" /> : "保存规则"}
          </Button>
        </div>
      </div>

      {/* Scrollable Content */}
      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        {/* 基本信息 */}
        <div className="rounded-lg border border-border shadow-sm bg-card overflow-hidden">
          <SectionHeader
            title="基本信息"
            expanded={expandedSections.has("basic")}
            onToggle={() => toggleSection("basic")}
          />
          {expandedSections.has("basic") && (
            <div className="p-4 space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label className="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                    规则 ID <span className="text-destructive">*</span>
                    <HelpIcon text="规则的唯一标识符，决定 URL 路径。例如：YouTube 会生成 /Rules/Clash Meta/YouTube.list" />
                  </Label>
                  <Input
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    placeholder="例如: YouTube"
                    className="font-mono"
                  />
                  <p className="text-[10px] text-muted-foreground">修改后将同时重命名规则文件</p>
                </div>
                <div className="space-y-2">
                  <Label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">显示名称（可选）</Label>
                  <Input
                    value={formData.displayName || ""}
                    onChange={(e) => setFormData({ ...formData, displayName: e.target.value })}
                    placeholder="例如: YouTube视频"
                  />
                  <p className="text-[10px] text-muted-foreground">界面显示的名称，留空则使用规则 ID</p>
                </div>
              </div>
              <div className="space-y-2">
                <Label className="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                  描述
                  <HelpIcon text={HELP_TEXTS.description} />
                </Label>
                <Textarea
                  value={formData.description || ""}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  placeholder="规则描述..."
                  rows={2}
                  className="resize-none"
                />
              </div>
            </div>
          )}
        </div>

        {/* 数据来源 */}
        <div className="rounded-lg border border-border shadow-sm bg-card overflow-hidden">
          <SectionHeader
            title="数据来源"
            help={HELP_TEXTS.sources}
            expanded={expandedSections.has("sources")}
            onToggle={() => toggleSection("sources")}
            badge={
              formData.sources && formData.sources.length > 0 ? (
                <Badge variant="secondary" className="ml-2">
                  {formData.sources.length} 个来源
                </Badge>
              ) : null
            }
          />
          {expandedSections.has("sources") && (
            <div className="p-4 space-y-3">
              {/* 来源列表 */}
              {formData.sources?.map((source, index) => {
                const Icon = SOURCE_TYPE_ICONS[source.type || "url"];
                return (
                  <div
                    key={index}
                    className="flex items-start gap-3 p-3 rounded-lg border bg-muted/20 hover:bg-muted/40 transition-colors"
                  >
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <Badge variant="outline" className="shrink-0 gap-1 bg-background">
                        <Icon className="w-3 h-3" />
                        {index + 1}
                      </Badge>

                      {source.type === "url" && (
                        <Input
                          value={source.url || ""}
                          onChange={(e) => updateSource(index, { url: e.target.value })}
                          placeholder="https://example.com/rules.list"
                          className="flex-1 h-8 text-sm"
                        />
                      )}

                      {source.type === "ref" && (
                        <Select
                          value={source.ref || ""}
                          onValueChange={(value) => updateSource(index, { ref: value })}
                        >
                          <SelectTrigger className="flex-1 min-w-0 h-8">
                            <SelectValue placeholder="选择引用规则" className="truncate" />
                          </SelectTrigger>
                          <SelectContent className="max-w-[300px]">
                            {availableRules.map((r) => (
                              <SelectItem key={r.name} value={r.name} className="truncate">
                                {r.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      )}

                      {source.type === "local" && (
                        <div className="flex-1 flex items-center gap-2">
                          <span className="text-sm text-muted-foreground truncate flex-1 font-mono">
                            {source.content ? `${source.content.split('\n').length} 行内容` : "未编辑"}
                          </span>
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            className="h-8"
                            onClick={() => {
                              setLocalContentDraft(source.content || "");
                              setEditingLocalContent(index);
                            }}
                          >
                            <Edit3 className="w-3 h-3 mr-1" />
                            编辑
                          </Button>
                        </div>
                      )}
                    </div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      onClick={() => removeSource(index)}
                      className="shrink-0 h-8 w-8 text-muted-foreground hover:text-destructive"
                    >
                      <Trash2 className="w-4 h-4" />
                    </Button>
                  </div>
                );
              })}

              {/* 添加来源按钮 */}
              <div className="flex flex-wrap gap-2 pt-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => addSource("url")}
                  className="bg-background shadow-xs hover:bg-muted"
                >
                  <Link2 className="w-3 h-3 mr-1" />
                  URL 来源
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => addSource("ref")}
                  className="bg-background shadow-xs hover:bg-muted"
                >
                  <FolderInput className="w-3 h-3 mr-1" />
                  引用规则
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => addSource("local")}
                  className="bg-background shadow-xs hover:bg-muted"
                >
                  <FileText className="w-3 h-3 mr-1" />
                  本地内容
                </Button>
              </div>
            </div>
          )}
        </div>

        {/* 后处理操作 */}
        <div className="rounded-lg border border-gray-200 dark:border-slate-700 overflow-hidden">
          <SectionHeader
            title="后处理操作"
            help={HELP_TEXTS.transforms}
            expanded={expandedSections.has("transforms")}
            onToggle={() => toggleSection("transforms")}
            badge={
              formData.transforms && formData.transforms.length > 0 ? (
                <Badge variant="secondary" className="ml-2">
                  {formData.transforms.length} 个操作
                </Badge>
              ) : null
            }
          />
          {expandedSections.has("transforms") && (
            <div className="p-4 space-y-3 bg-white dark:bg-slate-900">
              {/* 操作列表 */}
              {formData.transforms?.map((transform, index) => (
                <TransformCard
                  key={index}
                  transform={transform}
                  sources={formData.sources || []}
                  transformers={transformers}
                  onChange={(updates) => updateTransform(index, updates)}
                  onRemove={() => removeTransform(index)}
                  onDuplicate={() => duplicateTransform(index)}
                  onDragStart={() => handleDragStart(index)}
                  onDragOver={(e) => handleDragOver(e, index)}
                  onDragEnd={handleDragEnd}
                  isDragging={draggedIndex === index}
                />
              ))}

              {/* 添加操作按钮 */}
              <div className="p-4 rounded-lg border border-dashed border-gray-300 dark:border-slate-600 bg-gray-50 dark:bg-slate-800">
                <p className="text-sm text-gray-500 dark:text-gray-400 mb-3 flex items-center gap-2">
                  添加后处理操作
                  <HelpIcon text="对来源数据进行处理，可指定处理特定来源或全部" />
                </p>
                <div className="flex flex-wrap gap-2">
                  {Object.keys(transformers).length > 0 && (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => addTransform("use")}
                    >
                      <Code2 className="w-4 h-4 mr-1" />
                      预定义转换器
                    </Button>
                  )}
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => addTransform("replace")}
                  >
                    <Replace className="w-4 h-4 mr-1" />
                    正则替换
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => addTransform("remove_lines")}
                  >
                    <Eraser className="w-4 h-4 mr-1" />
                    正则删除
                  </Button>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* 合并配置 */}
        <div className="rounded-lg border border-gray-200 dark:border-slate-700 overflow-hidden">
          <SectionHeader
            title="合并配置"
            help={HELP_TEXTS.merge}
            expanded={expandedSections.has("merge")}
            onToggle={() => toggleSection("merge")}
          />
          {expandedSections.has("merge") && (
            <div className="p-4 bg-white dark:bg-slate-900">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label className="flex items-center gap-2">
                    合并策略
                    <HelpIcon text="concat: 顺序拼接 | union: 去重并集 | intersect: 交集" />
                  </Label>
                  <Select
                    value={formData.merge?.strategy || "concat"}
                    onValueChange={(value: MergeStrategy) =>
                      setFormData((prev) => ({
                        ...prev,
                        merge: {
                          strategy: value,
                          dedupe: prev.merge?.dedupe ?? false,
                        },
                      }))
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="concat">顺序拼接 (concat)</SelectItem>
                      <SelectItem value="union">集合并集 (union)</SelectItem>
                      <SelectItem value="intersect">集合交集 (intersect)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>去重</Label>
                  <div className="flex items-center gap-2 h-10">
                    <Switch
                      checked={formData.merge?.dedupe || false}
                      onCheckedChange={(dedupe) =>
                        setFormData((prev) => ({
                          ...prev,
                          merge: { strategy: prev.merge?.strategy || "concat", dedupe },
                        }))
                      }
                    />
                    <span className="text-sm text-gray-500">合并后去重</span>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* 输出配置 */}
        <div className="rounded-lg border border-gray-200 dark:border-slate-700 overflow-hidden">
          <SectionHeader
            title="输出配置"
            help={HELP_TEXTS.outputClients}
            expanded={expandedSections.has("output")}
            onToggle={() => toggleSection("output")}
          />
          {expandedSections.has("output") && (
            <div className="p-4 space-y-4 bg-white dark:bg-slate-900">
              <div className="space-y-2">
                <Label>输出客户端</Label>
                <div className="flex flex-wrap gap-3">
                  {clientsList.map((client) => (
                    <label
                      key={client.id}
                      className={`flex items-center gap-2 p-3 rounded-lg border cursor-pointer transition-colors ${formData.output.clients.includes(client.id)
                        ? "bg-blue-50 dark:bg-blue-900/20 border-blue-500"
                        : "bg-gray-50 dark:bg-slate-800 border-gray-200 dark:border-slate-700"
                        }`}
                    >
                      <Checkbox
                        checked={formData.output.clients.includes(client.id)}
                        onCheckedChange={() => toggleClient(client.id as ClientType)}
                      />
                      <Monitor className="w-4 h-4" />
                      <span>{client.displayName}</span>
                    </label>
                  ))}
                </div>
              </div>

              {/* 客户端差异化配置 */}
              {formData.output.clients.length > 0 && (
                <div className="space-y-3">
                  <Label className="flex items-center gap-2">
                    客户端差异化配置
                    <HelpIcon text={HELP_TEXTS.clientOverrides} />
                  </Label>
                  {formData.output.clients.map((client) => {
                    const clientConfig = clientsList.find(c => c.id === client);
                    return (
                      <ClientOverrideSection
                        key={client}
                        client={client as ClientType}
                        clientsList={clientsList}
                        clientGlobalTransforms={clientConfig?.transforms || []}
                        config={formData.output.client_overrides?.[client as ClientType]}
                        transformers={transformers}
                        onToggle={(enabled) => toggleClientOverride(client as ClientType, enabled)}
                        onToggleUseGlobal={(useGlobal) => toggleUseGlobalTransforms(client as ClientType, useGlobal)}
                        onAddTransform={(type) => addClientTransform(client as ClientType, type)}
                        onUpdateTransform={(index, updates) => updateClientTransform(client as ClientType, index, updates)}
                        onRemoveTransform={(index) => removeClientTransform(client as ClientType, index)}
                      />
                    );
                  })}
                </div>
              )}
            </div>
          )}
        </div>

      </div>

      {/* 本地内容编辑对话框 */}
      <Dialog open={editingLocalContent !== null} onOpenChange={(open) => !open && setEditingLocalContent(null)}>
        <DialogContent className="max-w-3xl max-h-[80vh] flex flex-col">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <FileText className="w-5 h-5 text-blue-500" />
              编辑本地内容
            </DialogTitle>
          </DialogHeader>
          <div className="flex-1 min-h-0">
            <Textarea
              value={localContentDraft}
              onChange={(e) => setLocalContentDraft(e.target.value)}
              placeholder="输入规则内容，每行一条..."
              className="h-96 font-mono text-sm resize-none"
            />
          </div>
          <div className="flex justify-end gap-3 pt-4">
            <Button variant="outline" onClick={() => setEditingLocalContent(null)}>
              取消
            </Button>
            <Button
              onClick={() => {
                if (editingLocalContent !== null) {
                  updateSource(editingLocalContent, { content: localContentDraft });
                  setEditingLocalContent(null);
                }
              }}
            >
              确定
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* 预览对话框 */}
      <Dialog open={isPreviewOpen} onOpenChange={setIsPreviewOpen}>
        <DialogContent className="max-w-4xl w-[90vw] h-[70vh] bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700 flex flex-col p-0">
          <DialogHeader className="px-6 pt-6 pb-4 border-b border-gray-200 dark:border-slate-700 shrink-0">
            <DialogTitle className="text-gray-900 dark:text-white flex items-center gap-2">
              <Eye className="w-5 h-5 text-blue-500" />
              预览: {formData.name || "未命名规则"}
            </DialogTitle>
          </DialogHeader>

          {isPreviewLoading ? (
            <div className="flex-1 flex items-center justify-center">
              <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
            </div>
          ) : previewData ? (
            <div className="flex-1 flex flex-col min-h-0 overflow-hidden">
              {/* 数据源状态 */}
              {previewData.diagnostics.sourceResults.length > 0 && (
                <div className="px-6 py-3 bg-gray-50 dark:bg-slate-900 border-b border-gray-200 dark:border-slate-700 shrink-0">
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
                        <span className="text-gray-700 dark:text-gray-300 truncate max-w-xs">
                          {source.url}
                        </span>
                        {source.size !== undefined && source.size > 0 && (
                          <span className="text-gray-500">({(source.size / 1024).toFixed(1)} KB)</span>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* 内容 Tabs */}
              <Tabs
                value={previewClient}
                onValueChange={(v) => setPreviewClient(v as ClientType)}
                className="flex-1 flex flex-col min-h-0"
              >
                <div className="px-6 py-3 border-b border-gray-200 dark:border-slate-700 flex items-center justify-between shrink-0">
                  <TabsList className="bg-gray-100 dark:bg-slate-900">
                    {Object.keys(previewData.contents).map((client) => (
                      <TabsTrigger
                        key={client}
                        value={client}
                        className="data-[state=active]:bg-white dark:data-[state=active]:bg-slate-800"
                      >
                        {clientsList.find(c => c.id === client)?.displayName || client}
                      </TabsTrigger>
                    ))}
                  </TabsList>
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    {previewData.contents[previewClient]?.split('\n').length || 0} 行
                  </span>
                </div>
                {Object.entries(previewData.contents).map(([client, content]) => (
                  <TabsContent
                    key={client}
                    value={client}
                    className="flex-1 m-0 relative min-h-0 overflow-hidden"
                  >
                    <div className="absolute top-2 right-2 z-10">
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
                    </div>
                    <div className="h-full overflow-auto bg-gray-50 dark:bg-slate-900">
                      <pre className="p-4 text-sm font-mono text-gray-800 dark:text-gray-200 whitespace-pre">
                        {content || "暂无内容"}
                      </pre>
                    </div>
                  </TabsContent>
                ))}
              </Tabs>
            </div>
          ) : (
            <div className="flex-1 flex items-center justify-center text-gray-500">
              无预览数据
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

// 后处理操作卡片
interface TransformCardProps {
  transform: Transform;
  sources: SourceConfig[];
  transformers: Record<string, ScriptTransformer>;
  onChange: (updates: Partial<Transform>) => void;
  onRemove: () => void;
  onDuplicate: () => void;
  onDragStart: () => void;
  onDragOver: (e: React.DragEvent) => void;
  onDragEnd: () => void;
  isDragging: boolean;
}

function TransformCard({
  transform,
  sources,
  transformers,
  onChange,
  onRemove,
  onDuplicate,
  onDragStart,
  onDragOver,
  onDragEnd,
  isDragging,
}: TransformCardProps) {
  const [expanded, setExpanded] = useState(true);

  const getTypeIcon = () => {
    switch (transform.type) {
      case "use": return Code2;
      case "replace": return Replace;
      case "remove_lines": return Eraser;
      default: return Settings2;
    }
  };

  const getTypeLabel = () => {
    switch (transform.type) {
      case "use": return "预定义转换器";
      case "replace": return "正则替换";
      case "remove_lines": return "正则删除";
      default: return "操作";
    }
  };

  const getTargetLabel = () => {
    if (transform.target === "all") return "全部来源";
    if (Array.isArray(transform.target) && transform.target.length > 0) {
      return `来源 ${transform.target.map((i) => i + 1).join(", ")}`;
    }
    return "全部来源";
  };

  const Icon = getTypeIcon();

  return (
    <div
      draggable
      onDragStart={onDragStart}
      onDragOver={onDragOver}
      onDragEnd={onDragEnd}
      className={`rounded-lg border border-gray-200 dark:border-slate-700 bg-gray-50 dark:bg-slate-800 transition-all ${isDragging ? "opacity-50 scale-95" : ""
        }`}
    >
      <div className="flex items-center justify-between p-3 border-b border-gray-200 dark:border-slate-700">
        <div className="flex items-center gap-2">
          <div className="cursor-grab active:cursor-grabbing text-gray-400 hover:text-gray-600">
            <GripVertical className="w-4 h-4" />
          </div>
          <button
            type="button"
            onClick={() => setExpanded(!expanded)}
            className="flex items-center gap-2"
          >
            {expanded ? (
              <ChevronDown className="w-4 h-4 text-gray-500" />
            ) : (
              <ChevronRight className="w-4 h-4 text-gray-500" />
            )}
            <Icon className="w-4 h-4 text-blue-500" />
            <span className="font-medium text-gray-900 dark:text-white">{getTypeLabel()}</span>
          </button>
          <Badge variant="outline" className="text-xs">
            {getTargetLabel()}
          </Badge>
        </div>
        <div className="flex items-center gap-1">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={onDuplicate}
            className="w-8 h-8 text-gray-400 hover:text-gray-600"
            title="复制"
          >
            <Copy className="w-4 h-4" />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={onRemove}
            className="w-8 h-8 text-gray-400 hover:text-red-500"
            title="删除"
          >
            <Trash2 className="w-4 h-4" />
          </Button>
        </div>
      </div>

      {expanded && (
        <div className="p-3 space-y-3">
          {/* 目标来源选择 */}
          <div className="space-y-2">
            <Label className="text-sm text-gray-500 flex items-center gap-2">
              处理目标
              <HelpIcon text={HELP_TEXTS.transformTarget} />
            </Label>
            <Select
              value={transform.target === "all" ? "all" : "custom"}
              onValueChange={(value) => {
                if (value === "all") {
                  onChange({ target: "all" });
                } else {
                  onChange({ target: [] });
                }
              }}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部来源</SelectItem>
                <SelectItem value="custom">指定来源</SelectItem>
              </SelectContent>
            </Select>

            {Array.isArray(transform.target) && sources.length > 0 && (
              <div className="flex flex-wrap gap-2 mt-2">
                {sources.map((source, idx) => {
                  const targetArray = transform.target as number[];
                  const isSelected = targetArray.includes(idx);
                  const Icon = SOURCE_TYPE_ICONS[source.type || "url"];
                  return (
                    <label
                      key={idx}
                      className={`flex items-center gap-1 px-2 py-1 rounded border cursor-pointer text-sm ${isSelected
                        ? "bg-blue-50 dark:bg-blue-900/20 border-blue-500"
                        : "bg-white dark:bg-slate-900 border-gray-200 dark:border-slate-700"
                        }`}
                    >
                      <Checkbox
                        checked={isSelected}
                        onCheckedChange={(checked) => {
                          const current = Array.isArray(transform.target) ? transform.target : [];
                          if (checked) {
                            onChange({ target: [...current, idx] });
                          } else {
                            onChange({ target: current.filter((i) => i !== idx) });
                          }
                        }}
                      />
                      <Icon className="w-3 h-3" />
                      <span>{idx + 1}</span>
                    </label>
                  );
                })}
              </div>
            )}
          </div>

          {/* 类型特定字段 */}
          {transform.type === "use" && (
            <div className="space-y-2">
              <Label className="text-sm text-gray-500 flex items-center gap-2">
                选择转换器
                <HelpIcon text={HELP_TEXTS.useTransformer} />
              </Label>
              <Select
                value={transform.use || ""}
                onValueChange={(value) => onChange({ use: value })}
              >
                <SelectTrigger>
                  <SelectValue placeholder="选择预定义转换器" />
                </SelectTrigger>
                <SelectContent>
                  {Object.entries(transformers).map(([name, t]) => (
                    <SelectItem key={name} value={name}>
                      {name} {t.description && `- ${t.description}`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {Object.keys(transformers).length === 0 && (
                <p className="text-sm text-amber-500">暂无预定义转换器，请先在配置中添加</p>
              )}
            </div>
          )}

          {transform.type === "replace" && (
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <Label className="text-sm text-gray-500 flex items-center gap-2">
                  正则表达式
                  <HelpIcon text={HELP_TEXTS.replace} />
                </Label>
                <Input
                  value={transform.pattern || ""}
                  onChange={(e) => onChange({ pattern: e.target.value })}
                  placeholder="匹配模式"
                />
              </div>
              <div className="space-y-2">
                <Label className="text-sm text-gray-500">替换为</Label>
                <Input
                  value={transform.replacement || ""}
                  onChange={(e) => onChange({ replacement: e.target.value })}
                  placeholder="替换内容（留空则删除）"
                />
              </div>
            </div>
          )}

          {transform.type === "remove_lines" && (
            <div className="space-y-2">
              <Label className="text-sm text-gray-500 flex items-center gap-2">
                正则表达式
                <HelpIcon text={HELP_TEXTS.removeLines} />
              </Label>
              <Input
                value={transform.pattern || ""}
                onChange={(e) => onChange({ pattern: e.target.value })}
                placeholder="匹配到的行将被删除"
              />
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// 客户端差异化配置组件
interface ClientOverrideSectionProps {
  client: ClientType;
  clientsList: ClientConfig[];
  clientGlobalTransforms: Transform[];
  config?: { enabled?: boolean; useGlobalTransforms?: boolean; transforms?: Transform[] };
  transformers: Record<string, ScriptTransformer>;
  onToggle: (enabled: boolean) => void;
  onToggleUseGlobal: (useGlobal: boolean) => void;
  onAddTransform: (type: "use" | "replace" | "remove_lines") => void;
  onUpdateTransform: (index: number, updates: Partial<Transform>) => void;
  onRemoveTransform: (index: number) => void;
}

function ClientOverrideSection({
  client,
  clientsList,
  clientGlobalTransforms,
  config,
  transformers,
  onToggle,
  onToggleUseGlobal,
  onAddTransform,
  onUpdateTransform,
  onRemoveTransform,
}: ClientOverrideSectionProps) {
  const [expanded, setExpanded] = useState(false);
  const useGlobalTransforms = config?.useGlobalTransforms ?? true;
  const transforms = config?.transforms || [];
  const hasGlobalTransforms = clientGlobalTransforms.length > 0;

  return (
    <div className="rounded-lg border border-gray-200 dark:border-slate-700 overflow-hidden">
      {/* 标题栏 */}
      <div className="flex items-center justify-between p-3 bg-gray-50 dark:bg-slate-800">
        <button
          type="button"
          onClick={() => setExpanded(!expanded)}
          className="flex items-center gap-2 flex-1 min-w-0"
        >
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-gray-500 shrink-0" />
          ) : (
            <ChevronRight className="w-4 h-4 text-gray-500 shrink-0" />
          )}
          <Monitor className="w-4 h-4 shrink-0" />
          <span className="font-medium text-gray-900 dark:text-white truncate">
            {clientsList.find(c => c.id === client)?.displayName || client}
          </span>
          {transforms.length > 0 && (
            <Badge variant="secondary" className="text-xs shrink-0">
              {transforms.length} 自定义
            </Badge>
          )}
        </button>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <Switch
              checked={config?.enabled ?? true}
              onCheckedChange={onToggle}
            />
            <span className="text-sm text-gray-500">启用</span>
          </div>
        </div>
      </div>

      {/* 展开内容 */}
      {expanded && (config?.enabled ?? true) && (
        <div className="p-3 bg-white dark:bg-slate-900 space-y-4 border-t border-gray-200 dark:border-slate-700">
          {/* 全局转换继承开关 */}
          <div className="flex items-start gap-2 p-2 rounded bg-gray-50 dark:bg-slate-800/50">
            <Checkbox
              checked={useGlobalTransforms}
              onCheckedChange={(c) => onToggleUseGlobal(!!c)}
              className="mt-1"
            />
            <div className="space-y-1">
              <span className="text-sm font-medium">应用全局客户端转换</span>
              <p className="text-xs text-gray-500">
                如果开启，将先应用客户端全局配置中的转换操作，再应用此处的自定义操作。
              </p>
              {hasGlobalTransforms ? (
                <div className="flex flex-wrap gap-1 mt-1">
                  {clientGlobalTransforms.map((t, i) => (
                    <Badge key={i} variant="outline" className="text-[10px] bg-background">
                      {t.type === "use" ? `转换器: ${t.use}` : t.type === "replace" ? "正则替换" : "删除行"}
                    </Badge>
                  ))}
                </div>
              ) : (
                <p className="text-xs text-amber-500">
                  (该客户端暂无全局转换配置)
                </p>
              )}
            </div>
          </div>

          <Separator />

          {/* 额外转换操作 */}
          <div className="space-y-3">
            <p className="text-sm font-medium flex items-center gap-2">
              额外转换操作
              <Badge variant="outline" className="text-xs font-normal">
                仅对该规则生效
              </Badge>
            </p>

            {transforms.map((transform, index) => (
              <TransformCard
                key={index}
                transform={transform}
                sources={[]} // 客户端转换通常不针对特定来源，或者需要传递sources？这里简化处理
                transformers={transformers}
                onChange={(updates) => onUpdateTransform(index, updates)}
                onRemove={() => onRemoveTransform(index)}
                onDuplicate={() => { }} // 暂时不支持 duplicating client specific transforms easy way
                isDragging={false}
                onDragStart={() => { }}
                onDragOver={() => { }}
                onDragEnd={() => { }}
              />
            ))}

            <div className="flex flex-wrap gap-2">
              {Object.keys(transformers).length > 0 && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => onAddTransform("use")}
                >
                  <Code2 className="w-4 h-4 mr-1" />
                  预定义
                </Button>
              )}
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => onAddTransform("replace")}
              >
                <Replace className="w-4 h-4 mr-1" />
                替换
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => onAddTransform("remove_lines")}
              >
                <Eraser className="w-4 h-4 mr-1" />
                删除
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

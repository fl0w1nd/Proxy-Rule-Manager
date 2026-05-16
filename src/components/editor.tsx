"use client";

import { useState, useEffect, useRef, useCallback } from "react";
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
  X,
  Tag,
  Globe,
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
import { saveConfig, getConfig, renameRule, previewRule, refreshRule, PreviewResponse, getClients, ClientConfig, getGeositeCatalog, type GeositeCatalogItem } from "@/lib/api-client";
import { LocalContentDialog } from "./editor-local-content";
import { PreviewDialog } from "./editor-preview";
import { createTransformByType } from "@/lib/transform-utils";
import { createListItemKey, createListItemKeys } from "@/lib/utils";
import { toast } from "sonner";
import { IconPicker } from "@/components/icon-picker";
import { isGeositeRule } from "@/lib/rule-classification";

import { useTheme } from "./theme-provider";

interface RuleEditorProps {
  rule: RuleConfig | null;
  config: RulesConfig | null;
  onSave: () => void;
  onCancel: () => void;
  /**
   * Called whenever the editor's saving state flips. Parents use this to
   * disable Dialog close so an in-flight save is never silently cancelled.
   */
  onSavingChange?: (saving: boolean) => void;
  /**
   * Called whenever the editor's dirty state flips. Parents can use this
   * to prompt for confirmation before destructive close actions.
   */
  onDirtyChange?: (dirty: boolean) => void;
}

// 帮助文本
const HELP_TEXTS = {
  ruleName: "规则的唯一标识符，用于生成文件路径。例如：YouTube 会生成 /Rules/clash_meta/YouTube.list",
  description: "对规则的简短描述，方便管理和查找",
  sources: "添加数据来源，支持四种类型：URL（远程规则）、引用（已有规则）、本地（自定义内容）、Geosite（上游地理站点规则）",
  sourceUrl: "从远程 URL 获取规则内容",
  sourceRef: "引用项目中已存在的其他规则",
  sourceLocal: "直接编写本地规则内容",
  sourceGeosite: "选择 geosite provider 与列表，系统会渲染成目标客户端规则",
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
  geosite: Globe,
};

// 规范化规则数据
function migrateRule(rule: RuleConfig): RuleConfig {
  const newRule = { ...rule };

  // 确保 sources 中有 type 字段
  if (newRule.sources) {
    newRule.sources = newRule.sources.map((s) => ({
      ...s,
      type: s.type || (s.provider && s.list ? "geosite" : s.url ? "url" : s.ref ? "ref" : "local"),
    }));
  }

  return newRule;
}

function createDefaultRule(): RuleConfig {
  return {
    name: "",
    displayName: "",
    description: "",
    sources: [],
    transforms: [],
    output: {
      clients: [],
    },
    tags: [],
  };
}

// 帮助图标组件
function HelpIcon({ text }: { text: string }) {
  return (
    <TooltipProvider delayDuration={200}>
      <Tooltip>
        <TooltipTrigger asChild>
          <HelpCircle className="w-4 h-4 text-muted-foreground hover:text-primary cursor-help inline-flex" />
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
    <div className="flex items-center justify-between border-b border-border/50 bg-surface-subtle/80 px-4 py-3 transition-colors">
      <button
        type="button"
        onClick={onToggle}
        className="flex min-w-0 flex-1 items-center gap-2 rounded-lg text-left focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-primary/15"
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

export function RuleEditor({
  rule,
  config,
  onSave,
  onCancel,
  onSavingChange,
  onDirtyChange,
}: RuleEditorProps) {
  const initialFormData = rule ? migrateRule(rule) : createDefaultRule();
  const isLockedGeositeRule = !!rule && isGeositeRule(initialFormData);
  const [formData, setFormData] = useState<RuleConfig>(initialFormData);
  // Snapshot of the form at mount-time / after save. We compare against
  // `formData` to decide whether to prompt before discarding edits.
  const initialSnapshotRef = useRef<string>(JSON.stringify(initialFormData));
  const [sourceKeys, setSourceKeys] = useState<string[]>(() =>
    createListItemKeys(initialFormData.sources?.length ?? 0)
  );
  const [transformKeys, setTransformKeys] = useState<string[]>(() =>
    createListItemKeys(initialFormData.transforms?.length ?? 0)
  );
  const [clientTransformKeys, setClientTransformKeys] = useState<Record<string, string[]>>(() => {
    const keys: Record<string, string[]> = {};
    const overrides = initialFormData.output.client_overrides || {};
    for (const [clientId, override] of Object.entries(overrides)) {
      keys[clientId] = createListItemKeys(override.transforms?.length ?? 0);
    }
    return keys;
  });
  const [isSaving, setIsSaving] = useState(false);
  // Notify the parent so it can lock the surrounding dialog while a save
  // is in flight (clicking the backdrop must not discard pending writes).
  useEffect(() => {
    onSavingChange?.(isSaving);
  }, [isSaving, onSavingChange]);

  // Dirty bit: cheap structural compare against the initial snapshot. We
  // skip this for the locked geosite rule because users can't actually
  // edit anything meaningful, and we want to avoid spurious confirm dialogs.
  const isDirty =
    !isLockedGeositeRule &&
    JSON.stringify(formData) !== initialSnapshotRef.current;
  useEffect(() => {
    onDirtyChange?.(isDirty);
  }, [isDirty, onDirtyChange]);
  const [expandedSections, setExpandedSections] = useState<Set<string>>(
    new Set(["basic", "sources", "transforms", "merge", "output"])
  );
  const [draggedIndex, setDraggedIndex] = useState<number | null>(null);
  const [editingLocalContent, setEditingLocalContent] = useState<number | null>(null);
  const [tagInput, setTagInput] = useState("");
  const { mode } = useTheme();

  // 预览相关状态
  const [isPreviewOpen, setIsPreviewOpen] = useState(false);
  const [isPreviewLoading, setIsPreviewLoading] = useState(false);
  const [previewData, setPreviewData] = useState<PreviewResponse | null>(null);

  // 动态客户端列表
  const [clientsList, setClientsList] = useState<ClientConfig[]>([]);
  const clientsListRef = useRef<ClientConfig[]>([]);
  const [geositeCatalogs, setGeositeCatalogs] = useState<Record<string, GeositeCatalogItem[]>>({});

  const fetchLatestClients = async (): Promise<ClientConfig[]> => {
    try {
      const { clients } = await getClients();
      setClientsList(clients);
      clientsListRef.current = clients;
      return clients;
    } catch (err) {
      console.error("Failed to load clients:", err);
      // Use ref to avoid stale closure capturing outdated clientsList state
      return clientsListRef.current;
    }
  };

  // 加载客户端列表
  useEffect(() => {
    getClients()
      .then(({ clients }) => {
        setClientsList(clients);
        setFormData((prev) => {
          if (rule || prev.output.clients.length > 0 || clients.length === 0) {
            return prev;
          }
          return {
            ...prev,
            output: {
              ...prev.output,
              clients: clients.map((c) => c.id),
            },
          };
        });
      })
      .catch((err) => {
        console.error("Failed to load clients:", err);
      });
  }, [rule]);

  const ensureGeositeCatalog = useCallback(async (provider: "v2fly" | "loyalsoldier") => {
    if (geositeCatalogs[provider]) {
      return geositeCatalogs[provider];
    }
    const result = await getGeositeCatalog(provider);
    setGeositeCatalogs((prev) => ({
      ...prev,
      [provider]: result.catalog,
    }));
    return result.catalog;
  }, [geositeCatalogs]);

  useEffect(() => {
    const geositeProviders = Array.from(
      new Set(
        (formData.sources || [])
          .filter((source) => source.type === "geosite")
          .map((source) => source.provider || "v2fly")
      )
    ) as Array<"v2fly" | "loyalsoldier">;

    for (const provider of geositeProviders) {
      void ensureGeositeCatalog(provider);
    }
  }, [ensureGeositeCatalog, formData.sources]);



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
      const newName = isLockedGeositeRule ? (oldName || formData.name.trim()) : formData.name.trim();
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
          (s.type === "local" && s.content) ||
          (s.type === "geosite" && s.provider && s.list)
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

      const geositeSources = (formData.sources || []).filter((source) => source.type === "geosite");
      for (const source of geositeSources) {
        const provider = (source.provider || "v2fly") as "v2fly" | "loyalsoldier";
        const catalog = await ensureGeositeCatalog(provider);
        const item = catalog.find((entry) => entry.name === (source.list || "").trim());
        if (!item) {
          toast.error(`Geosite 列表不存在: ${provider}/${source.list || ""}`);
          setIsSaving(false);
          return;
        }
        const invalidAttrs = (source.attrs || []).filter((attr) => !item.attrs.includes(attr));
        if (invalidAttrs.length > 0) {
          toast.error(`Geosite 属性不存在: ${provider}/${item.name} -> ${invalidAttrs.join(", ")}`);
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
      const { config: latestConfig, rev } = await getConfig();

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
      }, rev);

      // 保存成功后自动刷新该规则
      try {
        await refreshRule(cleanedData.name);
        toast.success("规则保存并刷新成功");
      } catch (refreshErr) {
        // 刷新失败不阻止保存成功
        console.error("Rule refresh failed:", refreshErr);
        toast.success("规则保存成功（刷新失败，请手动刷新）");
      }

      // Reset the snapshot so the editor is no longer "dirty" — without
      // this, closing the dialog would still trigger the confirm prompt.
      initialSnapshotRef.current = JSON.stringify(cleanedData);
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
      const result = await previewRule(undefined, formData, 10000);
      setPreviewData(result);
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
    if (type === "geosite") {
      newSource.provider = "v2fly";
      newSource.list = "";
      newSource.attrs = [];
      newSource.renderProfile = "mihomo-classical";
    }

    setFormData((prev) => ({
      ...prev,
      sources: [...(prev.sources || []), newSource],
    }));
    setSourceKeys((prev) => [...prev, createListItemKey()]);
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
    setSourceKeys((prev) => prev.filter((_, i) => i !== index));
  };

  // 后处理管理
  const addTransform = (type: "use" | "replace" | "remove_lines") => {
    setFormData((prev) => ({
      ...prev,
      transforms: [...(prev.transforms || []), createTransformByType(type)],
    }));
    setTransformKeys((prev) => [...prev, createListItemKey()]);
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
    setTransformKeys((prev) => prev.filter((_, i) => i !== index));
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
    setTransformKeys((prev) => {
      const nextKeys = [...prev];
      const [draggedKey] = nextKeys.splice(draggedIndex, 1);
      nextKeys.splice(index, 0, draggedKey);
      return nextKeys;
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
    const existingTransformCount =
      formData.output.client_overrides?.[client]?.transforms?.length ?? 0;
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
    setClientTransformKeys((prev) => {
      if (prev[client]) {
        return prev;
      }
      return {
        ...prev,
        [client]: createListItemKeys(existingTransformCount),
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
                createTransformByType(type),
              ],
            },
          },
        },
      };
    });
    setClientTransformKeys((prev) => ({
      ...prev,
      [client]: [...(prev[client] || []), createListItemKey()],
    }));
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
    setClientTransformKeys((prev) => ({
      ...prev,
      [client]: (prev[client] || []).filter((_, i) => i !== index),
    }));
  };

  // 获取可用的其他规则列表
  const availableRules = config?.rules.filter((r) => r.name !== formData.name) || [];
  const transformers = config?.transformers || {};

  const getGeositeCatalogItem = (source: SourceConfig) => {
    const provider = source.provider || "v2fly";
    return (geositeCatalogs[provider] || []).find((item) => item.name === (source.list || "").trim());
  };

  // 标签处理逻辑
  const handleAddTag = () => {
    const newTag = tagInput.trim();
    if (!newTag) return;

    if (formData.tags?.includes(newTag)) {
      toast.error("该标签已存在");
      return;
    }

    setFormData((prev) => ({
      ...prev,
      tags: [...(prev.tags || []), newTag],
    }));
    setTagInput("");
  };

  const handleRemoveTag = (index: number) => {
    setFormData((prev) => ({
      ...prev,
      tags: prev.tags?.filter((_, i) => i !== index) || [],
    }));
  };

  const editorTheme = mode === "dark" ? "vs-dark" : "light";

  const handleCancelClick = () => {
    if (isSaving) return;
    if (isDirty) {
      const ok = typeof window !== "undefined"
        ? window.confirm("有未保存的修改，确定要放弃吗？")
        : true;
      if (!ok) return;
    }
    onCancel();
  };

  return (
    <div className="flex flex-col h-full bg-background">
      {/* Sticky Header */}
      <div className="z-20 flex-none border-b border-border bg-background/92 px-6 py-4 shadow-[var(--shadow-xs)] backdrop-blur supports-[backdrop-filter]:bg-background/70">
        <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
          <div className="min-w-0">
            <h2 className="text-lg font-semibold tracking-tight">{formData.name || (rule ? rule.name : "新建规则")}</h2>
            <p className="text-xs text-muted-foreground">配置规则详情与转换逻辑</p>
          </div>
          <div className="flex flex-wrap items-center gap-2 md:justify-end">
            <Button variant="outline" onClick={handleCancelClick} disabled={isSaving}>取消</Button>
            <Button variant="outline" onClick={handlePreview} disabled={isSaving}>
              <Eye className="w-4 h-4 mr-1" />
              预览
            </Button>
            <Button onClick={handleSave} disabled={isSaving} className="min-w-[100px]">
              {isSaving ? <Loader2 className="w-4 h-4 animate-spin" /> : "保存规则"}
            </Button>
          </div>
        </div>
      </div>

      {/* Scrollable Content */}
      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        {/* 基本信息 */}
        <div className="overflow-hidden rounded-2xl border border-border bg-card shadow-[var(--shadow-sm)]">
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
                    <HelpIcon text="规则的唯一标识符，决定 URL 路径。例如：YouTube 会生成 /Rules/clash_meta/YouTube.list" />
                  </Label>
                  <Input
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    placeholder="例如: YouTube"
                    className="font-mono"
                    disabled={isLockedGeositeRule}
                  />
                  <p className="text-[10px] text-muted-foreground">
                    {isLockedGeositeRule ? "Geosite 规则 ID 由系统管理" : "修改后将同时重命名规则文件"}
                  </p>
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
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
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
                <div className="space-y-2">
                  <Label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                    图标
                  </Label>
                  <IconPicker
                    value={formData.icon}
                    onChange={(icon) => setFormData({ ...formData, icon })}
                  />
                  <p className="text-[10px] text-muted-foreground">从 Iconify 搜索图标，支持品牌、通用等</p>
                </div>
              </div>
              {/* 标签 */}
              <div className="space-y-2">
                <Label className="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                  <Tag className="w-3 h-3" />
                  标签
                  <HelpIcon text="为规则添加标签，方便分类和筛选" />
                </Label>
                <div className="flex flex-wrap gap-2 min-h-[32px]">
                  {(formData.tags || []).map((tag, index) => (
                    <Badge
                      key={`${tag}-${index}`}
                      variant="secondary"
                      className="group flex items-center gap-1 pr-1 hover:bg-accent"
                    >
                      {tag}
                      <button
                        type="button"
                        onClick={() => handleRemoveTag(index)}
                        className="ml-1 rounded-full p-0.5 hover:bg-destructive/20 hover:text-destructive transition-colors"
                        aria-label={`删除标签 ${tag}`}
                      >
                        <X className="w-3 h-3" />
                      </button>
                    </Badge>
                  ))}
                </div>
                <div className="flex gap-2">
                  <Input
                    value={tagInput}
                    onChange={(e) => setTagInput(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        e.preventDefault();
                        handleAddTag();
                      }
                    }}
                    placeholder="输入标签后按 Enter 添加"
                    className="flex-1"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleAddTag}
                    disabled={!tagInput.trim()}
                  >
                    添加
                  </Button>
                </div>
                <p className="text-[10px] text-muted-foreground">标签用于规则分类，可在规则列表中按标签筛选</p>
              </div>
            </div>
          )}
        </div>

        {/* 数据来源 */}
        <div className="overflow-hidden rounded-2xl border border-border bg-card shadow-[var(--shadow-sm)]">
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
              {isLockedGeositeRule ? (
                <p className="text-xs text-muted-foreground">
                  Geosite 规则的数据来源由系统管理，规则 ID 与来源保持一一对应。
                </p>
              ) : null}
              {/* 来源列表 */}
              {formData.sources?.map((source, index) => {
                const Icon = SOURCE_TYPE_ICONS[source.type || "url"];
                return (
                  <div
                    key={sourceKeys[index] ?? `source-${index}`}
                    className="flex items-start gap-3 rounded-xl border border-border/50 bg-surface-subtle/60 p-3 shadow-[var(--shadow-xs)] transition-colors hover:bg-accent/20"
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
                          disabled={isLockedGeositeRule}
                        />
                      )}

                      {source.type === "ref" && (
                        <Select
                          value={source.ref || ""}
                          onValueChange={(value) => updateSource(index, { ref: value })}
                          disabled={isLockedGeositeRule}
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
                              setEditingLocalContent(index);
                            }}
                            disabled={isLockedGeositeRule}
                          >
                            <Edit3 className="w-3 h-3 mr-1" />
                            编辑
                          </Button>
                        </div>
                      )}

                      {source.type === "geosite" && (
                        <div className="flex-1 grid gap-2 md:grid-cols-[140px_minmax(0,1fr)]">
                          <Select
                            value={source.provider || "v2fly"}
                            onValueChange={(value) => {
                              updateSource(index, {
                                provider: value as "v2fly" | "loyalsoldier",
                                list: "",
                                attrs: [],
                              });
                              void ensureGeositeCatalog(value as "v2fly" | "loyalsoldier");
                            }}
                            disabled={isLockedGeositeRule}
                          >
                            <SelectTrigger className="h-8">
                              <SelectValue placeholder="Provider" />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="v2fly">v2fly</SelectItem>
                              <SelectItem value="loyalsoldier">Loyalsoldier</SelectItem>
                            </SelectContent>
                          </Select>
                          <div className="space-y-2">
                            <Input
                              value={source.list || ""}
                              onChange={(e) => updateSource(index, { list: e.target.value, attrs: [] })}
                              placeholder="搜索 geosite 列表，例如 google"
                              className="h-8 text-sm"
                              list={`geosite-list-${index}`}
                              disabled={isLockedGeositeRule}
                            />
                            <datalist id={`geosite-list-${index}`}>
                              {(geositeCatalogs[source.provider || "v2fly"] || []).map((item) => (
                                <option key={`${source.provider || "v2fly"}-${item.name}`} value={item.name} />
                              ))}
                            </datalist>
                            {source.list && !getGeositeCatalogItem(source) ? (
                              <p className="text-[10px] text-destructive">该 geosite 列表不存在</p>
                            ) : null}
                            {getGeositeCatalogItem(source)?.attrs?.length ? (
                              <div className="flex flex-wrap gap-1">
                                {getGeositeCatalogItem(source)!.attrs.map((attr) => {
                                  const selected = (source.attrs || []).includes(attr);
                                  return (
                                    <button
                                      key={`${source.provider || "v2fly"}-${source.list}-${attr}`}
                                      type="button"
                                      onClick={() => updateSource(index, {
                                        attrs: selected
                                          ? (source.attrs || []).filter((item) => item !== attr)
                                          : [...(source.attrs || []), attr].sort(),
                                      })}
                                      className="inline-flex"
                                      disabled={isLockedGeositeRule}
                                    >
                                      <Badge variant={selected ? "blue" : "outline"} className="text-[10px]">
                                        @{attr}
                                      </Badge>
                                    </button>
                                  );
                                })}
                              </div>
                            ) : source.list ? (
                              <p className="text-[10px] text-muted-foreground">该列表没有可选 attrs</p>
                            ) : null}
                          </div>
                        </div>
                      )}
                    </div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      onClick={() => removeSource(index)}
                      className="shrink-0 h-8 w-8 text-muted-foreground hover:text-destructive"
                      disabled={isLockedGeositeRule}
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
                  className="bg-background hover:bg-accent"
                  disabled={isLockedGeositeRule}
                >
                  <Link2 className="w-3 h-3 mr-1" />
                  URL 来源
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => addSource("ref")}
                  className="bg-background hover:bg-accent"
                  disabled={isLockedGeositeRule}
                >
                  <FolderInput className="w-3 h-3 mr-1" />
                  引用规则
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => addSource("local")}
                  className="bg-background hover:bg-accent"
                  disabled={isLockedGeositeRule}
                >
                  <FileText className="w-3 h-3 mr-1" />
                  本地内容
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => addSource("geosite")}
                  className="bg-background hover:bg-accent"
                  disabled={isLockedGeositeRule}
                >
                  <Globe className="w-3 h-3 mr-1" />
                  Geosite
                </Button>
              </div>
            </div>
          )}
        </div>

        {/* 后处理操作 */}
        <div className="overflow-hidden rounded-2xl border border-border bg-card shadow-[var(--shadow-xs)]">
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
            <div className="p-4 space-y-3 bg-card">
              {/* 操作列表 */}
              {formData.transforms?.map((transform, index) => (
                <TransformCard
                  key={transformKeys[index] ?? `transform-${index}`}
                  transform={transform}
                  sources={formData.sources || []}
                  transformers={transformers}
                  onChange={(updates) => updateTransform(index, updates)}
                  onRemove={() => removeTransform(index)}
                  onDragStart={() => handleDragStart(index)}
                  onDragOver={(e) => handleDragOver(e, index)}
                  onDragEnd={handleDragEnd}
                  isDragging={draggedIndex === index}
                />
              ))}

              {/* 添加操作按钮 */}
              <div className="rounded-2xl border border-dashed border-border bg-surface-subtle/70 p-4 shadow-[var(--shadow-xs)]">
                <p className="text-sm text-muted-foreground mb-3 flex items-center gap-2">
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
        <div className="overflow-hidden rounded-2xl border border-border bg-card shadow-[var(--shadow-xs)]">
          <SectionHeader
            title="合并配置"
            help={HELP_TEXTS.merge}
            expanded={expandedSections.has("merge")}
            onToggle={() => toggleSection("merge")}
          />
          {expandedSections.has("merge") && (
            <div className="bg-card p-4">
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
                    <span className="text-sm text-muted-foreground">合并后去重</span>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* 输出配置 */}
        <div className="overflow-hidden rounded-2xl border border-border bg-card shadow-[var(--shadow-xs)]">
          <SectionHeader
            title="输出配置"
            help={HELP_TEXTS.outputClients}
            expanded={expandedSections.has("output")}
            onToggle={() => toggleSection("output")}
          />
          {expandedSections.has("output") && (
            <div className="p-4 space-y-4 bg-card">
              <div className="space-y-2">
                <Label>输出客户端</Label>
                <div className="flex flex-wrap gap-3">
                  {clientsList.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                      暂无可用客户端，请先在客户端管理中添加。
                    </p>
                  ) : (
                    clientsList.map((client) => (
                      <label
                        key={client.id}
                        className={`flex items-center gap-2 rounded-xl border p-3 cursor-pointer transition-all ${formData.output.clients.includes(client.id)
                          ? "border-primary/25 bg-primary-soft/70 shadow-[var(--shadow-xs)]"
                          : "border-border/50 bg-surface-subtle/60 hover:bg-accent/20"
                          }`}
                      >
                        <Checkbox
                          checked={formData.output.clients.includes(client.id)}
                          onCheckedChange={() => toggleClient(client.id as ClientType)}
                        />
                        <Monitor className="w-4 h-4" />
                        <span>{client.displayName}</span>
                      </label>
                    ))
                  )}
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
                        transformKeys={clientTransformKeys[client] || []}
                      />
                    );
                  })}
                </div>
              )}
            </div>
          )}
        </div>

      </div>

      <LocalContentDialog
        key={editingLocalContent ?? "closed"}
        open={editingLocalContent !== null}
        initialContent={editingLocalContent !== null ? (formData.sources?.[editingLocalContent]?.content ?? "") : ""}
        editorTheme={editorTheme}
        onSave={(content) => {
          if (editingLocalContent !== null) {
            updateSource(editingLocalContent, { content });
          }
        }}
        onClose={() => setEditingLocalContent(null)}
      />

      <PreviewDialog
        open={isPreviewOpen}
        onOpenChange={setIsPreviewOpen}
        ruleName={formData.name}
        isLoading={isPreviewLoading}
        previewData={previewData}
        clientsList={clientsList}
      />
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
  onDragStart?: () => void;
  onDragOver?: (e: React.DragEvent) => void;
  onDragEnd?: () => void;
  isDragging: boolean;
  draggable?: boolean;
  showTarget?: boolean;
}

function TransformCard({
  transform,
  sources,
  transformers,
  onChange,
  onRemove,
  onDragStart,
  onDragOver,
  onDragEnd,
  isDragging,
  draggable = true,
  showTarget = true,
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
      onDragOver={onDragOver}
      className={`rounded-2xl border border-border bg-surface-subtle/60 shadow-[var(--shadow-xs)] transition-all ${isDragging ? "scale-95 opacity-50" : ""
        }`}
    >
      <div className="flex items-center justify-between border-b border-border bg-background/55 p-3">
        <div className="flex items-center gap-2">
          {draggable && (
            <button
              type="button"
              draggable
              onDragStart={onDragStart}
              onDragEnd={onDragEnd}
            className="cursor-grab text-muted-foreground/50 hover:text-foreground active:cursor-grabbing rounded p-0.5 hover:bg-accent/50 transition-colors"
              title="拖动排序"
            >
              <GripVertical className="w-4 h-4" />
            </button>
          )}
          <button
            type="button"
            onClick={() => setExpanded(!expanded)}
            className="flex items-center gap-2 rounded-lg focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-primary/15"
          >
            {expanded ? (
              <ChevronDown className="w-4 h-4 text-muted-foreground" />
            ) : (
              <ChevronRight className="w-4 h-4 text-muted-foreground" />
            )}
            <Icon className="w-4 h-4 text-primary" />
            <span className="font-medium text-foreground">{getTypeLabel()}</span>
          </button>
          {showTarget && (
            <Badge variant="outline" className="text-xs">
              {getTargetLabel()}
            </Badge>
          )}
        </div>
        <div className="flex items-center gap-1">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={onRemove}
            className="w-8 h-8 text-muted-foreground hover:text-destructive"
            title="删除"
          >
            <Trash2 className="w-4 h-4" />
          </Button>
        </div>
      </div>

      {expanded && (
        <div className="space-y-3 p-3">
          {/* 目标来源选择 */}
          {showTarget && (
            <div className="space-y-2">
              <Label className="text-sm text-muted-foreground flex items-center gap-2">
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
                        className={`flex cursor-pointer items-center gap-1 rounded-lg border px-2.5 py-1.5 text-sm transition-colors ${isSelected
                          ? "border-primary/25 bg-primary-soft text-primary shadow-[var(--shadow-xs)]"
                          : "border-border bg-background hover:border-border-strong hover:bg-accent/20"
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
          )}

          {/* 类型特定字段 */}
          {transform.type === "use" && (
            <div className="space-y-2">
              <Label className="text-sm text-muted-foreground flex items-center gap-2">
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
                <p className="text-sm text-warning">暂无预定义转换器，请先在配置中添加</p>
              )}
            </div>
          )}

          {transform.type === "replace" && (
            <div className="space-y-3">
              <div className="space-y-2">
                <Label className="text-sm text-muted-foreground">正则标志</Label>
                <Select
                  value={transform.flags || "g"}
                  onValueChange={(value) => onChange({ flags: value })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="g">g（全局匹配）</SelectItem>
                    <SelectItem value="gm">gm（按行 + 全局）</SelectItem>
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">按行匹配请选 gm</p>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-2">
                  <Label className="text-sm text-muted-foreground flex items-center gap-2">
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
                  <Label className="text-sm text-muted-foreground">替换为</Label>
                  <Input
                    value={transform.replacement || ""}
                    onChange={(e) => onChange({ replacement: e.target.value })}
                    placeholder="替换内容（留空则删除）"
                  />
                </div>
              </div>
            </div>
          )}

          {transform.type === "remove_lines" && (
            <div className="space-y-2">
              <Label className="text-sm text-muted-foreground flex items-center gap-2">
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
  transformKeys: string[];
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
  transformKeys,
}: ClientOverrideSectionProps) {
  const [expanded, setExpanded] = useState(false);
  const useGlobalTransforms = config?.useGlobalTransforms ?? true;
  const transforms = config?.transforms || [];
  const hasGlobalTransforms = clientGlobalTransforms.length > 0;

  return (
    <div className="overflow-hidden rounded-2xl border border-border bg-card shadow-[var(--shadow-xs)]">
      {/* 标题栏 */}
      <div className="flex items-center justify-between border-b border-border/50 bg-surface-subtle/80 p-3">
        <button
          type="button"
          onClick={() => setExpanded(!expanded)}
          className="flex min-w-0 flex-1 items-center gap-2 rounded-lg focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-primary/15"
        >
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-muted-foreground shrink-0" />
          ) : (
            <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
          )}
          <Monitor className="w-4 h-4 shrink-0" />
          <span className="font-medium text-foreground truncate">
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
            <span className="text-sm text-muted-foreground">启用</span>
          </div>
        </div>
      </div>

      {/* 展开内容 */}
      {expanded && (config?.enabled ?? true) && (
        <div className="space-y-4 border-t border-border/50 bg-card p-3">
          {/* 全局转换继承开关 */}
          <div className="flex items-start gap-2 rounded-xl border border-border/50 bg-surface-subtle/60 p-3 shadow-[var(--shadow-xs)]">
            <Checkbox
              checked={useGlobalTransforms}
              onCheckedChange={(c) => onToggleUseGlobal(!!c)}
              className="mt-1"
            />
            <div className="space-y-1">
              <span className="text-sm font-medium">应用全局客户端转换</span>
              <p className="text-xs text-muted-foreground">
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
                  <p className="text-xs text-warning">
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
                key={transformKeys[index] ?? `${client}-transform-${index}`}
                transform={transform}
                sources={[]} // 客户端转换通常不针对特定来源，或者需要传递sources？这里简化处理
                transformers={transformers}
                onChange={(updates) => onUpdateTransform(index, updates)}
                onRemove={() => onRemoveTransform(index)}
                isDragging={false}
                onDragStart={() => { }}
                onDragOver={() => { }}
                onDragEnd={() => { }}
                draggable={false}
                showTarget={false}
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

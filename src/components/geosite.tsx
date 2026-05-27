"use client";

import { useCallback, useEffect, useMemo, useRef, useState, startTransition } from "react";
import {
  AlertTriangle,
  BookOpen,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Copy,
  Eye,
  Globe,
  HelpCircle,
  Loader2,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
  Square,
  CheckSquare,
  MinusSquare,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { SearchInput } from "@/components/ui/search-input";
import { EmptyState } from "@/components/ui/empty-state";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ScrollArea } from "@/components/ui/scroll-area";
import { CodeViewer } from "./code-viewer";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn, formatTimestamp } from "@/lib/utils";
import {
  getClients,
  getConfig,
  getGeositeCatalog,
  getGeositeProviders,
  importSelectedGeositeRules,
  lookupGeositeDomain,
  previewGeosite,
  previewRule,
  refreshRules,
  refreshGeositeProvider,
  syncGeositeProvider,
  saveConfig,
  batchDeleteRules,
  type ClientConfig,
  type GeositeCatalogItem,
  type GeositeProviderStatus,
  type GeositeStaleImport,
  type PreviewResponse,
} from "@/lib/api-client";
import { resolveOutputExt, type GeositeProvider, type RuleConfig, type RulesConfig, type BuiltinTransformer } from "@/lib/schema";
import { RuleEditor } from "./editor";
import { PreviewDialog } from "./editor-preview";
import { toast } from "sonner";
import { getPrimaryGeositeSource, isGeositeRule } from "@/lib/rule-classification";

const PROVIDERS: Array<{ id: GeositeProvider; label: string }> = [
  { id: "v2fly", label: "v2fly" },
  { id: "loyalsoldier", label: "Loyalsoldier" },
];

interface GeositeManagerProps {
  onRefresh?: () => void;
}

interface CatalogGroup {
  id: string;
  label: string;
  items: GeositeCatalogItem[];
}

interface RuleGroup {
  id: string;
  label: string;
  rules: RuleConfig[];
}

interface PreviewState {
  title: string;
  clientLabel: string;
  content: string;
  lineCount: number;
  truncated?: boolean;
  totalLines?: number;
}

interface SelectedImportItem {
  list: string;
  attrs?: string[];
}

function buildNameGroups<T>(items: T[], getName: (item: T) => string): Array<{ id: string; label: string; items: T[] }> {
  const prefixCounts = new Map<string, number>();
  for (const item of items) {
    const name = getName(item).toLowerCase();
    const [prefix, second] = name.split("-");
    if (prefix && second && prefix.length >= 3) {
      prefixCounts.set(prefix, (prefixCounts.get(prefix) || 0) + 1);
    }
  }

  const groupedPrefixes = new Set(
    Array.from(prefixCounts.entries())
      .filter(([, count]) => count >= 2)
      .map(([prefix]) => prefix)
  );

  const groups = new Map<string, T[]>();
  for (const item of items) {
    const lower = getName(item).toLowerCase();
    let key = "";
    if (lower.startsWith("geolocation-")) {
      key = "geolocation";
    } else if (lower.startsWith("category-")) {
      key = "category";
    } else {
      const [prefix] = lower.split("-");
      if (prefix && groupedPrefixes.has(prefix)) {
        key = prefix;
      } else {
        const initial = lower.charAt(0).toUpperCase();
        key = /^[A-Z]$/.test(initial) ? initial : "#";
      }
    }
    const bucket = groups.get(key) || [];
    bucket.push(item);
    groups.set(key, bucket);
  }

  const order = (value: string) => {
    if (value === "geolocation") return 0;
    if (value === "category") return 1;
    if (/^[A-Z#]$/.test(value)) return 3;
    return 2;
  };

  return Array.from(groups.entries())
    .sort((a, b) => {
      const diff = order(a[0]) - order(b[0]);
      if (diff !== 0) return diff;
      return a[0].localeCompare(b[0]);
    })
    .map(([label, groupItems]) => ({
      id: label,
      label,
      items: groupItems.sort((a, b) => getName(a).localeCompare(getName(b))),
    }));
}

function buildCatalogGroups(items: GeositeCatalogItem[]): CatalogGroup[] {
  return buildNameGroups(items, (item) => item.name);
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

function GeositeHelpContent() {
  return (
    <div className="space-y-5 text-sm text-foreground/80">
      <section className="space-y-2">
        <h3 className="text-base font-semibold text-foreground">默认输出格式</h3>
        <p className="leading-relaxed">
          从上游 Geosite 源（如 v2fly、loyalsoldier）获取的原始域名列表会被解析并转换为
          <strong className="text-foreground"> Mihomo (Clash Meta) Classical </strong>
          格式的路由规则。转换映射关系如下：
        </p>
        <div className="overflow-x-auto rounded-lg border border-border">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-muted/50">
              <tr>
                <th className="px-3 py-2 font-medium">上游语法</th>
                <th className="px-3 py-2 font-medium">示例</th>
                <th className="px-3 py-2 font-medium">转换结果</th>
                <th className="px-3 py-2 font-medium">匹配行为</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              <tr>
                <td className="px-3 py-2"><code className="rounded bg-muted px-1 py-0.5 text-xs">裸域名 / domain:</code></td>
                <td className="px-3 py-2"><code className="rounded bg-muted px-1 py-0.5 text-xs">alibaba.com</code></td>
                <td className="px-3 py-2"><code className="rounded bg-muted px-1 py-0.5 text-xs">DOMAIN-SUFFIX,alibaba.com</code></td>
                <td className="px-3 py-2">匹配域名及所有子域名</td>
              </tr>
              <tr>
                <td className="px-3 py-2"><code className="rounded bg-muted px-1 py-0.5 text-xs">full:</code></td>
                <td className="px-3 py-2"><code className="rounded bg-muted px-1 py-0.5 text-xs">full:www.alibaba.com</code></td>
                <td className="px-3 py-2"><code className="rounded bg-muted px-1 py-0.5 text-xs">DOMAIN,www.alibaba.com</code></td>
                <td className="px-3 py-2">精确匹配该域名</td>
              </tr>
              <tr>
                <td className="px-3 py-2"><code className="rounded bg-muted px-1 py-0.5 text-xs">keyword:</code></td>
                <td className="px-3 py-2"><code className="rounded bg-muted px-1 py-0.5 text-xs">keyword:google</code></td>
                <td className="px-3 py-2"><code className="rounded bg-muted px-1 py-0.5 text-xs">DOMAIN-KEYWORD,google</code></td>
                <td className="px-3 py-2">匹配包含关键词的域名</td>
              </tr>
              <tr>
                <td className="px-3 py-2"><code className="rounded bg-muted px-1 py-0.5 text-xs">regexp:</code></td>
                <td className="px-3 py-2"><code className="rounded bg-muted px-1 py-0.5 text-xs">{"regexp:^ad\\..*$"}</code></td>
                <td className="px-3 py-2"><code className="rounded bg-muted px-1 py-0.5 text-xs">{"DOMAIN-REGEX,^ad\\..*$"}</code></td>
                <td className="px-3 py-2">正则匹配域名</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p className="leading-relaxed text-muted-foreground">
          上游列表中不含 <code className="rounded bg-muted px-1 py-0.5 text-xs">.</code> 的裸名称（如{" "}
          <code className="rounded bg-muted px-1 py-0.5 text-xs">alibaba</code>、
          <code className="rounded bg-muted px-1 py-0.5 text-xs">taobao</code>）代表新通用顶级域名（gTLD），
          转换为 <code className="rounded bg-muted px-1 py-0.5 text-xs">DOMAIN-SUFFIX</code> 后可正确匹配整个 TLD 空间。
          <code className="rounded bg-muted px-1 py-0.5 text-xs">include:</code> 指令会在解析阶段自动展开合并，
          <code className="rounded bg-muted px-1 py-0.5 text-xs">@attr</code> 属性标签用于筛选子集（如{" "}
          <code className="rounded bg-muted px-1 py-0.5 text-xs">@ads</code>、
          <code className="rounded bg-muted px-1 py-0.5 text-xs">@!cn</code>）。
        </p>
      </section>

      <section className="space-y-2">
        <h3 className="text-base font-semibold text-foreground">适配其他客户端格式</h3>
        <p className="leading-relaxed">
          默认输出仅适用于 Mihomo (Clash Meta) Classical 格式。如果你的客户端使用其他路由规则格式（如 Surge、Quantumult X、Sing-Box 等），
          需要通过<strong className="text-foreground">转换器</strong>将默认输出转换为目标格式：
        </p>
        <ol className="list-decimal space-y-1.5 pl-5 leading-relaxed">
          <li>
            进入 <strong className="text-foreground">转换器管理</strong> 页面，创建一个脚本转换器，
            编写 <code className="rounded bg-muted px-1 py-0.5 text-xs">transform(content)</code> 函数将 Mihomo Classical 格式转换为目标客户端格式。
          </li>
          <li>
            进入 <strong className="text-foreground">客户端管理</strong> 页面，为对应客户端配置<strong className="text-foreground">全局转换器</strong>，
            选择刚创建的转换器。全局转换器会自动应用到该客户端的所有规则输出（包括 Geosite 规则）。
          </li>
        </ol>
        <p className="leading-relaxed text-muted-foreground">
          你也可以在单条规则的编辑界面中为其单独指定转换器，优先级高于客户端全局转换器。
        </p>
      </section>
    </div>
  );
}

export function GeositeManager({ onRefresh }: GeositeManagerProps) {
  const [providers, setProviders] = useState<GeositeProviderStatus[]>([]);
  const [provider, setProvider] = useState<GeositeProvider>("v2fly");
  const [clientId, setClientId] = useState("");
  const [clients, setClients] = useState<ClientConfig[]>([]);
  const [catalog, setCatalog] = useState<GeositeCatalogItem[]>([]);
  const [staleImports, setStaleImports] = useState<GeositeStaleImport[]>([]);
  const [isStaleDetailOpen, setIsStaleDetailOpen] = useState(false);
  const [isStaleCleaning, setIsStaleCleaning] = useState(false);
  const [config, setConfig] = useState<RulesConfig | null>(null);
  const [builtinTransformers, setBuiltinTransformers] = useState<BuiltinTransformer[]>([]);
  const [resolvedVersion, setResolvedVersion] = useState("");
  const [fetchedAt, setFetchedAt] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isUpdating, setIsUpdating] = useState(false);
  const [isImporting, setIsImporting] = useState(false);
  const [rulesSearch, setRulesSearch] = useState("");
  const [editingRule, setEditingRule] = useState<RuleConfig | null>(null);
  const [isEditorSaving, setIsEditorSaving] = useState(false);
  const [isEditorDirty, setIsEditorDirty] = useState(false);
  const [previewState, setPreviewState] = useState<PreviewState | null>(null);
  const [isPreviewLoading, setIsPreviewLoading] = useState(false);

  const [isImportDialogOpen, setIsImportDialogOpen] = useState(false);
  const [listSearch, setListSearch] = useState("");
  const [domainQuery, setDomainQuery] = useState("");
  const [domainMatches, setDomainMatches] = useState<string[]>([]);
  const [isLookupLoading, setIsLookupLoading] = useState(false);
  const [selectedImportNames, setSelectedImportNames] = useState<string[]>([]);
  const [selectedImportAttrs, setSelectedImportAttrs] = useState<Record<string, string[]>>({});
  const [focusedListName, setFocusedListName] = useState("");
  const [expandedGroups, setExpandedGroups] = useState<string[]>([]);
  const [expandedRuleGroups, setExpandedRuleGroups] = useState<string[]>([]);
  const [selectedRuleNames, setSelectedRuleNames] = useState<string[]>([]);
  const [isBatchDeleting, setIsBatchDeleting] = useState(false);
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const [isBatchDialogOpen, setIsBatchDialogOpen] = useState(false);
  const [batchTagInput, setBatchTagInput] = useState("");
  const [batchAddTags, setBatchAddTags] = useState(true);
  const [batchReplaceTags, setBatchReplaceTags] = useState(false);
  const [batchClientIds, setBatchClientIds] = useState<string[]>([]);
  const [batchAddClients, setBatchAddClients] = useState(true);
  const [batchReplaceClients, setBatchReplaceClients] = useState(false);
  const [isBatchSaving, setIsBatchSaving] = useState(false);
  const [showHelp, setShowHelp] = useState(false);
  const previewRequestRef = useRef(0);
  // Full preview response (with reports) for rule-level previews using
  // PreviewDialog. Catalog previews use the simpler previewState below.
  const [rulePreviewData, setRulePreviewData] = useState<PreviewResponse | null>(null);
  const [rulePreviewName, setRulePreviewName] = useState("");
  const [isRulePreviewLoading, setIsRulePreviewLoading] = useState(false);
  // Monotonic request id used to ignore out-of-order fetchAll responses.
  // Without this, switching the provider quickly can let an older response
  // overwrite the catalog/resolvedVersion that belongs to the new provider.
  const fetchAllRequestRef = useRef(0);

  const fetchAll = useCallback(async (selectedProvider: GeositeProvider = provider) => {
    const reqId = ++fetchAllRequestRef.current;
    try {
      const [{ providers: providerList }, { clients: clientList }, configResp] = await Promise.all([
        getGeositeProviders(),
        getClients(),
        getConfig(),
      ]);
      const { config: latestConfig } = configResp;

      if (reqId !== fetchAllRequestRef.current) {
        // A newer fetchAll has started; bail out without touching state.
        return;
      }

      setProviders(providerList);
      setClients(clientList);
      setConfig(latestConfig);
      setBuiltinTransformers(configResp.builtinTransformers ?? []);
      if (clientList.length > 0) {
        setClientId((current) => current || clientList[0].id);
      }

      const activeProvider = providerList.find((item) => item.provider === selectedProvider);
      if (activeProvider?.ready) {
        const catalogResult = await getGeositeCatalog(selectedProvider);
        if (reqId !== fetchAllRequestRef.current) return;
        setCatalog(catalogResult.catalog);
        setResolvedVersion(catalogResult.resolvedVersion);
        setFetchedAt(catalogResult.fetchedAt);
        setStaleImports(catalogResult.staleImports || []);
      } else {
        setCatalog([]);
        setResolvedVersion("");
        setFetchedAt("");
        setStaleImports([]);
      }
    } catch (error) {
      if (reqId !== fetchAllRequestRef.current) return;
      toast.error("加载 Geosite 失败: " + String(error));
    } finally {
      // Only the latest in-flight request is allowed to close the global
      // loading indicator, otherwise a fast stale response flips it off
      // while the user is still waiting on the current provider.
      if (reqId === fetchAllRequestRef.current) {
        setIsLoading(false);
      }
    }
  }, [provider]);

  // Single effect drives both mount and provider-change refreshes. We also
  // clear the catalog up-front on provider switch so the UI never shows
  // the previous provider's lists while the new one is loading.
  useEffect(() => {
    startTransition(() => {
      setCatalog([]);
      setResolvedVersion("");
      setFetchedAt("");
      setStaleImports([]);
    });
    startTransition(() => { void fetchAll(provider); });
    // fetchAll already captures `provider` in its closure, so we depend on
    // both to keep the lint rule happy and to re-fire on provider switch.
  }, [fetchAll, provider]);

  const providerStatus = useMemo(
    () => providers.find((item) => item.provider === provider) || null,
    [provider, providers]
  );

  const importedCount = useMemo(() => catalog.filter((item) => item.imported).length, [catalog]);

  const managedRules = useMemo(() => {
    const query = rulesSearch.trim().toLowerCase();
    return (config?.rules || [])
      .filter((rule) => {
        if (!isGeositeRule(rule)) return false;
        const source = getPrimaryGeositeSource(rule);
        if (!source || source.provider !== provider) return false;
        if (clientId && !rule.output.clients.includes(clientId)) return false;
        if (query.length === 0) return true;
        return [rule.name, rule.displayName || "", source.list || ""].some((value) =>
          value.toLowerCase().includes(query)
        );
      })
      .sort((a, b) => {
        const aName = getPrimaryGeositeSource(a)?.list || a.displayName || a.name;
        const bName = getPrimaryGeositeSource(b)?.list || b.displayName || b.name;
        return aName.localeCompare(bName);
      });
  }, [clientId, config?.rules, provider, rulesSearch]);

  const importedRuleGroups = useMemo((): RuleGroup[] => {
    if (managedRules.length === 0) return [];
    const getRuleListName = (rule: RuleConfig) => {
      const source = getPrimaryGeositeSource(rule);
      return source?.list || rule.displayName || rule.name;
    };
    return buildNameGroups(managedRules, getRuleListName).map((g) => ({
      ...g,
      rules: g.items as RuleConfig[],
    }));
  }, [managedRules]);

  useEffect(() => {
    startTransition(() => {
      setExpandedRuleGroups((current) => {
        const valid = current.filter((id) => importedRuleGroups.some((g) => g.id === id));
        if (valid.length > 0) return valid;
        return importedRuleGroups.slice(0, 8).map((g) => g.id);
      });
    });
  }, [importedRuleGroups]);

  // Clear selection when managed rules change
  useEffect(() => {
    startTransition(() => { setSelectedRuleNames((current) => current.filter((name) => managedRules.some((r) => r.name === name))); });
  }, [managedRules]);

  const toggleRuleSelection = (name: string) => {
    setSelectedRuleNames((current) =>
      current.includes(name) ? current.filter((n) => n !== name) : [...current, name]
    );
  };

  const toggleRuleGroupSelection = (group: RuleGroup) => {
    const groupNames = group.rules.map((r) => r.name);
    const everySelected = groupNames.every((name) => selectedRuleNames.includes(name));
    if (everySelected) {
      setSelectedRuleNames((current) => current.filter((name) => !groupNames.includes(name)));
    } else {
      setSelectedRuleNames((current) => Array.from(new Set([...current, ...groupNames])));
    }
  };

  const selectAllRules = () => {
    const allNames = managedRules.map((r) => r.name);
    const allSelected = allNames.length > 0 && allNames.every((name) => selectedRuleNames.includes(name));
    setSelectedRuleNames(allSelected ? [] : allNames);
  };

  const getGroupSelectionState = (group: RuleGroup): "all" | "some" | "none" => {
    const groupNames = group.rules.map((r) => r.name);
    const selectedInGroup = groupNames.filter((name) => selectedRuleNames.includes(name)).length;
    if (selectedInGroup === 0) return "none";
    if (selectedInGroup === groupNames.length) return "all";
    return "some";
  };

  const handleBatchDelete = async () => {
    if (selectedRuleNames.length === 0) return;
    setIsBatchDeleting(true);
    try {
      const result = await batchDeleteRules(selectedRuleNames);
      const failed = result.notFound.length + result.blocked.length;
      if (failed > 0) {
        toast.warning(`已删除 ${result.deleted.length} 条，${failed} 条失败`);
      } else {
        toast.success(`已删除 ${result.deleted.length} 条规则`);
      }
    } catch {
      toast.error("批量删除失败");
    }
    setSelectedRuleNames([]);
    setIsDeleteDialogOpen(false);
    await fetchAll(provider);
    onRefresh?.();
    setIsBatchDeleting(false);
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

  const handleDeleteStaleImports = async () => {
    if (staleImports.length === 0) return;
    const ruleNames = staleImports
      .map((item) => item.ruleName)
      .filter((name): name is string => Boolean(name));
    if (ruleNames.length === 0) {
      toast.error("没有可删除的规则");
      return;
    }
    setIsStaleCleaning(true);
    try {
      const result = await batchDeleteRules(ruleNames);
      const failed = result.notFound.length + result.blocked.length;
      if (failed > 0) {
        toast.warning(`已删除 ${result.deleted.length} 条，${failed} 条失败`);
      } else {
        toast.success(`已清理 ${result.deleted.length} 条失踪规则`);
      }
      setIsStaleDetailOpen(false);
      await fetchAll(provider);
      onRefresh?.();
    } catch {
      toast.error("清理失败");
    } finally {
      setIsStaleCleaning(false);
    }
  };

  const toggleBatchClient = (targetClientId: string) => {
    setBatchClientIds((current) =>
      current.includes(targetClientId)
        ? current.filter((id) => id !== targetClientId)
        : [...current, targetClientId]
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
    if (selectedRuleNames.length === 0) return;

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
        if (!selectedSet.has(rule.name) || !isGeositeRule(rule)) {
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

      // The batch partial-sync endpoint is async (202 Accepted); the engine
      // runs in the background under a single global sync lock and the
      // dashboard's SyncProgressPill takes over progress reporting. We
      // intentionally do NOT await completion here so the dialog can close
      // immediately even when the affected rule set is large.
      let syncDispatched = false;
      let syncBusy = false;
      if (shouldUpdateClients) {
        try {
          await refreshRules(selectedRuleNames);
          syncDispatched = true;
        } catch (error) {
          const err = error as Error & { code?: string; status?: number };
          if (err.code === "SYNC_ALREADY_RUNNING" || err.status === 409) {
            syncBusy = true;
          } else {
            throw error;
          }
        }
      }

      setIsBatchDialogOpen(false);
      setSelectedRuleNames([]);
      await fetchAll(provider);
      onRefresh?.();

      const ruleCount = selectedRuleNames.length;
      if (syncBusy) {
        toast.warning(`已保存 ${ruleCount} 条规则；当前已有同步在进行，请待其完成后手动触发刷新`);
      } else if (syncDispatched) {
        toast.success(`已保存 ${ruleCount} 条规则，已在后台同步中`);
      } else {
        toast.success(`已处理 ${ruleCount} 条规则`);
      }
    } catch (error) {
      toast.error("批量处理失败: " + String(error));
    } finally {
      setIsBatchSaving(false);
    }
  };

  const visibleCatalog = useMemo(() => {
    const query = listSearch.trim().toLowerCase();
    const matchSet = domainQuery.trim() ? new Set(domainMatches) : null;
    return catalog.filter((item) => {
      const matchesSearch = query.length === 0 || item.name.toLowerCase().includes(query);
      const matchesDomain = !matchSet || matchSet.has(item.name);
      return matchesSearch && matchesDomain;
    });
  }, [catalog, domainMatches, domainQuery, listSearch]);

  const catalogGroups = useMemo(() => buildCatalogGroups(visibleCatalog), [visibleCatalog]);

  useEffect(() => {
    startTransition(() => {
      setExpandedGroups((current) => {
        const valid = current.filter((id) => catalogGroups.some((group) => group.id === id));
        if (valid.length > 0) return valid;
        return catalogGroups.slice(0, 8).map((group) => group.id);
      });
    });
  }, [catalogGroups]);

  useEffect(() => {
    if (!isImportDialogOpen) return;

    const trimmed = domainQuery.trim();
    if (!trimmed) {
      startTransition(() => {
        setDomainMatches([]);
        setIsLookupLoading(false);
      });
      return;
    }

    const timer = window.setTimeout(() => {
      setIsLookupLoading(true);
      void lookupGeositeDomain(provider, trimmed)
        .then(({ matches }) => {
          setDomainMatches(matches);
          if (matches[0]) {
            setFocusedListName(matches[0]);
          }
        })
        .catch((error) => {
          toast.error("域名查询失败: " + String(error));
          setDomainMatches([]);
        })
        .finally(() => {
          setIsLookupLoading(false);
        });
    }, 250);

    return () => window.clearTimeout(timer);
  }, [domainQuery, isImportDialogOpen, provider]);

  useEffect(() => {
    if (!isImportDialogOpen) return;
    if (visibleCatalog.some((item) => item.name === focusedListName)) return;
    startTransition(() => { setFocusedListName(visibleCatalog[0]?.name || ""); });
  }, [focusedListName, isImportDialogOpen, visibleCatalog]);

  const getSelectedAttrs = (name: string) => selectedImportAttrs[name] || [];
  const isSelected = (name: string) => selectedImportNames.includes(name);

  const toggleListSelection = (name: string) => {
    setSelectedImportNames((current) => {
      if (current.includes(name)) {
        return current.filter((item) => item !== name);
      }
      return [...current, name];
    });
    const item = catalog.find((catalogItem) => catalogItem.name === name);
    setSelectedImportAttrs((current) => ({
      ...current,
      [name]: item?.attrs || [],
    }));
    setFocusedListName(name);
  };

  const toggleAttrSelection = (name: string, attr: string) => {
    setSelectedImportAttrs((current) => {
      const nextAttrs = current[name]?.includes(attr)
        ? current[name].filter((item) => item !== attr)
        : [...(current[name] || []), attr];
      const normalized = Array.from(new Set(nextAttrs)).sort();
      return { ...current, [name]: normalized };
    });
  };

  const toggleGroupSelection = (group: CatalogGroup) => {
    setSelectedImportNames((current) => {
      const groupNames = group.items.map((item) => item.name);
      const everySelected = groupNames.every((name) => current.includes(name));
      if (everySelected) {
        return current.filter((name) => !groupNames.includes(name));
      }
      return Array.from(new Set([...current, ...groupNames]));
    });
    setSelectedImportAttrs((current) => {
      const next = { ...current };
      for (const item of group.items) {
        next[item.name] = item.attrs;
      }
      return next;
    });
    if (group.items[0]) {
      setFocusedListName(group.items[0].name);
    }
  };

  const isGroupFullySelected = (group: CatalogGroup) => group.items.every((item) => selectedImportNames.includes(item.name));
  const groupSelectedCount = (group: CatalogGroup) => group.items.filter((item) => selectedImportNames.includes(item.name)).length;

  const toggleGroupExpanded = (groupId: string) => {
    setExpandedGroups((current) =>
      current.includes(groupId) ? current.filter((item) => item !== groupId) : [...current, groupId]
    );
  };

  const handleRefreshProvider = async () => {
    setIsRefreshing(true);
    try {
      const result = await refreshGeositeProvider(provider);
      toast.success(`已刷新 ${provider} 上游缓存，共 ${result.catalogCount} 个列表`);
      await fetchAll(provider);
    } catch (error) {
      toast.error("刷新失败: " + String(error));
    } finally {
      setIsRefreshing(false);
    }
  };

  // When the provider cache is missing (first-time pull), just fetch the
  // upstream data — no sync needed since the data is already fresh.
  // When the provider already has cached data, refresh the cache and then
  // sync only the geosite rules belonging to this provider.
  const handleUpdateImported = async () => {
    setIsUpdating(true);
    try {
      if (!providerStatus?.ready) {
        // First-time pull: fetch upstream data only.
        const result = await refreshGeositeProvider(provider);
        await fetchAll(provider);
        toast.success(`${provider} 数据已拉取：缓存 ${result.catalogCount} 个列表`);
      } else {
        // Subsequent update: refresh cache + sync provider's geosite rules.
        const result = await syncGeositeProvider(provider);
        await fetchAll(provider);
        const failedCount = result.sync.failedRules.length;
        if (failedCount > 0) {
          toast.warning(`${provider} 已更新：缓存 ${result.catalogCount} 列表，同步 ${result.sync.syncedRules.length} 条规则，${failedCount} 条失败`);
        } else if (result.sync.syncedRules.length === 0) {
          toast.success(`${provider} 已更新：缓存 ${result.catalogCount} 列表，无已导入规则需同步`);
        } else {
          toast.success(`${provider} 已更新：缓存 ${result.catalogCount} 列表，同步 ${result.sync.syncedRules.length} 条规则`);
        }
      }
      onRefresh?.();
    } catch (error) {
      const msg = String(error);
      const status = (error as Error & { status?: number }).status;
      const code = (error as Error & { code?: string }).code;
      if (status === 409 || code === "SYNC_ALREADY_RUNNING" || msg.includes("SYNC_ALREADY_RUNNING")) {
        toast.info(`${provider} 缓存已刷新；已有同步在进行，跳过新一轮触发`);
      } else {
        toast.error("更新失败: " + msg);
      }
    } finally {
      setIsUpdating(false);
    }
  };

  const handleImportSelection = async () => {
    if (!clientId) {
      toast.error("请选择客户端");
      return;
    }
    if (selectedImportNames.length === 0) {
      toast.error("请选择要导入的列表");
      return;
    }

    setIsImporting(true);
    try {
      const selectedItems: SelectedImportItem[] = selectedImportNames.flatMap((list) => {
        const attrs = getSelectedAttrs(list);
        const items: SelectedImportItem[] = [{ list }];
        for (const attr of attrs) {
          items.push({ list, attrs: [attr] });
        }
        return items;
      });
      const result = await importSelectedGeositeRules(provider, clientId, selectedItems);
      toast.success(`新增 ${result.created}，更新 ${result.updated}`);
      setIsImportDialogOpen(false);
      await fetchAll(provider);
      onRefresh?.();
    } catch (error) {
      toast.error("导入失败: " + String(error));
    } finally {
      setIsImporting(false);
    }
  };

  const openImportDialog = () => {
    setSelectedImportNames([]);
    setSelectedImportAttrs({});
    setFocusedListName("");
    setListSearch("");
    setDomainQuery("");
    setDomainMatches([]);
    setIsImportDialogOpen(true);
  };

  const handleOpenRulePreview = async (rule: RuleConfig) => {
    const requestId = previewRequestRef.current + 1;
    previewRequestRef.current = requestId;
    setIsRulePreviewLoading(true);
    setRulePreviewName(rule.displayName || rule.name);
    setRulePreviewData(null);

    try {
      const result = await previewRule(rule.name);
      if (previewRequestRef.current !== requestId) return;
      setRulePreviewData(result);
    } catch (error) {
      if (previewRequestRef.current !== requestId) return;
      toast.error("预览失败: " + String(error));
      setRulePreviewData(null);
      setRulePreviewName("");
    } finally {
      if (previewRequestRef.current !== requestId) return;
      setIsRulePreviewLoading(false);
    }
  };

  const handleOpenCatalogPreview = async (listName: string) => {
    if (!clientId) {
      toast.error("请选择客户端");
      return;
    }

    const requestId = previewRequestRef.current + 1;
    previewRequestRef.current = requestId;
    setIsPreviewLoading(true);
    setPreviewState({
      title: `${provider}/${listName}`,
      clientLabel: clients.find((item) => item.id === clientId)?.displayName || clientId,
      content: "",
      lineCount: 0,
    });

    try {
      // Ask for the backend hard cap (500k lines); virtualised renderer
       // handles arbitrarily large content, so there's no reason to truncate.
      const result = await previewGeosite(provider, listName, clientId, [], 500000);
      if (previewRequestRef.current !== requestId) return;
      setPreviewState({
        title: `${provider}/${listName}`,
        clientLabel: clients.find((item) => item.id === clientId)?.displayName || clientId,
        content: result.content,
        lineCount: result.totalLines ?? 0,
        truncated: result.truncated,
        totalLines: result.totalLines,
      });
    } catch (error) {
      if (previewRequestRef.current !== requestId) return;
      toast.error("预览失败: " + String(error));
      setPreviewState(null);
    } finally {
      if (previewRequestRef.current !== requestId) return;
      setIsPreviewLoading(false);
    }
  };

  const handleCopyUrl = async (rule: RuleConfig) => {
    if (!clientId || !rule.output.clients.includes(clientId)) {
      toast.error("当前客户端没有这条规则");
      return;
    }

    const source = getPrimaryGeositeSource(rule);
    if (!source?.list) {
      toast.error("规则缺少来源");
      return;
    }

    const outputName = source.attrs && source.attrs.length > 0
      ? `${source.list}@${Array.from(new Set(source.attrs.map((item) => item.trim().toLowerCase()).filter(Boolean))).sort().join("+")}`
      : source.list;
    const ext = resolveOutputExt(clients.find((c) => c.id === clientId)?.outputExt);
    const url = `${window.location.origin}/Rules/${encodeURIComponent(clientId)}/geosite/${encodeURIComponent(provider)}/${encodeURIComponent(outputName)}.${ext}`;
    try {
      await navigator.clipboard.writeText(url);
      toast.success("已复制 URL");
    } catch {
      toast.error("复制失败");
    }
  };

  const selectedCount = selectedImportNames.length;

  if (isLoading) {
    return (
      <div className="flex h-full min-h-0 items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-6 overflow-hidden">
      <Card className="shrink-0 border border-border/70">
        <CardContent className="flex flex-col gap-4 px-5 py-5">
          <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
            <div className="flex flex-wrap gap-2">
              {PROVIDERS.map((item) => (
                <Button
                  key={item.id}
                  variant={provider === item.id ? "default" : "outline"}
                  size="sm"
                  onClick={() => setProvider(item.id)}
                >
                  {item.label}
                </Button>
              ))}
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <Select value={clientId} onValueChange={setClientId}>
                <SelectTrigger className="w-[220px]">
                  <SelectValue placeholder="选择客户端" />
                </SelectTrigger>
                <SelectContent>
                  {clients.map((client) => (
                    <SelectItem key={client.id} value={client.id}>
                      {client.displayName}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button variant="outline" onClick={handleUpdateImported} disabled={isUpdating}>
                {isUpdating ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : providerStatus?.ready ? <RefreshCw className="mr-2 h-4 w-4" /> : <Globe className="mr-2 h-4 w-4" />}
                {providerStatus?.ready ? "立即更新" : "拉取数据"}
              </Button>
              <Button variant="success" onClick={openImportDialog} disabled={catalog.length === 0}>
                <Plus className="mr-2 h-4 w-4" />
                导入
              </Button>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2 text-xs">
            <Badge variant="outline">
              <Globe className="mr-1 h-3 w-3" />
              {provider}
            </Badge>
            <Badge variant="secondary">{catalog.length} 列表</Badge>
            <Badge variant="secondary">{importedCount} 已导入</Badge>
            <Badge variant="secondary">{resolvedVersion || "未缓存"}</Badge>
            <Badge variant="secondary">{fetchedAt ? formatTimestamp(fetchedAt) : "未刷新"}</Badge>
            {providerStatus?.ready ? <Badge variant="emerald">可用</Badge> : <Badge variant="outline">待刷新</Badge>}
            <button
              onClick={() => setShowHelp(true)}
              className="ml-auto inline-flex items-center gap-1 text-muted-foreground transition-colors hover:text-primary"
            >
              <HelpCircle className="h-3.5 w-3.5" />
              <span>格式说明</span>
            </button>
          </div>

          {staleImports.length > 0 && (
            <div className="flex flex-col gap-3 rounded-xl border border-destructive/30 bg-destructive/5 p-4 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-start gap-2 text-sm text-destructive">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <div>
                  <p className="font-medium">
                    检测到 {staleImports.length} 个已导入的 list 在上游已被删除
                  </p>
                  <p className="text-xs text-destructive/80">
                    上游 catalog 中已不存在这些列表，关联规则将无法继续同步。建议查看详情后清理。
                  </p>
                </div>
              </div>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setIsStaleDetailOpen(true)}
                >
                  查看详情
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="flex min-h-0 flex-1 flex-col border border-border/70">
        <CardHeader className="shrink-0 gap-3 pb-4">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <CardTitle>已导入规则</CardTitle>
            <div className="flex w-full max-w-md items-center gap-2">
              <SearchInput
                placeholder="搜索已导入规则..."
                value={rulesSearch}
                onChange={(event) => setRulesSearch(event.target.value)}
                fullWidth
              />
            </div>
          </div>
          {managedRules.length > 0 && (
            <div className="flex flex-wrap items-center gap-2 pt-1">
              <Button variant="outline" size="sm" onClick={selectAllRules}>
                {managedRules.length > 0 && managedRules.every((r) => selectedRuleNames.includes(r.name)) ? (
                  <>取消全选</>
                ) : (
                  <>全选</>
                )}
              </Button>
              {selectedRuleNames.length > 0 && (
                <Button variant="outline" size="sm" onClick={openBatchDialog}>
                  批量处理 ({selectedRuleNames.length})
                </Button>
              )}
              {selectedRuleNames.length > 0 && (
                <Button variant="destructive" size="sm" onClick={() => setIsDeleteDialogOpen(true)}>
                  <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                  删除 ({selectedRuleNames.length})
                </Button>
              )}
              <Badge variant="secondary">{managedRules.length} 条</Badge>
              {selectedRuleNames.length > 0 && (
                <Badge variant="blue">已选 {selectedRuleNames.length}</Badge>
              )}
            </div>
          )}
        </CardHeader>
        <CardContent className="min-h-0 flex-1 pt-0">
          {managedRules.length === 0 ? (
            <div className="flex h-full items-center justify-center">
              <EmptyState icon={Plus} title="当前 Provider 还没有规则" action={<Button onClick={openImportDialog}><Plus className="mr-2 h-4 w-4" />导入</Button>} />
            </div>
          ) : (
            <ScrollArea className="h-full pr-2">
              <div className="space-y-3 pb-4">
                {importedRuleGroups.map((group) => {
                  const expanded = expandedRuleGroups.includes(group.id);
                  const selState = getGroupSelectionState(group);
                  const selectedInGroup = group.rules.filter((r) => selectedRuleNames.includes(r.name)).length;
                  return (
                    <div
                      key={group.id}
                      className={cn(
                        "group rounded-2xl border border-border/70 transition-colors",
                        selState === "all" && "border-success/30 bg-success/5",
                        selState === "some" && "border-primary/20 bg-primary/3"
                      )}
                    >
                      <div className="flex items-center justify-between gap-3 px-4 py-3">
                        <button
                          type="button"
                          onClick={() =>
                            setExpandedRuleGroups((current) =>
                              current.includes(group.id)
                                ? current.filter((id) => id !== group.id)
                                : [...current, group.id]
                            )
                          }
                          className="flex min-w-0 flex-1 items-center gap-2 text-left"
                        >
                          {expanded ? <ChevronDown className="h-4 w-4 text-muted-foreground" /> : <ChevronRight className="h-4 w-4 text-muted-foreground" />}
                          <span className="truncate text-sm font-semibold text-foreground">{group.label}</span>
                          <Badge variant="secondary">{group.rules.length}</Badge>
                          {selectedInGroup > 0 && <Badge variant="emerald">{selectedInGroup}</Badge>}
                        </button>
                        <button
                          type="button"
                          onClick={() => toggleRuleGroupSelection(group)}
                          className={cn(
                            "shrink-0 transition-colors p-1",
                            selState === "none" ? "opacity-0 group-hover:opacity-100" : "opacity-100"
                          )}
                          title={selState === "all" ? "取消选择分组" : "选择分组"}
                        >
                          {selState === "all" ? <CheckSquare className="h-4 w-4 text-success" /> : selState === "some" ? <MinusSquare className="h-4 w-4 text-primary" /> : <Square className="h-4 w-4" />}
                        </button>
                      </div>

                      {expanded && (
                        <div className="border-t border-border/70 px-4 py-3 space-y-3">
                          {group.rules.map((rule) => {
                            const source = getPrimaryGeositeSource(rule);
                            const listName = rule.displayName || source?.list || rule.name;
                            const attrLabel = source?.attrs?.length ? source.attrs.join(", ") : "";
                            const selected = selectedRuleNames.includes(rule.name);
                            return (
                              <div
                                key={rule.name}
                                className={cn(
                                  "group/grid-item grid gap-3 rounded-2xl border px-4 py-4 transition-colors lg:grid-cols-[28px_minmax(0,1fr)_220px_260px]",
                                  selected ? "border-success/30 bg-success/5" : "border-border/70"
                                )}
                              >
                                <div className="flex items-center justify-center pt-1">
                                  <button
                                    type="button"
                                    onClick={() => toggleRuleSelection(rule.name)}
                                    className={cn(
                                      "transition-opacity",
                                      selected ? "opacity-100" : "opacity-0 group-hover/grid-item:opacity-100"
                                    )}
                                  >
                                    {selected
                                      ? <CheckSquare className="h-4 w-4 text-success" />
                                      : <Square className="h-4 w-4 text-muted-foreground" />
                                    }
                                  </button>
                                </div>
                                <div className="min-w-0">
                                  <div className="truncate text-base font-semibold text-foreground">{listName}</div>
                                  <div className="mt-1 truncate text-xs text-muted-foreground">{rule.name}</div>
                                  {attrLabel ? <div className="mt-1 truncate text-xs text-muted-foreground">attrs: {attrLabel}</div> : null}
                                </div>

                                <div className="flex flex-wrap content-start gap-2">
                                  {rule.output.clients.map((client) => (
                                    <Badge key={`${rule.name}-${client}`} variant={client === clientId ? "blue" : "secondary"}>
                                      {clients.find((item) => item.id === client)?.displayName || client}
                                    </Badge>
                                  ))}
                                </div>

                                <div className="flex flex-wrap justify-start gap-2 lg:justify-end">
                                  <Button variant="outline" size="sm" onClick={() => handleOpenRulePreview(rule)}>
                                    <Eye className="mr-1 h-3.5 w-3.5" />
                                    预览
                                  </Button>
                                  <Button variant="outline" size="sm" onClick={() => handleCopyUrl(rule)}>
                                    <Copy className="mr-1 h-3.5 w-3.5" />
                                    URL
                                  </Button>
                                  <Button variant="outline" size="sm" onClick={() => setEditingRule(rule)}>
                                    <Pencil className="mr-1 h-3.5 w-3.5" />
                                    编辑
                                  </Button>
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </ScrollArea>
          )}
        </CardContent>
      </Card>

      <Dialog open={isImportDialogOpen} onOpenChange={setIsImportDialogOpen}>
        <DialogContent className="flex h-[86vh] max-w-7xl flex-col overflow-hidden p-0">
          <DialogHeader className="shrink-0 border-b border-border px-6 py-5 pr-12">
            <div className="flex items-center gap-3">
              <DialogTitle>导入</DialogTitle>
              <Button variant="outline" size="sm" onClick={handleRefreshProvider} disabled={isRefreshing}>
                {isRefreshing ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <RefreshCw className="mr-2 h-4 w-4" />}
                刷新
              </Button>
            </div>
          </DialogHeader>

          <div className="shrink-0 border-b border-border px-6 py-4">
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <Badge variant="outline">
                <Globe className="mr-1 h-3 w-3" />
                {provider}
              </Badge>
              <Badge variant="secondary">
                {clients.find((client) => client.id === clientId)?.displayName || clientId || "未选择客户端"}
              </Badge>
            </div>
          </div>

          <div className="grid min-h-0 flex-1 gap-6 px-6 py-6 xl:grid-cols-[420px_minmax(0,1fr)]">
            <Card className="flex min-h-0 flex-col border border-border/70">
              <CardHeader className="shrink-0 pb-4">
                <CardTitle>域名查询</CardTitle>
              </CardHeader>
              <CardContent className="flex min-h-0 flex-1 flex-col gap-4">
                <SearchInput
                  placeholder="输入域名，例如 steampowered.com.8686c.com"
                  value={domainQuery}
                  onChange={(event) => setDomainQuery(event.target.value)}
                  fullWidth
                />

                <div className="min-h-0 flex-1 overflow-hidden rounded-2xl border border-border/70">
                  <ScrollArea className="h-full">
                    <div className="p-4">
                      {isLookupLoading ? (
                        <div className="flex items-center justify-center py-12">
                          <Loader2 className="h-6 w-6 animate-spin text-primary" />
                        </div>
                      ) : domainQuery.trim().length === 0 ? (
                        <div className="py-4 text-sm text-muted-foreground">输入域名进行查询</div>
                      ) : domainMatches.length === 0 ? (
                        <div className="py-4 text-sm text-muted-foreground">没有匹配结果</div>
                      ) : (
                        <div className="space-y-2">
                          {domainMatches.map((name) => (
                            <button
                              key={name}
                              type="button"
                              onClick={() => setFocusedListName(name)}
                              className={cn(
                                "flex w-full items-center justify-between rounded-xl px-3 py-2 text-left transition-colors",
                                focusedListName === name ? "bg-primary/8" : "hover:bg-accent"
                              )}
                            >
                              <span className="truncate text-sm font-medium text-foreground">{name}</span>
                              <span className="text-xs text-muted-foreground">匹配</span>
                            </button>
                          ))}
                        </div>
                      )}
                    </div>
                  </ScrollArea>
                </div>
              </CardContent>
            </Card>

            <Card className="flex min-h-0 flex-col border border-border/70">
              <CardHeader className="shrink-0 gap-4 pb-4">
                <div className="flex items-center gap-3">
                  <SearchInput
                    placeholder="搜索分组名称..."
                    value={listSearch}
                    onChange={(event) => setListSearch(event.target.value)}
                    fullWidth
                  />
                    <Button
                      variant="outline"
                      size="sm"
                      className="shrink-0"
                      onClick={() => {
                        const allNames = visibleCatalog.map((item) => item.name);
                        const allSelected = allNames.length > 0 && allNames.every((name) => selectedImportNames.includes(name));
                        setSelectedImportNames(allSelected ? [] : allNames);
                        setSelectedImportAttrs((current) => {
                          const next = { ...current };
                          for (const item of visibleCatalog) {
                            next[item.name] = item.attrs;
                          }
                          return next;
                        });
                      }}
                    >
                      {visibleCatalog.length > 0 && visibleCatalog.every((item) => selectedImportNames.includes(item.name)) ? "取消全选" : "全选"}
                    </Button>
                </div>
              </CardHeader>
              <CardContent className="min-h-0 flex-1 pt-0">
                <ScrollArea className="h-full pr-2">
                  <div className="space-y-3 pb-4">
                    {catalogGroups.map((group) => {
                      const expanded = expandedGroups.includes(group.id);
                      const fullySelected = isGroupFullySelected(group);
                      const selectedInGroup = groupSelectedCount(group);
                      return (
                        <div
                          key={group.id}
                          className={cn(
                            "rounded-2xl border border-border/70 transition-colors",
                            fullySelected && "border-success/30 bg-success/5"
                          )}
                        >
                          <div className="flex items-center justify-between gap-3 px-4 py-3">
                            <button
                              type="button"
                              onClick={() => toggleGroupExpanded(group.id)}
                              className="flex min-w-0 flex-1 items-center gap-2 text-left"
                            >
                              {expanded ? <ChevronDown className="h-4 w-4 text-muted-foreground" /> : <ChevronRight className="h-4 w-4 text-muted-foreground" />}
                              <span className="truncate text-sm font-semibold text-foreground">{group.label}</span>
                              <Badge variant="secondary">{group.items.length}</Badge>
                              {selectedInGroup > 0 ? <Badge variant="emerald">{selectedInGroup}</Badge> : null}
                            </button>
                            <Button variant="ghost" size="sm" className="shrink-0" onClick={() => toggleGroupSelection(group)}>
                              {fullySelected ? "取消分组" : "选中分组"}
                            </Button>
                          </div>

                          {expanded ? (
                            <div className="border-t border-border/70 px-2 py-2">
                              {group.items.map((item) => {
                                const selected = isSelected(item.name);
                                const focused = focusedListName === item.name;
                                return (
                                  <div
                                    key={item.name}
                                    className={cn(
                                      "group flex items-center gap-3 rounded-xl px-3 py-2 transition-colors",
                                      selected && "bg-success/10",
                                      !selected && focused && "bg-primary/8",
                                      !selected && !focused && "hover:bg-accent",
                                      item.imported && !selected && "text-success"
                                    )}
                                  >
                                    <div
                                      role="button"
                                      tabIndex={0}
                                      onClick={() => toggleListSelection(item.name)}
                                      onKeyDown={(event) => {
                                        if (event.key === "Enter" || event.key === " ") {
                                          event.preventDefault();
                                          toggleListSelection(item.name);
                                        }
                                      }}
                                      className="min-w-0 flex-1 cursor-pointer text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/30 focus-visible:ring-offset-2 focus-visible:ring-offset-background"
                                    >
                                      <div className="flex items-start justify-between gap-3">
                                        <div className="min-w-0">
                                          <div className="truncate text-sm font-medium">{item.name}</div>
                                          {getSelectedAttrs(item.name).length > 0 ? (
                                            <div className="mt-2 text-[11px] text-primary">
                                              将导入基础规则 + {getSelectedAttrs(item.name).length} 个属性规则
                                            </div>
                                          ) : null}
                                        </div>
                                        <Badge variant="secondary" className="shrink-0 text-[10px]">{item.entryCount}</Badge>
                                      </div>
                                      <div className="mt-2 flex flex-wrap gap-1">
                                        {item.attrs.slice(0, 6).map((attr) => (
                                          <button
                                            key={`${item.name}-${attr}`}
                                            type="button"
                                            onClick={(event) => {
                                              event.stopPropagation();
                                              toggleAttrSelection(item.name, attr);
                                            }}
                                            className="inline-flex"
                                          >
                                            <Badge
                                              variant={getSelectedAttrs(item.name).includes(attr) ? "blue" : "outline"}
                                              className="text-[10px]"
                                            >
                                              @{attr}
                                            </Badge>
                                          </button>
                                        ))}
                                        {item.attrs.length > 6 ? (
                                          <Badge variant="outline" className="text-[10px]">+{item.attrs.length - 6}</Badge>
                                        ) : null}
                                      </div>
                                    </div>
                                    <div className="flex items-center gap-2">
                                      {item.imported ? <CheckCircle2 className="h-4 w-4 text-success" /> : null}
                                      <Button
                                        variant="ghost"
                                        size="sm"
                                        className="h-7 px-2 opacity-0 transition-opacity group-hover:opacity-100"
                                        onClick={() => handleOpenCatalogPreview(item.name)}
                                        disabled={!clientId}
                                      >
                                        <Eye className="h-3.5 w-3.5" />
                                      </Button>
                                    </div>
                                  </div>
                                );
                              })}
                            </div>
                          ) : null}
                        </div>
                      );
                    })}
                  </div>
                </ScrollArea>
              </CardContent>
            </Card>
          </div>

          <DialogFooter className="shrink-0 border-t border-border px-6 py-4">
            <Button variant="outline" onClick={() => setIsImportDialogOpen(false)}>
              关闭
            </Button>
            <Button variant="success" onClick={handleImportSelection} disabled={isImporting || selectedCount === 0 || !clientId}>
              {isImporting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Plus className="mr-2 h-4 w-4" />}
              导入
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!previewState} onOpenChange={(open) => !open && setPreviewState(null)}>
        <DialogContent className="max-w-5xl">
          <DialogHeader>
            <DialogTitle className="flex flex-wrap items-center gap-2">
              <span>{previewState?.title}</span>
              <Badge variant="secondary">{previewState?.clientLabel}</Badge>
              <Badge variant="outline">{previewState?.lineCount || 0} 行</Badge>
              {previewState?.truncated ? (
                <Badge variant="outline" className="border-warning/30 bg-warning-soft text-warning">
                  内容已截断
                </Badge>
              ) : null}
            </DialogTitle>
          </DialogHeader>
          <div className="h-[70vh]">
            <CodeViewer
              content={previewState?.content || ""}
              loading={isPreviewLoading}
              className="h-full"
              height="100%"
            />
          </div>
        </DialogContent>
      </Dialog>

      {/* Rule preview with transform pipeline report */}
      <PreviewDialog
        open={!!rulePreviewData || isRulePreviewLoading}
        onOpenChange={(open) => {
          if (!open) {
            setRulePreviewData(null);
            setRulePreviewName("");
          }
        }}
        ruleName={rulePreviewName}
        isLoading={isRulePreviewLoading}
        previewData={rulePreviewData}
        clientsList={clients}
        transformers={config?.transformers}
      />

      {editingRule && config ? (
        <Dialog
          open={!!editingRule}
          onOpenChange={(open) => {
            if (open) return;
            if (isEditorSaving) return;
            if (isEditorDirty) {
              const ok = typeof window !== "undefined"
                ? window.confirm("有未保存的修改，确定要放弃吗？")
                : true;
              if (!ok) return;
            }
            setEditingRule(null);
          }}
        >
          <DialogContent
            className="max-h-[92vh] max-w-6xl overflow-auto p-0"
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
            <DialogTitle className="sr-only">编辑规则: {editingRule.name || '新建规则'}</DialogTitle>
            <RuleEditor
              rule={editingRule}
              config={config}
              builtinTransformers={builtinTransformers}
              onSavingChange={setIsEditorSaving}
              onDirtyChange={setIsEditorDirty}
              onSave={async () => {
                setEditingRule(null);
                await fetchAll(provider);
                onRefresh?.();
              }}
              onCancel={() => setEditingRule(null)}
            />
          </DialogContent>
        </Dialog>
      ) : null}

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
              <Label htmlFor="geosite-batch-tags">TAG</Label>
              <Input
                id="geosite-batch-tags"
                value={batchTagInput}
                onChange={(event) => setBatchTagInput(event.target.value)}
                placeholder="tag1, tag2"
              />
              <div className="flex flex-wrap items-center gap-4">
                <div className="flex items-center gap-2">
                  <Checkbox
                    id="geosite-batch-tag-add"
                    checked={batchAddTags}
                    onCheckedChange={(checked) => updateBatchTagMode("add", checked === true)}
                  />
                  <Label htmlFor="geosite-batch-tag-add">新增</Label>
                </div>
                <div className="flex items-center gap-2">
                  <Checkbox
                    id="geosite-batch-tag-replace"
                    checked={batchReplaceTags}
                    onCheckedChange={(checked) => updateBatchTagMode("replace", checked === true)}
                  />
                  <Label htmlFor="geosite-batch-tag-replace">覆盖</Label>
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
                        batchClientIds.map((selectedClientId) => (
                          <Badge key={selectedClientId} variant="blue">
                            {clients.find((client) => client.id === selectedClientId)?.displayName || selectedClientId}
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
                    id="geosite-batch-client-add"
                    checked={batchAddClients}
                    onCheckedChange={(checked) => updateBatchClientMode("add", checked === true)}
                  />
                  <Label htmlFor="geosite-batch-client-add">新增</Label>
                </div>
                <div className="flex items-center gap-2">
                  <Checkbox
                    id="geosite-batch-client-replace"
                    checked={batchReplaceClients}
                    onCheckedChange={(checked) => updateBatchClientMode("replace", checked === true)}
                  />
                  <Label htmlFor="geosite-batch-client-replace">覆盖</Label>
                </div>
              </div>
            </div>
          </div>
          <DialogFooter className="mt-6 gap-3">
            <Button variant="outline" onClick={() => setIsBatchDialogOpen(false)} disabled={isBatchSaving}>
              取消
            </Button>
            <Button onClick={handleBatchSave} disabled={isBatchSaving}>
              {isBatchSaving ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Help Dialog */}
      <Dialog open={showHelp} onOpenChange={setShowHelp}>
        <DialogContent className="max-w-2xl w-[90vw] max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <BookOpen className="w-5 h-5 text-primary" />
              Geosite 格式说明
            </DialogTitle>
            <DialogDescription>了解上游内容的默认转换格式与自定义适配方式</DialogDescription>
          </DialogHeader>
          <div className="prose dark:prose-invert max-w-none text-sm space-y-4">
            <GeositeHelpContent />
          </div>
        </DialogContent>
      </Dialog>

      {/* Batch Delete Confirmation Dialog */}
      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Trash2 className="h-5 w-5 text-destructive" />
              确认批量删除
            </DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            确定要删除 <strong className="text-foreground">{selectedRuleNames.length}</strong> 条 Geosite 规则吗？
          </p>
          <p className="text-xs text-destructive">此操作将同时删除所有客户端的规则文件，且无法恢复。</p>
          <DialogFooter className="gap-2 mt-2">
            <Button variant="outline" onClick={() => setIsDeleteDialogOpen(false)} disabled={isBatchDeleting}>
              取消
            </Button>
            <Button variant="destructive" onClick={handleBatchDelete} disabled={isBatchDeleting}>
              {isBatchDeleting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Trash2 className="mr-2 h-4 w-4" />}
              确认删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Stale Imports Detail Dialog */}
      <Dialog open={isStaleDetailOpen} onOpenChange={setIsStaleDetailOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-destructive" />
              已失踪的 Geosite 列表
            </DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            上游 catalog 中已不存在以下 list，关联规则在下次同步时会失败。一键删除可同步移除规则与已发布的产物文件。
          </p>
          <ScrollArea className="max-h-[50vh] rounded-lg border border-border/60">
            <div className="divide-y divide-border/60">
              {staleImports.map((item) => (
                <div key={item.name} className="flex flex-col gap-1 p-3 text-sm">
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge variant="outline" className="font-mono">{item.name}</Badge>
                    <Badge variant="secondary" className="font-mono text-xs">
                      {item.ruleName}
                    </Badge>
                  </div>
                  {item.clients.length > 0 && (
                    <div className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
                      <span>影响客户端:</span>
                      {item.clients.map((cid) => (
                        <Badge key={cid} variant="outline" className="text-[10px]">
                          {clients.find((c) => c.id === cid)?.displayName || cid}
                        </Badge>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </ScrollArea>
          <DialogFooter className="gap-2 mt-2">
            <Button variant="outline" onClick={() => setIsStaleDetailOpen(false)} disabled={isStaleCleaning}>
              关闭
            </Button>
            <Button variant="destructive" onClick={handleDeleteStaleImports} disabled={isStaleCleaning}>
              {isStaleCleaning ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Trash2 className="mr-2 h-4 w-4" />}
              删除全部失踪规则 ({staleImports.length})
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

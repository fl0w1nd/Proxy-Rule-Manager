"use client";

import { useMemo } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Plus, RotateCcw, Trash2 } from "lucide-react";
import {
  DEFAULT_SHADOWROCKET_MAPPING,
  type ShadowrocketAction,
  type ShadowrocketMapping,
  type ShadowrocketParams,
  SHADOWROCKET_ACTIONS,
  DEFAULT_SINGBOX_SOURCE_MAPPING,
  DEFAULT_SINGBOX_SOURCE_VERSION,
  SINGBOX_SOURCE_ACTIONS,
  SINGBOX_SOURCE_FIELDS,
  SINGBOX_SOURCE_VERSIONS,
  singboxFieldMinVersion,
  type SingboxSourceAction,
  type SingboxSourceField,
  type SingboxSourceMapping,
  type SingboxSourceParams,
  type SingboxSourceVersion,
} from "@/lib/schema";
import { AlertTriangle } from "lucide-react";

// BUILTIN_PARAMS_KEY enumerates the builtin: names we currently know how to
// edit inline. Anything else falls back to a hidden params editor (the user
// can still hand-edit the JSON via the API). Keep this in sync with the
// switch below so future builtins are easy to slot in.
const SHADOWROCKET_BUILTIN = "builtin:mihomo-to-shadowrocket";
const SINGBOX_SOURCE_BUILTIN = "builtin:mihomo-classical-to-singbox-source";

// isBuiltinConfigurable lets the transformers page decide whether to render
// the "edit params" affordance for a given built-in. Keep in lockstep with
// the switch inside BuiltinTransformParams.
export function isBuiltinConfigurable(name: string): boolean {
  return name === SHADOWROCKET_BUILTIN || name === SINGBOX_SOURCE_BUILTIN;
}

// shadowrocketParamsFromUnknown is a tolerant decoder: we accept the
// already-typed object, undefined, or a raw blob from the API. We normalise
// every row defensively so a malformed entry (missing `type`, unknown
// action enum, etc.) can never reach the render path and crash the editor
// with a TypeError on `.trim()` or `.includes()` against `undefined`.
function shadowrocketParamsFromUnknown(input: unknown): ShadowrocketParams {
  if (!input || typeof input !== "object") {
    return { rules: DEFAULT_SHADOWROCKET_MAPPING, unknownAction: "keep" };
  }
  const obj = input as Record<string, unknown>;
  const rules: ShadowrocketMapping[] = Array.isArray(obj.rules)
    ? (obj.rules as unknown[]).map(sanitizeRule)
    : DEFAULT_SHADOWROCKET_MAPPING;
  const unknownAction = SHADOWROCKET_ACTIONS.includes(obj.unknownAction as ShadowrocketAction)
    ? (obj.unknownAction as ShadowrocketAction)
    : "keep";
  return { rules, unknownAction };
}

// sanitizeRule is intentionally permissive: an unknown shape becomes a
// "keep" row with empty type, which still renders cleanly in the editor
// (the operator can fill in the type or delete the row).
function sanitizeRule(raw: unknown): ShadowrocketMapping {
  const r = (raw && typeof raw === "object" ? raw : {}) as Record<string, unknown>;
  const action: ShadowrocketAction = SHADOWROCKET_ACTIONS.includes(r.action as ShadowrocketAction)
    ? (r.action as ShadowrocketAction)
    : "keep";
  return {
    type: typeof r.type === "string" ? r.type : "",
    action,
    renameTo: typeof r.renameTo === "string" ? r.renameTo : undefined,
    reason: typeof r.reason === "string" ? r.reason : undefined,
  };
}

const ACTION_LABEL: Record<ShadowrocketAction, string> = {
  keep: "保留",
  rename: "重命名",
  drop: "丢弃",
};

interface BuiltinTransformParamsProps {
  // The full builtin: name selected in the parent context.
  use: string;
  // Current params blob for this built-in, sourced from
  // RulesConfig.builtinParams[name] (typed as unknown to match the schema).
  params: unknown;
  // Owner notifies the editor to write a fresh params blob; passing
  // `undefined` clears the field so the backend falls back to defaults.
  onChange: (next: unknown) => void;
}

// BuiltinTransformParams renders the per-builtin params editor. It returns
// null for builtins that don't expose configuration so the call site can be
// "always mount, let the component decide".
export function BuiltinTransformParams({ use, params, onChange }: BuiltinTransformParamsProps) {
  if (use === SHADOWROCKET_BUILTIN) {
    return (
      <ShadowrocketMappingEditor params={params} onChange={onChange} />
    );
  }
  if (use === SINGBOX_SOURCE_BUILTIN) {
    return (
      <SingboxSourceMappingEditor params={params} onChange={onChange} />
    );
  }
  return null;
}

function ShadowrocketMappingEditor({
  params,
  onChange,
}: {
  params: unknown;
  onChange: (next: unknown) => void;
}) {
  const value = useMemo(() => shadowrocketParamsFromUnknown(params), [params]);

  const update = (next: ShadowrocketParams) => {
    // Trim type / renameTo on serialisation so saved configs stay tidy,
    // but leave reason untrimmed — it is a human-readable note where
    // spaces are meaningful and mid-edit trimming would swallow the
    // spacebar. The backend validates type independently.
    const cleaned: ShadowrocketParams = {
      ...next,
      rules: next.rules.map((r) => ({
        ...r,
        type: (r.type ?? "").trim(),
        renameTo: r.renameTo?.trim() || undefined,
      })),
    };
    onChange(cleaned);
  };

  const updateRow = (index: number, patch: Partial<ShadowrocketMapping>) => {
    const rules = value.rules.slice();
    rules[index] = { ...rules[index], ...patch };
    update({ ...value, rules });
  };

  const removeRow = (index: number) => {
    update({ ...value, rules: value.rules.filter((_, i) => i !== index) });
  };

  const addRow = () => {
    update({
      ...value,
      rules: [...value.rules, { type: "", action: "keep" }],
    });
  };

  const resetDefaults = () => {
    onChange({ rules: DEFAULT_SHADOWROCKET_MAPPING, unknownAction: "keep" });
  };

  return (
    <div className="space-y-3 rounded-lg border border-dashed border-border bg-surface-subtle/40 p-3">
      <div className="flex items-center justify-between gap-2">
        <div className="space-y-0.5">
          <Label className="text-sm text-foreground">映射表</Label>
          <p className="text-xs text-muted-foreground">
            决定每个 mihomo classical 规则类型在输出中的行为。未列入此表的类型按
            「未识别行为」处理。
          </p>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={resetDefaults}
          title="恢复内置默认映射"
        >
          <RotateCcw className="w-3.5 h-3.5 mr-1" /> 还原默认
        </Button>
      </div>

      <div className="space-y-2">
        <div className="grid grid-cols-12 gap-2 text-xs text-muted-foreground px-1">
          <div className="col-span-3">规则类型</div>
          <div className="col-span-2">动作</div>
          <div className="col-span-3">重命名为</div>
          <div className="col-span-3">备注（在预览中显示）</div>
          <div className="col-span-1 text-right">操作</div>
        </div>
        {value.rules.length === 0 && (
          <p className="text-xs text-muted-foreground italic px-1 py-2">
            当前映射表为空：所有规则类型都会按「未识别行为」处理。
          </p>
        )}
        {value.rules.map((row, idx) => (
          <div key={idx} className="grid grid-cols-12 gap-2 items-center">
            <Input
              className="col-span-3 h-8 font-mono text-sm"
              value={row.type}
              placeholder="DOMAIN-SUFFIX"
              onChange={(e) => updateRow(idx, { type: e.target.value })}
            />
            <div className="col-span-2">
              <Select
                value={row.action}
                onValueChange={(v) =>
                  updateRow(idx, { action: v as ShadowrocketAction })
                }
              >
                <SelectTrigger className="h-8">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SHADOWROCKET_ACTIONS.map((a) => (
                    <SelectItem key={a} value={a}>
                      {ACTION_LABEL[a]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Input
              className="col-span-3 h-8 font-mono text-sm"
              value={row.renameTo || ""}
              placeholder={row.action === "rename" ? "FINAL" : "（仅重命名时填写）"}
              disabled={row.action !== "rename"}
              onChange={(e) => updateRow(idx, { renameTo: e.target.value })}
            />
            <Input
              className="col-span-3 h-8 text-sm"
              value={row.reason || ""}
              placeholder="可选说明，会在预览中展示"
              onChange={(e) => updateRow(idx, { reason: e.target.value })}
            />
            <div className="col-span-1 flex justify-end">
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="w-7 h-7 text-muted-foreground hover:text-destructive"
                onClick={() => removeRow(idx)}
                title="删除"
              >
                <Trash2 className="w-3.5 h-3.5" />
              </Button>
            </div>
          </div>
        ))}
      </div>

      <div className="flex items-center justify-between gap-3 pt-1">
        <div className="flex items-center gap-2">
          <Label className="text-xs text-muted-foreground">未识别类型</Label>
          <Select
            value={value.unknownAction || "keep"}
            onValueChange={(v) =>
              update({ ...value, unknownAction: v as ShadowrocketAction })
            }
          >
            <SelectTrigger className="h-8 w-32">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {SHADOWROCKET_ACTIONS.filter((a) => a !== "rename").map((a) => (
                <SelectItem key={a} value={a}>
                  {ACTION_LABEL[a]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <span className="text-xs text-muted-foreground">
            未在映射表中出现的规则类型将按该动作处理（默认保留）。
          </span>
        </div>
        <Button type="button" variant="outline" size="sm" onClick={addRow}>
          <Plus className="w-3.5 h-3.5 mr-1" /> 新增类型
        </Button>
      </div>
    </div>
  );
}

// --- sing-box source mapping editor ---

const SINGBOX_ACTION_LABEL: Record<SingboxSourceAction, string> = {
  map: "映射",
  drop: "丢弃",
};

// singboxParamsFromUnknown mirrors shadowrocketParamsFromUnknown: it
// tolerantly decodes whatever the API returns into a typed editor
// state. Unknown action enums and out-of-range versions fall back to
// safe defaults so a malformed payload can't crash the editor with a
// TypeError on `.includes()` against `undefined`.
function singboxParamsFromUnknown(input: unknown): SingboxSourceParams {
  if (!input || typeof input !== "object") {
    return { rules: DEFAULT_SINGBOX_SOURCE_MAPPING, version: DEFAULT_SINGBOX_SOURCE_VERSION };
  }
  const obj = input as Record<string, unknown>;
  const rules: SingboxSourceMapping[] = Array.isArray(obj.rules)
    ? (obj.rules as unknown[]).map(sanitizeSingboxRule)
    : DEFAULT_SINGBOX_SOURCE_MAPPING;
  const versionRaw = typeof obj.version === "number" ? obj.version : DEFAULT_SINGBOX_SOURCE_VERSION;
  const version: SingboxSourceVersion = (SINGBOX_SOURCE_VERSIONS as readonly number[]).includes(
    versionRaw,
  )
    ? (versionRaw as SingboxSourceVersion)
    : DEFAULT_SINGBOX_SOURCE_VERSION;
  return { rules, version };
}

function sanitizeSingboxRule(raw: unknown): SingboxSourceMapping {
  const r = (raw && typeof raw === "object" ? raw : {}) as Record<string, unknown>;
  const action: SingboxSourceAction = SINGBOX_SOURCE_ACTIONS.includes(
    r.action as SingboxSourceAction,
  )
    ? (r.action as SingboxSourceAction)
    : "drop";
  return {
    type: typeof r.type === "string" ? r.type : "",
    action,
    mapTo: typeof r.mapTo === "string" ? r.mapTo : undefined,
    reason: typeof r.reason === "string" ? r.reason : undefined,
  };
}

function SingboxSourceMappingEditor({
  params,
  onChange,
}: {
  params: unknown;
  onChange: (next: unknown) => void;
}) {
  const value = useMemo(() => singboxParamsFromUnknown(params), [params]);
  const activeVersion = value.version || DEFAULT_SINGBOX_SOURCE_VERSION;
  // Surface a banner when any "map" row references a field newer than
  // the selected schema version. The banner is read-only; we still let
  // the save go through so the operator can pick "fix here or fix in
  // backend error", but the backend validator rejects the same combo
  // for safety.
  const incompatibleRows = useMemo(() => {
    const out: { idx: number; row: SingboxSourceMapping; minVer: number }[] = [];
    value.rules.forEach((r, idx) => {
      if (r.action !== "map" || !r.mapTo) return;
      const minVer = singboxFieldMinVersion(r.mapTo);
      if (minVer > activeVersion) {
        out.push({ idx, row: r, minVer });
      }
    });
    return out;
  }, [value.rules, activeVersion]);

  const update = (next: SingboxSourceParams) => {
    // Trim type / mapTo on serialisation so saved configs stay tidy,
    // but leave reason untrimmed for the same reason as shadowrocket
    // (spaces inside a human-readable note are meaningful and trimming
    // mid-edit would eat the spacebar).
    const cleaned: SingboxSourceParams = {
      ...next,
      rules: next.rules.map((r) => ({
        ...r,
        type: (r.type ?? "").trim(),
        mapTo: r.mapTo?.trim() || undefined,
      })),
    };
    onChange(cleaned);
  };

  const updateRow = (index: number, patch: Partial<SingboxSourceMapping>) => {
    const rules = value.rules.slice();
    rules[index] = { ...rules[index], ...patch };
    update({ ...value, rules });
  };

  const removeRow = (index: number) => {
    update({ ...value, rules: value.rules.filter((_, i) => i !== index) });
  };

  const addRow = () => {
    update({
      ...value,
      rules: [...value.rules, { type: "", action: "map", mapTo: "domain" }],
    });
  };

  const resetDefaults = () => {
    onChange({ rules: DEFAULT_SINGBOX_SOURCE_MAPPING, version: DEFAULT_SINGBOX_SOURCE_VERSION });
  };

  return (
    <div className="space-y-3 rounded-lg border border-dashed border-border bg-surface-subtle/40 p-3">
      <div className="flex items-center justify-between gap-2">
        <div className="space-y-0.5">
          <Label className="text-sm text-foreground">映射表</Label>
          <p className="text-xs text-muted-foreground">
            把 mihomo classical 规则类型映射到 sing-box headless rule 字段，或显式丢弃。
            未在映射表中的类型一律丢弃，避免在 rule-set 中混入 sing-box 不识别的键。
          </p>
          <p className="text-[11px] text-muted-foreground/80 leading-relaxed">
            尾部 policy（如 <code className="font-mono">,DIRECT</code>）和 <code className="font-mono">no-resolve</code> 等修饰符会被丢弃：sing-box rule-set 不携带这两类信息，
            出/入站归属在主配置 <code className="font-mono">route.rules</code>，DNS 解析行为由 <code className="font-mono">dns.rules</code> 决定，且默认不会为 rule-set 内的
            <code className="font-mono"> ip_cidr</code> 主动触发解析。
          </p>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={resetDefaults}
          title="恢复内置默认映射"
        >
          <RotateCcw className="w-3.5 h-3.5 mr-1" /> 还原默认
        </Button>
      </div>

      <div className="flex items-center gap-2 flex-wrap">
        <Label className="text-xs text-muted-foreground">rule-set 版本</Label>
        <Select
          value={String(activeVersion)}
          onValueChange={(v) =>
            update({ ...value, version: Number(v) as SingboxSourceVersion })
          }
        >
          <SelectTrigger className="h-8 w-28">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {SINGBOX_SOURCE_VERSIONS.map((v) => (
              <SelectItem key={v} value={String(v)}>
                version {v}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <span className="text-xs text-muted-foreground">
          对应 sing-box rule-set source-format 版本号；默认 {DEFAULT_SINGBOX_SOURCE_VERSION}（sing-box 1.11+）。
        </span>
      </div>

      {incompatibleRows.length > 0 && (
        <div className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-xs text-destructive">
          <AlertTriangle className="w-4 h-4 mt-0.5 shrink-0" />
          <div className="space-y-0.5">
            <div className="font-medium">以下映射目标字段高于当前 rule-set version，保存会被后端拒绝：</div>
            <ul className="list-disc pl-4">
              {incompatibleRows.map(({ idx, row, minVer }) => (
                <li key={idx} className="font-mono">
                  {row.type || "(未命名类型)"} → {row.mapTo}（需 ≥ v{minVer}）
                </li>
              ))}
            </ul>
            <div>请提高 rule-set 版本，或把这些行改为「丢弃」。</div>
          </div>
        </div>
      )}

      <div className="space-y-2">
        <div className="grid grid-cols-12 gap-2 text-xs text-muted-foreground px-1">
          <div className="col-span-3">规则类型</div>
          <div className="col-span-2">动作</div>
          <div className="col-span-3">sing-box 字段</div>
          <div className="col-span-3">备注（在预览中显示）</div>
          <div className="col-span-1 text-right">操作</div>
        </div>
        {value.rules.length === 0 && (
          <p className="text-xs text-muted-foreground italic px-1 py-2">
            当前映射表为空：所有规则类型都会被丢弃，输出仅含空 rules 数组。
          </p>
        )}
        {value.rules.map((row, idx) => {
          const mapToValue = row.mapTo || "";
          const isKnownField = (SINGBOX_SOURCE_FIELDS as readonly string[]).includes(mapToValue);
          const rowMinVer = isKnownField ? singboxFieldMinVersion(mapToValue) : 1;
          const rowIncompatible = row.action === "map" && isKnownField && rowMinVer > activeVersion;
          return (
            <div key={idx} className="grid grid-cols-12 gap-2 items-center">
              <Input
                className="col-span-3 h-8 font-mono text-sm"
                value={row.type}
                placeholder="DOMAIN-SUFFIX"
                onChange={(e) => updateRow(idx, { type: e.target.value })}
              />
              <div className="col-span-2">
                <Select
                  value={row.action}
                  onValueChange={(v) =>
                    updateRow(idx, { action: v as SingboxSourceAction })
                  }
                >
                  <SelectTrigger className="h-8">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {SINGBOX_SOURCE_ACTIONS.map((a) => (
                      <SelectItem key={a} value={a}>
                        {SINGBOX_ACTION_LABEL[a]}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="col-span-3">
                {row.action === "map" ? (
                  <Select
                    value={isKnownField ? mapToValue : ""}
                    onValueChange={(v) =>
                      updateRow(idx, { mapTo: v as SingboxSourceField })
                    }
                  >
                    <SelectTrigger
                      className={
                        "h-8 font-mono" +
                        (rowIncompatible ? " border-destructive text-destructive" : "")
                      }
                      title={
                        rowIncompatible
                          ? `字段 ${mapToValue} 需 rule-set version ≥ v${rowMinVer}，当前 v${activeVersion}`
                          : undefined
                      }
                    >
                      <SelectValue placeholder={isKnownField ? undefined : mapToValue || "选择字段"} />
                    </SelectTrigger>
                    <SelectContent>
                      {SINGBOX_SOURCE_FIELDS.map((f) => {
                        const minVer = singboxFieldMinVersion(f);
                        const disabled = minVer > activeVersion;
                        return (
                          <SelectItem
                            key={f}
                            value={f}
                            className="font-mono"
                            disabled={disabled}
                            title={
                              disabled
                                ? `需 rule-set version ≥ v${minVer}（当前 v${activeVersion}）`
                                : undefined
                            }
                          >
                            <span className="flex items-center gap-1.5">
                              <span>{f}</span>
                              {minVer > 1 && (
                                <span className="text-[10px] text-muted-foreground">
                                  ≥v{minVer}
                                </span>
                              )}
                            </span>
                          </SelectItem>
                        );
                      })}
                    </SelectContent>
                  </Select>
                ) : (
                  <Input
                    className="h-8 font-mono text-sm"
                    value={mapToValue}
                    placeholder="（仅映射时填写）"
                    disabled
                  />
                )}
              </div>
              <Input
                className="col-span-3 h-8 text-sm"
                value={row.reason || ""}
                placeholder="可选说明，会在预览中展示"
                onChange={(e) => updateRow(idx, { reason: e.target.value })}
              />
              <div className="col-span-1 flex justify-end">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="w-7 h-7 text-muted-foreground hover:text-destructive"
                  onClick={() => removeRow(idx)}
                  title="删除"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                </Button>
              </div>
            </div>
          );
        })}
      </div>

      <div className="flex items-center justify-end pt-1">
        <Button type="button" variant="outline" size="sm" onClick={addRow}>
          <Plus className="w-3.5 h-3.5 mr-1" /> 新增类型
        </Button>
      </div>
    </div>
  );
}

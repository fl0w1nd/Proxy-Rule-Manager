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
} from "@/lib/schema";

// BUILTIN_PARAMS_KEY enumerates the builtin: names we currently know how to
// edit inline. Anything else falls back to a hidden params editor (the user
// can still hand-edit the JSON via the API). Keep this in sync with the
// switch below so future builtins are easy to slot in.
const SHADOWROCKET_BUILTIN = "builtin:mihomo-to-shadowrocket";

// shadowrocketParamsFromUnknown is a tolerant decoder: we accept the
// already-typed object, undefined, or a raw blob from the API. Anything we
// don't recognise becomes the curated defaults so the editor never refuses
// to render.
function shadowrocketParamsFromUnknown(input: unknown): ShadowrocketParams {
  if (!input || typeof input !== "object") {
    return { rules: DEFAULT_SHADOWROCKET_MAPPING, unknownAction: "keep" };
  }
  const obj = input as Partial<ShadowrocketParams> & Record<string, unknown>;
  const rules: ShadowrocketMapping[] = Array.isArray(obj.rules)
    ? (obj.rules as ShadowrocketMapping[])
    : DEFAULT_SHADOWROCKET_MAPPING;
  const unknownAction = SHADOWROCKET_ACTIONS.includes(obj.unknownAction as ShadowrocketAction)
    ? (obj.unknownAction as ShadowrocketAction)
    : "keep";
  return { rules, unknownAction };
}

const ACTION_LABEL: Record<ShadowrocketAction, string> = {
  keep: "保留",
  rename: "重命名",
  drop: "丢弃",
};

interface BuiltinTransformParamsProps {
  // The full builtin: name selected in the parent dropdown.
  use: string;
  // Current Transform.params blob (typed as unknown to match the schema).
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
    // Empty rule type rows would silently fall through to UnknownAction on
    // the backend; we keep them so the operator can finish typing mid-edit,
    // but trim them on serialisation so saved configs stay tidy.
    const cleaned: ShadowrocketParams = {
      ...next,
      rules: next.rules.map((r) => ({
        ...r,
        type: r.type.trim(),
        renameTo: r.renameTo?.trim() || undefined,
        reason: r.reason?.trim() || undefined,
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

"use client";

import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
  Loader2,
  CheckCircle,
  XCircle,
  ArrowRight,
  Filter,
  ChevronDown,
  ChevronRight,
  Workflow,
  GitBranch,
  Lock,
  AlertTriangle,
  Maximize2,
} from "lucide-react";
import { toast } from "sonner";
import { CodeViewer } from "./code-viewer";
import {
  ClientType,
  type ScriptTransformer,
  type StepReport,
  type TransformReport,
  TRANSFORM_STAGE,
  TRANSFORM_STEP_KIND,
} from "@/lib/schema";
import { PreviewResponse, ClientConfig } from "@/lib/api-client";

interface PreviewDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  ruleName: string;
  isLoading: boolean;
  previewData: PreviewResponse | null;
  clientsList: ClientConfig[];
  // Transformer descriptions for tooltip display on user-script step
  // reasons. When omitted, the reason text is shown without a tooltip.
  transformers?: Record<string, ScriptTransformer>;
  // Optional handler invoked when the user wants the full (untruncated)
  // content. The parent re-fetches the preview with a larger limit and
  // hands back a fresh `previewData`. When omitted, the truncation badge
  // becomes informational only.
  onLoadFull?: () => void;
  isReloadingFull?: boolean;
}

export function PreviewDialog({
  open,
  onOpenChange,
  ruleName,
  isLoading,
  previewData,
  clientsList,
  transformers,
  onLoadFull,
  isReloadingFull,
}: PreviewDialogProps) {
  const [activeClient, setActiveClient] = useState<ClientType | "">("");
  const [activeView, setActiveView] = useState<"content" | "report">("content");

  const availableClients = useMemo(
    () => (previewData ? (Object.keys(previewData.contents) as ClientType[]) : []),
    [previewData],
  );

  // resolvedClient derives the Tabs `value` from the user's last explicit
  // selection (kept in activeClient) AND the currently available client
  // set. When the previous selection is no longer present — e.g. the user
  // re-previewed a rule with a different output.clients list — we fall
  // back to the first available client during render instead of writing
  // setActiveClient from an effect (which triggers a render cycle and is
  // flagged by react-hooks/set-state-in-effect).
  const resolvedClient = activeClient && availableClients.includes(activeClient)
    ? activeClient
    : availableClients[0] || "";

  const getDisplayName = (clientId: string) =>
    clientsList.find((c) => c.id === clientId)?.displayName || clientId;

  const activeReport: TransformReport | undefined = useMemo(() => {
    if (!previewData || !previewData.reports) return undefined;
    return previewData.reports[resolvedClient as ClientType];
  }, [previewData, resolvedClient]);

  // Active content for the resolved client. Used by both the content view
  // and the header line counter so the two never disagree.
  const activeContent = useMemo(() => {
    if (!previewData || !resolvedClient) return "";
    return previewData.contents[resolvedClient as ClientType] || "";
  }, [previewData, resolvedClient]);

  // Header line counter: prefer the report's "rule count" (significant
  // lines, the same number that drives FinalStats and the step counters).
  // Falls back to raw line count only when the backend didn't return a
  // report — i.e. for callers that consume the preview API without admin
  // privileges (today nobody, kept for forward-compat).
  const headerCount = useMemo(() => {
    if (activeReport) {
      return { value: activeReport.finalStats.totalLines, label: "条规则" };
    }
    const raw = activeContent ? activeContent.split("\n").length : 0;
    return { value: raw, label: "行" };
  }, [activeReport, activeContent]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-5xl w-[92vw] h-[80vh] bg-background border-border flex flex-col p-0">
        <DialogHeader className="px-6 pt-6 pb-4 border-b border-border shrink-0">
          <DialogTitle className="text-foreground flex items-center gap-2 flex-wrap">
            <Eye className="w-5 h-5 text-primary" />
            预览: {ruleName || "未命名规则"}
            {previewData?.diagnostics.truncated && (
              <span className="inline-flex items-center gap-2">
                <Badge
                  variant="outline"
                  className="border-warning/25 bg-warning-soft text-warning"
                >
                  <AlertTriangle className="w-3 h-3 mr-1" />
                  已截断 — 显示前 {previewData.contents[resolvedClient as ClientType]?.split("\n").length.toLocaleString() ?? 0} 行 / 共 {previewData.diagnostics.totalLines.toLocaleString()} 行
                </Badge>
                {onLoadFull && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={onLoadFull}
                    disabled={!!isReloadingFull}
                    className="h-7 text-xs"
                  >
                    {isReloadingFull ? (
                      <>
                        <Loader2 className="w-3 h-3 mr-1 animate-spin" /> 加载中
                      </>
                    ) : (
                      <>
                        <Maximize2 className="w-3 h-3 mr-1" /> 加载完整内容
                      </>
                    )}
                  </Button>
                )}
              </span>
            )}
          </DialogTitle>
        </DialogHeader>

        {isLoading ? (
          <div className="flex-1 flex items-center justify-center">
            <Loader2 className="w-8 h-8 animate-spin text-primary" />
          </div>
        ) : previewData ? (
          <div className="flex-1 flex flex-col min-h-0 overflow-hidden">
            {previewData.diagnostics.sourceResults.length > 0 && (
              <div className="shrink-0 border-b border-border bg-surface-subtle/60 px-6 py-3">
                <p className="text-sm text-muted-foreground mb-2">数据源状态:</p>
                <div className="flex flex-wrap gap-4">
                  {previewData.diagnostics.sourceResults.map((source, i) => (
                    <div key={i} className="flex items-center gap-2 text-sm">
                      {source.success ? (
                        <CheckCircle className="w-4 h-4 text-success" />
                      ) : (
                        <XCircle className="w-4 h-4 text-destructive" />
                      )}
                      <span className="text-xs font-medium text-muted-foreground">#{i + 1}</span>
                      <span className="text-foreground/80 truncate max-w-xs">{source.url}</span>
                      {source.size !== undefined && source.size > 0 && (
                        <span className="text-muted-foreground">
                          ({(source.size / 1024).toFixed(1)} KB)
                        </span>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            <Tabs
              value={resolvedClient}
              onValueChange={(v) => setActiveClient(v as ClientType)}
              className="flex-1 flex flex-col min-h-0"
            >
              <div className="px-6 py-3 border-b border-border flex items-center justify-between shrink-0 gap-3">
                <TabsList>
                  {Object.keys(previewData.contents).map((client) => (
                    <TabsTrigger
                      key={client}
                      value={client}
                    >
                      {getDisplayName(client)}
                    </TabsTrigger>
                  ))}
                </TabsList>
                <div className="flex items-center gap-2">
                  {/* Sub-view switcher: content vs transform report. Report is
                      only available when the preview API returned a report
                      for the active client (admin endpoint always does, but
                      legacy mock responses may not). */}
                  <div className="inline-flex items-center rounded-lg border border-border bg-surface-subtle/60 p-0.5 text-xs">
                    <button
                      type="button"
                      onClick={() => setActiveView("content")}
                      className={`px-2.5 py-1 rounded-md transition-colors ${
                        activeView === "content"
                          ? "bg-primary text-primary-foreground shadow-[var(--shadow-xs)]"
                          : "text-muted-foreground hover:text-foreground"
                      }`}
                    >
                      规则内容
                    </button>
                    <button
                      type="button"
                      onClick={() => setActiveView("report")}
                      disabled={!activeReport}
                      className={`px-2.5 py-1 rounded-md transition-colors ${
                        activeView === "report"
                          ? "bg-primary text-primary-foreground shadow-[var(--shadow-xs)]"
                          : "text-muted-foreground hover:text-foreground disabled:opacity-40 disabled:cursor-not-allowed"
                      }`}
                      title={activeReport ? "查看转换流水线" : "本预览未返回流水线报告"}
                    >
                      <span className="inline-flex items-center gap-1">
                        <Workflow className="w-3 h-3" />
                        转换流水线
                      </span>
                    </button>
                  </div>
                  <span className="text-sm text-muted-foreground tabular-nums">
                    {headerCount.value.toLocaleString()} {headerCount.label}
                  </span>
                </div>
              </div>

              {Object.entries(previewData.contents).map(([client, content]) => (
                <TabsContent
                  key={client}
                  value={client}
                  className="flex-1 m-0 relative min-h-0 overflow-hidden"
                >
                  {activeView === "content" ? (
                    <>
                      <div className="absolute top-2 right-2 z-10">
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={async () => {
                            try {
                              await navigator.clipboard.writeText(content);
                              toast.success("已复制内容");
                            } catch {
                              toast.error("复制失败，请手动选择内容复制");
                            }
                          }}
                          className="border border-border/50 bg-background/90 shadow-[var(--shadow-xs)] hover:bg-background"
                          title="复制内容"
                        >
                          <Copy className="w-4 h-4" />
                        </Button>
                      </div>
                      <CodeViewer
                        content={content}
                        showLineNumbers={false}
                        className="h-full rounded-none border-none"
                        height="100%"
                      />
                    </>
                  ) : (
                    <TransformReportPanel
                      report={previewData.reports?.[client as ClientType]}
                      transformers={transformers}
                    />
                  )}
                </TabsContent>
              ))}
            </Tabs>
          </div>
        ) : (
          <div className="flex-1 flex items-center justify-center text-muted-foreground">
            无预览数据
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

// ---------- Transform report panel ----------

interface TransformReportPanelProps {
  report: TransformReport | undefined;
  transformers?: Record<string, ScriptTransformer>;
}

function TransformReportPanel({ report, transformers }: TransformReportPanelProps) {
  if (!report) {
    return (
      <div className="h-full flex items-center justify-center text-sm text-muted-foreground">
        本客户端没有可视化的转换流水线（管道为空或预览端未返回报告）
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto p-6 space-y-6">
      <FinalStatsCard stats={report.finalStats} />
      <PipelineTimeline steps={report.steps} transformers={transformers} />
    </div>
  );
}

interface FinalStatsCardProps {
  stats: TransformReport["finalStats"];
}

function FinalStatsCard({ stats }: FinalStatsCardProps) {
  const entries = Object.entries(stats.byType || {}).sort((a, b) => b[1] - a[1]);
  const max = entries.length > 0 ? Math.max(...entries.map(([, v]) => v)) : 0;

  return (
    <div className="rounded-2xl border border-border bg-card shadow-[var(--shadow-xs)]">
      <div className="flex items-baseline justify-between px-5 pt-4 pb-3 border-b border-border/50">
        <h3 className="text-sm font-semibold text-foreground flex items-center gap-1.5">
          <Filter className="w-4 h-4 text-primary" />
          最终内容统计
        </h3>
        <div className="flex items-center gap-3">
          <span className="text-xs text-muted-foreground">规则数</span>
          <span className="text-2xl font-bold text-foreground tabular-nums">
            {stats.totalLines.toLocaleString()}
          </span>
          {typeof stats.payloadCount === "number" && (
            <Badge variant="outline" className="border-primary/25 bg-primary-soft text-primary text-xs">
              YAML payload {stats.payloadCount.toLocaleString()}
            </Badge>
          )}
        </div>
      </div>
      <div className="px-5 py-4">
        {entries.length === 0 ? (
          <p className="text-xs text-muted-foreground italic">无规则行 — 可能内容被全部过滤或为空</p>
        ) : (
          <ul className="space-y-1.5">
            {entries.map(([type, count]) => (
              <li key={type} className="flex items-center gap-3 text-sm">
                <span className="font-mono text-xs text-foreground/80 w-32 truncate" title={type}>
                  {type}
                </span>
                <div className="flex-1 h-2 rounded-full bg-surface-subtle/80 overflow-hidden">
                  <div
                    className="h-full bg-primary/70"
                    style={{ width: `${max > 0 ? Math.max(4, (count / max) * 100) : 0}%` }}
                  />
                </div>
                <span className="text-xs tabular-nums text-muted-foreground w-14 text-right">
                  {count.toLocaleString()}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

interface PipelineTimelineProps {
  steps: StepReport[];
  transformers?: Record<string, ScriptTransformer>;
}

function PipelineTimeline({ steps, transformers }: PipelineTimelineProps) {
  if (steps.length === 0) {
    return (
      <div className="rounded-2xl border border-dashed border-border bg-surface-subtle/40 p-6 text-center text-sm text-muted-foreground">
        此规则未配置任何 transform，跳过 merge 之外的所有阶段
      </div>
    );
  }
  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold text-foreground flex items-center gap-1.5">
        <Workflow className="w-4 h-4 text-primary" />
        转换流水线
        <span className="text-xs font-normal text-muted-foreground">共 {steps.length} 步</span>
      </h3>
      <ol className="space-y-2">
        {steps.map((step, idx) => (
          <StepCard key={`${step.stage}-${step.index}-${step.sourceIndex ?? 0}-${idx}`} step={step} transformers={transformers} />
        ))}
      </ol>
    </div>
  );
}

interface StepCardProps {
  step: StepReport;
  transformers?: Record<string, ScriptTransformer>;
}

function StepCard({ step, transformers }: StepCardProps) {
  const [expanded, setExpanded] = useState(false);
  const delta = step.outputLines - step.inputLines;
  const droppedTotal = step.droppedTotal ?? step.dropped?.length ?? 0;
  const modifiedTotal = step.modifiedTotal ?? step.modified?.length ?? 0;
  const addedTotal = step.addedTotal ?? step.added?.length ?? 0;
  const hasDetails =
    (step.dropped?.length ?? 0) > 0 ||
    (step.modified?.length ?? 0) > 0 ||
    (step.added?.length ?? 0) > 0;

  const isBuiltin = step.kind === TRANSFORM_STEP_KIND.useBuiltin;

  return (
    <li className="rounded-xl border border-border bg-card shadow-[var(--shadow-xs)] overflow-hidden">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        disabled={!hasDetails}
        className={`w-full flex items-center gap-3 px-4 py-3 text-left transition-colors ${
          hasDetails ? "hover:bg-surface-subtle/60" : "cursor-default"
        }`}
      >
        <span className="flex items-center justify-center w-5 h-5 shrink-0">
          {hasDetails ? (
            expanded ? (
              <ChevronDown className="w-4 h-4 text-muted-foreground" />
            ) : (
              <ChevronRight className="w-4 h-4 text-muted-foreground" />
            )
          ) : (
            <GitBranch className="w-3.5 h-3.5 text-muted-foreground/50" />
          )}
        </span>
        <StageBadge stage={step.stage} />
        <span className="font-mono text-sm text-foreground flex items-center gap-1.5 min-w-0">
          {isBuiltin && <Lock className="w-3 h-3 text-muted-foreground shrink-0" />}
          <span className="truncate" title={step.label}>{step.label}</span>
        </span>
        <span className="ml-auto flex items-center gap-2 text-xs text-muted-foreground shrink-0" title="规则数（不含空行/注释）">
          <span className="tabular-nums">{step.inputLines.toLocaleString()}</span>
          <ArrowRight className="w-3 h-3" />
          <span className="tabular-nums">{step.outputLines.toLocaleString()}</span>
          <span className="text-[10px]">条</span>
          {delta !== 0 && (
            <span
              className={`px-1.5 py-0.5 rounded-md text-[10px] tabular-nums font-medium ${
                delta < 0
                  ? "bg-destructive/12 text-destructive"
                  : "bg-success-soft text-success"
              }`}
            >
              {delta > 0 ? "+" : ""}
              {delta.toLocaleString()}
            </span>
          )}
        </span>
        <span className="flex items-center gap-1.5 shrink-0">
          {droppedTotal > 0 && (
            <Badge variant="outline" className="border-destructive/25 bg-destructive/8 text-destructive text-[10px]">
              丢弃 {droppedTotal}
            </Badge>
          )}
          {modifiedTotal > 0 && (
            <Badge variant="outline" className="border-warning/25 bg-warning-soft text-warning text-[10px]">
              改写 {modifiedTotal}
            </Badge>
          )}
          {addedTotal > 0 && (
            <Badge variant="outline" className="border-success/25 bg-success-soft text-success text-[10px]">
              新增 {addedTotal}
            </Badge>
          )}
        </span>
      </button>

      {expanded && hasDetails && (
        <div className="border-t border-border/50 bg-surface-subtle/40 p-4 space-y-4">
          {step.dropped && step.dropped.length > 0 && (
            <DroppedSection dropped={step.dropped} total={droppedTotal} transformers={transformers} />
          )}
          {step.modified && step.modified.length > 0 && (
            <ModifiedSection modified={step.modified} total={modifiedTotal} transformers={transformers} />
          )}
          {step.added && step.added.length > 0 && (
            <AddedSection added={step.added} total={addedTotal} transformers={transformers} />
          )}
        </div>
      )}
    </li>
  );
}

function StageBadge({ stage }: { stage: string }) {
  const cls: Record<string, string> = {
    [TRANSFORM_STAGE.rule]: "border-primary/25 bg-primary-soft text-primary",
    [TRANSFORM_STAGE.merge]: "border-foreground/15 bg-surface-subtle/80 text-foreground",
    [TRANSFORM_STAGE.client]: "border-warning/25 bg-warning-soft text-warning",
    [TRANSFORM_STAGE.override]: "border-destructive/25 bg-destructive/8 text-destructive",
  };
  return (
    <Badge variant="outline" className={`text-[10px] font-medium uppercase shrink-0 ${cls[stage] || ""}`}>
      {stage}
    </Badge>
  );
}

// TransformerReasonTooltip wraps a reason string with a tooltip showing
// the transformer's description when the reason is a user-script
// transformer name. Builtin and regex reasons are shown as-is.
function TransformerReasonTooltip({ reason, transformers }: { reason: string; transformers?: Record<string, ScriptTransformer> }) {
  const desc = transformers?.[reason]?.description;
  if (!desc) return <>{reason}</>;
  return (
    <TooltipProvider delayDuration={300}>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="underline decoration-dotted decoration-muted-foreground/50 underline-offset-2 cursor-help">
            {reason}
          </span>
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-xs text-xs">
          {desc}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

function DroppedSection({ dropped, total, transformers }: { dropped: StepReport["dropped"]; total: number; transformers?: Record<string, ScriptTransformer> }) {
  return (
    <div className="space-y-2">
      <div className="flex items-baseline gap-2">
        <h4 className="text-xs font-semibold uppercase tracking-wide text-destructive">
          已丢弃 ({total})
        </h4>
        {total > (dropped?.length ?? 0) && (
          <span className="text-[10px] text-muted-foreground">
            仅显示前 {dropped?.length}，共 {total} 条
          </span>
        )}
      </div>
      <ul className="space-y-1.5">
        {dropped?.map((d, i) => (
          <li
            key={i}
            className="rounded-md border border-destructive/20 bg-destructive/4 px-3 py-2 text-xs"
          >
            <div className="flex items-baseline gap-2">
              {d.lineNo > 0 && (
                <span className="font-mono tabular-nums text-muted-foreground shrink-0">L{d.lineNo}</span>
              )}
              <code className="flex-1 font-mono text-foreground/90 break-all whitespace-pre-wrap">{d.text}{d.truncated ? "…" : ""}</code>
            </div>
            <div className="mt-0.5 ml-8 flex items-center gap-2 text-muted-foreground italic">
              <TransformerReasonTooltip reason={d.reason} transformers={transformers} />
              {d.truncated && (
                <Badge variant="outline" className="border-warning/40 bg-warning-soft text-warning text-[9px] uppercase">
                  样本截断
                </Badge>
              )}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

function ModifiedSection({ modified, total, transformers }: { modified: StepReport["modified"]; total: number; transformers?: Record<string, ScriptTransformer> }) {
  return (
    <div className="space-y-2">
      <div className="flex items-baseline gap-2">
        <h4 className="text-xs font-semibold uppercase tracking-wide text-warning">
          已改写 ({total})
        </h4>
        {total > (modified?.length ?? 0) && (
          <span className="text-[10px] text-muted-foreground">
            仅显示前 {modified?.length}，共 {total} 条
          </span>
        )}
      </div>
      <ul className="space-y-1.5">
        {modified?.map((m, i) => (
          <li
            key={i}
            className="rounded-md border border-warning/20 bg-warning-soft/50 px-3 py-2 text-xs"
          >
            <div className="flex items-baseline gap-2">
              {m.lineNo > 0 && (
                <span className="font-mono tabular-nums text-muted-foreground shrink-0">L{m.lineNo}</span>
              )}
              <div className="flex-1 space-y-1 min-w-0">
                <code className="block font-mono text-foreground/70 line-through break-all whitespace-pre-wrap">{m.from}{m.truncated ? "…" : ""}</code>
                <code className="block font-mono text-foreground break-all whitespace-pre-wrap">{m.to}{m.truncated ? "…" : ""}</code>
              </div>
            </div>
            <div className="mt-1 ml-8 flex items-center gap-2 text-muted-foreground italic">
              {m.reason && <TransformerReasonTooltip reason={m.reason} transformers={transformers} />}
              {m.truncated && (
                <Badge variant="outline" className="border-warning/40 bg-warning-soft text-warning text-[9px] uppercase">
                  样本截断
                </Badge>
              )}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

function AddedSection({ added, total, transformers }: { added: StepReport["added"]; total: number; transformers?: Record<string, ScriptTransformer> }) {
  return (
    <div className="space-y-2">
      <div className="flex items-baseline gap-2">
        <h4 className="text-xs font-semibold uppercase tracking-wide text-success">
          已新增 ({total})
        </h4>
        {total > (added?.length ?? 0) && (
          <span className="text-[10px] text-muted-foreground">
            仅显示前 {added?.length}，共 {total} 条
          </span>
        )}
      </div>
      <ul className="space-y-1.5">
        {added?.map((a, i) => (
          <li
            key={i}
            className="rounded-md border border-success/20 bg-success-soft/50 px-3 py-2 text-xs"
          >
            <div className="flex items-baseline gap-2">
              {a.lineNo > 0 && (
                <span className="font-mono tabular-nums text-muted-foreground shrink-0">L{a.lineNo}</span>
              )}
              <code className="flex-1 font-mono text-foreground/90 break-all whitespace-pre-wrap">{a.text}{a.truncated ? "…" : ""}</code>
            </div>
            <div className="mt-0.5 ml-8 flex items-center gap-2 text-muted-foreground italic">
              <TransformerReasonTooltip reason={a.reason} transformers={transformers} />
              {a.truncated && (
                <Badge variant="outline" className="border-warning/40 bg-warning-soft text-warning text-[9px] uppercase">
                  样本截断
                </Badge>
              )}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

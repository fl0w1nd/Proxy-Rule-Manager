"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Loader2, RefreshCw, X, ChevronDown } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { SyncProgress } from "@/lib/api-client";

// PHASE_LABELS maps backend phase keys to display labels. Unknown phases
// fall back to the raw key.
const PHASE_LABELS: Record<string, string> = {
  starting: "准备同步",
  acquire_lock: "等待锁",
  loading_config: "加载配置",
  refreshing_geosite: "拉取 Geosite",
  processing: "处理规则",
  finalizing: "写入结果",
  done: "已完成",
};

// The order drives the stage indicator. Phases outside the list do not
// participate in the index calculation.
const PHASE_ORDER = [
  "starting",
  "acquire_lock",
  "loading_config",
  "refreshing_geosite",
  "processing",
  "finalizing",
  "done",
];

export interface SyncProgressPillProps {
  // progress is the current sync state. running=true shows the full pill;
  // the parent should render a normal button otherwise.
  progress: SyncProgress;
  // onCancel is owned by the parent, which usually sets isCancelling=true
  // immediately after sending the cancel request.
  onCancel: () => void;
  // isCancelling controls the loading state of the cancel button and blocks
  // repeated clicks.
  isCancelling?: boolean;
  className?: string;
}

// formatElapsed renders a friendly elapsed-time string and avoids noisy 0ms labels.
function formatElapsed(ms?: number): string {
  if (!ms || ms < 0) return "0s";
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainder = seconds % 60;
  return `${minutes}m ${remainder}s`;
}

// resolvePercent degrades gracefully when total is unavailable so NaN or
// Infinity never reaches the CSS width value.
function resolvePercent(progress: SyncProgress): number {
  if (!progress.total || progress.total <= 0) return 0;
  return Math.min(100, Math.round((progress.processed / progress.total) * 100));
}

export function SyncProgressPill({
  progress,
  onCancel,
  isCancelling = false,
  className,
}: SyncProgressPillProps) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);

  // Clicking outside the pill or pressing Esc closes the panel locally and
  // leaves polling untouched.
  useEffect(() => {
    if (!open) return;
    const onPointer = (e: PointerEvent) => {
      if (!containerRef.current) return;
      if (e.target instanceof Node && containerRef.current.contains(e.target)) return;
      setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const percent = resolvePercent(progress);
  const phaseLabel = useMemo(() => {
    if (!progress.phase) return "同步中";
    return PHASE_LABELS[progress.phase] ?? progress.phase;
  }, [progress.phase]);
  const phaseIndex = progress.phase ? PHASE_ORDER.indexOf(progress.phase) : -1;

  const togglePanel = useCallback(() => {
    setOpen((v) => !v);
  }, []);

  // The current rule name is most useful during processing; other phases show
  // the phase detail instead.
  const headline =
    progress.phase === "processing" && progress.currentRule
      ? progress.currentRule
      : progress.phaseDetail || phaseLabel;

  return (
    <div ref={containerRef} className={cn("relative", className)}>
      <div className="flex items-center gap-2">
        {/* Pill body: click to expand the panel and show phase, progress, and counts. */}
        <button
          type="button"
          onClick={togglePanel}
          className={cn(
            "group inline-flex items-center gap-2 rounded-full border border-border bg-background pl-3 pr-2.5 py-1 text-sm font-medium shadow-[var(--shadow-xs)] transition-colors hover:bg-accent",
            open && "bg-accent",
          )}
          aria-expanded={open}
          aria-label="查看同步进度"
        >
          <Loader2 className="w-3.5 h-3.5 animate-spin text-primary" />
          <span className="max-w-[180px] truncate text-foreground" title={headline}>
            {headline}
          </span>
          <span className="rounded-full bg-muted px-2 py-0.5 text-xs tabular-nums text-muted-foreground">
            {progress.total > 0 ? `${progress.processed}/${progress.total}` : `${progress.processed}`}
          </span>
          <ChevronDown
            className={cn(
              "w-3.5 h-3.5 text-muted-foreground transition-transform",
              open && "rotate-180",
            )}
          />
        </button>
        {/* Separate cancel button so the click target stays unambiguous. */}
        <Button
          variant="destructive"
          size="icon-sm"
          onClick={onCancel}
          disabled={isCancelling || !!progress.cancelled}
          title={progress.cancelled ? "取消已请求" : "取消同步"}
        >
          {isCancelling ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <X className="w-3.5 h-3.5" />}
        </Button>
      </div>

      {/* Progress detail panel */}
      {open && (
        <div
          role="dialog"
          aria-label="同步进度详情"
          className="absolute right-0 mt-2 w-[360px] z-50 rounded-lg border border-border bg-popover text-popover-foreground shadow-[var(--shadow-lg)] p-4 space-y-3"
        >
          {/* Header: job type plus truncated jobId */}
          <div className="flex items-start justify-between gap-2">
            <div>
              <div className="text-sm font-medium">
                {progress.jobType === "full_sync" ? "全量同步" : progress.jobType ?? "同步"}
              </div>
              {progress.jobId && (
                <div className="text-[10px] text-muted-foreground tabular-nums">
                  job: {progress.jobId.slice(0, 8)}
                </div>
              )}
            </div>
            <div className="text-xs text-muted-foreground tabular-nums">
              已用 {formatElapsed(progress.elapsedMs)}
            </div>
          </div>

          {/* Progress bar and percentage */}
          <div className="space-y-1.5">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>{phaseLabel}{progress.phaseDetail ? ` · ${progress.phaseDetail}` : ""}</span>
              <span className="tabular-nums">
                {progress.total > 0
                  ? `${progress.processed}/${progress.total} (${percent}%)`
                  : `已处理 ${progress.processed}`}
              </span>
            </div>
            <div className="h-1.5 w-full rounded-full bg-muted overflow-hidden">
              <div
                className={cn(
                  "h-full rounded-full bg-primary transition-[width] duration-300",
                  progress.cancelled && "bg-destructive",
                )}
                style={{ width: progress.total > 0 ? `${percent}%` : "20%" }}
              />
            </div>
          </div>

          {/* Stage indicator showing the rough sync position; hidden for unknown phases. */}
          {phaseIndex >= 0 && (
            <div className="flex items-center gap-1">
              {PHASE_ORDER.filter((p) => p !== "starting").map((p) => {
                const idx = PHASE_ORDER.indexOf(p);
                const reached = phaseIndex >= idx;
                const current = phaseIndex === idx;
                return (
                  <div
                    key={p}
                    title={PHASE_LABELS[p]}
                    className={cn(
                      "h-1 flex-1 rounded-full transition-colors",
                      reached ? "bg-primary" : "bg-muted",
                      current && "ring-2 ring-primary/30",
                    )}
                  />
                );
              })}
            </div>
          )}

          {/* Counters: failures now, changed count after completion. */}
          {progress.failed > 0 && (
            <div className="flex items-center gap-2 text-xs">
              <span className="rounded-full bg-destructive/10 text-destructive px-2 py-0.5">
                失败 {progress.failed}
              </span>
              {progress.cancelled && (
                <span className="rounded-full bg-muted text-muted-foreground px-2 py-0.5">
                  已请求取消
                </span>
              )}
            </div>
          )}

          {/* Log tail: recent lines shown in reverse order so the newest stays on top. */}
          {progress.logTail && progress.logTail.length > 0 && (
            <div className="space-y-1">
              <div className="text-[11px] uppercase tracking-wider text-muted-foreground">
                最近日志
              </div>
              <div className="max-h-[140px] overflow-y-auto rounded-md bg-muted/50 px-2 py-1.5 font-mono text-[11px] leading-relaxed text-muted-foreground">
                {[...progress.logTail].reverse().slice(0, 12).map((line, i) => (
                  <div key={i} className="truncate" title={line}>
                    {line}
                  </div>
                ))}
              </div>
            </div>
          )}

          <div className="flex items-center justify-end gap-2 pt-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setOpen(false)}
            >
              收起
            </Button>
            <Button
              variant="destructive"
              size="sm"
              onClick={onCancel}
              disabled={isCancelling || !!progress.cancelled}
            >
              {isCancelling ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
              ) : (
                <X className="w-3.5 h-3.5" />
              )}
              {progress.cancelled ? "已请求取消" : "取消同步"}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

// SyncTriggerButton is a thin wrapper for parent components that need a
// sync button with the same icon language as the pill.
export function SyncTriggerButton({
  isPending,
  onClick,
  disabled,
  size = "default",
}: {
  isPending: boolean;
  onClick: () => void;
  disabled?: boolean;
  size?: "default" | "sm";
}) {
  return (
    <Button
      variant="default"
      size={size}
      onClick={onClick}
      disabled={disabled || isPending}
    >
      {isPending ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
      同步规则
    </Button>
  );
}

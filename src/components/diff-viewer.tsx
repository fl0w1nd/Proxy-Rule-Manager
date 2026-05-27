"use client";

import { useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { PatchDiff } from "@pierre/diffs/react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Columns2, Rows2 } from "lucide-react";

type DiffStyle = "unified" | "split";

interface DiffViewerProps {
  content: string;
  className?: string;
  defaultDiffStyle?: DiffStyle;
}

const LARGE_DIFF_LINE_THRESHOLD = 1_200;
const LARGE_DIFF_CHAR_THRESHOLD = 80_000;
const DIFF_LINE_HEIGHT = 22;
const DIFF_OVERSCAN = 24;

type DiffKind = "add" | "remove" | "meta" | "context";

function classifyLine(line: string): DiffKind {
  if (!line) return "context";
  const c = line.charCodeAt(0);
  if (c === 43 /* + */) return "add";
  if (c === 45 /* - */) return "remove";
  if (c === 64 /* @ */ && line.startsWith("@@")) return "meta";
  if (c === 92 /* \ */) return "meta";
  if (line.startsWith("diff ") || line.startsWith("index ")) return "meta";
  return "context";
}

function getDiffLineClassName(kind: DiffKind): string {
  switch (kind) {
    case "add":
      return "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300";
    case "remove":
      return "bg-rose-500/10 text-rose-700 dark:text-rose-300";
    case "meta":
      return "bg-muted/40 text-muted-foreground";
    default:
      return "text-foreground";
  }
}

function LargeDiffViewer({ content }: { content: string }) {
  const lines = useMemo(() => content.split("\n"), [content]);
  const parentRef = useRef<HTMLDivElement | null>(null);

  // eslint-disable-next-line react-hooks/incompatible-library -- TanStack Virtual is intentionally not memoized; we own scroll state locally.
  const virtualizer = useVirtualizer({
    count: lines.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => DIFF_LINE_HEIGHT,
    overscan: DIFF_OVERSCAN,
  });

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between gap-3 border-b border-border bg-surface-subtle px-3 py-2 text-xs text-muted-foreground shrink-0">
        <span>
          Large diff detected. Showing {lines.length.toLocaleString()} lines via virtual scrolling.
        </span>
      </div>
      <div
        ref={parentRef}
        className="flex-1 overflow-auto"
        style={{ contain: "strict" }}
      >
        <div
          style={{
            height: `${virtualizer.getTotalSize()}px`,
            width: "100%",
            position: "relative",
          }}
          className="font-mono text-sm leading-6"
        >
          {virtualizer.getVirtualItems().map((virtualRow) => {
            const line = lines[virtualRow.index];
            const kind = classifyLine(line);
            return (
              <div
                key={virtualRow.key}
                className={cn(
                  "absolute left-0 right-0 whitespace-pre px-4",
                  getDiffLineClassName(kind),
                )}
                style={{
                  top: 0,
                  transform: `translateY(${virtualRow.start}px)`,
                  height: `${DIFF_LINE_HEIGHT}px`,
                }}
              >
                {line || " "}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

export function DiffViewer({ content, className, defaultDiffStyle = "unified" }: DiffViewerProps) {
  const [diffStyle, setDiffStyle] = useState<DiffStyle>(defaultDiffStyle);

  if (!content) return null;

  const isUnifiedDiff = /^(?:Index:|diff |--- )/.test(content.trimStart());
  if (!isUnifiedDiff) {
    return (
      <div className={cn("overflow-x-auto rounded-xl border border-border bg-card p-4", className)}>
        <pre className="whitespace-pre-wrap break-all font-mono text-sm text-muted-foreground">
          {content}
        </pre>
      </div>
    );
  }

  const lineCount = content.split("\n").length;
  const useLargeDiffMode = lineCount > LARGE_DIFF_LINE_THRESHOLD || content.length > LARGE_DIFF_CHAR_THRESHOLD;

  return (
    <div className={cn("overflow-x-auto rounded-xl border border-border bg-card", className)}>
      {useLargeDiffMode ? (
        <LargeDiffViewer key={content} content={content} />
      ) : (
        <>
          <div className="flex items-center justify-end gap-1 border-b border-border bg-surface-subtle px-2 py-1.5">
            <Button
              variant={diffStyle === "unified" ? "secondary" : "ghost"}
              size="icon-sm"
              onClick={() => setDiffStyle("unified")}
              title="统一视图"
            >
              <Rows2 className="h-3.5 w-3.5" />
            </Button>
            <Button
              variant={diffStyle === "split" ? "secondary" : "ghost"}
              size="icon-sm"
              onClick={() => setDiffStyle("split")}
              title="并排视图"
            >
              <Columns2 className="h-3.5 w-3.5" />
            </Button>
          </div>
          <PatchDiff
            patch={content}
            options={{
              disableFileHeader: true,
              diffStyle,
            }}
          />
        </>
      )}
    </div>
  );
}

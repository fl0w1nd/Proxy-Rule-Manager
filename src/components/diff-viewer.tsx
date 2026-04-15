"use client";

import { useEffect, useMemo, useState } from "react";
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

const LARGE_DIFF_LINE_THRESHOLD = 1200;
const LARGE_DIFF_CHAR_THRESHOLD = 80_000;
const INITIAL_VISIBLE_LINES = 240;
const LOAD_MORE_LINES = 240;

interface ParsedDiffLine {
  key: string;
  kind: "add" | "remove" | "meta" | "context";
  content: string;
}

function parseDiffLines(content: string): ParsedDiffLine[] {
  return content.split("\n").map((line, index) => {
    let kind: ParsedDiffLine["kind"] = "context";
    if (line.startsWith("+")) {
      kind = "add";
    } else if (line.startsWith("-")) {
      kind = "remove";
    } else if (
      line.startsWith("@@") ||
      line.startsWith("diff ") ||
      line.startsWith("index ") ||
      line.startsWith("---") ||
      line.startsWith("+++") ||
      line.startsWith("\\")
    ) {
      kind = "meta";
    }

    return {
      key: `${index}-${line}`,
      kind,
      content: line,
    };
  });
}

function getDiffLineClassName(kind: ParsedDiffLine["kind"]): string {
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
  const parsedLines = useMemo(() => parseDiffLines(content), [content]);
  const [visibleLineCount, setVisibleLineCount] = useState(() =>
    Math.min(INITIAL_VISIBLE_LINES, parsedLines.length)
  );

  useEffect(() => {
    setVisibleLineCount(Math.min(INITIAL_VISIBLE_LINES, parsedLines.length));
  }, [parsedLines.length, content]);

  const visibleLines = parsedLines.slice(0, visibleLineCount);
  const remainingLineCount = Math.max(parsedLines.length - visibleLineCount, 0);

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between gap-3 border-b border-border bg-surface-subtle px-3 py-2 text-xs text-muted-foreground">
        <span>
          Large diff detected. Loaded {visibleLineCount.toLocaleString()} / {parsedLines.length.toLocaleString()} lines.
        </span>
        {remainingLineCount > 0 && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => setVisibleLineCount((current) => Math.min(current + LOAD_MORE_LINES, parsedLines.length))}
          >
            Load More ({Math.min(LOAD_MORE_LINES, remainingLineCount)} lines)
          </Button>
        )}
      </div>
      <div className="overflow-auto">
        <pre className="min-w-full p-0 font-mono text-sm leading-6">
          {visibleLines.map((line) => (
            <div key={line.key} className={cn("px-4", getDiffLineClassName(line.kind))}>
              {line.content || " "}
            </div>
          ))}
        </pre>
      </div>
    </div>
  );
}

export function DiffViewer({ content, className, defaultDiffStyle = "unified" }: DiffViewerProps) {
  const [diffStyle, setDiffStyle] = useState<DiffStyle>(defaultDiffStyle);

  if (!content) return null;

  const lineCount = content.split("\n").length;
  const useLargeDiffMode = lineCount > LARGE_DIFF_LINE_THRESHOLD || content.length > LARGE_DIFF_CHAR_THRESHOLD;

  return (
    <div className={cn("overflow-x-auto rounded-xl border border-border bg-card", className)}>
      {useLargeDiffMode ? (
        <LargeDiffViewer content={content} />
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

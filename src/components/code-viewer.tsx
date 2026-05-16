"use client";

import { useState } from "react";
import { Copy, Loader2, AlertTriangle } from "lucide-react";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { LazyMonacoEditor } from "./lazy-monaco";
import { useTheme } from "./theme-provider";
import { Button } from "./ui/button";

interface CodeViewerProps {
  content?: string | null;
  loading?: boolean;
  showLineNumbers?: boolean;
  emptyText?: string;
  className?: string;
  language?: string;
  height?: string;
  /**
   * Hard cap before falling back from Monaco to a virtualized plain renderer.
   * Defaults are tuned for typical mihomo rule files; oversized previews
   * (think geosite category-ads-all) would otherwise block the main thread
   * for several seconds while Monaco constructs its model.
   */
  largeLineThreshold?: number;
  largeByteThreshold?: number;
}

const DEFAULT_LARGE_LINE_THRESHOLD = 10_000;
const DEFAULT_LARGE_BYTE_THRESHOLD = 1_000_000;
const FALLBACK_VISIBLE_LINES = 5_000;

function countLines(text: string): number {
  if (!text) return 0;
  let n = 1;
  for (let i = 0; i < text.length; i++) {
    if (text.charCodeAt(i) === 10) n++;
  }
  if (text.charCodeAt(text.length - 1) === 10) n--;
  return n;
}

function computeVisibleText(text: string, lineCount: number, expanded: boolean): string {
  if (expanded) return text;
  if (lineCount <= FALLBACK_VISIBLE_LINES) return text;
  let count = 0;
  for (let i = 0; i < text.length; i++) {
    if (text.charCodeAt(i) === 10) {
      count++;
      if (count === FALLBACK_VISIBLE_LINES) {
        return text.slice(0, i);
      }
    }
  }
  return text;
}

export function CodeViewer({
  content,
  loading = false,
  showLineNumbers = true,
  emptyText = "暂无内容",
  className,
  language = "plaintext",
  height,
  largeLineThreshold = DEFAULT_LARGE_LINE_THRESHOLD,
  largeByteThreshold = DEFAULT_LARGE_BYTE_THRESHOLD,
}: CodeViewerProps) {
  const text = content ?? "";
  const { mode } = useTheme();
  const editorTheme = mode === "dark" ? "vs-dark" : "light";

  const lineCount = countLines(text);
  const byteSize = text.length;
  const isLarge =
    lineCount > largeLineThreshold || byteSize > largeByteThreshold;

  const resolvedHeight =
    height || `${Math.max(Math.min(lineCount * 20 + 40, 800), 120)}px`;

  if (loading) {
    return (
      <div
        className={cn(
          "overflow-auto rounded-xl bg-surface-subtle shadow-[var(--shadow-sm)]",
          className,
        )}
      >
        <div className="flex items-center justify-center h-64">
          <Loader2 className="w-6 h-6 animate-spin text-primary/50" />
        </div>
      </div>
    );
  }

  if (!text) {
    return (
      <div
        className={cn(
          "overflow-auto rounded-xl bg-surface-subtle shadow-[var(--shadow-sm)] p-4 text-sm text-muted-foreground font-mono",
          className,
        )}
      >
        {emptyText}
      </div>
    );
  }

  if (isLarge) {
    return (
      <LargeContentFallback
        text={text}
        lineCount={lineCount}
        byteSize={byteSize}
        className={className}
        height={resolvedHeight}
      />
    );
  }

  return (
    <div
      className={cn(
        "overflow-hidden rounded-xl border border-border/50 bg-surface-elevated shadow-[var(--shadow-sm)]",
        className,
      )}
    >
      <LazyMonacoEditor
        height={resolvedHeight}
        language={language}
        value={text}
        theme={editorTheme}
        options={{
          readOnly: true,
          minimap: { enabled: false },
          fontSize: 13,
          lineHeight: 20,
          lineNumbers: showLineNumbers ? "on" : "off",
          lineNumbersMinChars: showLineNumbers ? 4 : 0,
          scrollBeyondLastLine: false,
          automaticLayout: true,
          tabSize: 2,
          wordWrap: "off",
          renderLineHighlight: "none",
          overviewRulerBorder: false,
          hideCursorInOverviewRuler: true,
          overviewRulerLanes: 0,
          scrollbar: {
            horizontal: "visible",
            vertical: "visible",
            horizontalScrollbarSize: 10,
            verticalScrollbarSize: 10,
          },
          padding: { top: 12, bottom: 12 },
          domReadOnly: true,
          selectionHighlight: false,
          occurrencesHighlight: "off",
          folding: false,
          contextmenu: false,
        }}
      />
    </div>
  );
}

interface LargeContentFallbackProps {
  text: string;
  lineCount: number;
  byteSize: number;
  className?: string;
  height: string;
}

function LargeContentFallback({
  text,
  lineCount,
  byteSize,
  className,
  height,
}: LargeContentFallbackProps) {
  const [expanded, setExpanded] = useState(false);

  const visibleText = computeVisibleText(text, lineCount, expanded);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success("已复制全部内容");
    } catch {
      toast.error("复制失败，请使用浏览器右键");
    }
  };

  const sizeKb = (byteSize / 1024).toFixed(1);

  return (
    <div
      className={cn(
        "flex flex-col overflow-hidden rounded-xl border border-warning/30 bg-surface-elevated shadow-[var(--shadow-sm)]",
        className,
      )}
      style={{ height }}
    >
      <div className="flex items-center gap-2 border-b border-warning/30 bg-warning-soft/60 px-4 py-2 text-xs text-warning shrink-0">
        <AlertTriangle className="w-3.5 h-3.5 shrink-0" />
        <span className="flex-1 leading-relaxed">
          内容过大（{lineCount.toLocaleString()} 行 / {sizeKb} KB），已切换到精简渲染。
          {!expanded && lineCount > FALLBACK_VISIBLE_LINES && (
            <> 仅显示前 {FALLBACK_VISIBLE_LINES.toLocaleString()} 行。</>
          )}
        </span>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 px-2 text-xs"
          onClick={handleCopy}
        >
          <Copy className="w-3 h-3 mr-1" />
          复制全部
        </Button>
        {!expanded && lineCount > FALLBACK_VISIBLE_LINES && (
          <Button
            variant="outline"
            size="sm"
            className="h-7 px-2 text-xs"
            onClick={() => setExpanded(true)}
          >
            渲染全部
          </Button>
        )}
      </div>
      <pre className="flex-1 overflow-auto px-4 py-3 text-xs font-mono leading-5 text-foreground/85 whitespace-pre">
        {visibleText}
      </pre>
    </div>
  );
}

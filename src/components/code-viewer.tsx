"use client";

import { useMemo, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { LazyMonacoEditor } from "./lazy-monaco";
import { useTheme } from "./theme-provider";

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
   * Tuned conservatively: Monaco mounting + tokenization of a few thousand
   * plaintext lines is enough to block the main thread for multiple seconds,
   * especially when the dialog also has to do layout work. Anything above
   * the threshold goes through the lightweight virtualized renderer.
   */
  largeLineThreshold?: number;
  largeByteThreshold?: number;
}

const DEFAULT_LARGE_LINE_THRESHOLD = 1_500;
const DEFAULT_LARGE_BYTE_THRESHOLD = 150_000;
const VIRTUAL_LINE_HEIGHT = 20;
const VIRTUAL_OVERSCAN = 16;

function countLines(text: string): number {
  if (!text) return 0;
  let n = 1;
  for (let i = 0; i < text.length; i++) {
    if (text.charCodeAt(i) === 10) n++;
  }
  if (text.charCodeAt(text.length - 1) === 10) n--;
  return n;
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
      <VirtualizedContentFallback
        text={text}
        className={className}
        height={resolvedHeight}
        showLineNumbers={showLineNumbers}
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
          // Disable the expensive bracket pair colorization pass; plain text
          // does not benefit from it and it would otherwise touch every line.
          bracketPairColorization: { enabled: false },
        }}
      />
    </div>
  );
}

interface VirtualizedContentFallbackProps {
  text: string;
  className?: string;
  height: string;
  showLineNumbers: boolean;
}

function VirtualizedContentFallback({
  text,
  className,
  height,
  showLineNumbers,
}: VirtualizedContentFallbackProps) {
  const parentRef = useRef<HTMLDivElement | null>(null);

  // Splitting once and memoising avoids re-splitting on every scroll tick.
  const lines = useMemo(() => text.split("\n"), [text]);

  // eslint-disable-next-line react-hooks/incompatible-library -- TanStack Virtual is intentionally not memoized; we own scroll state locally.
  const virtualizer = useVirtualizer({
    count: lines.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => VIRTUAL_LINE_HEIGHT,
    overscan: VIRTUAL_OVERSCAN,
  });

  // Reserve a left gutter that visually matches Monaco's lineNumbersMinChars
  // (~4 chars × ~8px) so content does not slam into the rounded corner.
  const gutterWidth = showLineNumbers
    ? Math.max(String(lines.length).length, 2) * 8 + 16
    : 16;
  // Pad top/bottom so the first and last lines don't touch the border /
  // rounded corner, matching Monaco's `padding: { top: 12, bottom: 12 }`.
  const verticalPadding = 12;

  // Match the Monaco branch wrapper exactly so callers passing
  // `rounded-none border-none` get the same flat full-fill layout regardless
  // of which renderer is active.
  return (
    <div
      className={cn(
        "overflow-hidden rounded-xl border border-border/50 bg-surface-elevated shadow-[var(--shadow-sm)]",
        className,
      )}
      style={{ height }}
    >
      <div
        ref={parentRef}
        className="h-full overflow-auto font-mono text-[13px] leading-5"
        style={{ contain: "strict" }}
      >
        <div
          style={{
            height: `${virtualizer.getTotalSize() + verticalPadding * 2}px`,
            width: "100%",
            position: "relative",
          }}
        >
          {virtualizer.getVirtualItems().map((virtualRow) => {
            const line = lines[virtualRow.index];
            return (
              <div
                key={virtualRow.key}
                data-index={virtualRow.index}
                ref={virtualizer.measureElement}
                className="absolute left-0 right-0 flex items-start whitespace-pre"
                style={{
                  top: 0,
                  transform: `translateY(${virtualRow.start + verticalPadding}px)`,
                }}
              >
                {showLineNumbers ? (
                  <span
                    className="sticky left-0 inline-block select-none text-right text-muted-foreground/70 pr-3 pl-2"
                    style={{ width: gutterWidth, minWidth: gutterWidth }}
                  >
                    {virtualRow.index + 1}
                  </span>
                ) : (
                  <span
                    aria-hidden
                    className="inline-block shrink-0"
                    style={{ width: gutterWidth, minWidth: gutterWidth }}
                  />
                )}
                <span className="flex-1 pr-4 text-foreground/85">
                  {line || " "}
                </span>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

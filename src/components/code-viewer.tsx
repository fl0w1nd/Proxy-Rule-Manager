"use client";

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
}

export function CodeViewer({
  content,
  loading = false,
  showLineNumbers = true,
  emptyText = "暂无内容",
  className,
  language = "plaintext",
  height,
}: CodeViewerProps) {
  const text = content ?? "";
  const { theme } = useTheme();
  const editorTheme = theme === "dark" ? "vs-dark" : "light";
  const lines = text.split("\n").length;

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

  return (
    <div
      className={cn(
        "overflow-hidden rounded-xl border border-border/50 bg-surface-elevated shadow-[var(--shadow-sm)]",
        className,
      )}
    >
      <LazyMonacoEditor
        height={height || `${Math.max(Math.min(lines * 20 + 40, 800), 120)}px`}
        language={language}
        value={text}
        theme={editorTheme}
        options={{
          readOnly: true,
          minimap: { enabled: false },
          fontSize: 13,
          lineNumbers: showLineNumbers ? "on" : "off",
          scrollBeyondLastLine: false,
          automaticLayout: true,
          tabSize: 2,
          wordWrap: "on",
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
          lineDecorationsWidth: 0,
          contextmenu: false,
        }}
      />
    </div>
  );
}

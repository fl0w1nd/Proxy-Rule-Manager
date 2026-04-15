"use client";

import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

interface CodeViewerProps {
  content?: string | null;
  loading?: boolean;
  showLineNumbers?: boolean;
  emptyText?: string;
  className?: string;
}

export function CodeViewer({
  content,
  loading = false,
  showLineNumbers = true,
  emptyText = "暂无内容",
  className,
}: CodeViewerProps) {
  const text = content ?? "";
  const lines = text.split("\n");

  return (
    <div
      className={cn(
        "overflow-auto rounded-xl bg-surface-elevated/80 border border-border",
        className,
      )}
    >
      {loading ? (
        <div className="flex items-center justify-center h-64">
          <Loader2 className="w-6 h-6 animate-spin text-primary/50" />
        </div>
      ) : (
        <div className="flex text-sm font-mono min-w-max">
          {showLineNumbers && (
            <div className="py-4 pl-4 pr-3 text-right text-muted-foreground/60 select-none sticky left-0 bg-surface-subtle">
              {lines.map((_, i) => (
                <div key={i}>{i + 1}</div>
              ))}
            </div>
          )}
          <pre className="py-4 px-4 text-foreground whitespace-pre">
            {text || emptyText}
          </pre>
        </div>
      )}
    </div>
  );
}

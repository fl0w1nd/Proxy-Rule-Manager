"use client";

import { lazy, Suspense } from "react";
import type { EditorProps } from "@monaco-editor/react";
import { Loader2 } from "lucide-react";

const MonacoEditor = lazy(() => import("@monaco-editor/react"));

function EditorFallback({ height }: { height?: string | number }) {
  return (
    <div
      className="flex items-center justify-center rounded-xl border border-border/50 bg-surface-subtle/60"
      style={{ height: height || 300 }}
    >
      <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
    </div>
  );
}

export function LazyMonacoEditor(props: EditorProps) {
  return (
    <Suspense fallback={<EditorFallback height={props.height} />}>
      <MonacoEditor {...props} />
    </Suspense>
  );
}

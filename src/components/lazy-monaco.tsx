"use client";

import { lazy, Suspense } from "react";
import type { EditorProps } from "@monaco-editor/react";
import { Loader2 } from "lucide-react";

const MonacoEditor = lazy(() => import("@monaco-editor/react"));

function EditorFallback({ height }: { height?: string | number }) {
  return (
    <div
      className="flex items-center justify-center bg-muted/30 rounded-md border"
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

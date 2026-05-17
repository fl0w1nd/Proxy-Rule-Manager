"use client";

import { useCallback, useRef, useState } from "react";
import type { editor as monacoEditor } from "monaco-editor";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { FileText, Maximize2, Minimize2, AlertTriangle } from "lucide-react";
import { LazyMonacoEditor } from "./lazy-monaco";

interface LocalContentDialogProps {
  open: boolean;
  initialContent: string;
  editorTheme: string;
  onSave: (content: string) => void;
  onClose: () => void;
}

// Above this size we abandon Monaco entirely and fall back to a plain textarea.
// Monaco loses its appeal for huge plaintext sources and starts to noticeably
// stall the dialog open + keystroke loop.
const TEXTAREA_FALLBACK_LINE_THRESHOLD = 8_000;
const TEXTAREA_FALLBACK_BYTE_THRESHOLD = 600_000;

function countLines(text: string): number {
  if (!text) return 0;
  let n = 1;
  for (let i = 0; i < text.length; i++) {
    if (text.charCodeAt(i) === 10) n++;
  }
  return n;
}

export function LocalContentDialog({
  open,
  initialContent,
  editorTheme,
  onSave,
  onClose,
}: LocalContentDialogProps) {
  // Keep latest editor content in a ref so per-keystroke onChange does not
  // re-render the dialog (Monaco's controlled `value` mode would otherwise
  // run a full text diff against its model on every keystroke).
  const latestContentRef = useRef(initialContent);
  const editorRef = useRef<monacoEditor.IStandaloneCodeEditor | null>(null);
  const [isFullscreen, setIsFullscreen] = useState(false);

  // editor.tsx keys this dialog by source index, so the ref above is
  // always initialised from the latest `initialContent` on mount — no
  // need to re-sync during render.

  const lineCount = countLines(initialContent);
  const useTextareaFallback =
    lineCount > TEXTAREA_FALLBACK_LINE_THRESHOLD ||
    initialContent.length > TEXTAREA_FALLBACK_BYTE_THRESHOLD;

  const handleClose = useCallback(() => {
    setIsFullscreen(false);
    onClose();
  }, [onClose]);

  const handleSave = () => {
    const value =
      editorRef.current?.getValue() ?? latestContentRef.current ?? "";
    onSave(value);
    handleClose();
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) handleClose();
      }}
    >
      <DialogContent
        onEscapeKeyDown={(e) => {
          if (isFullscreen) {
            e.preventDefault();
            setIsFullscreen(false);
          }
        }}
        onPointerDownOutside={(e) => {
          if (isFullscreen) e.preventDefault();
        }}
        className={`flex flex-col min-h-0${isFullscreen ? " !fixed !inset-0 !w-screen !h-screen !max-w-none !max-h-none !left-0 !top-0 !translate-x-0 !translate-y-0 !rounded-none !transform-none" : " max-w-3xl max-h-[80vh]"}`}
      >
        <DialogHeader className={isFullscreen ? "shrink-0" : undefined}>
          <DialogTitle className="flex items-center gap-2">
            <FileText className="w-5 h-5 text-primary" />
            编辑本地内容
          </DialogTitle>
          <DialogDescription>编辑规则的本地内容数据来源</DialogDescription>
        </DialogHeader>

        <div className={`flex-1 min-h-0 space-y-2${isFullscreen ? " flex flex-col" : ""}`}>
          <div className="flex items-center justify-between px-1">
            <Label className="text-sm">内容</Label>
            <div className="flex items-center gap-2">
              {useTextareaFallback && (
                <span className="inline-flex items-center gap-1 text-[11px] text-warning">
                  <AlertTriangle className="w-3 h-3" />
                  大文件模式
                </span>
              )}
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setIsFullscreen(!isFullscreen)}
                className="h-7 px-2"
                title={isFullscreen ? "退出全屏 (ESC)" : "全屏编辑 (ESC 退出)"}
              >
                {isFullscreen ? (
                  <Minimize2 className="w-3.5 h-3.5 mr-1" />
                ) : (
                  <Maximize2 className="w-3.5 h-3.5 mr-1" />
                )}
                {isFullscreen ? "退出全屏" : "全屏"}
              </Button>
            </div>
          </div>
          <div
            className={`relative border border-border rounded-lg overflow-hidden ${isFullscreen ? "flex-1 min-h-0" : ""}`}
          >
            {useTextareaFallback ? (
              <textarea
                defaultValue={initialContent}
                spellCheck={false}
                onChange={(e) => {
                  latestContentRef.current = e.target.value;
                }}
                className="block w-full font-mono text-xs leading-5 px-3 py-2 bg-background text-foreground resize-none focus:outline-none"
                style={{ height: isFullscreen ? "100%" : "400px" }}
              />
            ) : (
              <LazyMonacoEditor
                height={isFullscreen ? "100%" : "400px"}
                language="plaintext"
                defaultValue={initialContent}
                onChange={(value) => {
                  // Update ref without triggering React re-renders.
                  latestContentRef.current = value ?? "";
                }}
                onMount={(editor) => {
                  editorRef.current = editor;
                }}
                theme={editorTheme}
                options={{
                  minimap: { enabled: isFullscreen },
                  fontSize: 13,
                  lineNumbers: "on",
                  scrollBeyondLastLine: false,
                  automaticLayout: true,
                  tabSize: 2,
                  wordWrap: "off",
                  padding: { top: 12, bottom: 12 },
                  scrollbar: {
                    horizontal: "visible",
                    vertical: "visible",
                    horizontalScrollbarSize: 10,
                    verticalScrollbarSize: 10,
                  },
                  // Disable expensive features that aren't useful for plain
                  // text rule sources; these are the main contributors to
                  // first-paint stalls on large files.
                  folding: false,
                  occurrencesHighlight: "off",
                  renderLineHighlight: "none",
                  bracketPairColorization: { enabled: false },
                  overviewRulerBorder: false,
                  overviewRulerLanes: 0,
                  hideCursorInOverviewRuler: true,
                  selectionHighlight: false,
                }}
              />
            )}
          </div>
        </div>

        <div className={`flex justify-end gap-3 pt-4${isFullscreen ? " shrink-0" : ""}`}>
          <Button variant="outline" onClick={handleClose}>
            取消
          </Button>
          <Button onClick={handleSave}>确定</Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

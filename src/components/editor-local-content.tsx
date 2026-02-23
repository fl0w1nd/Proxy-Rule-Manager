"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { FileText, Maximize2, Minimize2 } from "lucide-react";
import Editor from "@monaco-editor/react";

interface LocalContentDialogProps {
  open: boolean;
  initialContent: string;
  editorTheme: string;
  onSave: (content: string) => void;
  onClose: () => void;
}

export function LocalContentDialog({
  open,
  initialContent,
  editorTheme,
  onSave,
  onClose,
}: LocalContentDialogProps) {
  const [draft, setDraft] = useState(initialContent);
  const [isFullscreen, setIsFullscreen] = useState(false);

  const handleClose = () => {
    setIsFullscreen(false);
    onClose();
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
          <div
            className={`relative border border-border rounded-lg overflow-hidden ${isFullscreen ? "flex-1 min-h-0" : ""}`}
          >
            <Editor
              height={isFullscreen ? "100%" : "400px"}
              language="plaintext"
              value={draft}
              onChange={(value) => setDraft(value || "")}
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
              }}
            />
          </div>
        </div>

        <div className={`flex justify-end gap-3 pt-4${isFullscreen ? " shrink-0" : ""}`}>
          <Button variant="outline" onClick={handleClose}>
            取消
          </Button>
          <Button
            onClick={() => {
              onSave(draft);
              handleClose();
            }}
          >
            确定
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

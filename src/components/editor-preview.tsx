"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Copy, Eye, Loader2, CheckCircle, XCircle } from "lucide-react";
import { toast } from "sonner";
import { CodeViewer } from "./code-viewer";
import { ClientType } from "@/lib/schema";
import { PreviewResponse, ClientConfig } from "@/lib/api-client";

interface PreviewDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  ruleName: string;
  isLoading: boolean;
  previewData: PreviewResponse | null;
  clientsList: ClientConfig[];
}

export function PreviewDialog({
  open,
  onOpenChange,
  ruleName,
  isLoading,
  previewData,
  clientsList,
}: PreviewDialogProps) {
  const [activeClient, setActiveClient] = useState<ClientType | "">("");

  const resolvedClient =
    activeClient || (previewData ? (Object.keys(previewData.contents)[0] as ClientType) : "");

  const getDisplayName = (clientId: string) =>
    clientsList.find((c) => c.id === clientId)?.displayName || clientId;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl w-[90vw] h-[70vh] bg-background border-border flex flex-col p-0">
        <DialogHeader className="px-6 pt-6 pb-4 border-b border-border shrink-0">
          <DialogTitle className="text-foreground flex items-center gap-2">
            <Eye className="w-5 h-5 text-primary" />
            预览: {ruleName || "未命名规则"}
          </DialogTitle>
        </DialogHeader>

        {isLoading ? (
          <div className="flex-1 flex items-center justify-center">
            <Loader2 className="w-8 h-8 animate-spin text-primary" />
          </div>
        ) : previewData ? (
          <div className="flex-1 flex flex-col min-h-0 overflow-hidden">
            {previewData.diagnostics.sourceResults.length > 0 && (
              <div className="shrink-0 border-b border-border bg-surface-subtle/60 px-6 py-3">
                <p className="text-sm text-muted-foreground mb-2">数据源状态:</p>
                <div className="flex flex-wrap gap-4">
                  {previewData.diagnostics.sourceResults.map((source, i) => (
                    <div key={i} className="flex items-center gap-2 text-sm">
                      {source.success ? (
                        <CheckCircle className="w-4 h-4 text-success" />
                      ) : (
                        <XCircle className="w-4 h-4 text-destructive" />
                      )}
                      <span className="text-xs font-medium text-muted-foreground">#{i + 1}</span>
                      <span className="text-foreground/80 truncate max-w-xs">{source.url}</span>
                      {source.size !== undefined && source.size > 0 && (
                        <span className="text-muted-foreground">
                          ({(source.size / 1024).toFixed(1)} KB)
                        </span>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            <Tabs
              value={resolvedClient}
              onValueChange={(v) => setActiveClient(v as ClientType)}
              className="flex-1 flex flex-col min-h-0"
            >
              <div className="px-6 py-3 border-b border-border flex items-center justify-between shrink-0">
                <TabsList>
                  {Object.keys(previewData.contents).map((client) => (
                    <TabsTrigger
                      key={client}
                      value={client}
                    >
                      {getDisplayName(client)}
                    </TabsTrigger>
                  ))}
                </TabsList>
                <span className="text-sm text-muted-foreground">
                  {previewData.contents[resolvedClient as ClientType]?.split("\n").length || 0} 行
                </span>
              </div>

              {Object.entries(previewData.contents).map(([client, content]) => (
                <TabsContent
                  key={client}
                  value={client}
                  className="flex-1 m-0 relative min-h-0 overflow-hidden"
                >
                  <div className="absolute top-2 right-2 z-10">
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={async () => {
                        try {
                          await navigator.clipboard.writeText(content);
                          toast.success("已复制内容");
                        } catch {
                          toast.error("复制失败，请手动选择内容复制");
                        }
                      }}
                      className="border border-border/50 bg-background/90 shadow-[var(--shadow-xs)] hover:bg-background"
                      title="复制内容"
                    >
                      <Copy className="w-4 h-4" />
                    </Button>
                  </div>
                  <CodeViewer
                    content={content}
                    showLineNumbers={false}
                    className="h-full rounded-none border-none"
                    height="100%"
                  />
                </TabsContent>
              ))}
            </Tabs>
          </div>
        ) : (
          <div className="flex-1 flex items-center justify-center text-muted-foreground">
            无预览数据
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

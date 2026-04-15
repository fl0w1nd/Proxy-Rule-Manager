"use client";

import { useState } from "react";
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

export function DiffViewer({ content, className, defaultDiffStyle = "unified" }: DiffViewerProps) {
    const [diffStyle, setDiffStyle] = useState<DiffStyle>(defaultDiffStyle);

    if (!content) return null;

    return (
        <div className={cn("overflow-x-auto rounded-xl border border-border/70 bg-card", className)}>
            <div className="flex items-center justify-end gap-1 border-b border-border/60 bg-surface-subtle/60 px-2 py-1.5">
                <Button
                    variant={diffStyle === "unified" ? "secondary" : "ghost"}
                    size="icon"
                    className="h-6 w-6"
                    onClick={() => setDiffStyle("unified")}
                    title="统一视图"
                >
                    <Rows2 className="h-3.5 w-3.5" />
                </Button>
                <Button
                    variant={diffStyle === "split" ? "secondary" : "ghost"}
                    size="icon"
                    className="h-6 w-6"
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
        </div>
    );
}

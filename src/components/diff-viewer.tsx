"use client";

import { PatchDiff } from "@pierre/diffs/react";
import { cn } from "@/lib/utils";

interface DiffViewerProps {
    content: string;
    className?: string;
}

export function DiffViewer({ content, className }: DiffViewerProps) {
    if (!content) return null;

    return (
        <div className={cn("overflow-x-auto rounded-md border", className)}>
            <PatchDiff
                patch={content}
                options={{
                    disableFileHeader: true,
                    diffStyle: "unified",
                }}
            />
        </div>
    );
}

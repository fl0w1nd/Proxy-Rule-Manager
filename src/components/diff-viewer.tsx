import React from "react";
import { cn } from "@/lib/utils";

interface DiffViewerProps {
    content: string;
    className?: string;
}

export function DiffViewer({ content, className }: DiffViewerProps) {
    if (!content) return null;

    // Optimize: Memorize line processing to prevent re-parsing on every render
    const lines = React.useMemo(() => {
        const rawLines = content.split('\n');
        return rawLines.filter((line, index) => {
            // Basic heuristic: if it's the first few lines and starts with --- or +++
            if (index < 3 && (line.startsWith('--- ') || line.startsWith('+++ '))) {
                return false;
            }
            return true;
        });
    }, [content]);

    // Performance: Render initially a small batch to ensure smooth animation
    const INITIAL_BATCH = 100;
    const LOAD_BATCH = 500;
    const [visibleCount, setVisibleCount] = React.useState(INITIAL_BATCH);

    // Reset when content changes
    React.useEffect(() => {
        setVisibleCount(INITIAL_BATCH);
    }, [content]);

    const displayLines = lines.slice(0, visibleCount);
    const remainingCount = lines.length - visibleCount;

    const showMore = () => {
        setVisibleCount(prev => Math.min(prev + LOAD_BATCH, lines.length));
    };

    const showAll = () => {
        setVisibleCount(lines.length);
    };

    return (
        <div className={cn("font-mono text-xs overflow-x-auto bg-background p-4 rounded-md border min-w-full", className)}>
            <div className="w-max min-w-full">
                {displayLines.map((line, i) => {
                    let bgClass = "transparent";
                    let textClass = "text-foreground";
                    let marker = "";
                    let code = line;
                    let markerClass = "text-muted-foreground/40";

                    // Semantic Coloring
                    if (line.startsWith('@@')) {
                        // Header: @@ -1,7 +1,6 @@
                        bgClass = "bg-blue-500/5 dark:bg-blue-500/10";
                        textClass = "text-blue-600 dark:text-blue-400 font-semibold opacity-90";
                        code = line;
                        marker = "";
                    } else if (line.startsWith('+') && !line.startsWith('+++')) {
                        // Added
                        bgClass = "bg-green-500/10 dark:bg-green-500/20";
                        textClass = "text-green-800 dark:text-green-300";
                        marker = "+";
                        markerClass = "text-green-600 dark:text-green-400 select-none";
                        code = line.substring(1);
                    } else if (line.startsWith('-') && !line.startsWith('---')) {
                        // Removed
                        bgClass = "bg-red-500/10 dark:bg-red-500/20";
                        textClass = "text-red-800 dark:text-red-300";
                        marker = "-";
                        markerClass = "text-red-600 dark:text-red-400 select-none";
                        code = line.substring(1);
                    } else {
                        // Context
                        textClass = "text-muted-foreground";
                        marker = "";
                        code = line.startsWith(' ') ? line.substring(1) : line;
                    }

                    return (
                        <div key={i} className={cn("flex items-start leading-5 py-[1px]", bgClass)}>
                            <div className={cn("w-8 shrink-0 text-center select-none font-mono opacity-60", markerClass)}>
                                {marker}
                            </div>
                            <div className={cn("whitespace-pre flex-1 pl-1", textClass)}>
                                {code}
                            </div>
                        </div>
                    );
                })}

                {remainingCount > 0 && (
                    <div className="py-6 text-center bg-muted/20 mt-2 rounded flex flex-col items-center gap-2">
                        <span className="text-muted-foreground italic">
                            ... {remainingCount} more lines hidden ...
                        </span>
                        <div className="flex items-center gap-3">
                            <button
                                onClick={showMore}
                                className="px-3 py-1.5 text-xs bg-primary/10 hover:bg-primary/20 text-primary rounded transition-colors"
                            >
                                Load next {Math.min(LOAD_BATCH, remainingCount)} lines
                            </button>
                            <button
                                onClick={showAll}
                                className="px-3 py-1.5 text-xs hover:bg-muted text-muted-foreground rounded transition-colors"
                            >
                                Load all
                            </button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}

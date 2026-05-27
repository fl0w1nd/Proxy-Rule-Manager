import { cn } from "@/lib/utils";

interface LoadingStateProps {
  className?: string;
  label?: string;
}

export function LoadingState({ className, label }: LoadingStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-3 py-12 text-muted-foreground",
        className
      )}
      role="status"
      aria-live="polite"
    >
      <div className="flex items-center gap-1.5" aria-hidden="true">
        <span className="block h-1.5 w-1.5 rounded-full bg-primary animate-[bounce_1s_ease-in-out_infinite] [animation-delay:-0.2s]" />
        <span className="block h-1.5 w-1.5 rounded-full bg-primary animate-[bounce_1s_ease-in-out_infinite] [animation-delay:-0.1s]" />
        <span className="block h-1.5 w-1.5 rounded-full bg-primary animate-[bounce_1s_ease-in-out_infinite]" />
      </div>
      {label && (
        <p className="text-xs font-medium text-muted-foreground">{label}</p>
      )}
      <span className="sr-only">加载中</span>
    </div>
  );
}


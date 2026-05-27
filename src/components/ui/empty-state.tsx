import { cn } from "@/lib/utils";
import type { LucideIcon } from "lucide-react";

interface EmptyStateProps {
  icon: LucideIcon;
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}

export function EmptyState({ icon: Icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div className={cn("flex flex-col items-center justify-center py-16 text-center animate-fade-in", className)}>
      <div
        className="relative mx-auto mb-6 flex h-16 w-16 items-center justify-center rounded-2xl border border-border/60 bg-surface-subtle"
        style={{
          backgroundImage:
            "radial-gradient(120% 80% at 50% 0%, color-mix(in oklch, var(--foreground) 4%, transparent) 0%, transparent 60%)",
        }}
      >
        <Icon className="w-7 h-7 text-muted-foreground/50" strokeWidth={1.5} />
      </div>
      <p className="text-base font-semibold text-foreground tracking-tight">{title}</p>
      {description && (
        <p className="text-sm text-muted-foreground mt-1.5 max-w-sm mx-auto leading-relaxed">
          {description}
        </p>
      )}
      {action && <div className="mt-6">{action}</div>}
    </div>
  );
}

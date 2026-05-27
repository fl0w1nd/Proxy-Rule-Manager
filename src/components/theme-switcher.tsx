"use client";

import { useState, useRef, useEffect } from "react";
import { Check, Moon, Sun } from "lucide-react";
import { useTheme, BRAND_LIST } from "./theme-provider";
import { cn } from "@/lib/utils";

export function ThemeSwitcher({ compact = false }: { compact?: boolean }) {
  const { mode, brand, setBrand, toggleMode } = useTheme();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    if (open) document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  if (compact) {
    return (
      <div ref={ref} className="relative">
        <button
          onClick={() => setOpen(!open)}
          className="h-9 w-9 rounded-full inline-flex items-center justify-center hover:bg-sidebar-accent transition-colors"
          title="切换品牌主题"
        >
          <span
            className="w-3.5 h-3.5 rounded-full ring-1 ring-inset ring-black/10 dark:ring-white/15"
            style={{ backgroundColor: BRAND_LIST.find((b) => b.id === brand)?.accent }}
          />
        </button>
        {open && (
          <div className="absolute bottom-full mb-2 left-0 z-50 min-w-[140px] rounded-xl bg-popover border border-border shadow-[var(--shadow-md)] p-1 animate-fade-in">
            {BRAND_LIST.map((b) => (
              <button
                key={b.id}
                onClick={() => { setBrand(b.id); setOpen(false); }}
                className={cn(
                  "w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-colors",
                  brand === b.id
                    ? "bg-accent text-accent-foreground"
                    : "text-popover-foreground hover:bg-accent"
                )}
              >
                <span
                  className="w-3 h-3 rounded-full shrink-0 ring-1 ring-inset ring-black/10 dark:ring-white/15"
                  style={{ backgroundColor: b.accent }}
                />
                <span className="flex-1 text-left">{b.label}</span>
                {brand === b.id && <Check className="w-3 h-3 text-primary" />}
              </button>
            ))}
          </div>
        )}
      </div>
    );
  }

  return (
    <div ref={ref} className="space-y-1.5">
      <div className="flex items-center justify-between px-2">
        <span className="text-[10px] font-semibold text-muted-foreground uppercase tracking-[0.18em]">
          主题
        </span>
        <button
          onClick={toggleMode}
          className="h-7 w-7 rounded-full inline-flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
          title={mode === "light" ? "切换到暗色模式" : "切换到亮色模式"}
          aria-label={mode === "light" ? "切换到暗色模式" : "切换到亮色模式"}
        >
          {mode === "light" ? <Moon className="w-3.5 h-3.5" /> : <Sun className="w-3.5 h-3.5" />}
        </button>
      </div>

      <div className="grid grid-cols-5 gap-1 px-1">
        {BRAND_LIST.map((b) => (
          <button
            key={b.id}
            onClick={() => setBrand(b.id)}
            className={cn(
              "group flex flex-col items-center gap-1 py-1.5 rounded-lg transition-colors duration-150",
              brand === b.id
                ? "bg-accent"
                : "hover:bg-accent/60"
            )}
            title={b.label}
            aria-pressed={brand === b.id}
          >
            <span
              className={cn(
                "block w-4 h-4 rounded-full transition-all duration-200 ease-out ring-1 ring-inset ring-black/10 dark:ring-white/15",
                brand === b.id
                  ? "shadow-[0_0_0_2px_var(--background),0_0_0_3.5px_var(--primary)] scale-105"
                  : "group-hover:scale-105"
              )}
              style={{ backgroundColor: b.accent }}
            />
            <span className={cn(
              "text-[9px] leading-none truncate w-full text-center transition-colors",
              brand === b.id ? "text-foreground font-semibold" : "text-muted-foreground"
            )}>
              {b.label}
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}

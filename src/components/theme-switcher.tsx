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
            className="w-3.5 h-3.5 rounded-full border border-sidebar-border"
            style={{ backgroundColor: BRAND_LIST.find((b) => b.id === brand)?.accent }}
          />
        </button>
        {open && (
          <div className="absolute bottom-full mb-1 left-0 z-50 min-w-[140px] rounded-xl bg-popover border border-popover-border shadow-md p-1 animate-fade-in">
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
                  className="w-3 h-3 rounded-full shrink-0 border border-border"
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
        <span className="text-[11px] font-semibold text-muted-foreground uppercase tracking-widest">主题</span>
        <button
          onClick={toggleMode}
          className="h-7 w-7 rounded-full inline-flex items-center justify-center hover:bg-accent transition-colors"
          title={mode === "light" ? "切换到暗色模式" : "切换到亮色模式"}
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
              "flex flex-col items-center gap-1 py-1.5 rounded-lg transition-all",
              brand === b.id
                ? "bg-accent ring-1 ring-primary/30"
                : "hover:bg-accent"
            )}
            title={b.label}
          >
            <span
              className={cn(
                "w-4 h-4 rounded-full border transition-transform",
                brand === b.id ? "border-primary scale-110" : "border-border"
              )}
              style={{ backgroundColor: b.accent }}
            />
            <span className={cn(
              "text-[9px] leading-none truncate w-full text-center",
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

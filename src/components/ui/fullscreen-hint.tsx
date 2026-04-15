"use client";

import { useEffect, useRef } from "react";

export function FullscreenHint({ visible }: { visible: boolean }) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!visible || !ref.current) return;
    const el = ref.current;
    el.style.opacity = "1";
    const timer = setTimeout(() => {
      el.style.opacity = "0";
    }, 2500);
    return () => clearTimeout(timer);
  }, [visible]);

  if (!visible) return null;

  return (
    <div
      ref={ref}
      className="pointer-events-none select-none rounded-full bg-foreground/80 px-3 py-1 text-xs font-medium text-background transition-opacity duration-500"
      style={{ opacity: 0 }}
    >
      按 ESC 退出全屏
    </div>
  );
}

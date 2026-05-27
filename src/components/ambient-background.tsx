"use client";

export function AmbientBackground() {
  return (
    <div
      className="pointer-events-none fixed inset-0 z-0 overflow-hidden"
      aria-hidden="true"
    >
      {/* Primary atmosphere blob — top-left */}
      <div
        className="ambient-drift-a absolute -top-1/4 -left-1/4 h-[65vh] w-[65vh] rounded-full opacity-[0.05] dark:opacity-[0.06]"
        style={{
          background:
            "radial-gradient(circle at 30% 30%, var(--primary) 0%, transparent 65%)",
        }}
      />
      {/* Secondary atmosphere blob — bottom-right, cooler */}
      <div
        className="ambient-drift-b absolute -bottom-1/4 -right-1/4 h-[55vh] w-[55vh] rounded-full opacity-[0.04] dark:opacity-[0.05]"
        style={{
          background:
            "radial-gradient(circle at 70% 70%, var(--primary) 0%, transparent 70%)",
        }}
      />
      {/* Vignette: pulls focus to center */}
      <div
        className="absolute inset-0"
        style={{
          background:
            "radial-gradient(ellipse at center, transparent 50%, color-mix(in oklch, var(--foreground) 4%, transparent) 100%)",
        }}
      />
      {/* Film grain for tactile depth */}
      <div className="grain-overlay" />
    </div>
  );
}

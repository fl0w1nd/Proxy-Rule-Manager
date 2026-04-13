"use client";

export function AmbientBackground() {
  return (
    <div className="fixed inset-0 z-[1] pointer-events-none overflow-hidden">
      <div className="absolute inset-0 ambient-mesh" />
      <div className="absolute inset-0 ambient-noise" />
    </div>
  );
}

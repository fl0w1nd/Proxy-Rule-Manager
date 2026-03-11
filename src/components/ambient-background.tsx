"use client";

export function AmbientBackground() {
  return (
    <div className="fixed inset-0 z-[1] pointer-events-none overflow-hidden">
      <div className="absolute inset-0 ambient-flow" />
      <div className="absolute ambient-orb ambient-orb-1" />
      <div className="absolute ambient-orb ambient-orb-2" />
    </div>
  );
}

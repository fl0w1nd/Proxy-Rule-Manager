"use client";

export function AmbientBackground() {
  return (
    <div className="fixed inset-0 z-[1] pointer-events-none overflow-hidden">
      <div className="absolute inset-0 ambient-grain" />
      <div className="absolute inset-0 ambient-mesh" />
      <div className="absolute inset-0 ambient-flow" />
      <div className="absolute ambient-orb ambient-orb-1" />
      <div className="absolute ambient-orb ambient-orb-2" />
      <div className="absolute ambient-orb ambient-orb-3" />
      <div className="absolute ambient-orb ambient-orb-4" />
    </div>
  );
}

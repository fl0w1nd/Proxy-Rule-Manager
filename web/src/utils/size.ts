const suffixes: [string, number][] = [
  ['GB', 1024 * 1024 * 1024],
  ['MB', 1024 * 1024],
  ['KB', 1024],
  ['B', 1],
];

export function parseSizeBytes(input: string): number {
  const value = input.trim();
  if (/^\d+$/.test(value)) {
    const n = Number(value);
    if (!Number.isFinite(n) || n < 0) throw new Error('请输入大小，例如 8MB');
    return n;
  }
  const upper = value.toUpperCase();
  const suffix = suffixes.find(([unit]) => upper.endsWith(unit));
  const amount = suffix ? upper.slice(0, -suffix[0].length).trim() : '';
  const n = Number(amount);
  if (!suffix || !amount || !Number.isFinite(n) || n < 0) throw new Error('请输入大小，例如 8MB');
  return n * suffix[1];
}

export function normalizeSize(input: string): string {
  const value = input.trim();
  if (parseSizeBytes(value) <= 0) throw new Error('大小必须大于 0');
  return value;
}

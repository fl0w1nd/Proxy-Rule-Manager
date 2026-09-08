import { parseDurationSeconds } from './duration';
import { parseSizeBytes } from './size';

export type DurationUnit = 'ms' | 's' | 'm' | 'h' | 'd';
export type SizeUnit = 'KB' | 'MB' | 'GB';
export type QuantityKind = 'duration' | 'size';

const durationSizes: Record<DurationUnit, number> = { ms: 1e-3, s: 1, m: 60, h: 3600, d: 86400 };
const sizeSizes: Record<SizeUnit | 'B', number> = { B: 1, KB: 1024, MB: 1024 * 1024, GB: 1024 * 1024 * 1024 };

export const durationUnitLabels: Record<DurationUnit, string> = { ms: '毫秒', s: '秒', m: '分', h: '时', d: '天' };
export const sizeUnitLabels: Record<SizeUnit, string> = { KB: 'KB', MB: 'MB', GB: 'GB' };

export function prettyNumber(n: number): string {
  if (!Number.isFinite(n)) return '';
  const text = n.toFixed(6).replace(/\.?0+$/, '');
  return text === '-0' ? '0' : text;
}

export function quantityUnitOptions(kind: QuantityKind, units: readonly string[]): { value: string; label: string }[] {
  const labels = kind === 'duration' ? durationUnitLabels : sizeUnitLabels;
  return units.map((value) => ({ value, label: labels[value as keyof typeof labels] ?? value }));
}

export function splitDuration(input: string, units: readonly DurationUnit[]): { amount: string; unit: DurationUnit } {
  const simple = /^(?<amount>\d+(?:\.\d*)?|\.\d+)\s*(?<unit>ms|s|m|h|d)$/i.exec(input.trim());
  const unit = simple?.groups?.unit.toLowerCase() as DurationUnit | undefined;
  try {
    const seconds = unit ? Number(simple!.groups!.amount) * durationSizes[unit] : parseDurationSeconds(input);
    return promote(seconds, units, durationSizes, unit && units.includes(unit) ? unit : undefined);
  } catch {
    return { amount: '', unit: units[0] };
  }
}

export function joinDuration(amount: string, unit: DurationUnit): string {
  const text = amount.trim();
  if (!text) return '';
  if (unit === 'd') {
    const n = Number(text);
    if (!Number.isFinite(n)) return '';
    return `${prettyNumber(n * 24)}h`;
  }
  return `${text}${unit}`;
}

export function splitSize(input: string, units: readonly SizeUnit[]): { amount: string; unit: SizeUnit } {
  const simple = /^(?<amount>\d+(?:\.\d*)?|\.\d+)\s*(?<unit>GB|MB|KB|B)$/i.exec(input.trim());
  const unit = simple?.groups?.unit.toUpperCase() as SizeUnit | 'B' | undefined;
  try {
    const bytes = unit ? Number(simple!.groups!.amount) * sizeSizes[unit] : parseSizeBytes(input);
    return promote(bytes, units, sizeSizes, unit && units.includes(unit as SizeUnit) ? unit as SizeUnit : undefined);
  } catch {
    return { amount: '', unit: units[0] };
  }
}

export function joinSize(amount: string, unit: SizeUnit): string {
  const text = amount.trim();
  return text ? `${text}${unit}` : '';
}

export function splitQuantity(kind: QuantityKind, input: string, units: readonly string[]): { amount: string; unit: string } {
  return kind === 'duration'
    ? splitDuration(input, units as readonly DurationUnit[])
    : splitSize(input, units as readonly SizeUnit[]);
}

export function joinQuantity(kind: QuantityKind, amount: string, unit: string): string {
  return kind === 'duration' ? joinDuration(amount, unit as DurationUnit) : joinSize(amount, unit as SizeUnit);
}

function promote<U extends string>(total: number, units: readonly U[], sizes: Record<U, number>, start?: U): { amount: string; unit: U } {
  const floor = start ? sizes[start] : 0;
  const picked = [...units].sort((a, b) => sizes[b] - sizes[a]).find((unit) => sizes[unit] >= floor && total / sizes[unit] >= 1)
    ?? start
    ?? [...units].sort((a, b) => sizes[a] - sizes[b])[0];
  return { amount: prettyNumber(total / sizes[picked]), unit: picked };
}

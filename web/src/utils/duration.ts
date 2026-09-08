const units: Record<string, number> = { ns: 1e-9, us: 1e-6, 'µs': 1e-6, 'μs': 1e-6, ms: 1e-3, s: 1, m: 60, h: 3600 };
const duration = /^\+?(?:(?:\d+(?:\.\d*)?|\.\d+)(?:ns|us|µs|μs|ms|s|m|h))+$/;

export function parseDurationSeconds(input: string): number {
  const value = input.trim();
  if (!duration.test(value)) throw new Error('请输入带单位的时长，例如 25s、500ms 或 30m');
  let seconds = 0;
  for (const match of value.matchAll(/(\d+(?:\.\d*)?|\.\d+)(ns|us|µs|μs|ms|s|m|h)/g)) {
    seconds += Number(match[1]) * units[match[2]];
  }
  if (seconds > 9223372036.854776) throw new Error('时长必须大于 0，且在有效范围内');
  return seconds;
}

export function normalizeDuration(input: string): string {
  const value = input.trim();
  if (parseDurationSeconds(value) < 1e-9) throw new Error('时长必须大于 0，且在有效范围内');
  return value;
}

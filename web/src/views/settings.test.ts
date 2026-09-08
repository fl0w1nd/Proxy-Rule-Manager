import { describe, expect, it } from 'vitest';
import { buildSettingsPatch, readSettings, settingsChanged, validateSettings } from './settings';
import { normalizeDuration } from '../utils/duration';
import { normalizeSize } from '../utils/size';

describe('runtime settings patches', () => {
  const config = { update: { schedule: { mode: 'manual' }, fetch: { timeout: '20s', user_agent: 'PRM-UA' } } };

  it('submits the changed group, retaining source fields and absent defaults', () => {
    const baseline = readSettings(config);
    const form = structuredClone(baseline);
    form.fetch.timeout = '25s';
    expect(buildSettingsPatch(config, baseline, form)).toEqual([
      { op: 'update_fetch', value: { timeout: '25s', user_agent: 'PRM-UA' } },
    ]);
    expect(buildSettingsPatch(config, baseline, baseline)).toEqual([]);
    const unchanged = structuredClone(baseline);
    unchanged.fetch.retries = String(baseline.fetch.retries);
    expect(settingsChanged(baseline, unchanged)).toBe(false);
  });

  it('removes inactive schedule fields when switching modes', () => {
    const source = { update: { schedule: { mode: 'cron', cron: '0 3 * * *', timezone: 'Asia/Shanghai', interval: '2h' } } };
    const baseline = readSettings(source);
    const form = structuredClone(baseline);
    form.schedule.mode = 'manual';
    expect(buildSettingsPatch(source, baseline, form)).toEqual([
      { op: 'update_schedule', value: { mode: 'manual', timezone: 'Asia/Shanghai' } },
    ]);
    form.schedule.mode = 'interval';
    expect(buildSettingsPatch(source, baseline, form)).toEqual([
      { op: 'update_schedule', value: { mode: 'interval', interval: '2h', timezone: 'Asia/Shanghai' } },
    ]);
  });

  it('ignores edits to inactive schedule fields after returning to manual', () => {
    const baseline = readSettings(config);
    const form = structuredClone(baseline);
    form.schedule.cron = '0 4 * * *';
    form.schedule.interval = '45m';
    expect(settingsChanged(baseline, form)).toBe(false);
    expect(buildSettingsPatch(config, baseline, form)).toEqual([]);
  });

  it('serializes numeric history values and retains the required companion field', () => {
    const baseline = readSettings(config);
    const form = structuredClone(baseline);
    form.history.history_limit = '42';
    expect(buildSettingsPatch(config, baseline, form)).toEqual([
      { op: 'update_history', value: { history_retention: '168h', history_limit: 42 } },
    ]);
  });

  it('validates only submitted fields, positive quantities, cron, timezone and concurrency', () => {
    const source = { update: { schedule: { mode: 'manual' }, fetch: { timeout: '20s', max_download: 'bogus', user_agent: 'PRM-UA' } } };
    const baseline = readSettings(source);
    const form = structuredClone(baseline);
    form.fetch.timeout = '';
    form.fetch.per_host_concurrency = 20;
    form.history.history_limit = '';
    form.schedule.timezone = 'Not/AZone';
    form.fetch.user_agent = '  ';
    expect(Object.keys(validateSettings(form, baseline))).toEqual(expect.arrayContaining([
      'fetch.timeout', 'fetch.per_host_concurrency', 'history.history_limit', 'schedule.timezone', 'fetch.user_agent',
    ]));
    expect(validateSettings(form, baseline)['fetch.max_download']).toBeUndefined();
    form.fetch.max_download = '0MB';
    expect(validateSettings(form, baseline)['fetch.max_download']).toBe('请输入大于 0 的数字');
    const cron = structuredClone(baseline);
    cron.schedule.mode = 'cron';
    cron.schedule.cron = '0 3';
    expect(validateSettings(cron, baseline)['schedule.cron']).toBe('请输入五段 Cron，例如 0 3 * * *');
    expect(normalizeDuration(' 1h30m ')).toBe('1h30m');
    expect(normalizeDuration('500ms')).toBe('500ms');
    expect(normalizeSize('4096')).toBe('4096');
    expect(normalizeSize(' 8 MB ')).toBe('8 MB');
    expect(() => normalizeDuration('0s')).toThrow();
    expect(() => normalizeDuration('25')).toThrow();
    expect(() => normalizeSize('0')).toThrow();
  });
});

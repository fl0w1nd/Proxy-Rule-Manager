import { describe, expect, it } from 'vitest';
import { joinDuration, joinSize, splitDuration, splitSize } from './quantity';

describe('quantity split and join', () => {
  it('picks the largest duration unit that stays at least 1', () => {
    expect(splitDuration('15s', ['ms', 's', 'm'])).toEqual({ amount: '15', unit: 's' });
    expect(splitDuration('500ms', ['ms', 's'])).toEqual({ amount: '500', unit: 'ms' });
    expect(splitDuration('1h30m', ['m', 'h'])).toEqual({ amount: '1.5', unit: 'h' });
    expect(splitDuration('168h', ['h', 'd'])).toEqual({ amount: '7', unit: 'd' });
    expect(splitDuration('0.25m', ['ms', 's', 'm'])).toEqual({ amount: '0.25', unit: 'm' });
  });

  it('writes Go duration strings, converting days to hours', () => {
    expect(joinDuration('25', 's')).toBe('25s');
    expect(joinDuration('7', 'd')).toBe('168h');
    expect(joinDuration('1.5', 'h')).toBe('1.5h');
    expect(joinDuration('', 's')).toBe('');
  });

  it('picks the largest size unit that stays at least 1', () => {
    expect(splitSize('4MB', ['KB', 'MB', 'GB'])).toEqual({ amount: '4', unit: 'MB' });
    expect(splitSize('4096', ['KB', 'MB', 'GB'])).toEqual({ amount: '4', unit: 'KB' });
    expect(splitSize('50MB', ['KB', 'MB', 'GB'])).toEqual({ amount: '50', unit: 'MB' });
    expect(splitSize('0.5MB', ['KB', 'MB', 'GB'])).toEqual({ amount: '0.5', unit: 'MB' });
  });

  it('writes size strings without spaces', () => {
    expect(joinSize('8', 'MB')).toBe('8MB');
    expect(joinSize('1.5', 'GB')).toBe('1.5GB');
  });
});

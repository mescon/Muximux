import { describe, it, expect } from 'vitest';
import { parseDurationMs } from './duration';

describe('parseDurationMs', () => {
  it.each([
    ['7d', 7 * 86_400_000],
    ['1h', 3_600_000],
    ['90m', 5_400_000],
    ['1h30m', 5_400_000],
    ['1.5h', 5_400_000],
    [' 2h ', 7_200_000],
    ['45s', 45_000],
    ['500ms', 500],
    ['+1h', 3_600_000],
    ['+2d', 2 * 86_400_000],
    ['.5h', 1_800_000],
    ['1us', 0.001],
  ])('parses %s', (input, ms) => {
    expect(parseDurationMs(input)).toBe(ms);
  });

  it.each(['', 'soon', '1', 'h', '1d12h', '-1h', '1 h', '1x', 'd', '+', '.h', '1.2.3h', '1hm'])('rejects %j', (input) => {
    expect(parseDurationMs(input)).toBeNull();
  });

  it('stays fast on long hostile input', () => {
    const start = performance.now();
    expect(parseDurationMs('99h'.repeat(5000) + 'x')).toBeNull();
    expect(performance.now() - start).toBeLessThan(200);
  });
});

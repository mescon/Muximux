/**
 * parseDurationMs mirrors the server's config.ParseFlexDuration: a whole
 * number of days ("7d") or a Go duration ("90m", "1h30m", "1.5h").
 * Returns milliseconds, or null when the server would reject the value.
 */
const UNIT_MS: Record<string, number> = {
  ns: 1e-6,
  us: 1e-3,
  'µs': 1e-3,
  ms: 1,
  s: 1000,
  m: 60_000,
  h: 3_600_000,
};

export function parseDurationMs(input: string): number | null {
  const s = input.trim();
  const days = /^(\d+)d$/.exec(s);
  if (days) return Number(days[1]) * 86_400_000;
  if (!/^(?:(?:\d+\.?\d*|\.\d+)(?:ns|us|µs|ms|s|m|h))+$/.test(s)) return null;
  let total = 0;
  for (const part of s.matchAll(/(\d+\.?\d*|\.\d+)(ns|us|µs|ms|s|m|h)/g)) {
    total += Number(part[1]) * UNIT_MS[part[2]];
  }
  return total;
}

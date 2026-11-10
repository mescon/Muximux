/**
 * parseDurationMs mirrors the server's config.ParseFlexDuration: a whole
 * number of days ("7d") or a Go duration ("90m", "1h30m", "1.5h", "+2h").
 * Returns milliseconds, or null when the server would reject the value.
 *
 * Parsed with a single left-to-right scan rather than a regular
 * expression, so the cost is linear in the input whatever it holds.
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
// Longest first, so "ms" is not read as "m" followed by "s".
const UNITS = ['ns', 'us', 'µs', 'ms', 's', 'm', 'h'];

const isDigit = (c: string) => c >= '0' && c <= '9';

function parseDays(s: string): number | null {
  const body = s.slice(0, -1);
  if (body === '' || ![...body].every(isDigit)) return null;
  return Number(body) * 86_400_000;
}

/** Reads a decimal number at s[i]; returns it and the next index. */
function readNumber(s: string, i: number): [number, number] | null {
  let j = i;
  let digits = 0;
  while (j < s.length && isDigit(s[j])) { j++; digits++; }
  if (j < s.length && s[j] === '.') {
    j++;
    while (j < s.length && isDigit(s[j])) { j++; digits++; }
  }
  if (digits === 0) return null;
  return [Number(s.slice(i, j)), j];
}

function readUnit(s: string, i: number): string | null {
  for (const u of UNITS) {
    if (s.startsWith(u, i)) return u;
  }
  return null;
}

export function parseDurationMs(input: string): number | null {
  let s = input.trim();
  if (s.startsWith('+')) s = s.slice(1);
  if (s.endsWith('d')) return parseDays(s);
  if (s === '') return null;
  let total = 0;
  let i = 0;
  while (i < s.length) {
    const num = readNumber(s, i);
    if (!num) return null;
    const unit = readUnit(s, num[1]);
    if (!unit) return null;
    total += num[0] * UNIT_MS[unit];
    i = num[1] + unit.length;
  }
  return total;
}

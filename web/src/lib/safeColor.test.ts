import { describe, it, expect } from 'vitest';
import { safeColor } from './safeColor';

describe('safeColor', () => {
  it('accepts valid CSS colour forms unchanged', () => {
    for (const c of ['#fff', '#ffffff', '#ffffffff', 'red', 'transparent', 'currentColor',
      'rgb(1,2,3)', 'rgba(1,2,3,0.5)', 'hsl(120, 50%, 50%)', 'var(--accent-primary)']) {
      expect(safeColor(c)).toBe(c);
    }
  });

  it('trims surrounding whitespace on a valid colour', () => {
    expect(safeColor('  #abc  ')).toBe('#abc');
  });

  it('rejects CSS injection and returns the fallback', () => {
    for (const bad of [
      'red; background-image: url(https://evil/x)',
      '#fff);}body{display:none',
      'url(javascript:alert(1))',
      'expression(alert(1))',
      'red /* comment */',
      'var(--x); position:fixed',
      '',
    ]) {
      expect(safeColor(bad, 'FALLBACK')).toBe('FALLBACK');
    }
  });

  it('returns the fallback for null/undefined', () => {
    expect(safeColor(null, 'x')).toBe('x');
    expect(safeColor(undefined, 'x')).toBe('x');
    expect(safeColor('')).toBe('');
  });
});

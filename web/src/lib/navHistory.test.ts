import { describe, it, expect } from 'vitest';
import { NAV_STATE, isMuximuxEntry } from './navHistory';

describe('isMuximuxEntry', () => {
  it('accepts NAV_STATE', () => {
    expect(isMuximuxEntry(NAV_STATE)).toBe(true);
  });
  it('rejects null, empty and non-true values', () => {
    expect(isMuximuxEntry(null)).toBe(false);
    expect(isMuximuxEntry({})).toBe(false);
    expect(isMuximuxEntry({ muximux: 'yes' })).toBe(false);
    expect(isMuximuxEntry('muximux')).toBe(false);
  });
});

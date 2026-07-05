import { describe, it, expect } from 'vitest';
import { isSafeAppUrl } from './appUrl';

describe('isSafeAppUrl', () => {
  it('accepts absolute http(s) URLs', () => {
    expect(isSafeAppUrl('http://radarr:7878')).toBe(true);
    expect(isSafeAppUrl('https://radarr.example.com/path')).toBe(true);
  });

  it('accepts a single-slash same-origin path', () => {
    expect(isSafeAppUrl('/proxy/radarr/')).toBe(true);
    expect(isSafeAppUrl('/local/thing')).toBe(true);
  });

  it('rejects empty, protocol-relative, and backslash paths', () => {
    expect(isSafeAppUrl('')).toBe(false);
    expect(isSafeAppUrl('//evil.example.com')).toBe(false);
    expect(isSafeAppUrl('/\\evil.example.com')).toBe(false);
  });

  it('rejects a bare schemeless string (not resolved against a base)', () => {
    expect(isSafeAppUrl('not-a-url')).toBe(false);
    expect(isSafeAppUrl('radarr:7878')).toBe(false);
  });

  it('rejects dangerous schemes', () => {
    expect(isSafeAppUrl('javascript:alert(1)')).toBe(false);
    expect(isSafeAppUrl('file:///etc/passwd')).toBe(false);
    expect(isSafeAppUrl('data:text/html,<script>1</script>')).toBe(false);
  });
});

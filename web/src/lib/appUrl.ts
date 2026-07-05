// isSafeAppUrl is the single frontend definition of a valid app URL, shared
// by the zod form schema (schemas.ts) and the iframe src guard (AppFrame). It
// mirrors the backend config.validateAppURL rule: a single-slash same-origin
// path is allowed (proxied/local apps), a protocol-relative "//" or "/\" is
// not, and any absolute URL must use http or https so a javascript:/file:/
// data: URL can never become an iframe src.
export function isSafeAppUrl(raw: string): boolean {
  if (!raw) return false;
  if (raw.startsWith('/')) {
    return !raw.startsWith('//') && !raw.startsWith('/\\');
  }
  // Parse without a base URL (like the backend's url.Parse) so a bare,
  // schemeless string such as "not-a-url" is NOT silently resolved into a
  // valid relative URL -- only a genuine absolute http(s) URL passes.
  try {
    const u = new URL(raw);
    return u.protocol === 'http:' || u.protocol === 'https:';
  } catch {
    return false;
  }
}

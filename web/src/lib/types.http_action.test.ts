import { describe, it, expect } from 'vitest';
import { makeApp } from './types';
import type { App, FireActionResult } from './types';

describe('App.open_mode http_action extension', () => {
  it('accepts http_action as an open_mode', () => {
    const app: App = makeApp({ open_mode: 'http_action' });
    expect(app.open_mode).toBe('http_action');
  });

  it('accepts http_action fields as optional', () => {
    const app: App = makeApp({
      open_mode: 'http_action',
      http_action_method: 'POST',
      http_action_headers: { 'X-Token': 'abc' },
      http_action_confirm: true,
      http_action_show_toast: false,
    });
    expect(app.http_action_method).toBe('POST');
    expect(app.http_action_headers).toEqual({ 'X-Token': 'abc' });
    expect(app.http_action_confirm).toBe(true);
    expect(app.http_action_show_toast).toBe(false);
  });

  it('exports FireActionResult shape', () => {
    const ok: FireActionResult = { status: 200, latency_ms: 12, url_host: 'a.b', method: 'POST' };
    const err: FireActionResult = { error: 'fail', latency_ms: 0 };
    expect(ok.status).toBe(200);
    expect(err.error).toBe('fail');
  });
});

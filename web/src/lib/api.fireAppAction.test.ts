import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireAppAction } from './api';
import type { FireActionResult } from './types';

describe('fireAppAction', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs to /api/app-action/{name} with X-Requested-With', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ status: 200, latency_ms: 42, url_host: 'example.com', method: 'POST' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const r: FireActionResult = await fireAppAction('My App');

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/api/app-action/My%20App');
    expect((init as RequestInit).method).toBe('POST');
    const headers = (init as RequestInit).headers as Record<string, string>;
    expect(headers['X-Requested-With']).toBe('XMLHttpRequest');
    expect(r.status).toBe(200);
    expect(r.url_host).toBe('example.com');
  });

  it('returns the JSON body on 502 (network error) instead of throwing', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'connection refused', latency_ms: 5, method: 'POST' }), {
        status: 502,
        headers: { 'Content-Type': 'application/json' },
      })
    );
    const r: FireActionResult = await fireAppAction('Webhook');
    expect(r.error).toBe('connection refused');
    expect(r.latency_ms).toBe(5);
  });

  it('throws ApiError on 403 (auth denied) so callers can branch', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('Access denied', { status: 403 })
    );
    await expect(fireAppAction('Restricted')).rejects.toThrow(/403/);
  });

  it('encodes special characters in name', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ status: 200, latency_ms: 0 }), { status: 200, headers: { 'Content-Type': 'application/json' } })
    );
    await fireAppAction('a/b?c');
    const [url] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('a%2Fb%3Fc');
  });
});

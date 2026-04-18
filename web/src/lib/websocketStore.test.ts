import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';

// We need to mock healthStore before importing websocketStore
vi.mock('./healthStore', async () => {
  const { writable } = await import('svelte/store');
  return {
    healthData: writable(new Map()),
  };
});

// Mock WebSocket class
class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  readyState = MockWebSocket.CONNECTING;
  onopen: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;

  url: string;

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }

  close = vi.fn(() => {
    this.readyState = MockWebSocket.CLOSED;
  });

  send = vi.fn();

  // Helper to simulate events
  simulateOpen() {
    this.readyState = MockWebSocket.OPEN;
    if (this.onopen) this.onopen(new Event('open'));
  }

  simulateClose(code = 1000, reason = '') {
    this.readyState = MockWebSocket.CLOSED;
    if (this.onclose) this.onclose({ code, reason } as CloseEvent);
  }

  simulateError() {
    if (this.onerror) this.onerror(new Event('error'));
  }

  simulateMessage(data: string) {
    if (this.onmessage) this.onmessage({ data } as MessageEvent);
  }

  static instances: MockWebSocket[] = [];
  static clear() {
    MockWebSocket.instances = [];
  }
}

describe('websocketStore', () => {
  let originalWebSocket: typeof WebSocket;

  beforeEach(() => {
    vi.useFakeTimers();
    MockWebSocket.clear();

    originalWebSocket = globalThis.WebSocket;
    (globalThis as unknown as Record<string, unknown>).WebSocket = MockWebSocket;

    // Mock window.location
    Object.defineProperty(window, 'location', {
      value: {
        protocol: 'http:',
        host: 'localhost:3000',
      },
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    (globalThis as unknown as Record<string, unknown>).WebSocket = originalWebSocket;
    vi.restoreAllMocks();
  });

  // Use dynamic import to get a fresh module for each test
  async function getModule() {
    // Clear module cache
    vi.resetModules();
    const mod = await import('./websocketStore');
    return mod;
  }

  describe('connect', () => {
    it('creates a WebSocket connection with correct URL', async () => {
      const { connect, connectionState } = await getModule();

      connect();

      expect(MockWebSocket.instances).toHaveLength(1);
      expect(MockWebSocket.instances[0].url).toBe('ws://localhost:3000/ws');
      expect(get(connectionState)).toBe('connecting');
    });

    it('uses wss: protocol for https: pages', async () => {
      Object.defineProperty(window, 'location', {
        value: { protocol: 'https:', host: 'example.com' },
        writable: true,
        configurable: true,
      });

      const { connect } = await getModule();
      connect();

      expect(MockWebSocket.instances[0].url).toBe('wss://example.com/ws');
    });

    it('does not create a new connection if already connecting', async () => {
      const { connect } = await getModule();

      connect();
      const ws = MockWebSocket.instances[0];
      ws.readyState = MockWebSocket.CONNECTING;

      connect(); // Should be a no-op

      expect(MockWebSocket.instances).toHaveLength(1);
    });

    it('does not create a new connection if already open', async () => {
      const { connect } = await getModule();

      connect();
      const ws = MockWebSocket.instances[0];
      ws.readyState = MockWebSocket.OPEN;

      connect(); // Should be a no-op

      expect(MockWebSocket.instances).toHaveLength(1);
    });

    it('sets connected state on open', async () => {
      const { connect, connectionState } = await getModule();

      connect();
      MockWebSocket.instances[0].simulateOpen();

      expect(get(connectionState)).toBe('connected');
    });

    it('sets disconnected state on close', async () => {
      const { connect, connectionState } = await getModule();

      connect();
      MockWebSocket.instances[0].simulateOpen();
      MockWebSocket.instances[0].simulateClose();

      expect(get(connectionState)).toBe('disconnected');
    });

    it('sets error message on WebSocket error', async () => {
      const { connect, lastError } = await getModule();

      connect();
      MockWebSocket.instances[0].simulateError();

      expect(get(lastError)).toBe('WebSocket connection error');
    });

    it('clears last error on new connection', async () => {
      const { connect, lastError } = await getModule();

      // Set an error first
      lastError.set('some error');

      connect();

      expect(get(lastError)).toBeNull();
    });

    it('handles WebSocket constructor failure', async () => {
      // Make WebSocket constructor throw
      (globalThis as unknown as Record<string, unknown>).WebSocket = class {
        constructor() {
          throw new Error('WebSocket not supported');
        }
      };

      const { connect, connectionState, lastError } = await getModule();

      connect();

      expect(get(connectionState)).toBe('error');
      expect(get(lastError)).toBe('Failed to create WebSocket connection');
    });
  });

  describe('message handling', () => {
    it('parses and handles incoming messages', async () => {
      const { connect, on } = await getModule();
      const handler = vi.fn();

      on('config_updated', handler);
      connect();

      const ws = MockWebSocket.instances[0];
      ws.simulateOpen();
      ws.simulateMessage(JSON.stringify({ type: 'config_updated', payload: { foo: 'bar' } }));

      expect(handler).toHaveBeenCalledWith({ foo: 'bar' });
    });

    it('handles multiple newline-separated messages', async () => {
      const { connect, on } = await getModule();
      const handler = vi.fn();

      on('config_updated', handler);
      connect();

      const ws = MockWebSocket.instances[0];
      ws.simulateOpen();

      const msg1 = JSON.stringify({ type: 'config_updated', payload: 'first' });
      const msg2 = JSON.stringify({ type: 'config_updated', payload: 'second' });
      ws.simulateMessage(`${msg1}\n${msg2}`);

      expect(handler).toHaveBeenCalledTimes(2);
      expect(handler).toHaveBeenCalledWith('first');
      expect(handler).toHaveBeenCalledWith('second');
    });

    it('handles invalid JSON gracefully', async () => {
      const { connect } = await getModule();
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

      connect();
      const ws = MockWebSocket.instances[0];
      ws.simulateOpen();
      ws.simulateMessage('not valid json');

      expect(consoleSpy).toHaveBeenCalledWith('Error parsing WebSocket message', expect.any(Error));
      consoleSpy.mockRestore();
    });

    it('handles health_changed event by updating healthData', async () => {
      const { connect } = await getModule();
      const { healthData } = await import('./healthStore');

      connect();
      const ws = MockWebSocket.instances[0];
      ws.simulateOpen();

      const healthPayload = [
        { name: 'app1', status: 'healthy', statusCode: 200, url: 'http://app1', lastChecked: '' },
        { name: 'app2', status: 'unhealthy', statusCode: 500, url: 'http://app2', lastChecked: '' },
      ];
      ws.simulateMessage(JSON.stringify({ type: 'health_changed', payload: healthPayload }));

      const data = get(healthData);
      expect(data.get('app1')).toEqual(healthPayload[0]);
      expect(data.get('app2')).toEqual(healthPayload[1]);
    });

    it('handles app_health_changed event by updating single app', async () => {
      const { connect } = await getModule();
      const { healthData } = await import('./healthStore');

      // Set initial health data
      healthData.set(new Map([
        ['app1', { name: 'app1', status: 'healthy' as const, response_time_ms: 0, last_check: '', uptime_percent: 100, check_count: 1, success_count: 1 }],
      ]));

      connect();
      const ws = MockWebSocket.instances[0];
      ws.simulateOpen();

      const payload = {
        app: 'app1',
        health: { name: 'app1', status: 'unhealthy', statusCode: 500, url: 'http://app1', lastChecked: '' },
      };
      ws.simulateMessage(JSON.stringify({ type: 'app_health_changed', payload }));

      const data = get(healthData);
      expect(data.get('app1')?.status).toBe('unhealthy');
    });

    it('handles handler errors gracefully', async () => {
      const { connect, on } = await getModule();
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
      const errorHandler = vi.fn(() => { throw new Error('handler error'); });
      const goodHandler = vi.fn();

      on('config_updated', errorHandler);
      on('config_updated', goodHandler);
      connect();

      const ws = MockWebSocket.instances[0];
      ws.simulateOpen();
      ws.simulateMessage(JSON.stringify({ type: 'config_updated', payload: 'test' }));

      expect(errorHandler).toHaveBeenCalled();
      expect(goodHandler).toHaveBeenCalled();
      expect(consoleSpy).toHaveBeenCalledWith('Error in event handler', expect.any(Error));
      consoleSpy.mockRestore();
    });
  });

  describe('reconnection', () => {
    it('attempts to reconnect after close', async () => {
      const { connect, connectionState } = await getModule();

      connect();
      const ws = MockWebSocket.instances[0];
      ws.simulateOpen();
      ws.simulateClose();

      expect(get(connectionState)).toBe('disconnected');

      // Advance past reconnect delay (base 1000ms + up to 1000ms jitter)
      vi.advanceTimersByTime(3000);

      // A new WebSocket should have been created
      expect(MockWebSocket.instances.length).toBeGreaterThan(1);
    });

    it('gives up after max reconnect attempts', async () => {
      const { connect, connectionState, lastError } = await getModule();

      connect();

      // Each close triggers a reconnect timeout. The timeout callback increments
      // reconnectAttempts and calls connect(). We do NOT simulateOpen() because
      // that would reset reconnectAttempts to 0.
      // We need to trigger close on each new WS instance that the reconnect creates.
      for (let i = 0; i < 11; i++) {
        const ws = MockWebSocket.instances[MockWebSocket.instances.length - 1];
        ws.simulateClose(); // triggers reconnect timeout
        // Advance timer past max delay (30s cap + 1s jitter)
        vi.advanceTimersByTime(60000);
      }

      expect(get(connectionState)).toBe('error');
      expect(get(lastError)).toBe('Failed to connect after multiple attempts');
    });

    it('resets reconnect attempts on successful connection', async () => {
      const { connect } = await getModule();

      connect();
      const ws1 = MockWebSocket.instances[0];
      ws1.simulateOpen();
      ws1.simulateClose();

      // Reconnect
      vi.advanceTimersByTime(3000);
      const ws2 = MockWebSocket.instances[MockWebSocket.instances.length - 1];
      ws2.simulateOpen(); // Successful reconnect resets attempts

      // Close and reconnect again - should still work
      ws2.simulateClose();
      vi.advanceTimersByTime(3000);

      expect(MockWebSocket.instances.length).toBeGreaterThan(2);
    });
  });

  describe('disconnect', () => {
    it('closes WebSocket and sets disconnected state', async () => {
      const { connect, disconnect, connectionState } = await getModule();

      connect();
      const ws = MockWebSocket.instances[0];
      ws.simulateOpen();

      disconnect();

      expect(ws.close).toHaveBeenCalled();
      expect(get(connectionState)).toBe('disconnected');
    });

    it('clears reconnect timeout', async () => {
      const { connect, disconnect } = await getModule();

      connect();
      MockWebSocket.instances[0].simulateOpen();
      MockWebSocket.instances[0].simulateClose();

      // At this point a reconnect timeout should be scheduled
      const instanceCountBefore = MockWebSocket.instances.length;

      disconnect();

      // Advance past any reconnect delay
      vi.advanceTimersByTime(60000);

      // No new connection should have been made
      expect(MockWebSocket.instances.length).toBe(instanceCountBefore);
    });

    it('prevents reconnection after disconnect', async () => {
      const { connect, disconnect } = await getModule();

      connect();
      MockWebSocket.instances[0].simulateOpen();

      disconnect();

      // Even if we try to trigger close handler, no reconnect should happen
      vi.advanceTimersByTime(60000);

      // Only the original WebSocket should exist
      expect(MockWebSocket.instances).toHaveLength(1);
    });

    it('handles disconnect when not connected', async () => {
      const { disconnect, connectionState } = await getModule();

      // Should not throw
      disconnect();

      expect(get(connectionState)).toBe('disconnected');
    });
  });

  describe('on (event handler registration)', () => {
    it('registers and invokes event handlers', async () => {
      const { connect, on } = await getModule();
      const handler = vi.fn();

      on('config_updated', handler);
      connect();

      MockWebSocket.instances[0].simulateOpen();
      MockWebSocket.instances[0].simulateMessage(
        JSON.stringify({ type: 'config_updated', payload: 'test' })
      );

      expect(handler).toHaveBeenCalledWith('test');
    });

    it('returns unsubscribe function', async () => {
      const { connect, on } = await getModule();
      const handler = vi.fn();

      const unsub = on('config_updated', handler);
      connect();

      MockWebSocket.instances[0].simulateOpen();

      // Unsubscribe
      unsub();

      MockWebSocket.instances[0].simulateMessage(
        JSON.stringify({ type: 'config_updated', payload: 'test' })
      );

      expect(handler).not.toHaveBeenCalled();
    });

    it('supports multiple handlers for the same event', async () => {
      const { connect, on } = await getModule();
      const handler1 = vi.fn();
      const handler2 = vi.fn();

      on('config_updated', handler1);
      on('config_updated', handler2);
      connect();

      MockWebSocket.instances[0].simulateOpen();
      MockWebSocket.instances[0].simulateMessage(
        JSON.stringify({ type: 'config_updated', payload: 'data' })
      );

      expect(handler1).toHaveBeenCalledWith('data');
      expect(handler2).toHaveBeenCalledWith('data');
    });
  });

  describe('isConnected', () => {
    it('returns true when connected', async () => {
      const { connect, isConnected } = await getModule();

      connect();
      MockWebSocket.instances[0].simulateOpen();

      expect(isConnected()).toBe(true);
    });

    it('returns false when disconnected', async () => {
      const { isConnected } = await getModule();

      expect(isConnected()).toBe(false);
    });

    it('returns false after disconnect', async () => {
      const { connect, disconnect, isConnected } = await getModule();

      connect();
      MockWebSocket.instances[0].simulateOpen();
      disconnect();

      expect(isConnected()).toBe(false);
    });
  });
});

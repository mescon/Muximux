import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import type { LogEntry } from './types';

// Hoisted mock references so they are available inside vi.mock factories
const { mockOn, mockFetchRecentLogs } = vi.hoisted(() => ({
  mockOn: vi.fn(),
  mockFetchRecentLogs: vi.fn(),
}));

vi.mock('./websocketStore', () => ({
  on: mockOn,
}));

vi.mock('./api', () => ({
  fetchRecentLogs: mockFetchRecentLogs,
}));

function makeEntry(overrides: Partial<LogEntry> = {}): LogEntry {
  return {
    timestamp: overrides.timestamp ?? new Date().toISOString(),
    level: overrides.level ?? 'info',
    message: overrides.message ?? 'test message',
    source: overrides.source ?? 'test',
    ...overrides,
  };
}

describe('logStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset modules so the `initialized` flag is cleared between tests
    vi.resetModules();
  });

  async function freshImport() {
    return import('./logStore');
  }

  it('logEntries should start as an empty array', async () => {
    const { logEntries } = await freshImport();
    expect(get(logEntries)).toEqual([]);
  });

  it('clearLogs should empty the store', async () => {
    const { logEntries, clearLogs } = await freshImport();

    // Manually add some entries
    logEntries.set([makeEntry(), makeEntry()]);
    expect(get(logEntries)).toHaveLength(2);

    clearLogs();
    expect(get(logEntries)).toEqual([]);
  });

  it('initLogStore should call on("log_entry", ...) and fetchRecentLogs', async () => {
    mockFetchRecentLogs.mockResolvedValue([]);

    const { initLogStore } = await freshImport();
    initLogStore();

    expect(mockOn).toHaveBeenCalledTimes(1);
    expect(mockOn).toHaveBeenCalledWith('log_entry', expect.any(Function));
    expect(mockFetchRecentLogs).toHaveBeenCalledWith(500);
  });

  it('initLogStore should only initialize once (idempotent)', async () => {
    mockFetchRecentLogs.mockResolvedValue([]);

    const { initLogStore } = await freshImport();
    initLogStore();
    initLogStore();
    initLogStore();

    expect(mockOn).toHaveBeenCalledTimes(1);
    expect(mockFetchRecentLogs).toHaveBeenCalledTimes(1);
  });

  it('should add entry to store when WS callback fires', async () => {
    mockFetchRecentLogs.mockResolvedValue([]);

    const { initLogStore, logEntries } = await freshImport();
    initLogStore();

    // Get the WS callback that was registered
    const wsCallback = mockOn.mock.calls[0][1];
    const entry = makeEntry({ message: 'ws entry' });

    wsCallback(entry);

    const entries = get(logEntries);
    expect(entries).toHaveLength(1);
    expect(entries[0].message).toBe('ws entry');
  });

  it('should trim oldest entries when exceeding MAX_LOG_ENTRIES (2000)', async () => {
    mockFetchRecentLogs.mockResolvedValue([]);

    const { initLogStore, logEntries } = await freshImport();
    initLogStore();

    // Pre-fill the store with 2000 entries
    const existingEntries: LogEntry[] = [];
    for (let i = 0; i < 2000; i++) {
      existingEntries.push(makeEntry({ message: `entry-${i}` }));
    }
    logEntries.set(existingEntries);

    // Fire one more via WS callback
    const wsCallback = mockOn.mock.calls[0][1];
    wsCallback(makeEntry({ message: 'overflow-entry' }));

    const entries = get(logEntries);
    expect(entries).toHaveLength(2000);
    // First entry should be entry-1 (entry-0 was trimmed)
    expect(entries[0].message).toBe('entry-1');
    // Last entry should be the new one
    expect(entries[entries.length - 1].message).toBe('overflow-entry');
  });

  it('should merge history and deduplicate by timestamp|message', async () => {
    const sharedTimestamp = '2024-01-01T12:00:00Z';
    const historyEntries: LogEntry[] = [
      makeEntry({ timestamp: sharedTimestamp, message: 'shared msg' }),
      makeEntry({ timestamp: '2024-01-01T11:00:00Z', message: 'history only' }),
    ];

    // Return the history after a small delay to simulate fetch
    mockFetchRecentLogs.mockResolvedValue(historyEntries);

    const { initLogStore, logEntries } = await freshImport();
    initLogStore();

    // Simulate a WS entry that arrived during fetch (same as a history entry)
    const wsCallback = mockOn.mock.calls[0][1];
    wsCallback(makeEntry({ timestamp: sharedTimestamp, message: 'shared msg' }));
    // Also add a unique WS entry
    wsCallback(makeEntry({ timestamp: '2024-01-01T12:01:00Z', message: 'ws only' }));

    // Wait for fetchRecentLogs promise to resolve
    await vi.waitFor(() => {
      const entries = get(logEntries);
      // Should have: 2 history + 1 unique WS (the duplicate should be deduped)
      expect(entries).toHaveLength(3);
    });

    const entries = get(logEntries);
    const messages = entries.map(e => e.message);
    // History entries come first
    expect(messages[0]).toBe('shared msg');
    expect(messages[1]).toBe('history only');
    // Then unique WS entries
    expect(messages[2]).toBe('ws only');
  });

  it('should handle fetchRecentLogs failure gracefully', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    mockFetchRecentLogs.mockRejectedValue(new Error('Network error'));

    const { initLogStore, logEntries } = await freshImport();
    initLogStore();

    // Wait for the rejection to be handled
    await vi.waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        'Failed to load log history:',
        expect.any(Error),
      );
    });

    // Store should still be functional (empty or with WS entries only)
    expect(get(logEntries)).toEqual([]);

    consoleError.mockRestore();
  });
});

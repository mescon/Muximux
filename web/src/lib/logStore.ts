import { writable } from 'svelte/store';
import type { LogEntry } from './types';
import { on } from './websocketStore';
import { fetchRecentLogs } from './api';

const MAX_LOG_ENTRIES = 2000;

export const logEntries = writable<LogEntry[]>([]);

let initialized = false;

export function initLogStore(): void {
  if (initialized) return;
  initialized = true;

  // Seed with recent history, then subscribe to real-time updates.
  // The WS subscription starts immediately so no entries are missed
  // while the HTTP fetch is in flight.
  on('log_entry', (payload) => {
    const entry = payload as LogEntry;
    logEntries.update(entries => {
      const updated = [...entries, entry];
      if (updated.length > MAX_LOG_ENTRIES) {
        return updated.slice(-MAX_LOG_ENTRIES);
      }
      return updated;
    });
  });

  fetchRecentLogs(500)
    .then(history => {
      logEntries.update(wsEntries => {
        // Merge: history first, then any WS entries that arrived during fetch.
        // Deduplicate by timestamp+message to avoid showing the same entry twice.
        const seen = new Set(history.map(e => `${e.timestamp}|${e.message}`));
        const unique = wsEntries.filter(e => !seen.has(`${e.timestamp}|${e.message}`));
        return [...history, ...unique];
      });
    })
    .catch(e => console.error('Failed to load log history:', e));
}

export function clearLogs(): void {
  logEntries.set([]);
}

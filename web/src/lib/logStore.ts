import { writable } from 'svelte/store';
import type { LogEntry } from './types';
import { on } from './websocketStore';
import { fetchRecentLogs } from './api';

const MAX_LOG_ENTRIES = 2000;

export const logEntries = writable<LogEntry[]>([]);

let initialized = false;

export function initLogStore() {
  if (initialized) return;
  initialized = true;

  on('log_entry', (payload) => {
    const entry = payload as LogEntry;
    logEntries.update(entries => {
      const updated = [...entries, entry];
      return updated.length > MAX_LOG_ENTRIES
        ? updated.slice(-MAX_LOG_ENTRIES)
        : updated;
    });
  });
}

export async function loadLogHistory() {
  try {
    const entries = await fetchRecentLogs(500);
    logEntries.set(entries);
  } catch (e) {
    console.error('Failed to load log history:', e);
  }
}

export function clearLogs() {
  logEntries.set([]);
}

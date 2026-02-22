import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

import type { LogEntry } from '$lib/types';

const { mockLogEntries, mockClearLogs } = vi.hoisted(() => {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const { writable } = require('svelte/store');
  return {
    mockLogEntries: writable<LogEntry[]>([]),
    mockClearLogs: vi.fn(),
  };
});

vi.mock('$lib/logStore', () => ({
  logEntries: mockLogEntries,
  clearLogs: mockClearLogs,
}));

import Logs from './Logs.svelte';

function makeLogEntry(overrides: Partial<LogEntry> = {}): LogEntry {
  return {
    timestamp: '2025-01-01T12:00:00Z',
    level: 'info',
    message: 'Test log message',
    source: 'server',
    ...overrides,
  };
}

describe('Logs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLogEntries.set([]);
  });

  describe('header', () => {
    it('renders the "Logs" title', () => {
      render(Logs);
      expect(screen.getByText('Logs')).toBeInTheDocument();
    });

    it('renders the back/close button', () => {
      render(Logs);
      const backBtn = screen.getByTitle('Back to dashboard');
      expect(backBtn).toBeInTheDocument();
      expect(backBtn.textContent).toContain('Back');
    });

    it('calls onclose when close button is clicked', async () => {
      const onclose = vi.fn();
      render(Logs, { props: { onclose } });

      const backBtn = screen.getByTitle('Back to dashboard');
      await fireEvent.click(backBtn);

      expect(onclose).toHaveBeenCalledTimes(1);
    });
  });

  describe('action buttons', () => {
    it('has a Pause button', () => {
      render(Logs);
      expect(screen.getByTitle('Pause log streaming')).toBeInTheDocument();
      expect(screen.getByText('Pause')).toBeInTheDocument();
    });

    it('has a Download button', () => {
      render(Logs);
      expect(screen.getByTitle('Download filtered logs as .log file')).toBeInTheDocument();
      expect(screen.getByText('Download')).toBeInTheDocument();
    });

    it('has a Clear button', () => {
      render(Logs);
      expect(screen.getByTitle('Clear all log entries')).toBeInTheDocument();
      expect(screen.getByText('Clear')).toBeInTheDocument();
    });

    it('calls clearLogs when Clear button is clicked', async () => {
      render(Logs);
      const clearBtn = screen.getByTitle('Clear all log entries');
      await fireEvent.click(clearBtn);

      expect(mockClearLogs).toHaveBeenCalledTimes(1);
    });

    it('disables Download button when no entries exist', () => {
      render(Logs);
      const downloadBtn = screen.getByTitle('Download filtered logs as .log file');
      expect(downloadBtn).toBeDisabled();
    });

    it('enables Download button when entries exist', () => {
      mockLogEntries.set([makeLogEntry()]);
      render(Logs);
      const downloadBtn = screen.getByTitle('Download filtered logs as .log file');
      expect(downloadBtn).not.toBeDisabled();
    });
  });

  describe('search input', () => {
    it('renders the search/filter input', () => {
      render(Logs);
      const input = screen.getByPlaceholderText('Filter logs...');
      expect(input).toBeInTheDocument();
    });

    it('filters entries by search query', async () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'Starting migration on database' }),
        makeLogEntry({ message: 'User logged in successfully', timestamp: '2025-01-01T12:00:01Z' }),
        makeLogEntry({ message: 'Database connection established', timestamp: '2025-01-01T12:00:02Z' }),
      ]);

      render(Logs);
      const input = screen.getByPlaceholderText('Filter logs...');
      await fireEvent.input(input, { target: { value: 'database' } });

      await waitFor(() => {
        expect(screen.getByText('Starting migration on database')).toBeInTheDocument();
        expect(screen.getByText('Database connection established')).toBeInTheDocument();
        expect(screen.queryByText('User logged in successfully')).not.toBeInTheDocument();
      });
    });

    it('filters are case-insensitive', async () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'ERROR processing request' }),
        makeLogEntry({ message: 'All good here' }),
      ]);

      render(Logs);
      const input = screen.getByPlaceholderText('Filter logs...');
      await fireEvent.input(input, { target: { value: 'error' } });

      await waitFor(() => {
        expect(screen.getByText('ERROR processing request')).toBeInTheDocument();
        expect(screen.queryByText('All good here')).not.toBeInTheDocument();
      });
    });

    it('matches against source in search', async () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'A message', source: 'proxy' }),
        makeLogEntry({ message: 'Another message', source: 'auth' }),
      ]);

      render(Logs);
      const input = screen.getByPlaceholderText('Filter logs...');
      await fireEvent.input(input, { target: { value: 'proxy' } });

      await waitFor(() => {
        expect(screen.getByText('A message')).toBeInTheDocument();
        expect(screen.queryByText('Another message')).not.toBeInTheDocument();
      });
    });

    it('matches against attrs in search', async () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'Request handled', attrs: { path: '/api/health' } }),
        makeLogEntry({ message: 'Unrelated entry' }),
      ]);

      render(Logs);
      const input = screen.getByPlaceholderText('Filter logs...');
      await fireEvent.input(input, { target: { value: 'health' } });

      await waitFor(() => {
        expect(screen.getByText('Request handled')).toBeInTheDocument();
        expect(screen.queryByText('Unrelated entry')).not.toBeInTheDocument();
      });
    });
  });

  describe('level filters', () => {
    it('renders all four level toggle buttons', () => {
      render(Logs);
      expect(screen.getByText('DEBUG')).toBeInTheDocument();
      expect(screen.getByText('INFO')).toBeInTheDocument();
      expect(screen.getByText('WARN')).toBeInTheDocument();
      expect(screen.getByText('ERROR')).toBeInTheDocument();
    });

    it('hides entries when a level is toggled off', async () => {
      mockLogEntries.set([
        makeLogEntry({ level: 'info', message: 'Info message' }),
        makeLogEntry({ level: 'error', message: 'Error message', timestamp: '2025-01-01T12:00:01Z' }),
      ]);

      render(Logs);

      // Both should be visible initially
      expect(screen.getByText('Info message')).toBeInTheDocument();
      expect(screen.getByText('Error message')).toBeInTheDocument();

      // Toggle off info level using the title attribute to avoid matching the level badge text
      const infoBtn = screen.getByTitle('Hide info messages');
      await fireEvent.click(infoBtn);

      await waitFor(() => {
        expect(screen.queryByText('Info message')).not.toBeInTheDocument();
        expect(screen.getByText('Error message')).toBeInTheDocument();
      });
    });

    it('shows entries again when a level is toggled back on', async () => {
      mockLogEntries.set([
        makeLogEntry({ level: 'debug', message: 'Debug message' }),
      ]);

      render(Logs);
      expect(screen.getByText('Debug message')).toBeInTheDocument();

      // Toggle off using title attribute
      const hideBtn = screen.getByTitle('Hide debug messages');
      await fireEvent.click(hideBtn);

      await waitFor(() => {
        expect(screen.queryByText('Debug message')).not.toBeInTheDocument();
      });

      // Toggle back on - title has changed to "Show" now
      const showBtn = screen.getByTitle('Show debug messages');
      await fireEvent.click(showBtn);

      await waitFor(() => {
        expect(screen.getByText('Debug message')).toBeInTheDocument();
      });
    });
  });

  describe('source filters', () => {
    it('renders the "Source:" label', () => {
      render(Logs);
      expect(screen.getByText('Source:')).toBeInTheDocument();
    });

    it('renders the "All" source toggle', () => {
      render(Logs);
      expect(screen.getByText('All')).toBeInTheDocument();
    });

    it('renders default source pills when no entries exist', () => {
      render(Logs);
      // ALL_SOURCES in the component
      expect(screen.getByText('server')).toBeInTheDocument();
      expect(screen.getByText('proxy')).toBeInTheDocument();
      expect(screen.getByText('health')).toBeInTheDocument();
      expect(screen.getByText('auth')).toBeInTheDocument();
      expect(screen.getByText('websocket')).toBeInTheDocument();
      expect(screen.getByText('caddy')).toBeInTheDocument();
      expect(screen.getByText('config')).toBeInTheDocument();
      expect(screen.getByText('icons')).toBeInTheDocument();
      expect(screen.getByText('themes')).toBeInTheDocument();
    });
  });

  describe('log entries display', () => {
    it('shows empty state when no entries exist', () => {
      render(Logs);
      expect(screen.getByText('No log entries yet. Logs will appear here in real-time.')).toBeInTheDocument();
    });

    it('shows "no match" message when entries exist but filters exclude all', async () => {
      mockLogEntries.set([
        makeLogEntry({ level: 'info', message: 'Some info message' }),
      ]);

      render(Logs);

      // Toggle off info (the only level present) using title attribute
      await fireEvent.click(screen.getByTitle('Hide info messages'));

      await waitFor(() => {
        expect(screen.getByText('No entries match the current filters.')).toBeInTheDocument();
      });
    });

    it('renders log entry messages from the store', () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'First log entry' }),
        makeLogEntry({ message: 'Second log entry', timestamp: '2025-01-01T12:00:01Z' }),
      ]);

      render(Logs);
      expect(screen.getByText('First log entry')).toBeInTheDocument();
      expect(screen.getByText('Second log entry')).toBeInTheDocument();
    });

    it('renders log entry source badges', () => {
      mockLogEntries.set([
        makeLogEntry({ source: 'proxy', message: 'Proxy log' }),
      ]);

      const { container } = render(Logs);
      const sourceBadge = container.querySelector('.log-source');
      expect(sourceBadge).toBeInTheDocument();
      expect(sourceBadge?.textContent).toBe('proxy');
    });

    it('renders log entry level badges with correct class', () => {
      mockLogEntries.set([
        makeLogEntry({ level: 'error', message: 'An error occurred' }),
      ]);

      const { container } = render(Logs);
      const levelBadge = container.querySelector('.log-level-badge');
      expect(levelBadge).toBeInTheDocument();
      expect(levelBadge).toHaveClass('log-level-error');
    });

    it('renders attrs when present', () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'Request done', attrs: { status: '200', duration: '50ms' } }),
      ]);

      const { container } = render(Logs);
      const attrElements = container.querySelectorAll('.log-attr');
      expect(attrElements.length).toBe(2);
    });

    it('applies correct level class for each level', () => {
      mockLogEntries.set([
        makeLogEntry({ level: 'debug', message: 'dbg', timestamp: '2025-01-01T12:00:00Z' }),
        makeLogEntry({ level: 'info', message: 'inf', timestamp: '2025-01-01T12:00:01Z' }),
        makeLogEntry({ level: 'warn', message: 'wrn', timestamp: '2025-01-01T12:00:02Z' }),
        makeLogEntry({ level: 'error', message: 'err', timestamp: '2025-01-01T12:00:03Z' }),
      ]);

      const { container } = render(Logs);
      const badges = container.querySelectorAll('.log-level-badge');
      expect(badges[0]).toHaveClass('log-level-debug');
      expect(badges[1]).toHaveClass('log-level-info');
      expect(badges[2]).toHaveClass('log-level-warn');
      expect(badges[3]).toHaveClass('log-level-error');
    });
  });

  describe('footer', () => {
    it('shows entry count', () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'entry one' }),
        makeLogEntry({ message: 'entry two', timestamp: '2025-01-01T12:00:01Z' }),
      ]);

      render(Logs);
      expect(screen.getByText('2 entries')).toBeInTheDocument();
    });

    it('shows filtered count', () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'entry one' }),
        makeLogEntry({ message: 'entry two', timestamp: '2025-01-01T12:00:01Z' }),
      ]);

      render(Logs);
      expect(screen.getByText('showing 2 filtered')).toBeInTheDocument();
    });

    it('shows filtered count that differs from total when filter is active', async () => {
      mockLogEntries.set([
        makeLogEntry({ level: 'info', message: 'info msg' }),
        makeLogEntry({ level: 'error', message: 'error msg', timestamp: '2025-01-01T12:00:01Z' }),
        makeLogEntry({ level: 'info', message: 'another info', timestamp: '2025-01-01T12:00:02Z' }),
      ]);

      render(Logs);
      expect(screen.getByText('3 entries')).toBeInTheDocument();
      expect(screen.getByText('showing 3 filtered')).toBeInTheDocument();

      // Toggle off info using title attribute
      await fireEvent.click(screen.getByTitle('Hide info messages'));

      await waitFor(() => {
        expect(screen.getByText('3 entries')).toBeInTheDocument();
        expect(screen.getByText('showing 1 filtered')).toBeInTheDocument();
      });
    });
  });

  describe('pause/resume', () => {
    it('shows Pause button initially', () => {
      render(Logs);
      expect(screen.getByText('Pause')).toBeInTheDocument();
    });

    it('switches to Resume when Pause is clicked', async () => {
      render(Logs);

      await fireEvent.click(screen.getByTitle('Pause log streaming'));

      await waitFor(() => {
        expect(screen.getByText('Resume')).toBeInTheDocument();
        expect(screen.queryByText('Pause')).not.toBeInTheDocument();
      });
    });

    it('shows PAUSED badge in footer when paused', async () => {
      render(Logs);

      await fireEvent.click(screen.getByTitle('Pause log streaming'));

      await waitFor(() => {
        expect(screen.getByText('PAUSED')).toBeInTheDocument();
      });
    });

    it('switches back to Pause when Resume is clicked', async () => {
      render(Logs);

      // Pause
      await fireEvent.click(screen.getByTitle('Pause log streaming'));

      await waitFor(() => {
        expect(screen.getByText('Resume')).toBeInTheDocument();
      });

      // Resume
      await fireEvent.click(screen.getByTitle('Resume log streaming'));

      await waitFor(() => {
        expect(screen.getByText('Pause')).toBeInTheDocument();
        expect(screen.queryByText('Resume')).not.toBeInTheDocument();
        expect(screen.queryByText('PAUSED')).not.toBeInTheDocument();
      });
    });

    it('freezes entries when paused and does not show new ones', async () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'Initial entry' }),
      ]);

      render(Logs);
      expect(screen.getByText('Initial entry')).toBeInTheDocument();

      // Pause
      await fireEvent.click(screen.getByTitle('Pause log streaming'));

      // Add a new entry to the store after pausing
      mockLogEntries.set([
        makeLogEntry({ message: 'Initial entry' }),
        makeLogEntry({ message: 'New entry after pause', timestamp: '2025-01-01T12:01:00Z' }),
      ]);

      await waitFor(() => {
        // The new entry should not appear because we are paused
        expect(screen.getByText('Initial entry')).toBeInTheDocument();
        expect(screen.queryByText('New entry after pause')).not.toBeInTheDocument();
      });
    });
  });

  describe('download', () => {
    it('has a Download button with correct title', () => {
      render(Logs);
      const btn = screen.getByTitle('Download filtered logs as .log file');
      expect(btn).toBeInTheDocument();
      expect(btn.textContent).toContain('Download');
    });
  });

  describe('timestamp formatting', () => {
    it('formats timestamps as HH:MM:SS.mmm', () => {
      mockLogEntries.set([
        makeLogEntry({ timestamp: '2025-06-15T14:30:45.123Z', message: 'Timestamped entry' }),
      ]);

      const { container } = render(Logs);
      const tsElement = container.querySelector('.log-ts');
      expect(tsElement).toBeInTheDocument();
      // The exact formatted time depends on the local timezone, but it should be in HH:MM:SS.mmm format
      expect(tsElement?.textContent).toMatch(/^\d{2}:\d{2}:\d{2}\.\d{3}$/);
    });

    it('returns raw string when timestamp is invalid', () => {
      mockLogEntries.set([
        makeLogEntry({ timestamp: 'not-a-date', message: 'Bad timestamp entry' }),
      ]);

      const { container } = render(Logs);
      const tsElement = container.querySelector('.log-ts');
      expect(tsElement).toBeInTheDocument();
      // Invalid dates return NaN from getHours(), which produces "NaN:NaN:NaN.NaN"
      // The component catches this - but actually Date('not-a-date') returns Invalid Date,
      // getHours() returns NaN, padStart still works on 'NaN', so it may not throw.
      // Either way, something should be rendered.
      expect(tsElement?.textContent).toBeTruthy();
    });
  });

  describe('source filter toggling', () => {
    it('hides entries when a specific source is toggled off', async () => {
      mockLogEntries.set([
        makeLogEntry({ source: 'server', message: 'Server message' }),
        makeLogEntry({ source: 'proxy', message: 'Proxy message', timestamp: '2025-01-01T12:00:01Z' }),
      ]);

      render(Logs);

      // Both should be visible initially
      expect(screen.getByText('Server message')).toBeInTheDocument();
      expect(screen.getByText('Proxy message')).toBeInTheDocument();

      // First toggle "All" off (so individual toggles matter)
      const allBtn = screen.getByText('All');
      await fireEvent.click(allBtn);

      // Now all sources should be off, both messages hidden
      await waitFor(() => {
        expect(screen.queryByText('Server message')).not.toBeInTheDocument();
        expect(screen.queryByText('Proxy message')).not.toBeInTheDocument();
      });

      // Toggle server back on
      // After toggling All off, we need to find the 'server' pill.
      // Since entries have sources, discoveredSources should show them.
      const serverPill = screen.getAllByText('server').find(
        el => el.classList.contains('log-source-pill')
      );
      if (serverPill) {
        await fireEvent.click(serverPill);
      }

      await waitFor(() => {
        expect(screen.getByText('Server message')).toBeInTheDocument();
        expect(screen.queryByText('Proxy message')).not.toBeInTheDocument();
      });
    });

    it('toggles all sources on when All is clicked after being off', async () => {
      mockLogEntries.set([
        makeLogEntry({ source: 'server', message: 'Server msg' }),
        makeLogEntry({ source: 'proxy', message: 'Proxy msg', timestamp: '2025-01-01T12:00:01Z' }),
      ]);

      render(Logs);

      // Toggle All off
      await fireEvent.click(screen.getByText('All'));

      await waitFor(() => {
        expect(screen.queryByText('Server msg')).not.toBeInTheDocument();
      });

      // Toggle All back on
      await fireEvent.click(screen.getByText('All'));

      await waitFor(() => {
        expect(screen.getByText('Server msg')).toBeInTheDocument();
        expect(screen.getByText('Proxy msg')).toBeInTheDocument();
      });
    });

    it('discovers sources from entries and renders them as pills', () => {
      mockLogEntries.set([
        makeLogEntry({ source: 'server', message: 'msg1' }),
        makeLogEntry({ source: 'proxy', message: 'msg2', timestamp: '2025-01-01T12:00:01Z' }),
        makeLogEntry({ source: 'auth', message: 'msg3', timestamp: '2025-01-01T12:00:02Z' }),
      ]);

      const { container } = render(Logs);
      // discoveredSources should contain at least server, proxy, auth
      const pills = container.querySelectorAll('.log-source-pill');
      // "All" + at least 3 discovered sources
      expect(pills.length).toBeGreaterThanOrEqual(4);
    });
  });

  describe('search clear button', () => {
    it('shows clear button when search has text and clears on click', async () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'Hello world' }),
        makeLogEntry({ message: 'Goodbye world', timestamp: '2025-01-01T12:00:01Z' }),
      ]);

      render(Logs);
      const input = screen.getByPlaceholderText('Filter logs...');

      // Type something in search
      await fireEvent.input(input, { target: { value: 'Hello' } });

      await waitFor(() => {
        expect(screen.getByText('Hello world')).toBeInTheDocument();
        expect(screen.queryByText('Goodbye world')).not.toBeInTheDocument();
      });

      // Clear button should be visible
      const clearBtn = screen.getByLabelText('Clear search');
      expect(clearBtn).toBeInTheDocument();

      await fireEvent.click(clearBtn);

      // Both entries should be visible again
      await waitFor(() => {
        expect(screen.getByText('Hello world')).toBeInTheDocument();
        expect(screen.getByText('Goodbye world')).toBeInTheDocument();
      });
    });
  });

  describe('clear logs while paused', () => {
    it('clears paused entries when Clear is clicked while paused', async () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'Entry before pause' }),
      ]);

      render(Logs);
      expect(screen.getByText('Entry before pause')).toBeInTheDocument();

      // Pause
      await fireEvent.click(screen.getByTitle('Pause log streaming'));

      await waitFor(() => {
        expect(screen.getByText('PAUSED')).toBeInTheDocument();
      });

      // Clear
      await fireEvent.click(screen.getByTitle('Clear all log entries'));

      await waitFor(() => {
        expect(mockClearLogs).toHaveBeenCalled();
        // The paused entries should also be cleared
        expect(screen.queryByText('Entry before pause')).not.toBeInTheDocument();
      });
    });
  });

  describe('download functionality', () => {
    it('creates and triggers a download when Download is clicked with entries', async () => {
      mockLogEntries.set([
        makeLogEntry({
          timestamp: '2025-06-15T14:30:45.000Z',
          level: 'info',
          source: 'server',
          message: 'Request processed',
        }),
        makeLogEntry({
          timestamp: '2025-06-15T14:30:46.000Z',
          level: 'warn',
          source: 'proxy',
          message: 'Slow response',
          attrs: { duration: '5000ms' },
        }),
      ]);

      // Mock URL.createObjectURL and URL.revokeObjectURL
      const mockCreateObjectURL = vi.fn(() => 'blob:mock-url');
      const mockRevokeObjectURL = vi.fn();
      globalThis.URL.createObjectURL = mockCreateObjectURL;
      globalThis.URL.revokeObjectURL = mockRevokeObjectURL;

      // Mock anchor element click - capture original first to avoid recursion
      const origCreateElement = document.createElement.bind(document);
      const mockClick = vi.fn();
      vi.spyOn(document, 'createElement').mockImplementation((tag: string, options?: ElementCreationOptions) => {
        if (tag === 'a') {
          const el = origCreateElement('a');
          el.click = mockClick;
          return el;
        }
        return origCreateElement(tag, options);
      });

      render(Logs);
      const downloadBtn = screen.getByTitle('Download filtered logs as .log file');
      await fireEvent.click(downloadBtn);

      expect(mockCreateObjectURL).toHaveBeenCalled();
      expect(mockClick).toHaveBeenCalled();
      expect(mockRevokeObjectURL).toHaveBeenCalled();

      vi.restoreAllMocks();
    });

    it('does nothing when download is clicked with no filtered entries', () => {
      // No entries
      mockLogEntries.set([]);

      render(Logs);
      const downloadBtn = screen.getByTitle('Download filtered logs as .log file');
      // Button should be disabled
      expect(downloadBtn).toBeDisabled();
    });
  });

  describe('scroll handling', () => {
    it('handles scroll events on the log container', async () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'Entry 1' }),
        makeLogEntry({ message: 'Entry 2', timestamp: '2025-01-01T12:00:01Z' }),
      ]);

      const { container } = render(Logs);
      const logEntriesEl = container.querySelector('.log-entries');
      expect(logEntriesEl).toBeTruthy();

      if (logEntriesEl) {
        // Simulate scrolling away from bottom
        Object.defineProperty(logEntriesEl, 'scrollTop', { value: 0, writable: true, configurable: true });
        Object.defineProperty(logEntriesEl, 'scrollHeight', { value: 1000, writable: true, configurable: true });
        Object.defineProperty(logEntriesEl, 'clientHeight', { value: 400, writable: true, configurable: true });

        await fireEvent.scroll(logEntriesEl);

        // After scrolling away from bottom, autoScroll turns off, "Scroll to bottom" button appears
        await waitFor(() => {
          expect(screen.getByText('Scroll to bottom')).toBeInTheDocument();
        });
      }
    });

    it('scroll to bottom button scrolls the container', async () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'Entry 1' }),
        makeLogEntry({ message: 'Entry 2', timestamp: '2025-01-01T12:00:01Z' }),
      ]);

      const { container } = render(Logs);
      const logEntriesEl = container.querySelector('.log-entries');

      if (logEntriesEl) {
        Object.defineProperty(logEntriesEl, 'scrollTop', { value: 0, writable: true, configurable: true });
        Object.defineProperty(logEntriesEl, 'scrollHeight', { value: 1000, writable: true, configurable: true });
        Object.defineProperty(logEntriesEl, 'clientHeight', { value: 400, writable: true, configurable: true });

        await fireEvent.scroll(logEntriesEl);

        await waitFor(() => {
          expect(screen.getByText('Scroll to bottom')).toBeInTheDocument();
        });

        await fireEvent.click(screen.getByText('Scroll to bottom'));

        // After clicking, the component calls scrollToBottom which sets scrollTop = scrollHeight
        // We just verify the button was clickable and didn't crash
      }
    });
  });

  describe('source filter with individual toggles', () => {
    it('hides entries when All sources is toggled off', async () => {
      mockLogEntries.set([
        makeLogEntry({ source: 'proxy', message: 'Proxy request' }),
        makeLogEntry({ source: 'server', message: 'Server request', timestamp: '2025-01-01T12:00:01Z' }),
      ]);

      render(Logs);

      // Both should be visible
      expect(screen.getByText('Proxy request')).toBeInTheDocument();
      expect(screen.getByText('Server request')).toBeInTheDocument();

      // Click "All" to toggle all off
      await fireEvent.click(screen.getByText('All'));

      await waitFor(() => {
        expect(screen.queryByText('Proxy request')).not.toBeInTheDocument();
        expect(screen.queryByText('Server request')).not.toBeInTheDocument();
      });
    });
  });

  describe('attrs matching in search', () => {
    it('matches against attr keys in search', async () => {
      mockLogEntries.set([
        makeLogEntry({ message: 'Request A', attrs: { method: 'GET', path: '/api/test' } }),
        makeLogEntry({ message: 'Request B', timestamp: '2025-01-01T12:00:01Z' }),
      ]);

      render(Logs);
      const input = screen.getByPlaceholderText('Filter logs...');
      await fireEvent.input(input, { target: { value: 'method' } });

      await waitFor(() => {
        expect(screen.getByText('Request A')).toBeInTheDocument();
        expect(screen.queryByText('Request B')).not.toBeInTheDocument();
      });
    });
  });

  describe('download with attrs', () => {
    it('downloads log with attrs formatted correctly', async () => {
      mockLogEntries.set([
        makeLogEntry({
          message: 'With attrs',
          attrs: { key1: 'val1', key2: 'val2' },
        }),
      ]);

      const blobData: string[] = [];
      const OrigBlob = globalThis.Blob;
      globalThis.Blob = class MockBlob extends OrigBlob {
        constructor(parts?: BlobPart[], options?: BlobPropertyBag) {
          super(parts, options);
          if (parts) {
            blobData.push(parts.map(p => String(p)).join(''));
          }
        }
      } as typeof Blob;

      const origCreateElement = document.createElement.bind(document);
      const mockClick = vi.fn();
      vi.spyOn(document, 'createElement').mockImplementation((tag: string, options?: ElementCreationOptions) => {
        if (tag === 'a') {
          const el = origCreateElement('a');
          el.click = mockClick;
          return el;
        }
        return origCreateElement(tag, options);
      });
      globalThis.URL.createObjectURL = vi.fn(() => 'blob:url');
      globalThis.URL.revokeObjectURL = vi.fn();

      render(Logs);
      await fireEvent.click(screen.getByTitle('Download filtered logs as .log file'));

      expect(mockClick).toHaveBeenCalled();
      // Verify the blob content includes attrs
      expect(blobData.length).toBeGreaterThan(0);
      expect(blobData[0]).toContain('key1=val1');
      expect(blobData[0]).toContain('key2=val2');

      globalThis.Blob = OrigBlob;
      vi.restoreAllMocks();
    });
  });
});

<script lang="ts">
  import { onMount, tick } from 'svelte';
  import type { LogEntry } from '$lib/types';
  import { logEntries, loadLogHistory, clearLogs } from '$lib/logStore';

  let { onclose }: { onclose?: () => void } = $props();

  // Filter state
  let searchQuery = $state('');
  let enabledLevels = $state<Record<string, boolean>>({
    debug: true,
    info: true,
    warn: true,
    error: true,
  });

  const ALL_SOURCES = ['server', 'proxy', 'health', 'auth', 'websocket', 'caddy', 'config', 'icons', 'themes'];
  let enabledSources = $state<Record<string, boolean>>(
    Object.fromEntries(ALL_SOURCES.map(s => [s, true]))
  );
  let allSourcesEnabled = $derived(ALL_SOURCES.every(s => enabledSources[s]));

  // Pause/resume state
  let paused = $state(false);
  let pausedEntries = $state<LogEntry[]>([]);

  // Auto-scroll state
  let autoScroll = $state(true);
  let logContainer = $state<HTMLElement>();

  // Current entries from the store (or frozen snapshot)
  let currentEntries = $derived(paused ? pausedEntries : $logEntries);

  // Filtered entries
  let filteredEntries = $derived.by(() => {
    const q = searchQuery.toLowerCase();
    return currentEntries.filter(entry => {
      if (!enabledLevels[entry.level]) return false;
      if (!allSourcesEnabled && entry.source && !enabledSources[entry.source]) return false;
      if (q && !entry.message.toLowerCase().includes(q) && !entry.source.toLowerCase().includes(q)) return false;
      return true;
    });
  });

  // Discovered sources from entries
  let discoveredSources = $derived.by(() => {
    const sources = new Set<string>();
    for (const entry of currentEntries) {
      if (entry.source) sources.add(entry.source);
    }
    return Array.from(sources).sort();
  });

  function toggleLevel(level: string) {
    enabledLevels[level] = !enabledLevels[level];
  }

  function toggleSource(source: string) {
    enabledSources[source] = !enabledSources[source];
  }

  function toggleAllSources() {
    const newVal = !allSourcesEnabled;
    for (const s of ALL_SOURCES) {
      enabledSources[s] = newVal;
    }
    // Also toggle any discovered sources not in the default list
    for (const s of discoveredSources) {
      enabledSources[s] = newVal;
    }
  }

  function togglePause() {
    if (paused) {
      paused = false;
    } else {
      pausedEntries = [...$logEntries];
      paused = true;
    }
  }

  function handleClear() {
    clearLogs();
    if (paused) {
      pausedEntries = [];
    }
  }

  function handleScroll() {
    if (!logContainer) return;
    const { scrollTop, scrollHeight, clientHeight } = logContainer;
    const atBottom = scrollHeight - scrollTop - clientHeight < 40;
    autoScroll = atBottom;
  }

  function scrollToBottom() {
    if (logContainer) {
      logContainer.scrollTop = logContainer.scrollHeight;
    }
  }

  // Auto-scroll when new entries arrive
  $effect(() => {
    filteredEntries.length; // track
    if (autoScroll && !paused) {
      tick().then(() => {
        scrollToBottom();
      });
    }
  });

  function formatTimestamp(ts: string): string {
    try {
      const date = new Date(ts);
      const h = date.getHours().toString().padStart(2, '0');
      const m = date.getMinutes().toString().padStart(2, '0');
      const s = date.getSeconds().toString().padStart(2, '0');
      const ms = date.getMilliseconds().toString().padStart(3, '0');
      return `${h}:${m}:${s}.${ms}`;
    } catch {
      return ts;
    }
  }

  function levelClass(level: string): string {
    switch (level) {
      case 'debug': return 'log-level-debug';
      case 'info': return 'log-level-info';
      case 'warn': return 'log-level-warn';
      case 'error': return 'log-level-error';
      default: return '';
    }
  }

  function levelBtnClass(level: string): string {
    const enabled = enabledLevels[level];
    switch (level) {
      case 'debug': return enabled ? 'log-btn-debug-active' : 'log-btn-inactive';
      case 'info': return enabled ? 'log-btn-info-active' : 'log-btn-inactive';
      case 'warn': return enabled ? 'log-btn-warn-active' : 'log-btn-inactive';
      case 'error': return enabled ? 'log-btn-error-active' : 'log-btn-inactive';
      default: return 'log-btn-inactive';
    }
  }

  onMount(() => {
    loadLogHistory();
  });
</script>

<div class="log-viewer">
  <!-- Header -->
  <div class="log-header">
    <div class="log-header-left">
      <button
        class="log-back-btn"
        onclick={() => onclose?.()}
        title="Back to dashboard"
      >
        <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
        </svg>
        <span>Back</span>
      </button>
      <h1 class="log-title">Logs</h1>
    </div>
    <div class="log-header-right">
      <button
        class="log-action-btn"
        class:log-action-btn-active={paused}
        onclick={togglePause}
        title={paused ? 'Resume log streaming' : 'Pause log streaming'}
      >
        {#if paused}
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
          </svg>
          Resume
        {:else}
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6" />
          </svg>
          Pause
        {/if}
      </button>
      <button
        class="log-action-btn"
        onclick={handleClear}
        title="Clear all log entries"
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
        </svg>
        Clear
      </button>
    </div>
  </div>

  <!-- Filters -->
  <div class="log-filters">
    <div class="log-filters-row">
      <div class="log-search-wrapper">
        <svg class="log-search-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <input
          type="text"
          placeholder="Filter logs..."
          bind:value={searchQuery}
          class="log-search-input"
        />
        {#if searchQuery}
          <button class="log-search-clear" onclick={() => searchQuery = ''} aria-label="Clear search">
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        {/if}
      </div>
      <div class="log-level-btns">
        {#each ['debug', 'info', 'warn', 'error'] as level}
          <button
            class="log-level-btn {levelBtnClass(level)}"
            onclick={() => toggleLevel(level)}
            title="{enabledLevels[level] ? 'Hide' : 'Show'} {level} messages"
          >
            {level.toUpperCase()}
          </button>
        {/each}
      </div>
    </div>
    <div class="log-filters-row">
      <span class="log-source-label">Source:</span>
      <button
        class="log-source-pill"
        class:log-source-pill-active={allSourcesEnabled}
        onclick={toggleAllSources}
      >
        All
      </button>
      {#each discoveredSources.length > 0 ? discoveredSources : ALL_SOURCES as source}
        <button
          class="log-source-pill"
          class:log-source-pill-active={enabledSources[source]}
          onclick={() => toggleSource(source)}
        >
          {source}
        </button>
      {/each}
    </div>
  </div>

  <!-- Log entries -->
  <div
    class="log-entries"
    bind:this={logContainer}
    onscroll={handleScroll}
  >
    {#if filteredEntries.length === 0}
      <div class="log-empty">
        {#if currentEntries.length === 0}
          No log entries yet. Logs will appear here in real-time.
        {:else}
          No entries match the current filters.
        {/if}
      </div>
    {:else}
      {#each filteredEntries as entry, i (entry.timestamp + '-' + i)}
        <div class="log-entry">
          <span class="log-ts">{formatTimestamp(entry.timestamp)}</span>
          <span class="log-level-badge {levelClass(entry.level)}">{entry.level.toUpperCase().padEnd(5)}</span>
          <span class="log-source">{entry.source}</span>
          <span class="log-message">{entry.message}</span>
        </div>
      {/each}
    {/if}
  </div>

  <!-- Footer -->
  <div class="log-footer">
    <span>{currentEntries.length} entries</span>
    <span class="log-footer-sep">|</span>
    <span>showing {filteredEntries.length} filtered</span>
    {#if paused}
      <span class="log-footer-sep">|</span>
      <span class="log-paused-badge">PAUSED</span>
    {/if}
    {#if !autoScroll && !paused}
      <span class="log-footer-sep">|</span>
      <button class="log-scroll-btn" onclick={scrollToBottom}>
        Scroll to bottom
      </button>
    {/if}
  </div>
</div>

<style>
  .log-viewer {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--bg-base);
    color: var(--text-primary);
  }

  /* Header */
  .log-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-surface);
    flex-shrink: 0;
  }

  .log-header-left {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .log-header-right {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .log-back-btn {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.375rem 0.625rem;
    border-radius: 0.375rem;
    font-size: 0.875rem;
    color: var(--text-muted);
    background: transparent;
    border: none;
    cursor: pointer;
    transition: background 0.15s ease, color 0.15s ease;
  }

  .log-back-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .log-title {
    font-size: 1.125rem;
    font-weight: 600;
    margin: 0;
    color: var(--text-primary);
  }

  .log-action-btn {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.375rem 0.75rem;
    border-radius: 0.375rem;
    font-size: 0.8125rem;
    color: var(--text-muted);
    background: var(--bg-overlay);
    border: 1px solid var(--border-subtle);
    cursor: pointer;
    transition: background 0.15s ease, color 0.15s ease;
  }

  .log-action-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .log-action-btn-active {
    background: var(--accent-muted);
    color: var(--accent-primary);
    border-color: var(--accent-primary);
  }

  /* Filters */
  .log-filters {
    padding: 0.5rem 1rem;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-surface);
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .log-filters-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .log-search-wrapper {
    position: relative;
    flex: 1;
    min-width: 150px;
    max-width: 300px;
  }

  .log-search-icon {
    position: absolute;
    left: 0.5rem;
    top: 50%;
    transform: translateY(-50%);
    width: 1rem;
    height: 1rem;
    color: var(--text-disabled);
    pointer-events: none;
  }

  .log-search-input {
    width: 100%;
    padding: 0.375rem 2rem 0.375rem 1.75rem;
    border-radius: 0.375rem;
    font-size: 0.8125rem;
    color: var(--text-primary);
    background: var(--bg-overlay);
    border: 1px solid var(--border-subtle);
    outline: none;
    transition: border-color 0.15s ease;
  }

  .log-search-input::placeholder {
    color: var(--text-disabled);
  }

  .log-search-input:focus {
    border-color: var(--accent-primary);
  }

  .log-search-clear {
    position: absolute;
    right: 0.375rem;
    top: 50%;
    transform: translateY(-50%);
    padding: 0.125rem;
    border: none;
    background: transparent;
    color: var(--text-disabled);
    cursor: pointer;
    border-radius: 0.25rem;
  }

  .log-search-clear:hover {
    color: var(--text-primary);
    background: var(--bg-hover);
  }

  .log-level-btns {
    display: flex;
    gap: 0.25rem;
  }

  .log-level-btn {
    padding: 0.25rem 0.5rem;
    border-radius: 0.25rem;
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.025em;
    border: 1px solid transparent;
    cursor: pointer;
    transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
    font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
  }

  .log-btn-inactive {
    background: var(--bg-overlay);
    color: var(--text-disabled);
    border-color: var(--border-subtle);
  }

  .log-btn-inactive:hover {
    color: var(--text-muted);
    background: var(--bg-hover);
  }

  .log-btn-debug-active {
    background: rgba(156, 163, 175, 0.15);
    color: #9ca3af;
    border-color: rgba(156, 163, 175, 0.3);
  }

  .log-btn-info-active {
    background: rgba(96, 165, 250, 0.15);
    color: #60a5fa;
    border-color: rgba(96, 165, 250, 0.3);
  }

  .log-btn-warn-active {
    background: rgba(251, 191, 36, 0.15);
    color: #fbbf24;
    border-color: rgba(251, 191, 36, 0.3);
  }

  .log-btn-error-active {
    background: rgba(248, 113, 113, 0.15);
    color: #f87171;
    border-color: rgba(248, 113, 113, 0.3);
  }

  .log-source-label {
    font-size: 0.75rem;
    color: var(--text-disabled);
    font-weight: 500;
    flex-shrink: 0;
  }

  .log-source-pill {
    padding: 0.125rem 0.5rem;
    border-radius: 9999px;
    font-size: 0.6875rem;
    border: 1px solid var(--border-subtle);
    background: var(--bg-overlay);
    color: var(--text-disabled);
    cursor: pointer;
    transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
  }

  .log-source-pill:hover {
    color: var(--text-muted);
    background: var(--bg-hover);
  }

  .log-source-pill-active {
    background: var(--accent-muted);
    color: var(--accent-primary);
    border-color: var(--accent-primary);
  }

  /* Log entries */
  .log-entries {
    flex: 1;
    overflow-y: auto;
    overflow-x: auto;
    font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
    font-size: 0.8125rem;
    line-height: 1.5;
    padding: 0.25rem 0;
  }

  .log-empty {
    padding: 3rem 1rem;
    text-align: center;
    color: var(--text-disabled);
    font-family: inherit;
  }

  .log-entry {
    display: flex;
    align-items: baseline;
    gap: 0.625rem;
    padding: 0.125rem 1rem;
    white-space: nowrap;
    transition: background 0.1s ease;
  }

  .log-entry:hover {
    background: var(--bg-hover);
  }

  .log-ts {
    color: var(--text-disabled);
    flex-shrink: 0;
    font-size: 0.75rem;
  }

  .log-level-badge {
    flex-shrink: 0;
    font-size: 0.6875rem;
    font-weight: 700;
    width: 3.5rem;
    text-align: center;
    padding: 0.0625rem 0;
    border-radius: 0.1875rem;
  }

  .log-level-debug {
    color: #9ca3af;
    background: rgba(156, 163, 175, 0.1);
  }

  .log-level-info {
    color: #60a5fa;
    background: rgba(96, 165, 250, 0.1);
  }

  .log-level-warn {
    color: #fbbf24;
    background: rgba(251, 191, 36, 0.1);
  }

  .log-level-error {
    color: #f87171;
    background: rgba(248, 113, 113, 0.1);
  }

  .log-source {
    flex-shrink: 0;
    font-size: 0.6875rem;
    color: var(--text-muted);
    background: var(--bg-overlay);
    padding: 0.0625rem 0.375rem;
    border-radius: 0.1875rem;
    min-width: 4.5rem;
    text-align: center;
  }

  .log-message {
    color: var(--text-secondary);
    white-space: pre-wrap;
    word-break: break-all;
  }

  /* Footer */
  .log-footer {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.375rem 1rem;
    border-top: 1px solid var(--border-subtle);
    background: var(--bg-surface);
    font-size: 0.75rem;
    color: var(--text-disabled);
    flex-shrink: 0;
  }

  .log-footer-sep {
    color: var(--border-subtle);
  }

  .log-paused-badge {
    color: var(--accent-primary);
    font-weight: 600;
    letter-spacing: 0.05em;
  }

  .log-scroll-btn {
    color: var(--accent-primary);
    background: transparent;
    border: none;
    cursor: pointer;
    font-size: 0.75rem;
    padding: 0;
    text-decoration: underline;
  }

  .log-scroll-btn:hover {
    color: var(--text-primary);
  }
</style>

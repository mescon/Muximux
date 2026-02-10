<script lang="ts">
  import { onMount } from 'svelte';
  import { fade, fly } from 'svelte/transition';
  import type { App } from '$lib/types';
  import AppIcon from './AppIcon.svelte';
  import { isMobileViewport } from '$lib/useSwipe';
  import { keybindings, formatKeybinding, type KeyAction } from '$lib/keybindingsStore';
  import { captureKeybindings } from '$lib/keybindingCaptureStore';

  // Props with callbacks instead of dispatchers
  let {
    apps,
    onselect,
    onaction,
    onclose
  }: {
    apps: App[];
    onselect?: (app: App) => void;
    onaction?: (actionId: string) => void;
    onclose?: () => void;
  } = $props();

  // State
  let query = $state('');
  let selectedIndex = $state(0);
  let inputElement = $state<HTMLInputElement>();
  let resultsElement = $state<HTMLElement>();
  let isMobile = $state(false);

  // Recent apps from localStorage
  let recentAppNames = $state<string[]>([]);

  // Command types
  interface Command {
    id: string;
    type: 'app' | 'action' | 'setting';
    label: string;
    description?: string;
    shortcut?: string;
    icon?: string;
    app?: App;
  }

  // Map action IDs to keybinding actions
  const actionToKeybinding: Record<string, KeyAction> = {
    'settings': 'settings',
    'shortcuts': 'shortcuts',
    'fullscreen': 'fullscreen',
    'refresh': 'refresh',
    'home': 'home',
  };

  // Get shortcut string for an action from keybindings store
  function getShortcut(actionId: string): string | undefined {
    const keybindingAction = actionToKeybinding[actionId];
    if (!keybindingAction) return undefined;

    const binding = $keybindings.find(b => b.action === keybindingAction);
    if (!binding) return undefined;

    return formatKeybinding(binding);
  }

  // All available actions (no "Open Search" — we're already in the palette)
  const actions = $derived([
    { id: 'settings', type: 'action' as const, label: 'Open Settings', shortcut: getShortcut('settings'), icon: 'settings' },
    { id: 'shortcuts', type: 'action' as const, label: 'Show Keyboard Shortcuts', shortcut: getShortcut('shortcuts'), icon: 'help' },
    { id: 'fullscreen', type: 'action' as const, label: 'Toggle Fullscreen', shortcut: getShortcut('fullscreen'), icon: 'fullscreen' },
    { id: 'refresh', type: 'action' as const, label: 'Refresh Current App', shortcut: getShortcut('refresh'), icon: 'refresh' },
    { id: 'home', type: 'action' as const, label: 'Go to Splash Screen', shortcut: getShortcut('home'), icon: 'home' },
    { id: 'toggle-keybindings', type: 'action' as const, label: $captureKeybindings ? 'Pause Keyboard Shortcuts' : 'Resume Keyboard Shortcuts', icon: 'keyboard' },
    { id: 'theme-dark', type: 'setting' as const, label: 'Set Dark Theme', icon: 'moon' },
    { id: 'theme-light', type: 'setting' as const, label: 'Set Light Theme', icon: 'sun' },
    { id: 'theme-system', type: 'setting' as const, label: 'Use System Theme', icon: 'system' },
  ] as Command[]);

  // Create app commands
  const appCommands = $derived(apps.map((app, i) => ({
    id: `app-${app.name}`,
    type: 'app' as const,
    label: app.name,
    description: app.group || 'Switch to app',
    shortcut: i < 9 ? `${i + 1}` : undefined,
    app,
  })));

  // All commands combined
  const allCommands = $derived([...appCommands, ...actions]);

  // Fuzzy match function
  function fuzzyMatch(text: string, pattern: string): number {
    text = text.toLowerCase();
    pattern = pattern.toLowerCase();

    if (text === pattern) return 1000;
    if (text.includes(pattern)) {
      const index = text.indexOf(pattern);
      return 100 - index;
    }

    let score = 0;
    let patternIdx = 0;
    let consecutive = 0;

    for (let i = 0; i < text.length && patternIdx < pattern.length; i++) {
      if (text[i] === pattern[patternIdx]) {
        score += 10 + consecutive * 5;
        consecutive++;
        patternIdx++;
      } else {
        consecutive = 0;
      }
    }

    return patternIdx === pattern.length ? score : 0;
  }

  // Get command score (incorporates recency for apps, number matching)
  function getCommandScore(cmd: Command, q: string): number {
    if (!q) {
      // No query — recency for apps, fixed order for actions/settings
      if (cmd.type === 'app' && cmd.app) {
        const recentIndex = recentAppNames.indexOf(cmd.app.name);
        return recentIndex >= 0 ? 1000 - recentIndex : 0;
      }
      return 0;
    }

    // Number query matches app position (1-9)
    const trimmed = q.trim();
    if (cmd.type === 'app' && /^[1-9]$/.test(trimmed)) {
      const appIndex = apps.findIndex(a => a.name === cmd.app?.name);
      if (appIndex === parseInt(trimmed) - 1) return 2000;
    }

    const nameScore = fuzzyMatch(cmd.label, q);
    const descScore = cmd.description ? fuzzyMatch(cmd.description, q) * 0.5 : 0;
    return Math.max(nameScore, descScore);
  }

  // Filter and sort commands
  const filteredCommands = $derived(query
    ? allCommands
        .map(cmd => ({ cmd, score: getCommandScore(cmd, query) }))
        .filter(({ score }) => score > 0)
        .sort((a, b) => b.score - a.score)
        .map(({ cmd }) => cmd)
    : allCommands);

  // When no query: show Recent apps (up to 3), then remaining apps, then actions, then settings
  // When query: flat filtered list grouped by type
  const showRecentHeader = $derived(!query && recentAppNames.length > 0);
  const recentCommands = $derived(showRecentHeader
    ? filteredCommands.filter(c => c.type === 'app' && c.app && recentAppNames.includes(c.app.name)).slice(0, 3)
    : []);
  const otherAppCommands = $derived(showRecentHeader
    ? filteredCommands.filter(c => c.type === 'app' && (!c.app || !recentAppNames.includes(c.app.name)))
    : filteredCommands.filter(c => c.type === 'app'));
  const actionCommands = $derived(filteredCommands.filter(c => c.type === 'action'));
  const settingCommands = $derived(filteredCommands.filter(c => c.type === 'setting'));

  const hasRecent = $derived(recentCommands.length > 0);
  const hasApps = $derived(otherAppCommands.length > 0);
  const hasActions = $derived(actionCommands.length > 0);
  const hasSettings = $derived(settingCommands.length > 0);

  // Global index mapping for keyboard navigation
  const flatCommands = $derived([
    ...recentCommands,
    ...otherAppCommands,
    ...actionCommands,
    ...settingCommands,
  ]);

  // Reset selection when it exceeds bounds or query changes
  $effect(() => {
    if (selectedIndex >= flatCommands.length) {
      selectedIndex = Math.max(0, flatCommands.length - 1);
    }
  });

  $effect(() => {
    // Reset selection when query changes
    query;
    selectedIndex = 0;
  });

  onMount(() => {
    inputElement?.focus();
    isMobile = isMobileViewport();

    // Load recent apps
    const stored = localStorage.getItem('muximux_recent_apps');
    if (stored) {
      try {
        recentAppNames = JSON.parse(stored);
      } catch {
        recentAppNames = [];
      }
    }

    const handleResize = () => { isMobile = isMobileViewport(); };
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
    };
  });

  function selectApp(app: App) {
    // Update recent apps
    recentAppNames = [app.name, ...recentAppNames.filter(n => n !== app.name)].slice(0, 10);
    localStorage.setItem('muximux_recent_apps', JSON.stringify(recentAppNames));

    onselect?.(app);
  }

  function executeCommand(cmd: Command) {
    if (cmd.type === 'app' && cmd.app) {
      selectApp(cmd.app);
    } else {
      onaction?.(cmd.id);
    }
    onclose?.();
  }

  function handleKeydown(event: KeyboardEvent) {
    // Ctrl/Cmd + 1-9 quick select
    if ((event.ctrlKey || event.metaKey) && event.key >= '1' && event.key <= '9') {
      event.preventDefault();
      const index = parseInt(event.key) - 1;
      if (apps[index]) {
        selectApp(apps[index]);
        onclose?.();
      }
      return;
    }

    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        selectedIndex = Math.min(selectedIndex + 1, flatCommands.length - 1);
        scrollToSelected();
        break;
      case 'ArrowUp':
        event.preventDefault();
        selectedIndex = Math.max(selectedIndex - 1, 0);
        scrollToSelected();
        break;
      case 'PageDown':
        event.preventDefault();
        selectedIndex = Math.min(selectedIndex + 10, flatCommands.length - 1);
        scrollToSelected();
        break;
      case 'PageUp':
        event.preventDefault();
        selectedIndex = Math.max(selectedIndex - 10, 0);
        scrollToSelected();
        break;
      case 'Home':
        event.preventDefault();
        selectedIndex = 0;
        scrollToSelected();
        break;
      case 'End':
        event.preventDefault();
        selectedIndex = flatCommands.length - 1;
        scrollToSelected();
        break;
      case 'Enter':
        event.preventDefault();
        if (flatCommands[selectedIndex]) {
          executeCommand(flatCommands[selectedIndex]);
        }
        break;
      case 'Escape':
        onclose?.();
        break;
    }
  }

  function scrollToSelected() {
    // Wait a tick for the DOM to update, then scroll the selected item into view
    requestAnimationFrame(() => {
      if (!resultsElement) return;
      const item = resultsElement.querySelector('[data-selected="true"]') as HTMLElement;
      if (item) {
        item.scrollIntoView({ block: 'nearest' });
      }
    });
  }

  function getOpenModeIcon(mode: string): string {
    switch (mode) {
      case 'new_tab': return '↗';
      case 'new_window': return '⧉';
      default: return '';
    }
  }

  function getIconSvg(icon: string | undefined): string {
    switch (icon) {
      case 'search':
        return 'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z';
      case 'settings':
        return 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z';
      case 'help':
        return 'M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z';
      case 'fullscreen':
        return 'M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4';
      case 'refresh':
        return 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15';
      case 'home':
        return 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6';
      case 'keyboard':
        return 'M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4';
      case 'moon':
        return 'M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z';
      case 'sun':
        return 'M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z';
      case 'system':
        return 'M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z';
      default:
        return 'M13 10V3L4 14h7v7l9-11h-7z';
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Backdrop -->
<div
  class="command-palette-backdrop fixed inset-0 backdrop-blur-sm z-50 flex {isMobile ? 'items-end' : 'items-start justify-center pt-[15vh]'}"
  onclick={(e) => {
    if (e.currentTarget === e.target) {
      onclose?.();
    }
  }}
  role="dialog"
  aria-modal="true"
  aria-label="Command palette"
  tabindex="-1"
  transition:fade={{ duration: 150 }}
>
  <!-- Command palette modal -->
  <div
    class="command-palette-modal w-full shadow-2xl overflow-hidden
           {isMobile
             ? 'rounded-t-2xl max-h-[85vh] border-b-0'
             : 'max-w-xl rounded-xl mx-4'}"
    onclick={(e) => e.stopPropagation()}
    role="presentation"
    in:fly={{ y: isMobile ? 100 : -20, duration: 200 }}
    out:fade={{ duration: 100 }}
  >
    <!-- Mobile drag handle -->
    {#if isMobile}
      <div class="flex justify-center pt-3 pb-1">
        <div class="w-10 h-1 rounded-full" style="background: var(--bg-active);"></div>
      </div>
    {/if}

    <!-- Search input -->
    <div class="p-4 border-b" style="border-color: var(--border-subtle);">
      <div class="flex items-center space-x-3">
        <svg class="w-5 h-5 flex-shrink-0" style="color: var(--accent-primary);" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <input
          bind:this={inputElement}
          bind:value={query}
          type="text"
          placeholder="Search apps and commands..."
          class="command-palette-input flex-1 bg-transparent outline-none text-lg min-w-0"
          onkeydown={handleKeydown}
        />
        <kbd class="command-palette-kbd hidden sm:inline-block px-2 py-1 text-xs rounded flex-shrink-0">esc</kbd>
      </div>
    </div>

    <!-- Results -->
    <div bind:this={resultsElement} class="{isMobile ? 'max-h-[60vh]' : 'max-h-80'} overflow-auto">
      {#if flatCommands.length === 0}
        <div class="p-4 text-center" style="color: var(--text-disabled);">
          No results found for "{query}"
        </div>
      {:else}
        <!-- Recent section (only when no query) -->
        {#if hasRecent}
          <div class="px-4 pt-3 pb-1">
            <span class="text-xs font-semibold uppercase tracking-wider" style="color: var(--text-disabled);">Recent</span>
          </div>
          <ul class="pb-2">
            {#each recentCommands as cmd}
              {@const globalIndex = flatCommands.indexOf(cmd)}
              <li>
                <button
                  class="command-palette-item w-full px-4 min-h-[52px] flex items-center space-x-3 text-left
                         {isMobile ? 'py-3.5' : 'py-3'}"
                  style="background: {globalIndex === selectedIndex ? 'var(--bg-hover)' : 'transparent'};"
                  data-selected={globalIndex === selectedIndex}
                  onclick={() => executeCommand(cmd)}
                  onmouseenter={() => selectedIndex = globalIndex}
                >
                  {#if cmd.app}
                    <AppIcon icon={cmd.app.icon} name={cmd.app.name} color={cmd.app.color} size="md" />
                  {/if}
                  <div class="flex-1 min-w-0">
                    <div class="font-medium truncate" style="color: var(--text-primary);">
                      {cmd.label}
                      {#if cmd.app && cmd.app.open_mode !== 'iframe'}
                        <span class="ml-1 text-xs opacity-60">{getOpenModeIcon(cmd.app.open_mode)}</span>
                      {/if}
                    </div>
                    {#if cmd.description}
                      <div class="text-sm truncate" style="color: var(--text-disabled);">{cmd.description}</div>
                    {/if}
                  </div>
                  {#if cmd.shortcut}
                    <kbd class="command-palette-kbd hidden sm:inline-block px-2 py-1 text-xs rounded flex-shrink-0">
                      {cmd.shortcut}
                    </kbd>
                  {/if}
                </button>
              </li>
            {/each}
          </ul>
        {/if}

        <!-- Apps section -->
        {#if hasApps}
          <div class="px-4 pt-2 pb-1 {hasRecent ? 'border-t' : 'pt-3'}" style="{hasRecent ? 'border-color: var(--border-subtle);' : ''}">
            <span class="text-xs font-semibold uppercase tracking-wider" style="color: var(--text-disabled);">Apps</span>
          </div>
          <ul class="pb-2">
            {#each otherAppCommands as cmd}
              {@const globalIndex = flatCommands.indexOf(cmd)}
              <li>
                <button
                  class="command-palette-item w-full px-4 min-h-[52px] flex items-center space-x-3 text-left
                         {isMobile ? 'py-3.5' : 'py-3'}"
                  style="background: {globalIndex === selectedIndex ? 'var(--bg-hover)' : 'transparent'};"
                  data-selected={globalIndex === selectedIndex}
                  onclick={() => executeCommand(cmd)}
                  onmouseenter={() => selectedIndex = globalIndex}
                >
                  {#if cmd.app}
                    <AppIcon icon={cmd.app.icon} name={cmd.app.name} color={cmd.app.color} size="md" />
                  {/if}
                  <div class="flex-1 min-w-0">
                    <div class="font-medium truncate" style="color: var(--text-primary);">
                      {cmd.label}
                      {#if cmd.app && cmd.app.open_mode !== 'iframe'}
                        <span class="ml-1 text-xs opacity-60">{getOpenModeIcon(cmd.app.open_mode)}</span>
                      {/if}
                    </div>
                    {#if cmd.description}
                      <div class="text-sm truncate" style="color: var(--text-disabled);">{cmd.description}</div>
                    {/if}
                  </div>
                  {#if cmd.shortcut}
                    <kbd class="command-palette-kbd hidden sm:inline-block px-2 py-1 text-xs rounded flex-shrink-0">
                      {cmd.shortcut}
                    </kbd>
                  {/if}
                </button>
              </li>
            {/each}
          </ul>
        {/if}

        <!-- Actions section -->
        {#if hasActions}
          <div class="px-4 pt-2 pb-1 {hasRecent || hasApps ? 'border-t' : ''}" style="{hasRecent || hasApps ? 'border-color: var(--border-subtle);' : ''}">
            <span class="text-xs font-semibold uppercase tracking-wider" style="color: var(--text-disabled);">Actions</span>
          </div>
          <ul class="pb-2">
            {#each actionCommands as cmd}
              {@const globalIndex = flatCommands.indexOf(cmd)}
              <li>
                <button
                  class="command-palette-item w-full px-4 min-h-[48px] flex items-center space-x-3 text-left
                         {isMobile ? 'py-3' : 'py-2.5'}"
                  style="background: {globalIndex === selectedIndex ? 'var(--bg-hover)' : 'transparent'};"
                  data-selected={globalIndex === selectedIndex}
                  onclick={() => executeCommand(cmd)}
                  onmouseenter={() => selectedIndex = globalIndex}
                >
                  <div class="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0" style="background: var(--bg-hover);">
                    <svg class="w-4 h-4" style="color: var(--text-muted);" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getIconSvg(cmd.icon)} />
                    </svg>
                  </div>
                  <div class="flex-1 min-w-0">
                    <div class="truncate" style="color: var(--text-primary);">{cmd.label}</div>
                  </div>
                  {#if cmd.shortcut}
                    <kbd class="command-palette-kbd hidden sm:inline-block px-2 py-1 text-xs rounded flex-shrink-0">
                      {cmd.shortcut}
                    </kbd>
                  {/if}
                </button>
              </li>
            {/each}
          </ul>
        {/if}

        <!-- Settings section -->
        {#if hasSettings}
          <div class="px-4 pt-2 pb-1 {hasRecent || hasApps || hasActions ? 'border-t' : ''}" style="{hasRecent || hasApps || hasActions ? 'border-color: var(--border-subtle);' : ''}">
            <span class="text-xs font-semibold uppercase tracking-wider" style="color: var(--text-disabled);">Settings</span>
          </div>
          <ul class="pb-2">
            {#each settingCommands as cmd}
              {@const globalIndex = flatCommands.indexOf(cmd)}
              <li>
                <button
                  class="command-palette-item w-full px-4 min-h-[48px] flex items-center space-x-3 text-left
                         {isMobile ? 'py-3' : 'py-2.5'}"
                  style="background: {globalIndex === selectedIndex ? 'var(--bg-hover)' : 'transparent'};"
                  data-selected={globalIndex === selectedIndex}
                  onclick={() => executeCommand(cmd)}
                  onmouseenter={() => selectedIndex = globalIndex}
                >
                  <div class="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0" style="background: var(--bg-hover);">
                    <svg class="w-4 h-4" style="color: var(--text-muted);" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getIconSvg(cmd.icon)} />
                    </svg>
                  </div>
                  <div class="flex-1 min-w-0">
                    <div class="truncate" style="color: var(--text-primary);">{cmd.label}</div>
                  </div>
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      {/if}
    </div>

    <!-- Footer hints -->
    {#if !isMobile}
      <div class="px-4 py-2 border-t text-xs flex items-center space-x-4" style="border-color: var(--border-subtle); color: var(--text-disabled);">
        <span>↑↓ Navigate</span>
        <span>⏎ Execute</span>
        <span>⌘1-9 Quick select</span>
        <span>esc Close</span>
      </div>
    {:else}
      <div class="px-4 py-3 pb-safe border-t text-center" style="border-color: var(--border-subtle);">
        <span class="text-xs" style="color: var(--text-disabled);">Tap outside to close</span>
      </div>
    {/if}
  </div>
</div>

<style>
  /* Command palette theming */
  .command-palette-backdrop {
    background: rgba(0, 0, 0, 0.6);
  }

  .command-palette-modal {
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
  }

  .command-palette-input {
    color: var(--text-primary);
  }

  .command-palette-input::placeholder {
    color: var(--text-disabled);
  }

  .command-palette-kbd {
    background: var(--bg-overlay);
    color: var(--text-disabled);
  }

  .command-palette-item:hover {
    background: var(--bg-hover) !important;
  }
</style>

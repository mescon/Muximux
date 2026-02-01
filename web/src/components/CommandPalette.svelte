<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import { fade, fly } from 'svelte/transition';
  import type { App } from '$lib/types';
  import AppIcon from './AppIcon.svelte';
  import { isMobileViewport } from '$lib/useSwipe';

  export let apps: App[];

  const dispatch = createEventDispatcher<{
    select: App;
    action: string;
    close: void;
  }>();

  let query = '';
  let selectedIndex = 0;
  let inputElement: HTMLInputElement;
  let isMobile = false;

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

  // All available commands
  const actions: Command[] = [
    { id: 'search', type: 'action', label: 'Open Search', shortcut: '/', icon: 'search' },
    { id: 'settings', type: 'action', label: 'Open Settings', shortcut: 'Ctrl+,', icon: 'settings' },
    { id: 'shortcuts', type: 'action', label: 'Show Keyboard Shortcuts', shortcut: '?', icon: 'help' },
    { id: 'fullscreen', type: 'action', label: 'Toggle Fullscreen', shortcut: 'F', icon: 'fullscreen' },
    { id: 'refresh', type: 'action', label: 'Refresh Current App', shortcut: 'R', icon: 'refresh' },
    { id: 'home', type: 'action', label: 'Go to Splash Screen', shortcut: 'Esc', icon: 'home' },
    { id: 'theme-dark', type: 'setting', label: 'Set Dark Theme', icon: 'moon' },
    { id: 'theme-light', type: 'setting', label: 'Set Light Theme', icon: 'sun' },
    { id: 'theme-system', type: 'setting', label: 'Use System Theme', icon: 'system' },
  ];

  // Create app commands
  $: appCommands = apps.map((app, i) => ({
    id: `app-${app.name}`,
    type: 'app' as const,
    label: app.name,
    description: app.group || 'Switch to app',
    shortcut: i < 9 ? `${i + 1}` : undefined,
    app,
  }));

  // All commands combined
  $: allCommands = [...appCommands, ...actions];

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

  // Filter and sort commands
  $: filteredCommands = query
    ? allCommands
        .map(cmd => ({
          cmd,
          score: Math.max(
            fuzzyMatch(cmd.label, query),
            cmd.description ? fuzzyMatch(cmd.description, query) * 0.5 : 0
          ),
        }))
        .filter(({ score }) => score > 0)
        .sort((a, b) => b.score - a.score)
        .map(({ cmd }) => cmd)
    : allCommands;

  $: if (selectedIndex >= filteredCommands.length) {
    selectedIndex = Math.max(0, filteredCommands.length - 1);
  }

  // Reset selection when query changes
  $: query, selectedIndex = 0;

  onMount(() => {
    inputElement?.focus();
    isMobile = isMobileViewport();

    const handleResize = () => { isMobile = isMobileViewport(); };
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
    };
  });

  function executeCommand(cmd: Command) {
    if (cmd.type === 'app' && cmd.app) {
      dispatch('select', cmd.app);
    } else {
      dispatch('action', cmd.id);
    }
    dispatch('close');
  }

  function handleKeydown(event: KeyboardEvent) {
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        selectedIndex = Math.min(selectedIndex + 1, filteredCommands.length - 1);
        break;
      case 'ArrowUp':
        event.preventDefault();
        selectedIndex = Math.max(selectedIndex - 1, 0);
        break;
      case 'Enter':
        event.preventDefault();
        if (filteredCommands[selectedIndex]) {
          executeCommand(filteredCommands[selectedIndex]);
        }
        break;
      case 'Escape':
        dispatch('close');
        break;
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

  // Group commands by type for display
  $: groupedCommands = {
    apps: filteredCommands.filter(c => c.type === 'app'),
    actions: filteredCommands.filter(c => c.type === 'action'),
    settings: filteredCommands.filter(c => c.type === 'setting'),
  };

  $: hasApps = groupedCommands.apps.length > 0;
  $: hasActions = groupedCommands.actions.length > 0;
  $: hasSettings = groupedCommands.settings.length > 0;

  // Global index mapping for keyboard navigation
  $: flatCommands = [
    ...groupedCommands.apps,
    ...groupedCommands.actions,
    ...groupedCommands.settings,
  ];
</script>

<!-- Backdrop -->
<div
  class="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex {isMobile ? 'items-end' : 'items-start justify-center pt-[15vh]'}"
  on:click={() => dispatch('close')}
  on:keydown={handleKeydown}
  role="dialog"
  aria-modal="true"
  aria-label="Command palette"
  transition:fade={{ duration: 150 }}
>
  <!-- Command palette modal -->
  <div
    class="w-full bg-gray-800 shadow-2xl border border-gray-700 overflow-hidden
           {isMobile
             ? 'rounded-t-2xl max-h-[85vh] border-b-0'
             : 'max-w-xl rounded-xl mx-4'}"
    on:click|stopPropagation
    role="presentation"
    in:fly={{ y: isMobile ? 100 : -20, duration: 200 }}
    out:fade={{ duration: 100 }}
  >
    <!-- Mobile drag handle -->
    {#if isMobile}
      <div class="flex justify-center pt-3 pb-1">
        <div class="w-10 h-1 bg-gray-600 rounded-full"></div>
      </div>
    {/if}

    <!-- Search input -->
    <div class="p-4 border-b border-gray-700">
      <div class="flex items-center space-x-3">
        <svg class="w-5 h-5 text-brand-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z" />
        </svg>
        <input
          bind:this={inputElement}
          bind:value={query}
          type="text"
          placeholder="Type a command or search..."
          class="flex-1 bg-transparent text-white placeholder-gray-500 outline-none text-lg min-w-0"
          on:keydown={handleKeydown}
        />
        <kbd class="hidden sm:inline-block px-2 py-1 text-xs text-gray-500 bg-gray-700 rounded flex-shrink-0">esc</kbd>
      </div>
    </div>

    <!-- Results -->
    <div class="{isMobile ? 'max-h-[60vh]' : 'max-h-80'} overflow-auto">
      {#if filteredCommands.length === 0}
        <div class="p-4 text-center text-gray-500">
          No commands found matching "{query}"
        </div>
      {:else}
        <!-- Apps section -->
        {#if hasApps}
          <div class="px-4 pt-3 pb-1">
            <span class="text-xs font-semibold text-gray-500 uppercase tracking-wider">Apps</span>
          </div>
          <ul class="pb-2">
            {#each groupedCommands.apps as cmd}
              {@const globalIndex = flatCommands.indexOf(cmd)}
              <li>
                <button
                  class="w-full px-4 min-h-[52px] flex items-center space-x-3 text-left
                         {isMobile ? 'py-3.5' : 'py-3'}
                         {globalIndex === selectedIndex ? 'bg-gray-700' : 'hover:bg-gray-700/50'}"
                  on:click={() => executeCommand(cmd)}
                  on:mouseenter={() => selectedIndex = globalIndex}
                >
                  {#if cmd.app}
                    <AppIcon icon={cmd.app.icon} name={cmd.app.name} color={cmd.app.color} size="md" />
                  {/if}
                  <div class="flex-1 min-w-0">
                    <div class="text-white font-medium truncate">{cmd.label}</div>
                    {#if cmd.description}
                      <div class="text-sm text-gray-500 truncate">{cmd.description}</div>
                    {/if}
                  </div>
                  {#if cmd.shortcut}
                    <kbd class="hidden sm:inline-block px-2 py-1 text-xs text-gray-500 bg-gray-700 rounded flex-shrink-0">
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
          <div class="px-4 pt-2 pb-1 {hasApps ? 'border-t border-gray-700' : ''}">
            <span class="text-xs font-semibold text-gray-500 uppercase tracking-wider">Actions</span>
          </div>
          <ul class="pb-2">
            {#each groupedCommands.actions as cmd}
              {@const globalIndex = flatCommands.indexOf(cmd)}
              <li>
                <button
                  class="w-full px-4 min-h-[48px] flex items-center space-x-3 text-left
                         {isMobile ? 'py-3' : 'py-2.5'}
                         {globalIndex === selectedIndex ? 'bg-gray-700' : 'hover:bg-gray-700/50'}"
                  on:click={() => executeCommand(cmd)}
                  on:mouseenter={() => selectedIndex = globalIndex}
                >
                  <div class="w-8 h-8 rounded-lg bg-gray-700 flex items-center justify-center flex-shrink-0">
                    <svg class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getIconSvg(cmd.icon)} />
                    </svg>
                  </div>
                  <div class="flex-1 min-w-0">
                    <div class="text-white truncate">{cmd.label}</div>
                  </div>
                  {#if cmd.shortcut}
                    <kbd class="hidden sm:inline-block px-2 py-1 text-xs text-gray-500 bg-gray-700 rounded flex-shrink-0">
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
          <div class="px-4 pt-2 pb-1 {hasApps || hasActions ? 'border-t border-gray-700' : ''}">
            <span class="text-xs font-semibold text-gray-500 uppercase tracking-wider">Settings</span>
          </div>
          <ul class="pb-2">
            {#each groupedCommands.settings as cmd}
              {@const globalIndex = flatCommands.indexOf(cmd)}
              <li>
                <button
                  class="w-full px-4 min-h-[48px] flex items-center space-x-3 text-left
                         {isMobile ? 'py-3' : 'py-2.5'}
                         {globalIndex === selectedIndex ? 'bg-gray-700' : 'hover:bg-gray-700/50'}"
                  on:click={() => executeCommand(cmd)}
                  on:mouseenter={() => selectedIndex = globalIndex}
                >
                  <div class="w-8 h-8 rounded-lg bg-gray-700 flex items-center justify-center flex-shrink-0">
                    <svg class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getIconSvg(cmd.icon)} />
                    </svg>
                  </div>
                  <div class="flex-1 min-w-0">
                    <div class="text-white truncate">{cmd.label}</div>
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
      <div class="px-4 py-2 border-t border-gray-700 text-xs text-gray-500 flex items-center space-x-4">
        <span>↑↓ Navigate</span>
        <span>⏎ Execute</span>
        <span>esc Close</span>
      </div>
    {:else}
      <div class="px-4 py-3 pb-safe border-t border-gray-700 text-center">
        <span class="text-xs text-gray-500">Tap outside to close</span>
      </div>
    {/if}
  </div>
</div>

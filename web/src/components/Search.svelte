<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import { fade, fly } from 'svelte/transition';
  import type { App } from '$lib/types';
  import AppIcon from './AppIcon.svelte';
  import { isMobileViewport } from '$lib/useSwipe';

  export let apps: App[];

  const dispatch = createEventDispatcher<{
    select: App;
    close: void;
  }>();

  let query = '';
  let selectedIndex = 0;
  let inputElement: HTMLInputElement;
  let isMobile = false;

  // Recent apps from localStorage
  let recentAppNames: string[] = [];

  onMount(() => {
    inputElement?.focus();
    isMobile = isMobileViewport();

    // Update on resize
    const handleResize = () => { isMobile = isMobileViewport(); };
    window.addEventListener('resize', handleResize);

    // Load recent apps
    const stored = localStorage.getItem('muximux_recent_apps');
    if (stored) {
      try {
        recentAppNames = JSON.parse(stored);
      } catch {
        recentAppNames = [];
      }
    }

    return () => {
      window.removeEventListener('resize', handleResize);
    };
  });

  // Fuzzy match function
  function fuzzyMatch(text: string, pattern: string): number {
    text = text.toLowerCase();
    pattern = pattern.toLowerCase();

    // Exact match gets highest score
    if (text === pattern) return 1000;

    // Contains match
    if (text.includes(pattern)) {
      // Prefer matches at the start
      const index = text.indexOf(pattern);
      return 100 - index;
    }

    // Fuzzy character match
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

    // Only return score if all pattern chars were found
    return patternIdx === pattern.length ? score : 0;
  }

  // Get app match score
  function getAppScore(app: App, q: string): number {
    if (!q) {
      // No query - sort by recency
      const recentIndex = recentAppNames.indexOf(app.name);
      return recentIndex >= 0 ? 1000 - recentIndex : 0;
    }

    const nameScore = fuzzyMatch(app.name, q);
    const groupScore = app.group ? fuzzyMatch(app.group, q) * 0.5 : 0;

    return Math.max(nameScore, groupScore);
  }

  // Filter and sort apps
  $: filteredApps = apps
    .map(app => ({ app, score: getAppScore(app, query) }))
    .filter(({ score }) => !query || score > 0)
    .sort((a, b) => b.score - a.score)
    .map(({ app }) => app);

  $: if (selectedIndex >= filteredApps.length) {
    selectedIndex = Math.max(0, filteredApps.length - 1);
  }

  // Reset selection when query changes
  $: query, selectedIndex = 0;

  function selectApp(app: App) {
    // Update recent apps
    recentAppNames = [app.name, ...recentAppNames.filter(n => n !== app.name)].slice(0, 10);
    localStorage.setItem('muximux_recent_apps', JSON.stringify(recentAppNames));

    dispatch('select', app);
  }

  function handleKeydown(event: KeyboardEvent) {
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        selectedIndex = Math.min(selectedIndex + 1, filteredApps.length - 1);
        break;
      case 'ArrowUp':
        event.preventDefault();
        selectedIndex = Math.max(selectedIndex - 1, 0);
        break;
      case 'Enter':
        event.preventDefault();
        if (filteredApps[selectedIndex]) {
          selectApp(filteredApps[selectedIndex]);
        }
        break;
      case 'Escape':
        dispatch('close');
        break;
      default:
        // Quick select with Cmd/Ctrl + number
        if ((event.metaKey || event.ctrlKey) && event.key >= '1' && event.key <= '9') {
          event.preventDefault();
          const index = parseInt(event.key) - 1;
          if (filteredApps[index]) {
            selectApp(filteredApps[index]);
          }
        }
    }
  }

  function getOpenModeIcon(mode: string): string {
    switch (mode) {
      case 'new_tab': return '↗';
      case 'new_window': return '⧉';
      default: return '';
    }
  }

  // Show section headers
  $: showRecentHeader = !query && recentAppNames.length > 0;
  $: recentApps = showRecentHeader
    ? filteredApps.filter(app => recentAppNames.includes(app.name)).slice(0, 3)
    : [];
  $: otherApps = showRecentHeader
    ? filteredApps.filter(app => !recentAppNames.includes(app.name))
    : filteredApps;
</script>

<!-- Backdrop -->
<div
  class="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex {isMobile ? 'items-end' : 'items-start justify-center pt-[15vh]'}"
  on:click={() => dispatch('close')}
  on:keydown={handleKeydown}
  role="dialog"
  aria-modal="true"
  aria-label="Search apps"
  transition:fade={{ duration: 150 }}
>
  <!-- Search modal - bottom sheet on mobile, centered modal on desktop -->
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
    <!-- Mobile drag handle for bottom sheet -->
    {#if isMobile}
      <div class="flex justify-center pt-3 pb-1">
        <div class="w-10 h-1 bg-gray-600 rounded-full"></div>
      </div>
    {/if}

    <!-- Search input -->
    <div class="p-4 border-b border-gray-700">
      <div class="flex items-center space-x-3">
        <svg class="w-5 h-5 text-gray-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <input
          bind:this={inputElement}
          bind:value={query}
          type="text"
          placeholder="Search apps..."
          class="flex-1 bg-transparent text-white placeholder-gray-500 outline-none text-lg min-w-0"
          on:keydown={handleKeydown}
        />
        <kbd class="hidden sm:inline-block px-2 py-1 text-xs text-gray-500 bg-gray-700 rounded flex-shrink-0">esc</kbd>
      </div>
    </div>

    <!-- Results - taller on mobile for better scrolling -->
    <div class="{isMobile ? 'max-h-[60vh]' : 'max-h-80'} overflow-auto">
      {#if filteredApps.length === 0}
        <div class="p-4 text-center text-gray-500">
          No apps found matching "{query}"
        </div>
      {:else}
        <!-- Recent apps section -->
        {#if showRecentHeader && recentApps.length > 0}
          <div class="px-4 pt-3 pb-1">
            <span class="text-xs font-semibold text-gray-500 uppercase tracking-wider">Recent</span>
          </div>
          <ul class="pb-2">
            {#each recentApps as app, i}
              {@const globalIndex = i}
              <li>
                <button
                  class="w-full px-4 min-h-[52px] flex items-center space-x-3 text-left
                         {isMobile ? 'py-3.5' : 'py-3'}
                         {globalIndex === selectedIndex ? 'bg-gray-700' : 'hover:bg-gray-700/50'}"
                  on:click={() => selectApp(app)}
                  on:mouseenter={() => selectedIndex = globalIndex}
                >
                  <AppIcon icon={app.icon} name={app.name} color={app.color} size="lg" />
                  <div class="flex-1 min-w-0">
                    <div class="text-white font-medium truncate">
                      {app.name}
                      {#if app.open_mode !== 'iframe'}
                        <span class="ml-1 text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
                      {/if}
                    </div>
                    {#if app.group}
                      <div class="text-sm text-gray-500 truncate">{app.group}</div>
                    {/if}
                  </div>
                  {#if globalIndex < 9}
                    <kbd class="hidden sm:inline-block px-2 py-1 text-xs text-gray-500 bg-gray-700 rounded flex-shrink-0">
                      ⌘{globalIndex + 1}
                    </kbd>
                  {/if}
                </button>
              </li>
            {/each}
          </ul>
        {/if}

        <!-- All apps section -->
        {#if otherApps.length > 0}
          {#if showRecentHeader}
            <div class="px-4 pt-2 pb-1 border-t border-gray-700">
              <span class="text-xs font-semibold text-gray-500 uppercase tracking-wider">All Apps</span>
            </div>
          {/if}
          <ul class="py-2">
            {#each otherApps as app, i}
              {@const globalIndex = showRecentHeader ? recentApps.length + i : i}
              <li>
                <button
                  class="w-full px-4 min-h-[52px] flex items-center space-x-3 text-left
                         {isMobile ? 'py-3.5' : 'py-3'}
                         {globalIndex === selectedIndex ? 'bg-gray-700' : 'hover:bg-gray-700/50'}"
                  on:click={() => selectApp(app)}
                  on:mouseenter={() => selectedIndex = globalIndex}
                >
                  <AppIcon icon={app.icon} name={app.name} color={app.color} size="lg" />
                  <div class="flex-1 min-w-0">
                    <div class="text-white font-medium truncate">
                      {app.name}
                      {#if app.open_mode !== 'iframe'}
                        <span class="ml-1 text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
                      {/if}
                    </div>
                    {#if app.group}
                      <div class="text-sm text-gray-500 truncate">{app.group}</div>
                    {/if}
                  </div>
                  {#if globalIndex < 9}
                    <kbd class="hidden sm:inline-block px-2 py-1 text-xs text-gray-500 bg-gray-700 rounded flex-shrink-0">
                      ⌘{globalIndex + 1}
                    </kbd>
                  {/if}
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      {/if}
    </div>

    <!-- Footer hints - hidden on mobile to save space -->
    {#if !isMobile}
      <div class="px-4 py-2 border-t border-gray-700 text-xs text-gray-500 flex items-center space-x-4">
        <span>↑↓ Navigate</span>
        <span>⏎ Open</span>
        <span>⌘1-9 Quick select</span>
        <span>esc Close</span>
      </div>
    {:else}
      <!-- Mobile-friendly close hint with safe area padding -->
      <div class="px-4 py-3 pb-safe border-t border-gray-700 text-center">
        <span class="text-xs text-gray-500">Tap outside to close</span>
      </div>
    {/if}
  </div>
</div>

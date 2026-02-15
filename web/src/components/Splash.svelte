<script lang="ts">
  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import type { App, Config } from '$lib/types';
  import AppIcon from './AppIcon.svelte';
  import HealthIndicator from './HealthIndicator.svelte';
  import MuximuxLogo from './MuximuxLogo.svelte';

  let { apps, config, showHealth = true, onselect, onsettings }: {
    apps: App[];
    config: Config;
    showHealth?: boolean;
    onselect?: (app: App) => void;
    onsettings?: () => void;
  } = $props();

  let mounted = $state(false);
  onMount(() => {
    // Trigger staggered animations after mount
    mounted = true;
  });

  // Group apps by their group
  let groupedApps = $derived(apps.reduce((acc, app) => {
    const group = app.group || 'Ungrouped';
    if (!acc[group]) acc[group] = [];
    acc[group].push(app);
    return acc;
  }, {} as Record<string, App[]>));

  // Sort apps within groups and get group order
  let groups = $derived.by(() => {
    // Sort within groups
    for (const group of Object.keys(groupedApps)) {
      groupedApps[group].sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
    }
    return Object.keys(groupedApps).sort((a, b) => {
      if (a === 'Ungrouped') return 1;
      if (b === 'Ungrouped') return -1;
      const groupA = config.groups.find(g => g.name === a);
      const groupB = config.groups.find(g => g.name === b);
      return (groupA?.order ?? 0) - (groupB?.order ?? 0);
    });
  });

  function getOpenModeIcon(mode: string): string {
    switch (mode) {
      case 'new_tab': return '\u2197';
      case 'new_window': return '\u29C9';
      default: return '';
    }
  }

  function getAppIndex(app: App): number {
    const allApps = groups.flatMap(g => groupedApps[g]);
    return allApps.findIndex(a => a.name === app.name);
  }

  // Calculate stagger delay based on position
  function getStaggerDelay(groupIndex: number, appIndex: number): string {
    const delay = (groupIndex * 50) + (appIndex * 30);
    return `${delay}ms`;
  }
</script>

<div
  class="h-full overflow-auto scrollbar-styled"
  style="background: var(--bg-base);"
  in:fade={{ duration: 200, delay: 50 }}
  out:fade={{ duration: 150 }}
>
  <!-- Subtle gradient overlay for depth -->
  <div class="absolute inset-0 pointer-events-none opacity-50"
    style="background: radial-gradient(ellipse at top, var(--accent-subtle) 0%, transparent 50%);">
  </div>

  <div class="relative max-w-7xl mx-auto px-6 py-8 md:px-8 md:py-12">
    <!-- Header -->
    <header class="text-center mb-10 md:mb-14">
      <div class="flex justify-center mb-4">
        <MuximuxLogo height="80" class="text-[var(--accent-primary)]" />
      </div>
      <p class="text-sm md:text-base" style="color: var(--text-muted);">
        Select an application to get started
      </p>

      <!-- Quick keyboard hints -->
      <div class="mt-4 flex items-center justify-center gap-4 text-xs" style="color: var(--text-muted);">
        <span class="flex items-center gap-1.5">
          <kbd class="kbd">Ctrl</kbd>
          <kbd class="kbd">K</kbd>
          <span class="ml-1">Search</span>
        </span>
        <span class="flex items-center gap-1.5">
          <kbd class="kbd">1</kbd>
          <span>-</span>
          <kbd class="kbd">9</kbd>
          <span class="ml-1">Quick access</span>
        </span>
        <span class="flex items-center gap-1.5">
          <kbd class="kbd">?</kbd>
          <span class="ml-1">Shortcuts</span>
        </span>
      </div>
    </header>

    <!-- App grid by groups -->
    {#each groups as group, groupIndex (group)}
      <section class="mb-8 md:mb-10">
        <!-- Group header -->
        {#if groups.length > 1 || group !== 'Ungrouped'}
          <div class="flex items-center gap-3 mb-4">
            <h2 class="font-display text-xs font-semibold tracking-widest uppercase"
                style="color: var(--text-muted);">
              {group}
            </h2>
            <div class="flex-1 h-px" style="background: var(--border-subtle);"></div>
            <span class="text-xs tabular-nums" style="color: var(--text-disabled);">
              {groupedApps[group].length} {groupedApps[group].length === 1 ? 'app' : 'apps'}
            </span>
          </div>
        {/if}

        <!-- App cards grid -->
        <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3 md:gap-4">
          {#each groupedApps[group] as app, appIndex (app.name)}
            {@const globalIndex = getAppIndex(app)}
            <button
              class="app-card group opacity-0"
              class:animate-slide-up={mounted}
              style="animation-delay: {getStaggerDelay(groupIndex, appIndex)};"
              onclick={() => onselect?.(app)}
            >
              <!-- Health indicator -->
              {#if showHealth}
                <div class="absolute top-2.5 right-2.5 z-10">
                  <HealthIndicator appName={app.name} size="sm" />
                </div>
              {/if}

              <!-- Keyboard shortcut badge (1-9) -->
              {#if globalIndex < 9}
                <div class="absolute top-2.5 left-2.5 z-10">
                  <span class="kbd">
                    {globalIndex + 1}
                  </span>
                </div>
              {/if}

              <!-- App icon with glow effect on hover -->
              <div class="relative mb-3 mt-2">
                <!-- Glow effect -->
                <div
                  class="absolute inset-0 rounded-full opacity-0 group-hover:opacity-40 transition-opacity blur-xl"
                  style="background: {app.color || 'var(--accent-primary)'};"
                ></div>
                <div class="relative">
                  <AppIcon icon={app.icon} name={app.name} color={app.color} size="xl" />
                </div>
              </div>

              <!-- App name -->
              <span class="text-sm font-medium text-center transition-colors"
                    style="color: var(--text-secondary);">
                <span class="group-hover:text-[var(--text-primary)]">{app.name}</span>
                {#if app.open_mode !== 'iframe'}
                  <span class="ml-1 text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
                {/if}
              </span>

              <!-- Color accent bar at bottom -->
              <div
                class="app-card-accent"
                style="background: {app.color || 'var(--accent-primary)'};"
              ></div>
            </button>
          {/each}
        </div>
      </section>
    {/each}

    <!-- Empty state -->
    {#if apps.length === 0}
      <div class="flex flex-col items-center justify-center py-16 text-center">
        <div class="w-16 h-16 mb-6 rounded-2xl flex items-center justify-center"
             style="background: var(--bg-surface); border: 1px solid var(--border-subtle);">
          <svg class="w-8 h-8" style="color: var(--text-muted);" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
                  d="M4 5a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM14 5a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1V5zM4 15a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1H5a1 1 0 01-1-1v-4zM14 15a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" />
          </svg>
        </div>
        <h3 class="font-display text-lg font-medium mb-2" style="color: var(--text-primary);">
          No applications yet
        </h3>
        <p class="text-sm mb-6 max-w-xs" style="color: var(--text-muted);">
          Add your first application to get started with your dashboard.
        </p>
        <button
          class="btn btn-primary"
          onclick={() => onsettings?.()}
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          Add Application
        </button>
      </div>
    {/if}

    <!-- Footer with quick stats -->
    {#if apps.length > 0}
      <footer class="mt-8 pt-6 text-center" style="border-top: 1px solid var(--border-subtle);">
        <div class="flex items-center justify-center gap-6 text-xs" style="color: var(--text-muted);">
          <span class="flex items-center gap-1.5">
            <span class="w-2 h-2 rounded-full" style="background: var(--status-success);"></span>
            <span class="tabular-nums">{apps.filter(a => a.enabled).length}</span> active
          </span>
          <span class="flex items-center gap-1.5">
            <span class="tabular-nums">{groups.length}</span> {groups.length === 1 ? 'group' : 'groups'}
          </span>
          <button
            class="flex items-center gap-1.5 hover:text-[var(--text-secondary)] transition-colors"
            onclick={() => onsettings?.()}
          >
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
            Settings
          </button>
        </div>
      </footer>
    {/if}
  </div>
</div>

<style>
  /* Ensure animations work properly */
  .app-card.animate-slide-up {
    animation: slideUp 0.35s ease-out forwards;
  }

  @keyframes slideUp {
    from {
      opacity: 0;
      transform: translateY(12px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>

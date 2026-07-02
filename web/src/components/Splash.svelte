<script lang="ts">
  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import type { App, Config, DockerState } from '$lib/types';
  import AppIcon from './AppIcon.svelte';
  import HealthIndicator from './HealthIndicator.svelte';
  import MuximuxLogo from './MuximuxLogo.svelte';
  import DockerLogo from './DockerLogo.svelte';
  import DockerStatePill from './DockerStatePill.svelte';
  import ConfirmDockerActionModal from './ConfirmDockerActionModal.svelte';
  import { dockerStateStore, refreshDockerState } from '$lib/dockerStateStore';
  import { currentUser } from '$lib/authStore';
  import { dockerStart, dockerStop, dockerRestart } from '$lib/api';
  import { toasts } from '$lib/toastStore';
  import { keybindings, formatKeybinding } from '$lib/keybindingsStore';
  import * as m from '$lib/paraglide/messages.js';

  let { apps, config, showHealth = true, onselect, onsettings, onabout }: {
    apps: App[];
    config: Config;
    showHealth?: boolean;
    onselect?: (app: App) => void;
    onsettings?: () => void;
    onabout?: () => void;
  } = $props();

  let mounted = $state(false);

  // Collapsible group state — persisted to localStorage
  let collapsedGroups = $state<Record<string, boolean>>({});

  // Pending stop/restart awaiting confirmation in the modal.
  let pendingAction = $state<{ app: App; action: 'stop' | 'restart' } | null>(null);
  // True while the confirmed docker action is running, so the confirm modal
  // stays open in a disabled/spinner state until it resolves.
  let pendingActionRunning = $state(false);

  // Lifecycle actions that make sense for the current container state.
  // running -> stop/restart; stopped -> start; transitional/missing ->
  // none (nothing safe to offer). Mirrors the backend gate's intent.
  function allowedActions(ds: DockerState | undefined): ('start' | 'stop' | 'restart')[] {
    switch (ds?.status) {
      case 'running':
        return ['stop', 'restart'];
      case 'exited':
      case 'dead':
        return ['start'];
      default:
        return [];
    }
  }

  function actionLabel(action: 'start' | 'stop' | 'restart'): string {
    return action === 'start'
      ? m.docker_popover_action_start()
      : action === 'stop'
        ? m.docker_popover_action_stop()
        : m.docker_popover_action_restart();
  }

  // health_badge_placement: 'off' suppresses all Docker chrome on the
  // overview (the status cluster and the action footer). 'overview',
  // 'overview_and_nav', and the unset default all show it. The navbar
  // has its own gate (navBadgesOn, which requires 'overview_and_nav').
  let dockerChromeOn = $derived(config?.discovery?.docker?.health_badge_placement !== 'off');

  onMount(() => {
    // Trigger staggered animations after mount
    mounted = true;
    const stored = localStorage.getItem('muximux_splash_groups');
    if (stored) {
      try { collapsedGroups = JSON.parse(stored); } catch { /* ignore */ }
    }
    // Seed the Docker state map from the backend snapshot. After this
    // the WebSocket docker_state_changed event keeps it current.
    void refreshDockerState();
  });

  // start fires immediately; stop/restart route through the confirm
  // modal first (the backend re-checks the lifecycle gate regardless).
  async function fireAction(app: App, action: 'start' | 'stop' | 'restart') {
    if (action === 'start') {
      await runAction(app, action);
      return;
    }
    pendingAction = { app, action };
  }

  async function runAction(app: App, action: 'start' | 'stop' | 'restart') {
    const fn = action === 'start' ? dockerStart : action === 'stop' ? dockerStop : dockerRestart;
    let res;
    try {
      res = await fn(app.name);
    } catch (e) {
      // Backstop: the api layer is written not to throw, but guard here
      // too so an unexpected rejection still surfaces as a toast rather
      // than a silent unhandled rejection.
      toasts.error(`Failed to ${action} ${app.name}: ${e instanceof Error ? e.message : 'unknown error'}`);
      return;
    }
    if (res.error) {
      toasts.error(`Failed to ${action} ${app.name}: ${res.error}`);
    } else {
      const verb = action === 'start' ? 'Started' : action === 'stop' ? 'Stopped' : 'Restarted';
      toasts.success(`${verb} ${app.name} (${res.latency_ms}ms)`);
    }
  }

  function toggleGroupCollapse(group: string) {
    collapsedGroups[group] = !collapsedGroups[group];
    localStorage.setItem('muximux_splash_groups', JSON.stringify(collapsedGroups));
  }

  // Group apps by their group. Disabled apps are skipped here so
  // the splash tile grid stays in sync with the nav even when the
  // upstream data includes disabled entries (admins receive them
  // for editing in Settings; they shouldn't show up here).
  let groupedApps = $derived(apps.reduce((acc, app) => {
    if (!app.enabled) return acc;
    const group = app.group || 'Ungrouped';
    if (!acc[group]) acc[group] = [];
    acc[group].push(app);
    return acc;
  }, {} as Record<string, App[]>));

  let sortedGroupedApps = $derived.by(() => {
    const result: Record<string, App[]> = {};
    for (const [group, appList] of Object.entries(groupedApps)) {
      result[group] = [...appList].sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
    }
    return result;
  });

  let groups = $derived(
    Object.keys(sortedGroupedApps).sort((a, b) => {
      if (a === 'Ungrouped') return 1;
      if (b === 'Ungrouped') return -1;
      const groupA = config.groups.find(g => g.name === a);
      const groupB = config.groups.find(g => g.name === b);
      return (groupA?.order ?? 0) - (groupB?.order ?? 0);
    })
  );

  function getOpenModeIcon(mode: string): string {
    switch (mode) {
      case 'new_tab': return '\u2197';
      case 'new_window': return '\u29C9';
      default: return '';
    }
  }

  // Returns the keyboard shortcut (1-9) assigned to this app, if any.
  // Only explicitly assigned shortcuts are shown — there is no positional fallback.
  function getDisplayKey(app: App): number | undefined {
    return app.shortcut || undefined;
  }

  // Dynamic shortcut labels from keybindings store
  let searchLabel = $derived(formatKeybinding($keybindings.find(b => b.action === 'search')!));
  let shortcutsLabel = $derived(formatKeybinding($keybindings.find(b => b.action === 'shortcuts')!));

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
        {m.splash_selectApp()}
      </p>

      <!-- Quick keyboard hints -->
      <div class="mt-4 flex items-center justify-center gap-4 text-xs" style="color: var(--text-muted);">
        <span class="flex items-center gap-1.5">
          <kbd class="kbd">{searchLabel}</kbd>
          <span class="ms-1">{m.common_search()}</span>
        </span>
        <span class="flex items-center gap-1.5">
          <kbd class="kbd">1</kbd>
          <span>-</span>
          <kbd class="kbd">9</kbd>
          <span class="ms-1">{m.splash_quickAccess()}</span>
        </span>
        <span class="flex items-center gap-1.5">
          <kbd class="kbd">{shortcutsLabel}</kbd>
          <span class="ms-1">{m.splash_allShortcuts()}</span>
        </span>
      </div>
    </header>

    <!-- App grid by groups -->
    {#each groups as group, groupIndex (group)}
      <section class="mb-8 md:mb-10">
        <!-- Group header -->
        {#if groups.length > 1 || group !== 'Ungrouped'}
          <button
            class="w-full flex items-center gap-3 mb-4 cursor-pointer"
            onclick={() => toggleGroupCollapse(group)}
          >
            <svg
              class="w-3.5 h-3.5 flex-shrink-0 transition-transform duration-200"
              style="color: var(--text-muted); transform: rotate({collapsedGroups[group] ? '-90deg' : '0deg'});"
              fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
            >
              <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
            </svg>
            <h2 class="font-display text-xs font-semibold tracking-widest uppercase"
                style="color: var(--text-muted);">
              {group}
            </h2>
            <div class="flex-1 h-px" style="background: var(--border-subtle);"></div>
            <span class="text-xs tabular-nums" style="color: var(--text-disabled);">
              {sortedGroupedApps[group].length} {sortedGroupedApps[group].length === 1 ? m.splash_appSingular() : m.splash_appPlural()}
            </span>
          </button>
        {/if}

        <!-- App cards grid -->
        {#if !collapsedGroups[group]}
          <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3 md:gap-4">
            {#each sortedGroupedApps[group] as app, appIndex (app.name)}
              {@const displayKey = getDisplayKey(app)}
              {@const ds = app.docker_key ? $dockerStateStore.get(app.name) : undefined}
              {@const canLifecycle = !!app.docker_key && !!$currentUser?.can_use_docker_lifecycle}
              <!-- Relative wrapper anchors the docker control cluster, which
                   lives as a sibling of the card button (not a child) so we
                   never nest a <button> inside a <button>. -->
              <div class="app-card-wrapper relative">
                <button
                  class="app-card group opacity-0"
                  class:animate-slide-up={mounted}
                  class:exited={ds?.status === 'exited' || ds?.status === 'dead'}
                  style="animation-delay: {getStaggerDelay(groupIndex, appIndex)};"
                  onclick={() => onselect?.(app)}
                >
                  {#if app.docker_key}
                    <!-- Passive Docker status indicator (all users): logo ->
                         status dot -> HTTP health dot. The lifecycle action
                         buttons live in a separate footer BELOW the card so
                         they never overlap the open-app tap target. Hidden
                         entirely when health_badge_placement is 'off'. -->
                    {#if dockerChromeOn}
                      <div class="docker-cluster absolute top-2.5 end-2.5 z-10 flex items-center gap-1">
                        <DockerLogo size="sm" class="text-slate-500" />
                        {#if ds}
                          <DockerStatePill state={ds} />
                        {/if}
                        {#if showHealth && app.health_check === true}
                          <HealthIndicator appName={app.name} size="sm" />
                        {/if}
                      </div>
                    {/if}
                  {:else if showHealth && app.health_check === true}
                    <!-- Health indicator - per-app control (non-Docker apps) -->
                    <div class="absolute top-2.5 end-2.5 z-10">
                      <HealthIndicator appName={app.name} size="sm" />
                    </div>
                  {/if}

                  <!-- Keyboard shortcut badge (1-9) -->
                  {#if displayKey !== undefined}
                    <div class="absolute top-2.5 start-2.5 z-10">
                      <span class="kbd">
                        {displayKey}
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
                    <div class="relative app-icon-wrapper">
                      <AppIcon icon={app.icon} name={app.name} color={app.color} size="xl" />
                    </div>
                  </div>

                  <!-- App name -->
                  <span class="text-sm font-medium text-center transition-colors"
                        style="color: var(--text-secondary);">
                    <span class="group-hover:text-[var(--text-primary)]">{app.name}</span>
                    {#if app.open_mode !== 'iframe'}
                      <span class="ms-1 text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
                    {/if}
                  </span>

                  <!-- Color accent bar at bottom -->
                  <div
                    class="app-card-accent"
                    style="background: {app.color || 'var(--accent-primary)'};"
                  ></div>
                </button>

                {#if canLifecycle && dockerChromeOn && ds && allowedActions(ds).length > 0}
                  <!-- Action footer BELOW the card, outside the open-app
                       button's box, so tapping to open the app can never hit
                       a lifecycle action by accident. On touch it stays
                       visible; on pointer devices it reveals on hover/focus
                       (see the @media block in <style>). -->
                  <div class="docker-control-footer" role="group" aria-label="Container actions for {app.name}">
                    {#each allowedActions(ds) as action (action)}
                      <button
                        class="docker-action-btn"
                        class:start={action === 'start'}
                        type="button"
                        aria-label={actionLabel(action)}
                        title={actionLabel(action)}
                        onclick={(e) => { e.stopPropagation(); fireAction(app, action); }}
                      >
                        {#if action === 'start'}
                          <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M8 5v14l11-7z" /></svg>
                        {:else if action === 'stop'}
                          <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><rect x="6" y="6" width="12" height="12" rx="1.5" /></svg>
                        {:else}
                          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-2.64-6.36" /><path d="M21 3v6h-6" /></svg>
                        {/if}
                      </button>
                    {/each}
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
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
          {m.splash_noAppsTitle()}
        </h3>
        <p class="text-sm mb-6 max-w-xs" style="color: var(--text-muted);">
          {m.splash_noAppsDesc()}
        </p>
        {#if onsettings}
          <button
            class="btn btn-primary"
            onclick={() => onsettings?.()}
          >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
            {m.splash_addApplication()}
          </button>
        {/if}
      </div>
    {/if}

    <!-- Footer with quick stats -->
    {#if apps.length > 0}
      <footer class="mt-8 pt-6 text-center" style="border-top: 1px solid var(--border-subtle);">
        <div class="flex items-center justify-center gap-6 text-xs" style="color: var(--text-muted);">
          <span class="flex items-center gap-1.5">
            <span class="w-2 h-2 rounded-full" style="background: var(--status-success);"></span>
            <span class="tabular-nums">{apps.filter(a => a.enabled).length}</span> {m.splash_active()}
          </span>
          <span class="flex items-center gap-1.5">
            <span class="tabular-nums">{groups.length}</span> {groups.length === 1 ? m.splash_groupSingular() : m.splash_groupPlural()}
          </span>
          {#if onsettings}
            <button
              class="flex items-center gap-1.5 hover:text-[var(--text-secondary)] transition-colors"
              onclick={() => onsettings?.()}
            >
              <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                      d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
              </svg>
              {m.nav_settings()}
            </button>
          {/if}
          <button
            class="flex items-center gap-1.5 hover:text-[var(--text-secondary)] transition-colors"
            onclick={() => onabout?.()}
          >
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            {m.settings_about()}
          </button>
        </div>
      </footer>
    {/if}
  </div>
</div>

{#if pendingAction}
  {@const ds = $dockerStateStore.get(pendingAction.app.name)}
  <ConfirmDockerActionModal
    appName={pendingAction.app.name}
    action={pendingAction.action}
    image={ds?.image ?? ''}
    uptimeOrExit={ds?.status === 'running' ? 'running' : `exit ${ds?.exit_code ?? 0}`}
    loading={pendingActionRunning}
    onconfirm={async () => {
      if (pendingActionRunning) return;
      const a = pendingAction!;
      // Keep the modal open (disabled) until the action resolves, then
      // close. Closing before the await would drop all in-flight feedback
      // and let the operator re-fire a long restart.
      pendingActionRunning = true;
      try {
        await runAction(a.app, a.action);
      } finally {
        pendingActionRunning = false;
        pendingAction = null;
      }
    }}
    oncancel={() => { if (!pendingActionRunning) pendingAction = null; }}
  />
{/if}

<style>
  /* Ensure animations work properly */
  .app-card.animate-slide-up {
    animation: slideUp 0.35s ease-out forwards;
  }

  /* Dim + desaturate the icon when the container is stopped/dead so
     the card reads as "not running" at a glance. */
  .app-card.exited :global(.app-icon-wrapper) {
    filter: grayscale(0.6);
    opacity: 0.65;
  }
  /* Shrink the wrapper to the card's own width. A <button> sizes to its
     content (it ignores flex/block stretch), so the wrapper was filling
     the whole grid cell while the card sat narrower at the start. Fitting
     the wrapper to the card lets the action footer span the card's real
     width and stay centred under it. */
  .app-card-wrapper {
    width: fit-content;
  }
  /* Action footer: a strip of lifecycle buttons BELOW the card, never
     overlapping the open-app button. Touch-first -- visible by default so
     it's reachable on phones (no hover) and can't be confused with the
     open-app tap because it sits outside the card. */
  .docker-control-footer {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    padding-top: 0.375rem;
  }
  /* On pointer-capable devices, lift the footer out of flow and reveal it
     just below the card on hover/focus -- keeps the resting grid clean and
     avoids reflow. Touch devices skip this block and keep it in flow. */
  @media (hover: hover) and (pointer: fine) {
    .docker-control-footer {
      position: absolute;
      top: 100%;
      inset-inline: 0;
      z-index: 12;
      opacity: 0;
      transform: translateY(-0.25rem);
      pointer-events: none;
      transition: opacity 0.14s ease, transform 0.14s ease;
    }
    .app-card-wrapper:hover .docker-control-footer,
    .app-card-wrapper:focus-within .docker-control-footer {
      opacity: 1;
      transform: translateY(0);
      pointer-events: auto;
    }
  }
  .docker-action-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.75rem;
    height: 1.75rem;
    padding: 0;
    border: 1px solid var(--border-subtle, transparent);
    border-radius: 0.5rem;
    background: var(--bg-elevated);
    color: var(--text-secondary);
    cursor: pointer;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.25);
    transition: color 0.12s ease, background 0.12s ease;
  }
  .docker-action-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  .docker-action-btn.start:hover {
    color: var(--status-success, #22c55e);
  }
  .docker-action-btn svg {
    width: 0.875rem;
    height: 0.875rem;
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

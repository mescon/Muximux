<script lang="ts">
  import { onMount } from 'svelte';
  import { flip } from 'svelte/animate';
  import { type App, type Group, type DiscoveryDockerStatus, stampAppId } from '$lib/types';
  import AppIcon from '../AppIcon.svelte';
  import { dndzone, type DndEvent } from 'svelte-dnd-action';
  import * as m from '$lib/paraglide/messages.js';
  import { fetchDiscoveryDockerStatus } from '$lib/api';
  import { moveItem } from '$lib/reorder';
  import { announce } from '$lib/announce';

  let {
    dndGroups = $bindable(),
    dndGroupedApps = $bindable(),
    localAppsCount,
    localGroupsCount,
    onstartEditApp,
    onstartEditGroup,
    onshowAddApp,
    onshowAddGroup,
    onsyncGroupOrder,
    onsyncAppOrder,
    ondiscoveryconfigure,
    ondiscoveryscan,
  }: {
    dndGroups: Group[];
    dndGroupedApps: Record<string, App[]>;
    localAppsCount: number;
    localGroupsCount: number;
    onstartEditApp: (app: App) => void;
    onstartEditGroup: (group: Group) => void;
    onshowAddApp: () => void;
    onshowAddGroup: () => void;
    onsyncGroupOrder: (groups: Group[]) => void;
    onsyncAppOrder: (groupName: string, items: App[]) => void;
    /** Called when the operator clicks the Discover button while it's in
     *  CTA / disabled state — navigates them to Settings → Discovery. */
    ondiscoveryconfigure?: () => void;
    /** Called when the button is active and clicked — opens the
     *  Discover modal in 'apps' mode. */
    ondiscoveryscan?: () => void;
  } = $props();

  // Capability state for the "Discover from Docker" button. Loaded
  // once on mount; the button stays mounted with one of four
  // appearances per the docker-discovery plan's gating ladder:
  //   - !configured       -> CTA: "Set up Docker discovery →" (link)
  //   - !reachable        -> disabled with tooltip
  //   - !strategy_ok      -> disabled with tooltip
  //   - configured + reachable + strategy_ok -> active (clicking
  //     opens the Discover modal)
  let discoveryStatus = $state<DiscoveryDockerStatus | null>(null);
  onMount(async () => {
    try {
      discoveryStatus = await fetchDiscoveryDockerStatus();
    } catch (e) {
      // The discovery status endpoint is admin-only, so a 403 here
      // is expected for non-admin sessions and shouldn't surface
      // anywhere. Other failures (5xx, network) DO matter - they
      // silently hide the "Discover from Docker" button without
      // any signal to the admin. Log to the console so the operator
      // can find the cause; non-admin 403s stay quiet.
      const msg = e instanceof Error ? e.message : String(e);
      if (!/403|forbidden|unauthor/i.test(msg)) {
        console.warn('discovery status fetch failed:', msg);
      }
    }
  });

  let discoveryButtonState = $derived.by(() => {
    if (!discoveryStatus) return 'hidden' as const;
    if (!discoveryStatus.configured) return 'cta' as const;
    if (!discoveryStatus.reachable) return 'unreachable' as const;
    if (!discoveryStatus.strategy_ok) return 'strategy_blocked' as const;
    return 'active' as const;
  });

  let discoveryTooltip = $derived.by(() => {
    if (!discoveryStatus) return '';
    switch (discoveryButtonState) {
      case 'cta':              return 'Configure a Docker daemon endpoint in Settings → Discovery to enable.';
      case 'unreachable':      return `Docker daemon unreachable: ${discoveryStatus.last_error ?? 'see Settings → Discovery'}`;
      case 'strategy_blocked': return `Strategy "${discoveryStatus.strategy}" cannot identify Muximux's container. Set network_filter or switch to host_port in Settings → Discovery.`;
      case 'active':           return 'Scan the configured Docker daemon for containers and add them as apps.';
      default:                 return '';
    }
  });

  // Drag and drop config
  const flipDurationMs = 200;

  // Delete confirmation state (owned by this component)
  let confirmDeleteApp = $state<App | null>(null);
  let confirmDeleteGroup = $state<Group | null>(null);

  function handleDeleteApp(app: App) {
    confirmDeleteApp = app;
  }

  function handleDeleteGroup(group: Group) {
    confirmDeleteGroup = group;
  }

  function confirmDeleteAppAction() {
    if (confirmDeleteApp) {
      // Remove from all dndGroupedApps entries
      for (const groupName of Object.keys(dndGroupedApps)) {
        dndGroupedApps[groupName] = dndGroupedApps[groupName].filter(
          a => a.name !== confirmDeleteApp!.name
        );
      }
      confirmDeleteApp = null;
      // Sync deletion back to parent
      onsyncAppOrder('__delete__', []);
    }
  }

  function confirmDeleteGroupAction() {
    if (confirmDeleteGroup) {
      dndGroups = dndGroups.filter(g => g.name !== confirmDeleteGroup!.name);
      // Move apps from deleted group to ungrouped
      const orphaned = dndGroupedApps[confirmDeleteGroup.name] || [];
      if (orphaned.length > 0) {
        const ungrouped = dndGroupedApps[''] || [];
        orphaned.forEach(a => { a.group = ''; });
        dndGroupedApps[''] = [...ungrouped, ...orphaned];
      }
      delete dndGroupedApps[confirmDeleteGroup.name];
      confirmDeleteGroup = null;
      // Sync deletion back to parent
      onsyncGroupOrder(dndGroups);
      onsyncAppOrder('__rebuild__', []);
    }
  }

  // DnD handlers for groups
  function handleGroupDndConsider(e: CustomEvent<DndEvent<Group>>) {
    dndGroups = e.detail.items;
  }
  function handleGroupDndFinalize(e: CustomEvent<DndEvent<Group>>) {
    dndGroups = e.detail.items;
    dndGroups.forEach((g, i) => { g.order = i; });
    onsyncGroupOrder(dndGroups);
  }

  // DnD handlers for apps within a group
  function handleAppDndConsider(e: CustomEvent<DndEvent<App>>, groupName: string) {
    dndGroupedApps[groupName] = e.detail.items;
  }
  function handleAppDndFinalize(e: CustomEvent<DndEvent<App>>, groupName: string) {
    const newItems = e.detail.items;
    newItems.forEach((a, i) => { a.group = groupName; a.order = i; stampAppId(a); });
    dndGroupedApps[groupName] = newItems;
    onsyncAppOrder(groupName, newItems);
  }

  // Keyboard reordering: the same result as a drag, driven by the up/down
  // buttons on each row so keyboard-only and pointer-averse users can
  // reorder too. moveItem returns the original array unchanged at a bound,
  // which we treat as a silent no-op (the button is aria-disabled there).
  function moveGroup(index: number, direction: -1 | 1) {
    const next = moveItem(dndGroups, index, direction);
    if (next === dndGroups) return;
    const moved = next[index + direction];
    next.forEach((g, i) => { g.order = i; });
    dndGroups = next;
    onsyncGroupOrder(dndGroups);
    announce(m.apps_movedTo({ name: moved.name, position: `${index + direction + 1}`, total: `${next.length}` }));
  }

  function moveApp(groupName: string, index: number, direction: -1 | 1) {
    const list = dndGroupedApps[groupName] || [];
    const next = moveItem(list, index, direction);
    if (next === list) return;
    const moved = next[index + direction];
    next.forEach((a, i) => { a.group = groupName; a.order = i; stampAppId(a); });
    dndGroupedApps[groupName] = next;
    onsyncAppOrder(groupName, next);
    announce(m.apps_movedTo({ name: moved.name, position: `${index + direction + 1}`, total: `${next.length}` }));
  }
</script>

{#snippet moveButtons(upLabel: string, downLabel: string, atTop: boolean, atBottom: boolean, onUp: () => void, onDown: () => void)}
  <button
    type="button"
    class="btn btn-ghost btn-icon btn-sm"
    class:opacity-30={atTop}
    class:cursor-not-allowed={atTop}
    aria-label={upLabel}
    aria-disabled={atTop}
    data-testid="move-up"
    onclick={() => { if (!atTop) onUp(); }}
  >
    <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
    </svg>
  </button>
  <button
    type="button"
    class="btn btn-ghost btn-icon btn-sm"
    class:opacity-30={atBottom}
    class:cursor-not-allowed={atBottom}
    aria-label={downLabel}
    aria-disabled={atBottom}
    data-testid="move-down"
    onclick={() => { if (!atBottom) onDown(); }}
  >
    <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
    </svg>
  </button>
{/snippet}

{#snippet appRowContent(app: App, groupName: string, index: number, total: number)}
  <!-- Drag handle -->
  <div class="flex-shrink-0 text-text-disabled hover:text-text-muted">
    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
    </svg>
  </div>
  <div class="flex-shrink-0">
    <AppIcon icon={app.icon} name={app.name} color={app.color} size="md" />
  </div>
  <div class="flex-1 min-w-0">
    <div class="flex items-center gap-2 flex-wrap">
      <span class="font-medium text-text-primary text-sm truncate">{app.name}</span>
      {#if app.default}
        <span class="text-xs bg-brand-500/20 text-brand-400 px-1.5 py-0.5 rounded">{m.common_default()}</span>
      {/if}
      {#if !app.enabled}
        <span class="text-xs bg-bg-overlay text-text-muted px-1.5 py-0.5 rounded">{m.common_disabled()}</span>
      {/if}
      {#if app.proxy}
        <span class="app-indicator" title={m.apps_proxyTitle()}>
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>
        </span>
      {/if}
      {#if app.open_mode && app.open_mode !== 'iframe'}
        <span class="app-indicator" title={m.apps_opensIn({ mode: app.open_mode === 'new_tab' ? m.apps_newTab() : app.open_mode === 'new_window' ? m.apps_newWindow() : app.open_mode.replace('_', ' ') })}>
          {#if app.open_mode === 'new_tab'}
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
          {:else if app.open_mode === 'new_window'}
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
          {:else}
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M17 8l4 4m0 0l-4 4m4-4H3" /></svg>
          {/if}
        </span>
      {/if}
      {#if app.scale && app.scale !== 1}
        <span class="app-indicator" title="Scaled to {Math.round(app.scale * 100)}%">
          {Math.round(app.scale * 100)}%
        </span>
      {/if}
      {#if app.docker_key}
        <span
          class="app-indicator text-blue-400"
          title="Auto-managed by Docker discovery. URL refreshes from container {app.docker_key}. Detach via Settings → Discovery → Currently tracked."
          data-testid="docker-managed-badge"
          aria-label="Docker-managed"
        >
          <!-- Docker whale glyph (simplified). The shape distinguishes
               docker-managed apps at a glance without competing with
               the Lucide icon set used elsewhere. -->
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path d="M22.5 11.5h-2.7c-.1-1.4-1.2-2.5-2.6-2.5-.4 0-.8.1-1.2.3V6.4c0-.2-.2-.4-.4-.4h-2.4c-.2 0-.4.2-.4.4v2.9h-1V4.4c0-.2-.2-.4-.4-.4H8.9c-.2 0-.4.2-.4.4v4.9h-1V6.4c0-.2-.2-.4-.4-.4H4.7c-.2 0-.4.2-.4.4v2.9H1.5c-.3 0-.5.2-.5.5 0 1.6.4 3.1 1.1 4.4.7 1.4 1.7 2.4 2.9 3 .3.1.6.3.9.4 1.4.5 2.9.7 4.4.7 1.7 0 3.4-.3 5-.9 1.6-.6 2.9-1.5 3.9-2.7.6-.7 1.1-1.5 1.5-2.4h.3c1 0 1.8-.7 2-1.6.1-.3.1-.5.1-.8 0-.1-.1-.2-.2-.3-.1 0-.2-.1-.3-.1z"/>
          </svg>
        </span>
      {/if}
    </div>
    <span class="text-xs text-text-muted truncate block">{app.url}</span>
  </div>
  <!-- App actions -->
  {#if confirmDeleteApp?.name === app.name}
    <div class="flex items-center gap-1">
      <span class="text-xs text-red-400 me-1">{m.common_deleteConfirm()}</span>
      <button class="btn btn-danger btn-sm"
              onclick={confirmDeleteAppAction}>{m.common_yes()}</button>
      <button class="btn btn-secondary btn-sm"
              onclick={() => confirmDeleteApp = null}>{m.common_no()}</button>
    </div>
  {:else}
    <div class="flex items-center gap-1 app-actions">
      {@render moveButtons(
        m.apps_moveUp({ name: app.name }),
        m.apps_moveDown({ name: app.name }),
        index === 0,
        index === total - 1,
        () => moveApp(groupName, index, -1),
        () => moveApp(groupName, index, 1)
      )}
      <button class="btn btn-ghost btn-icon btn-sm"
              onclick={() => onstartEditApp(app)} title={m.common_edit()}>
        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
        </svg>
      </button>
      <button class="btn btn-ghost btn-icon btn-sm hover:!text-red-400"
              onclick={() => handleDeleteApp(app)} title={m.common_delete()}>
        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
        </svg>
      </button>
    </div>
  {/if}
{/snippet}

<div class="space-y-4">
  <!-- Action buttons -->
  <div class="flex justify-between items-center">
    <h3 class="text-sm font-medium text-text-secondary">{m.apps_heading()}</h3>
    <div class="flex gap-2">
      {#if discoveryButtonState !== 'hidden'}
        <button
          class="btn btn-sm flex items-center gap-1
                 {discoveryButtonState === 'active' ? 'btn-secondary' : ''}
                 {discoveryButtonState === 'cta' ? 'btn-secondary' : ''}
                 {discoveryButtonState === 'unreachable' || discoveryButtonState === 'strategy_blocked' ? 'btn-secondary opacity-50 cursor-not-allowed' : ''}"
          onclick={() => {
            if (discoveryButtonState === 'active') ondiscoveryscan?.();
            else ondiscoveryconfigure?.();
          }}
          disabled={discoveryButtonState === 'unreachable' || discoveryButtonState === 'strategy_blocked'}
          title={discoveryTooltip}
          type="button"
        >
          <!-- Docker whale (simplified) -->
          <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
            <path d="M22.6 8.5h-3.7v-3h-3v3h-3v-3h-3v3h-3v-3h-3v3H1.4v3h.7c.7 1 1.6 1.7 2.7 2.1.5.2 1 .3 1.5.3h12.4c2.2 0 4.1-1 5.5-2.7-.6-.2-1.1-.3-1.6-.3z"/>
          </svg>
          {#if discoveryButtonState === 'cta'}
            Set up Docker discovery →
          {:else if discoveryButtonState === 'unreachable'}
            Docker discovery unreachable
          {:else if discoveryButtonState === 'strategy_blocked'}
            Docker discovery: configure strategy
          {:else}
            Discover from Docker
          {/if}
        </button>
      {/if}
      <button
        class="btn btn-secondary btn-sm flex items-center gap-1"
        onclick={onshowAddGroup}
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 14v6m-3-3h6M6 10h2a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v2a2 2 0 002 2zm10 0h2a2 2 0 002-2V6a2 2 0 00-2-2h-2a2 2 0 00-2 2v2a2 2 0 002 2zM6 20h2a2 2 0 002-2v-2a2 2 0 00-2-2H6a2 2 0 00-2 2v2a2 2 0 002 2z" />
        </svg>
        {m.apps_addGroup()}
      </button>
      <button
        class="btn btn-primary btn-sm flex items-center gap-1"
        onclick={onshowAddApp}
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        {m.apps_addApp()}
      </button>
    </div>
  </div>

  <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-text-disabled">
    <span>{m.apps_dragHelp()} {m.apps_reorderHelp()}</span>
    <span class="flex items-center gap-3 text-text-disabled">
      <span class="flex items-center gap-1"><span class="app-indicator"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg></span> {m.apps_proxy()}</span>
      <span class="flex items-center gap-1"><span class="app-indicator"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg></span> {m.apps_newTab()}</span>
      <span class="flex items-center gap-1"><span class="app-indicator"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg></span> {m.apps_newWindow()}</span>
      <span class="flex items-center gap-1"><span class="app-indicator">50%</span> {m.apps_scale()}</span>
      <span class="flex items-center gap-1"><span class="app-indicator">&#x2328;</span> {m.apps_keyboard()}</span>
    </span>
  </div>

  <!-- Groups with their apps (dnd-zone for group reordering) -->
  <div class="space-y-3" use:dndzone={{items: dndGroups, flipDurationMs, type: 'groups', dropTargetStyle: {}}} onconsider={handleGroupDndConsider} onfinalize={handleGroupDndFinalize}>
    {#each dndGroups as group, groupIndex ((group as Group & Record<string, unknown>).id)}
      {@const appsInGroup = dndGroupedApps[group.name] || []}
      <div class="rounded-lg border border-border" animate:flip={{duration: flipDurationMs}}>
        <!-- Group header -->
        <div class="flex items-center gap-3 p-3 bg-bg-elevated/30 rounded-t-lg cursor-grab active:cursor-grabbing">
          <!-- Drag handle -->
          <div class="flex-shrink-0 text-text-disabled hover:text-text-secondary">
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
            </svg>
          </div>

          <!-- Group icon -->
          <div class="flex-shrink-0">
            {#if group.icon?.name}
              <AppIcon icon={group.icon} name={group.name} color={group.color || '#374151'} size="sm" showBackground={true} />
            {:else}
              <span class="w-6 h-6 rounded flex-shrink-0 block" style="background-color: {group.color || '#374151'}"></span>
            {/if}
          </div>

          <!-- Group info -->
          <div class="flex-1 min-w-0">
            <span class="font-medium text-text-primary text-sm">{group.name}</span>
            <span class="text-xs text-text-disabled ms-2">{m.apps_appCount({ count: `${appsInGroup.length}` })}</span>
          </div>

          <!-- Group actions -->
          {#if confirmDeleteGroup?.name === group.name}
            <div class="flex items-center gap-1">
              <span class="text-xs text-red-400 me-1">{m.common_deleteConfirm()}</span>
              <button class="btn btn-danger btn-sm"
                      onclick={confirmDeleteGroupAction}>{m.common_yes()}</button>
              <button class="btn btn-secondary btn-sm"
                      onclick={() => confirmDeleteGroup = null}>{m.common_no()}</button>
            </div>
          {:else}
            <div class="flex items-center gap-1 app-actions">
              {@render moveButtons(
                m.apps_moveUp({ name: group.name }),
                m.apps_moveDown({ name: group.name }),
                groupIndex === 0,
                groupIndex === dndGroups.length - 1,
                () => moveGroup(groupIndex, -1),
                () => moveGroup(groupIndex, 1)
              )}
              <button class="btn btn-ghost btn-icon btn-sm"
                      onclick={() => onstartEditGroup(group)} title={m.apps_editGroup()}>
                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button class="btn btn-ghost btn-icon btn-sm hover:!text-red-400"
                      onclick={() => handleDeleteGroup(group)} title={m.apps_deleteGroup()}>
                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </div>
          {/if}
        </div>

        <!-- Apps in this group (dnd-zone for app reordering + cross-group) -->
        <div class="p-2 space-y-1 min-h-[36px]" use:dndzone={{items: appsInGroup, flipDurationMs, type: 'apps', dropTargetStyle: {}}} onconsider={(e) => handleAppDndConsider(e, group.name)} onfinalize={(e) => handleAppDndFinalize(e, group.name)}>
          {#if appsInGroup.length === 0}
            <div class="text-center py-3 text-text-disabled text-sm italic">{m.apps_noAppsInGroup()}</div>
          {/if}
          {#each appsInGroup as app, appIndex ((app as App & Record<string, unknown>).id)}
            <div
              class="flex items-center gap-3 p-2 rounded-md group/app hover:bg-bg-hover/30 cursor-grab active:cursor-grabbing"
              animate:flip={{duration: flipDurationMs}}
            >
              {@render appRowContent(app, group.name, appIndex, appsInGroup.length)}
            </div>
          {/each}
        </div>
      </div>
    {/each}
  </div>

  <!-- Ungrouped apps -->
  {#if (dndGroupedApps[''] || []).length > 0 || localGroupsCount > 0}
    {@const ungroupedApps = dndGroupedApps[''] || []}
    <div class="rounded-lg border border-border border-dashed" class:hidden={ungroupedApps.length === 0 && localGroupsCount === 0}>
      <div class="p-3 bg-bg-elevated/20 rounded-t-lg">
        <span class="text-sm font-medium text-text-muted">{m.apps_ungrouped()}</span>
        {#if ungroupedApps.length > 0}
          <span class="text-xs text-text-disabled ms-2">{m.apps_appCount({ count: `${ungroupedApps.length}` })}</span>
        {:else}
          <span class="text-xs text-text-disabled ms-2">{m.apps_dragToUngroup()}</span>
        {/if}
      </div>
      <div class="p-2 space-y-1 min-h-[36px]" use:dndzone={{items: ungroupedApps, flipDurationMs, type: 'apps', dropTargetStyle: {}}} onconsider={(e) => handleAppDndConsider(e, '')} onfinalize={(e) => handleAppDndFinalize(e, '')}>
        {#each ungroupedApps as app, appIndex ((app as App & Record<string, unknown>).id)}
          <div
            class="flex items-center gap-3 p-2 rounded-md group/app hover:bg-bg-hover/30 cursor-grab active:cursor-grabbing"
            animate:flip={{duration: flipDurationMs}}
          >
            {@render appRowContent(app, '', appIndex, ungroupedApps.length)}
          </div>
        {/each}
      </div>
    </div>
  {/if}

  {#if localAppsCount === 0 && localGroupsCount === 0}
    <div class="text-center py-8 text-text-muted">
      {m.apps_noAppsConfigured()}
    </div>
  {/if}
</div>

<style>
  /* Action button group pill background */
  .app-actions {
    background: var(--bg-overlay, rgba(0, 0, 0, 0.4));
    border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
    border-radius: 6px;
    padding: 2px;
  }

  .app-actions svg {
    width: 1rem;
    height: 1rem;
  }
</style>

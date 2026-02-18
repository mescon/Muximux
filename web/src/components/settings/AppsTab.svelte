<script lang="ts">
  import { flip } from 'svelte/animate';
  import type { App, Group } from '$lib/types';
  import AppIcon from '../AppIcon.svelte';
  import { dndzone, type DndEvent } from 'svelte-dnd-action';

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
  } = $props();

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
    newItems.forEach((a, i) => { a.group = groupName; a.order = i; (a as App & Record<string, unknown>).id = a.name; });
    dndGroupedApps[groupName] = newItems;
    onsyncAppOrder(groupName, newItems);
  }
</script>

{#snippet appRowContent(app: App)}
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
        <span class="text-xs bg-brand-500/20 text-brand-400 px-1.5 py-0.5 rounded">Default</span>
      {/if}
      {#if !app.enabled}
        <span class="text-xs bg-bg-overlay text-text-muted px-1.5 py-0.5 rounded">Disabled</span>
      {/if}
      {#if app.proxy}
        <span class="app-indicator" title="Proxied through server">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>
        </span>
      {/if}
      {#if app.open_mode && app.open_mode !== 'iframe'}
        <span class="app-indicator" title="Opens in {app.open_mode.replace('_', ' ')}">
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
    </div>
    <span class="text-xs text-text-muted truncate block">{app.url}</span>
  </div>
  <!-- App actions -->
  {#if confirmDeleteApp?.name === app.name}
    <div class="flex items-center gap-1">
      <span class="text-xs text-red-400 mr-1">Delete?</span>
      <button class="btn btn-danger btn-sm"
              onclick={confirmDeleteAppAction}>Yes</button>
      <button class="btn btn-secondary btn-sm"
              onclick={() => confirmDeleteApp = null}>No</button>
    </div>
  {:else}
    <div class="flex items-center gap-1 opacity-0 group-hover/app:opacity-100 focus-within:opacity-100 transition-opacity app-actions">
      <button class="btn btn-ghost btn-icon btn-sm"
              tabindex="-1"
              onclick={() => onstartEditApp(app)} title="Edit">
        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
        </svg>
      </button>
      <button class="btn btn-ghost btn-icon btn-sm hover:!text-red-400"
              tabindex="-1"
              onclick={() => handleDeleteApp(app)} title="Delete">
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
    <h3 class="text-sm font-medium text-text-secondary">Apps & Groups</h3>
    <div class="flex gap-2">
      <button
        class="btn btn-secondary btn-sm flex items-center gap-1"
        onclick={onshowAddGroup}
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 14v6m-3-3h6M6 10h2a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v2a2 2 0 002 2zm10 0h2a2 2 0 002-2V6a2 2 0 00-2-2h-2a2 2 0 00-2 2v2a2 2 0 002 2zM6 20h2a2 2 0 002-2v-2a2 2 0 00-2-2H6a2 2 0 00-2 2v2a2 2 0 002 2z" />
        </svg>
        Add Group
      </button>
      <button
        class="btn btn-primary btn-sm flex items-center gap-1"
        onclick={onshowAddApp}
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        Add App
      </button>
    </div>
  </div>

  <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-text-disabled">
    <span>Drag apps to reorder or move between groups. Drag group headers to reorder groups.</span>
    <span class="flex items-center gap-3 text-text-disabled">
      <span class="flex items-center gap-1"><span class="app-indicator"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg></span> Proxy</span>
      <span class="flex items-center gap-1"><span class="app-indicator"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg></span> New tab</span>
      <span class="flex items-center gap-1"><span class="app-indicator"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg></span> New window</span>
      <span class="flex items-center gap-1"><span class="app-indicator">50%</span> Scale</span>
      <span class="flex items-center gap-1"><span class="app-indicator">&#x2328;</span> Keyboard</span>
    </span>
  </div>

  <!-- Groups with their apps (dnd-zone for group reordering) -->
  <div class="space-y-3" use:dndzone={{items: dndGroups, flipDurationMs, type: 'groups', dropTargetStyle: {}}} onconsider={handleGroupDndConsider} onfinalize={handleGroupDndFinalize}>
    {#each dndGroups as group ((group as Group & Record<string, unknown>).id)}
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
            <span class="text-xs text-text-disabled ml-2">{appsInGroup.length} apps</span>
          </div>

          <!-- Group actions -->
          {#if confirmDeleteGroup?.name === group.name}
            <div class="flex items-center gap-1">
              <span class="text-xs text-red-400 mr-1">Delete?</span>
              <button class="btn btn-danger btn-sm"
                      onclick={confirmDeleteGroupAction}>Yes</button>
              <button class="btn btn-secondary btn-sm"
                      onclick={() => confirmDeleteGroup = null}>No</button>
            </div>
          {:else}
            <div class="flex items-center gap-1 app-actions">
              <button class="btn btn-ghost btn-icon btn-sm"
                      onclick={() => onstartEditGroup(group)} title="Edit group">
                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button class="btn btn-ghost btn-icon btn-sm hover:!text-red-400"
                      onclick={() => handleDeleteGroup(group)} title="Delete group">
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
            <div class="text-center py-3 text-text-disabled text-sm italic">No apps in this group</div>
          {/if}
          {#each appsInGroup as app ((app as App & Record<string, unknown>).id)}
            <div
              class="flex items-center gap-3 p-2 rounded-md group/app hover:bg-bg-hover/30 cursor-grab active:cursor-grabbing"
              animate:flip={{duration: flipDurationMs}}
            >
              {@render appRowContent(app)}
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
        <span class="text-sm font-medium text-text-muted">Ungrouped</span>
        {#if ungroupedApps.length > 0}
          <span class="text-xs text-text-disabled ml-2">{ungroupedApps.length} apps</span>
        {:else}
          <span class="text-xs text-text-disabled ml-2">Drag apps here to ungroup them</span>
        {/if}
      </div>
      <div class="p-2 space-y-1 min-h-[36px]" use:dndzone={{items: ungroupedApps, flipDurationMs, type: 'apps', dropTargetStyle: {}}} onconsider={(e) => handleAppDndConsider(e, '')} onfinalize={(e) => handleAppDndFinalize(e, '')}>
        {#each ungroupedApps as app ((app as App & Record<string, unknown>).id)}
          <div
            class="flex items-center gap-3 p-2 rounded-md group/app hover:bg-bg-hover/30 cursor-grab active:cursor-grabbing"
            animate:flip={{duration: flipDurationMs}}
          >
            {@render appRowContent(app)}
          </div>
        {/each}
      </div>
    </div>
  {/if}

  {#if localAppsCount === 0 && localGroupsCount === 0}
    <div class="text-center py-8 text-text-muted">
      No applications or groups configured. Click "Add App" to get started.
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

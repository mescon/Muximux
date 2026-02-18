<script lang="ts">
  import type { App, Config } from '$lib/types';

  let {
    localConfig = $bindable(),
    localApps = $bindable(),
    onexport,
    onimportselect,
  }: {
    localConfig: Config;
    localApps: App[];
    onexport: () => void;
    onimportselect: (e: Event) => void;
  } = $props();

  let importFileInput = $state<HTMLInputElement | undefined>(undefined);

  const navPositions = [
    { value: 'top', label: 'Top Bar', description: 'Horizontal bar at the top' },
    { value: 'left', label: 'Left Sidebar', description: 'Vertical sidebar on the left' },
    { value: 'right', label: 'Right Sidebar', description: 'Vertical sidebar on the right' },
    { value: 'bottom', label: 'Bottom Bar', description: 'Horizontal bar at the bottom' },
    { value: 'floating', label: 'Floating', description: 'Minimal floating button' }
  ] as const;
</script>

<div class="space-y-6">
  <!-- Dashboard Title -->
  <div>
    <label for="title" class="block text-sm font-medium text-text-secondary mb-2">
      Dashboard Title
    </label>
    <input
      id="title"
      type="text"
      bind:value={localConfig.title}
      class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary
             focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
      placeholder="Muximux"
    />
    <p class="text-xs text-text-disabled mt-1.5">
      Variables: <code class="text-text-muted">%title%</code> (app name),
      <code class="text-text-muted">%group%</code>,
      <code class="text-text-muted">%version%</code>,
      <code class="text-text-muted">%url%</code>.
      Example: <code class="text-text-muted">Muximux - %title%</code>
    </p>
  </div>

  <!-- Navigation Position -->
  <div>
    <span class="block text-sm font-medium text-text-secondary mb-2">
      Navigation Position
    </span>
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
      {#each navPositions as pos (pos.value)}
        <button
          class="p-3 rounded-lg border text-left transition-colors
                 {localConfig.navigation.position === pos.value
                   ? 'border-brand-500 bg-brand-500/10 text-text-primary'
                   : 'border-border-subtle hover:border-border-strong text-text-secondary'}"
          onclick={() => localConfig.navigation.position = pos.value}
        >
          <div class="font-medium">{pos.label}</div>
          <div class="text-xs text-text-muted mt-1">{pos.description}</div>
        </button>
      {/each}
    </div>
  </div>

  <!-- Bar Style (only shown when top or bottom is selected) -->
  {#if localConfig.navigation.position === 'top' || localConfig.navigation.position === 'bottom'}
    <div>
      <span class="block text-sm font-medium text-text-secondary mb-2">
        Bar Style
      </span>
      <div class="grid grid-cols-2 gap-3">
        {#each [
          { value: 'grouped', label: 'Group Dropdowns', description: 'Apps organized in dropdown menus by group' },
          { value: 'flat', label: 'Flat List', description: 'All apps in a single scrollable row' }
        ] as style (style.value)}
          <button
            class="p-3 rounded-lg border text-left transition-colors
                   {(localConfig.navigation.bar_style || 'grouped') === style.value
                     ? 'border-brand-500 bg-brand-500/10 text-text-primary'
                     : 'border-border-subtle hover:border-border-strong text-text-secondary'}"
            onclick={() => localConfig.navigation.bar_style = style.value as 'grouped' | 'flat'}
          >
            <div class="font-medium text-sm">{style.label}</div>
            <div class="text-xs text-text-muted mt-1">{style.description}</div>
          </button>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Floating Position (only shown when floating is selected) -->
  {#if localConfig.navigation.position === 'floating'}
    <div>
      <span class="block text-sm font-medium text-text-secondary mb-2">
        Floating Button Position
      </span>
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {#each [
          { value: 'bottom-right', label: 'Bottom Right' },
          { value: 'bottom-left', label: 'Bottom Left' },
          { value: 'top-right', label: 'Top Right' },
          { value: 'top-left', label: 'Top Left' }
        ] as fp (fp.value)}
          <button
            class="p-2 rounded-lg border text-center text-sm transition-colors
                   {(localConfig.navigation.floating_position || 'bottom-right') === fp.value
                     ? 'border-brand-500 bg-brand-500/10 text-text-primary'
                     : 'border-border-subtle hover:border-border-strong text-text-secondary'}"
            onclick={() => localConfig.navigation.floating_position = fp.value as 'bottom-right' | 'bottom-left' | 'top-right' | 'top-left'}
          >
            {fp.label}
          </button>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Navigation Options -->
  <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
    <label class="flex items-center gap-3 p-3 bg-bg-hover rounded-lg cursor-pointer">
      <input
        type="checkbox"
        bind:checked={localConfig.navigation.show_labels}
        class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
      />
      <div>
        <div class="text-sm text-text-primary">Show Labels</div>
        <div class="text-xs text-text-muted">Display app names next to icons</div>
      </div>
    </label>

    <label class="flex items-center gap-3 p-3 bg-bg-hover rounded-lg cursor-pointer">
      <input
        type="checkbox"
        bind:checked={localConfig.navigation.show_logo}
        class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
      />
      <div>
        <div class="text-sm text-text-primary">Show Logo</div>
        <div class="text-xs text-text-muted">Display the Muximux logo in the menu</div>
      </div>
    </label>

    <label class="flex items-center gap-3 p-3 bg-bg-hover rounded-lg cursor-pointer">
      <input
        type="checkbox"
        bind:checked={localConfig.navigation.show_app_colors}
        class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
      />
      <div>
        <div class="text-sm text-text-primary">App Color Accents</div>
        <div class="text-xs text-text-muted">Highlight the active app with its color</div>
      </div>
    </label>

    <label class="flex items-center gap-3 p-3 bg-bg-hover rounded-lg cursor-pointer">
      <input
        type="checkbox"
        bind:checked={localConfig.navigation.show_icon_background}
        class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
      />
      <div>
        <div class="text-sm text-text-primary">Icon Background</div>
        <div class="text-xs text-text-muted">Show colored circle behind app icons</div>
      </div>
    </label>

    <div class="p-3 bg-bg-hover rounded-lg sm:col-span-2">
      <div class="flex items-center justify-between mb-2">
        <div>
          <div class="text-sm text-text-primary">Icon Size</div>
          <div class="text-xs text-text-muted">Scale app icons in the navigation</div>
        </div>
        <span class="text-sm text-text-secondary tabular-nums">{localConfig.navigation.icon_scale}×</span>
      </div>
      <input type="range" min="0.5" max="2" step="0.25"
        bind:value={localConfig.navigation.icon_scale}
        class="w-full" />
    </div>

    <label class="flex items-center gap-3 p-3 bg-bg-hover rounded-lg cursor-pointer">
      <input
        type="checkbox"
        bind:checked={localConfig.navigation.show_splash_on_startup}
        class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
      />
      <div>
        <div class="text-sm text-text-primary">Start on Overview</div>
        <div class="text-xs text-text-muted">Show the dashboard overview when Muximux opens</div>
      </div>
    </label>

    <div class="p-3 bg-bg-hover rounded-lg sm:col-span-2">
      <label class="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          bind:checked={localConfig.navigation.auto_hide}
          class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
        />
        <div class="flex-1">
          <div class="text-sm text-text-primary">Auto-hide Menu</div>
          <div class="text-xs text-text-muted">Automatically collapse the menu after inactivity</div>
        </div>
      </label>
      {#if localConfig.navigation.auto_hide}
        <div class="flex items-center gap-3 mt-3 pt-3 border-t border-border-subtle">
          <div class="flex-1 text-xs text-text-muted pl-7">Hide after</div>
          <select
            bind:value={localConfig.navigation.auto_hide_delay}
            class="px-2 py-1 text-xs bg-bg-overlay border border-border-strong rounded text-text-primary focus:ring-brand-500 focus:border-brand-500"
          >
            <option value="0.25s">0.25s</option>
            <option value="0.5s">0.5s</option>
            <option value="1s">1s</option>
            <option value="2s">2s</option>
            <option value="3s">3s</option>
          </select>
        </div>
        <label class="flex items-center gap-3 mt-2 pl-7 cursor-pointer">
          <input
            type="checkbox"
            bind:checked={localConfig.navigation.show_shadow}
            class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
          />
          <div class="text-xs text-text-muted">Shadow — show a drop shadow on the expanded menu</div>
        </label>
      {/if}
    </div>

    {#if localConfig.navigation.position === 'left' || localConfig.navigation.position === 'right'}
      <div class="p-3 bg-bg-hover rounded-lg sm:col-span-2">
        <label class="flex items-center gap-3 cursor-pointer">
          <input
            type="checkbox"
            bind:checked={localConfig.navigation.hide_sidebar_footer}
            class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
          />
          <div class="flex-1">
            <div class="text-sm text-text-primary">Collapsible Footer</div>
            <div class="text-xs text-text-muted">Hide utility buttons in a drawer that reveals on hover</div>
          </div>
        </label>
      </div>
    {/if}
  </div>

  <!-- Health Monitoring bulk actions -->
  <div class="pt-4 border-t border-border">
    <div class="flex items-center justify-between mb-1">
      <h3 class="text-sm font-medium text-text-secondary">Health Checks</h3>
      <div class="flex gap-2">
        <button
          class="text-xs px-2 py-1 rounded text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors"
          onclick={() => localApps.forEach(a => a.health_check = undefined)}
        >Enable all</button>
        <button
          class="text-xs px-2 py-1 rounded text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors"
          onclick={() => localApps.forEach(a => a.health_check = false)}
        >Disable all</button>
      </div>
    </div>
    <p class="text-xs text-text-disabled">Toggle per app in the app editor</p>
  </div>

  <!-- Advanced -->
  <div class="pt-4 border-t border-border">
    <h3 class="text-sm font-medium text-text-secondary mb-3">Advanced</h3>

    <div class="flex items-center gap-3 mb-4">
      <label for="log-level" class="text-sm text-text-muted whitespace-nowrap">Log Level</label>
      <select
        id="log-level"
        bind:value={localConfig.log_level}
        class="px-2 py-1 text-sm bg-bg-elevated border border-border-subtle rounded-md text-text-primary
               focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
      >
        <option value="debug">Debug</option>
        <option value="info">Info</option>
        <option value="warn">Warning</option>
        <option value="error">Error</option>
      </select>
      <span class="text-xs text-text-disabled">Takes effect on restart</span>
    </div>

    <div class="flex items-center gap-3 mb-4">
      <label for="proxy-timeout" class="text-sm text-text-muted whitespace-nowrap">Proxy Timeout</label>
      <input
        id="proxy-timeout"
        type="text"
        bind:value={localConfig.proxy_timeout}
        placeholder="30s"
        class="w-20 px-2 py-1 text-sm bg-bg-elevated border border-border-subtle rounded-md text-text-primary
               focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
      />
      <span class="text-xs text-text-disabled">Max wait time for proxied backends (e.g. 30s, 1m)</span>
    </div>

    <div class="flex flex-wrap gap-3">
      <button
        class="btn btn-secondary btn-sm flex items-center gap-2"
        onclick={onexport}
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
        </svg>
        Export Config
      </button>
      <button
        class="btn btn-secondary btn-sm flex items-center gap-2"
        onclick={() => importFileInput?.click()}
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
        </svg>
        Import Config
      </button>
      <input
        bind:this={importFileInput}
        type="file"
        accept=".yaml,.yml"
        class="hidden"
        onchange={onimportselect}
      />
    </div>
    <p class="text-xs text-text-disabled mt-2">
      Export your current configuration or import a previously saved one.
    </p>
  </div>
</div>

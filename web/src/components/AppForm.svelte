<script lang="ts">
  import type { App, Group } from '$lib/types';
  import { openModes } from '$lib/constants';
  import AppIcon from './AppIcon.svelte';

  let {
    app = $bindable(),
    mode,
    groups,
    allApps,
    errors = {},
    onopenicon,
    ondefaultchange,
    onclearerror,
  }: {
    app: App;
    mode: 'create' | 'edit';
    groups: Group[];
    allApps: App[];
    errors?: Record<string, string>;
    onopenicon?: () => void;
    ondefaultchange?: (checked: boolean) => void;
    onclearerror?: (field: string) => void;
  } = $props();

  let prefix = $derived(mode === 'create' ? 'create' : 'edit');

  function clearError(field: string) {
    onclearerror?.(field);
  }
</script>

{#snippet helpTip(text: string)}
  <span class="help-trigger relative ml-1 inline-block align-middle">
    <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
    </svg>
    <!-- eslint-disable-next-line svelte/no-at-html-tags -- tooltip text is hardcoded, not user input -->
    <span class="help-tooltip">{@html text}</span>
  </span>
{/snippet}

<div class="space-y-4">
  <!-- Identity -->
  <div>
    <label for="{prefix}-app-name" class="block text-sm font-medium text-text-secondary mb-1">
      Name
      {@render helpTip('Displayed in the navigation bar and page title. Also used as a unique identifier — renaming an app creates a new proxy route.')}
    </label>
    <input
      id="{prefix}-app-name"
      type="text"
      bind:value={app.name}
      oninput={() => clearError('name')}
      class="w-full px-3 py-2 bg-bg-elevated border rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 {errors.name ? 'border-red-500' : 'border-border-subtle'}"
      placeholder="My App"
    />
    {#if errors.name}<p class="text-red-400 text-xs mt-1">{errors.name}</p>{/if}
  </div>

  <div>
    <label for="{prefix}-app-url" class="block text-sm font-medium text-text-secondary mb-1">
      URL
      {@render helpTip('The full address of the application. Used to load the app in an iframe, or as the link when opened in a new tab/window.')}
    </label>
    <input
      id="{prefix}-app-url"
      type="url"
      bind:value={app.url}
      oninput={() => clearError('url')}
      class="w-full px-3 py-2 bg-bg-elevated border rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 {errors.url ? 'border-red-500' : 'border-border-subtle'}"
      placeholder="http://localhost:8080"
    />
    {#if errors.url}<p class="text-red-400 text-xs mt-1">{errors.url}</p>{/if}
  </div>

  <div>
    <span class="block text-sm font-medium text-text-secondary mb-1">Icon</span>
    <div class="flex items-center gap-3">
      <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => onopenicon?.()}>
        <AppIcon icon={app.icon} name={app.name || 'App'} color={app.color} size="lg" />
      </button>
      <div class="flex-1">
        <button
          class="btn btn-secondary btn-sm w-full text-left"
          onclick={() => onopenicon?.()}
        >
          {app.icon?.name || 'Choose icon...'}
        </button>
        <p class="text-xs text-text-muted mt-1">
          {app.icon?.type === 'dashboard' ? 'Dashboard Icon' : app.icon?.type === 'lucide' ? 'Lucide Icon' : app.icon?.type === 'custom' ? 'Custom Icon' : app.icon?.type === 'url' ? 'URL Icon' : 'No icon set'}
        </p>
      </div>
    </div>
    <div class="flex items-center gap-4 mt-2">
      {#if app.icon?.type === 'lucide'}
        <label class="flex items-center gap-2 text-xs text-text-muted">
          Icon color
          <input type="color" value={app.icon.color || '#ffffff'} oninput={(e) => app.icon.color = (e.target as HTMLInputElement).value} class="w-8 h-8 rounded cursor-pointer" />
          {#if app.icon.color}
            <button class="text-text-disabled hover:text-text-secondary" onclick={() => app.icon.color = ''} title="Reset to theme default">&times;</button>
          {/if}
        </label>
      {/if}
      <label class="flex items-center gap-2 text-xs text-text-muted">
        Icon background
        <input type="color" value={app.icon.background || app.color || '#374151'} oninput={(e) => app.icon.background = (e.target as HTMLInputElement).value} class="w-8 h-8 rounded cursor-pointer" />
        <button class="text-text-disabled hover:text-text-secondary text-xs" onclick={() => app.icon.background = 'transparent'} title="Transparent">none</button>
        {#if app.icon.background}
          <button class="text-text-disabled hover:text-text-secondary" onclick={() => app.icon.background = ''} title="Reset to app color">&times;</button>
        {/if}
      </label>
    </div>
  </div>

  <div>
    <label for="{prefix}-app-color" class="block text-sm font-medium text-text-secondary mb-1">
      App color
      {@render helpTip("The app's accent color — used for the active tab indicator and sidebar highlight when \"Show App Colors\" is enabled. Also used as the default icon background unless overridden above.")}
    </label>
    <div class="flex items-center gap-2">
      <input
        id="{prefix}-app-color"
        type="color"
        bind:value={app.color}
        class="w-10 h-10 rounded cursor-pointer"
      />
      <input
        type="text"
        bind:value={app.color}
        class="flex-1 px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
      />
    </div>
  </div>

  <div>
    <label for="{prefix}-app-group" class="block text-sm font-medium text-text-secondary mb-1">
      Group
      {@render helpTip('Groups organize apps into collapsible sections in the sidebar. Apps with no group appear under "Ungrouped."')}
    </label>
    <select
      id="{prefix}-app-group"
      bind:value={app.group}
      class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500"
    >
      <option value="">No group</option>
      {#each groups as group (group.name)}
        <option value={group.name}>{group.name}</option>
      {/each}
    </select>
  </div>

  <!-- Display -->
  <div class="border-t border-border pt-3">
    <h4 class="text-xs font-medium text-text-disabled uppercase tracking-wide mb-3">Display</h4>
    <div class="space-y-3">
      <label class="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          bind:checked={app.enabled}
          class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
        />
        <div>
          <span class="text-sm text-text-primary">Enabled
            {@render helpTip('Disabled apps are hidden from non-admin users and excluded from the navigation entirely.')}
          </span>
          <p class="text-xs text-text-muted">Show this app in the navigation</p>
        </div>
      </label>
      <label class="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          checked={app.default}
          onchange={(e) => {
            app.default = (e.currentTarget as HTMLInputElement).checked;
            ondefaultchange?.(app.default);
          }}
          class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
        />
        <div>
          <span class="text-sm text-text-primary">Default app
            {@render helpTip('Only one app can be the default. Setting this will clear the default flag on any other app.')}
          </span>
          <p class="text-xs text-text-muted">Load this app on startup (disables the overview screen)</p>
        </div>
      </label>
      <div>
        <label for="{prefix}-app-mode" class="block text-sm font-medium text-text-secondary mb-1">
          Open Mode
          {@render helpTip('<b>Embedded</b> — loads inside Muximux in an iframe. Best for most apps.<br/><b>New Tab</b> — opens in a separate browser tab.<br/><b>New Window</b> — opens in a popup window.')}
        </label>
        <select
          id="{prefix}-app-mode"
          bind:value={app.open_mode}
          class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500"
        >
          {#each openModes as mode (mode.value)}
            <option value={mode.value}>{mode.label}</option>
          {/each}
        </select>
      </div>
      <div>
        <label for="{prefix}-app-scale" class="block text-sm font-medium text-text-secondary mb-1">
          Scale: {Math.round(app.scale * 100)}%
          {@render helpTip('Zoom level for the embedded iframe. Useful for apps with small or large UIs. Only applies to iframe open mode.')}
        </label>
        <input
          id="{prefix}-app-scale"
          type="range"
          min="0.5"
          max="2"
          step="0.05"
          bind:value={app.scale}
          class="w-full"
        />
      </div>
    </div>
  </div>

  <!-- Proxy -->
  <div class="border-t border-border pt-3">
    <h4 class="text-xs font-medium text-text-disabled uppercase tracking-wide mb-3">Proxy</h4>
    <div class="space-y-3">
      <label class="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          bind:checked={app.proxy}
          class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
        />
        <div>
          <span class="text-sm text-text-primary">Use reverse proxy
            {@render helpTip('Routes all traffic through the built-in Caddy reverse proxy. The app URL is rewritten to a local <code>/proxy/app-name/</code> path, avoiding CORS, mixed-content, and cookie-domain issues.')}
          </span>
          <p class="text-xs text-text-muted">Route traffic through the built-in proxy to avoid CORS and mixed-content issues</p>
        </div>
      </label>
      {#if app.proxy}
        <div class="ml-7 space-y-3 border-l-2 border-border pl-4 min-w-0 overflow-hidden">
          <label class="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              checked={app.proxy_skip_tls_verify !== false}
              onchange={(e) => { app.proxy_skip_tls_verify = (e.target as HTMLInputElement).checked ? undefined : false; }}
              class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
            />
            <div>
              <span class="text-sm text-text-primary">Skip TLS verification
                {@render helpTip('Enabled by default. Disable this only if the backend has a valid, trusted TLS certificate and you want strict verification.')}
              </span>
              <p class="text-xs text-text-muted">Disable for backends with valid certificates</p>
            </div>
          </label>
          <div>
            <span class="block text-sm text-text-muted mb-1">Custom headers</span>
            <p class="text-xs text-text-disabled mb-2">Sent to the backend on every proxied request (e.g. Authorization, X-Api-Key)</p>
            {#each Object.entries(app.proxy_headers ?? {}) as [key, value] (key)}
              <div class="flex gap-2 mb-2">
                <input type="text" value={key} placeholder="Header name"
                  class="flex-1 min-w-0 px-2 py-1 text-sm bg-bg-elevated border border-border-subtle rounded text-text-primary placeholder-text-disabled"
                  onchange={(e) => {
                    const headers = { ...(app.proxy_headers ?? {}) };
                    const oldKey = key;
                    const newKey = (e.target as HTMLInputElement).value.trim();
                    if (newKey && newKey !== oldKey) {
                      delete headers[oldKey];
                      headers[newKey] = value;
                      app.proxy_headers = headers;
                    }
                  }}
                />
                <input type="text" value={value} placeholder="Value"
                  class="flex-1 min-w-0 px-2 py-1 text-sm bg-bg-elevated border border-border-subtle rounded text-text-primary placeholder-text-disabled"
                  onchange={(e) => {
                    const headers = { ...(app.proxy_headers ?? {}) };
                    headers[key] = (e.target as HTMLInputElement).value;
                    app.proxy_headers = headers;
                  }}
                />
                <button class="px-2 py-1 text-text-muted hover:text-red-400" title="Remove header"
                  onclick={() => {
                    const headers = { ...(app.proxy_headers ?? {}) };
                    delete headers[key];
                    app.proxy_headers = Object.keys(headers).length > 0 ? headers : undefined;
                  }}
                >
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
                </button>
              </div>
            {/each}
            <button class="text-xs text-brand-400 hover:text-brand-300"
              onclick={() => {
                app.proxy_headers = { ...(app.proxy_headers ?? {}), '': '' };
              }}
            >+ Add header</button>
          </div>
        </div>
      {/if}
    </div>
  </div>

  <!-- Advanced -->
  <div class="border-t border-border pt-3">
    <h4 class="text-xs font-medium text-text-disabled uppercase tracking-wide mb-3">Advanced</h4>
    <div class="space-y-3">
      <label class="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          checked={app.health_check === true}
          onchange={(e) => {
            app.health_check = (e.target as HTMLInputElement).checked ? true : undefined;
          }}
          class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
        />
        <div>
          <span class="text-sm text-text-primary">Health check
            {@render helpTip('Periodically pings the app URL (or health URL if set) and shows a status indicator in the navigation.')}
          </span>
          <p class="text-xs text-text-muted">Monitor availability of this app</p>
        </div>
      </label>
      {#if app.health_check === true}
        <div class="ml-7 pl-4 border-l-2 border-border">
          <label for="{prefix}-app-health-url" class="block text-sm text-text-muted mb-1">Health check URL</label>
          <input
            id="{prefix}-app-health-url"
            type="url"
            bind:value={app.health_url}
            placeholder={app.url || 'Uses app URL if empty'}
            class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
          <p class="text-xs text-text-disabled mt-1">Leave blank to use the app URL</p>
        </div>
      {/if}
      <div class="flex items-center gap-3">
        <div class="flex-1">
          <span class="text-sm text-text-primary">Keyboard Shortcut
            {@render helpTip('Assigns a number key (1–9) to quickly switch to this app. Each number can only be assigned to one app.')}
          </span>
          <p class="text-xs text-text-muted">Press this number key to switch to this app</p>
        </div>
        <select
          value={app.shortcut ?? ''}
          onchange={(e) => {
            const val = (e.target as HTMLSelectElement).value;
            app.shortcut = val ? parseInt(val) : undefined;
          }}
          class="px-2 py-1 text-sm bg-bg-elevated border border-border-subtle rounded text-text-primary focus:ring-brand-500 focus:border-brand-500"
        >
          <option value="">None</option>
          {#each [1,2,3,4,5,6,7,8,9] as n (n)}
            {@const taken = allApps.find(a => a.shortcut === n && a.name !== app.name)}
            <option value={n} disabled={!!taken}>{n}{taken ? ` (${taken.name})` : ''}</option>
          {/each}
        </select>
      </div>
      <label class="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          bind:checked={app.force_icon_background}
          class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
        />
        <div>
          <span class="text-sm text-text-primary">Force icon background
            {@render helpTip('Always show a colored background circle behind this app\'s icon, even when the global "Show Icon Backgrounds" setting is off.')}
          </span>
          <p class="text-xs text-text-muted">Show background even when global icon backgrounds are off</p>
        </div>
      </label>
      <label class="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          bind:checked={app.icon.invert}
          class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
        />
        <div>
          <span class="text-sm text-text-primary">Invert icon colors
            {@render helpTip('Inverts the icon\'s colors (black becomes white, white becomes black). Useful for dark icons that are hard to see on dark backgrounds.')}
          </span>
          <p class="text-xs text-text-muted">Flip dark icons to light and vice versa</p>
        </div>
      </label>
      <div>
        <label for="{prefix}-app-min-role" class="block text-sm font-medium text-text-secondary mb-1">
          Minimum Role
          {@render helpTip('Restricts visibility based on user role. Users below the selected role won\'t see this app in the navigation or API responses.')}
        </label>
        <select
          id="{prefix}-app-min-role"
          bind:value={app.min_role}
          class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500"
        >
          <option value="">Everyone (default)</option>
          <option value="power-user">Power User</option>
          <option value="admin">Admin</option>
        </select>
        <p class="text-xs text-text-muted mt-1">Users below this role won't see this app</p>
      </div>
    </div>
  </div>
</div>

<style>
  /* Help tooltips */
  .help-tooltip {
    display: none;
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    width: 240px;
    padding: 8px 10px;
    border-radius: 8px;
    background: var(--bg-overlay, #1f2937);
    border: 1px solid var(--border-default, #374151);
    color: var(--text-secondary, #d1d5db);
    font-size: 11px;
    line-height: 1.4;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
    z-index: 70;
    pointer-events: none;
  }

  .help-trigger:hover > .help-tooltip {
    display: block;
  }
</style>

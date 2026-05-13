<script lang="ts">
  import { onMount } from 'svelte';
  import type { DiscoverySuggestion, DiscoveryImportItem, DiscoveryImportResult, AppIcon as AppIconType } from '$lib/types';
  import { scanDockerContainers, importDockerSuggestions } from '$lib/api';
  import AppIcon from '../AppIcon.svelte';
  import IconBrowser from '../IconBrowser.svelte';

  // mode controls per-row default behaviour. Same modal opens from
  // either the Apps tab (operator wants apps in the menu) or the
  // Gateway tab (operator wants subdomains, no menu entry):
  //   'apps'     - default each row to "create app", gateway off
  //   'gateway'  - default each row to "no app", gateway on
  // The operator can flip both per row regardless of opener.
  type Mode = 'apps' | 'gateway';

  let { open = $bindable(false), mode = 'apps' as Mode, onclose, onimported }: {
    open: boolean;
    mode?: Mode;
    onclose: () => void;
    /** Fired after a successful import so the parent can refresh
     *  the apps / gateway lists. */
    onimported?: () => void;
  } = $props();

  // Scan state.
  type RowState = {
    s: DiscoverySuggestion;
    selected: boolean;
    createApp: boolean;
    createGateway: boolean;
    gatewayDomain: string;
    nameOverride: string;
    // Operator-picked icon override. Lets the user choose any icon
    // (dashboard / lucide / custom) per row before import, which
    // matters most for low-confidence rows where the catalog gave us
    // nothing to start from.
    iconOverride: AppIconType | null;
    // Routing radio per row, only meaningful when createApp is true.
    // - 'direct':  App.URL = container URL
    // - 'proxy':   App.proxy = true; menu links to /proxy/<slug>
    // - 'gateway': App.URL = https://<gatewayDomain>; auto-checks
    //              createGateway since it's a hard requirement
    routing: 'direct' | 'proxy' | 'gateway';
  };

  let scanning = $state(false);
  let scanError = $state<string | null>(null);
  let scanBlocked = $state<string | null>(null);
  let rows = $state<RowState[]>([]);

  // Re-scan when the modal opens; reset state every time so the
  // operator gets fresh suggestions and can't accidentally import
  // stale data after the daemon state changed.
  $effect(() => {
    if (open) load();
  });

  async function load() {
    scanning = true;
    scanError = null;
    scanBlocked = null;
    rows = [];
    try {
      const r = await scanDockerContainers();
      if (r.scan_blocked) {
        scanBlocked = r.scan_blocked;
        return;
      }
      if (r.error) {
        scanError = r.error;
        return;
      }
      rows = (r.suggestions ?? []).map((s) => ({
        s,
        selected: false,
        // Per-mode defaults documented above.
        createApp: mode === 'apps',
        createGateway: mode === 'gateway' && !!s.suggested_domain,
        gatewayDomain: s.suggested_domain ?? '',
        nameOverride: s.name,
        iconOverride: null,
        routing: 'direct' as const,
      }));
    } catch (e) {
      scanError = e instanceof Error ? e.message : 'Scan failed';
    } finally {
      scanning = false;
    }
  }

  // Selection helpers. The "Select all" checkbox toggles every row's
  // `selected` flag; the visible counter at the bottom uses these.
  let selectedCount = $derived(rows.filter(r => r.selected).length);
  let allSelected = $derived(rows.length > 0 && rows.every(r => r.selected));

  function toggleAll() {
    const v = !allSelected;
    rows = rows.map(r => ({ ...r, selected: v }));
  }

  function stabilityHint(s: DiscoverySuggestion): { tone: 'gray' | 'amber' | 'red'; tip: string } {
    switch (s.stability) {
      case 'recreate-fragile':
        return { tone: 'amber', tip: 'This container name will change on docker-compose --force-recreate. Add label muximux.discovery.id=<stable-key> for reliable tracking.' };
      case 'task-fragile':
        return { tone: 'red', tip: 'Swarm task name; reschedule will break tracking. Strongly recommend a muximux.discovery.id label.' };
      default:
        return { tone: 'gray', tip: 'Stable identifier.' };
    }
  }

  // Map the backend's confidence enum to a self-explanatory chip
  // label + tooltip. The raw values ("high"/"medium"/"low") are
  // accurate but unhelpful on first read - the chip should answer
  // "what does this confidence mean for me?" at a glance.
  function confidenceHint(s: DiscoverySuggestion): { label: string; tip: string } {
    switch (s.confidence) {
      case 'high':
        return {
          label: 'label match',
          tip: 'High confidence: this container carries muximux.app.* labels, so name/icon/port were taken from them directly. No guessing.',
        };
      case 'medium':
        return {
          label: 'catalog match',
          tip: "Medium confidence: this container's image matches Muximux's curated catalog (Sonarr, Plex, etc.) so name, icon and default port come from a known-good source. Review before importing.",
        };
      case 'low':
      default:
        return {
          label: 'guessed',
          tip: "Low confidence: no muximux.app.* labels and no catalog match. Name was titleized from the container name, icon is blank, and port was picked from the first exposed port. Review and pick an icon before importing.",
        };
    }
  }

  // Import state. importing tracks the in-flight POST; importResult
  // holds the per-item statuses so we can render badges on each row
  // after the response. importTopError is for transport-level
  // failures (network, 500); per-item errors live in importResult.
  let importing = $state(false);
  let importResult = $state<DiscoveryImportResult | null>(null);
  let importTopError = $state<string | null>(null);

  async function runImport() {
    importing = true;
    importResult = null;
    importTopError = null;
    try {
      const items: DiscoveryImportItem[] = rows
        .filter(r => r.selected && (r.createApp || r.createGateway))
        .map(r => {
          const item: DiscoveryImportItem = {
            key: r.s.key,
            strategy: r.s.effective_strategy,
          };
          if (r.createApp) {
            item.app = {
              name: r.nameOverride.trim() || r.s.name,
              url: r.s.url,
              icon: r.iconOverride ?? { type: 'dashboard', name: r.s.icon ?? '' },
              group: r.s.group ?? '',
              health_url: r.s.health_url,
              enabled: true,
            };
            // Routing only matters when an app is being created;
            // omit when the row is gateway-only to keep the wire
            // payload tight.
            item.routing = r.routing;
          }
          if (r.createGateway) {
            item.gateway = {
              domain: r.gatewayDomain.trim(),
              backend_url: r.s.url,
              tls: 'auto',
            };
          }
          return item;
        });
      if (items.length === 0) return;

      const res = await importDockerSuggestions({ items });
      importResult = res;
      if (res.success) {
        onimported?.();
      }
    } catch (e) {
      importTopError = e instanceof Error ? e.message : 'Import failed';
    } finally {
      importing = false;
    }
  }

  // Reactive invariant: routing=gateway requires createGateway. The
  // radio's onchange auto-checks createGateway when the operator
  // picks Gateway, but a separate uncheck of "Add gateway site"
  // (toggled in this component) would leave routing=gateway with
  // createGateway=false. This effect forces createGateway back
  // on, so submit-time validation can't be reached in that state.
  $effect(() => {
    for (const r of rows) {
      if (r.createApp && r.routing === 'gateway' && !r.createGateway) {
        r.createGateway = true;
      }
    }
  });

  // Per-row status lookup for badge rendering. Keyed on suggestion.key
  // because that's the stable identifier the import endpoint preserves.
  function statusFor(key: string) {
    return importResult?.items.find(i => i.key === key);
  }

  // Inline icon-picker state. We mount IconBrowser inside this modal
  // rather than bubble events to the parent because (a) the parent's
  // picker is bound to its own targets ('newApp', 'editApp', ...) and
  // adding a 'discoverRow' target would leak modal state across
  // unrelated components, and (b) the import flow needs the icon to
  // round-trip through this component's row state regardless.
  let iconPickerForKey = $state<string | null>(null);

  // Effective icon for the row's preview: explicit override > catalog
  // suggestion > empty placeholder.
  function effectiveIcon(r: RowState): AppIconType {
    return r.iconOverride ?? { type: 'dashboard', name: r.s.icon ?? '' };
  }

  function openIconPicker(key: string) {
    iconPickerForKey = key;
  }

  function closeIconPicker() {
    iconPickerForKey = null;
  }

  function handleIconSelect(detail: { name: string; variant: string; type: string }) {
    if (!iconPickerForKey) return;
    const idx = rows.findIndex(r => r.s.key === iconPickerForKey);
    if (idx < 0) return;
    const t = detail.type;
    if (t === 'dashboard' || t === 'lucide' || t === 'custom' || t === 'url') {
      rows[idx].iconOverride = { type: t, name: detail.name, variant: detail.variant };
    }
    iconPickerForKey = null;
  }
</script>

{#if open}
  <div class="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4" role="dialog" aria-modal="true">
    <div class="bg-bg-base border border-border rounded-lg shadow-xl w-full max-w-4xl max-h-[90vh] flex flex-col">
      <header class="px-5 py-4 border-b border-border flex items-center justify-between">
        <div>
          <h2 class="text-lg font-semibold text-text-primary">Discover from Docker</h2>
          <p class="text-xs text-text-muted mt-0.5">
            {#if mode === 'apps'}
              Each container becomes an app in your menu by default. Toggle "Gateway" to also expose it on a subdomain.
            {:else}
              Each container becomes a gateway-only subdomain by default. Toggle "App" to also add it to the dashboard menu.
            {/if}
          </p>
        </div>
        <button class="btn btn-secondary btn-sm" onclick={onclose} type="button">Close</button>
      </header>

      <div class="flex-1 overflow-y-auto p-5">
        {#if scanning}
          <div class="text-text-muted text-sm">Scanning Docker daemon…</div>
        {:else if scanBlocked}
          <div class="p-3 rounded-md border border-amber-500/40 bg-amber-500/10 text-amber-300 text-sm">
            <div class="font-medium mb-1">Scan blocked</div>
            <div>{scanBlocked}</div>
          </div>
        {:else if scanError}
          <div class="p-3 rounded-md border border-red-500/40 bg-red-500/10 text-red-300 text-sm">
            <div class="font-medium mb-1">Scan failed</div>
            <div>{scanError}</div>
          </div>
        {:else if rows.length === 0}
          <div class="text-text-muted text-sm">
            No running containers found on the configured daemon. Containers must be running and (when network_strategy is container_ip) attached to a network Muximux can reach.
          </div>
        {:else}
          <div class="mb-3 flex items-center gap-2 text-sm">
            <label class="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" checked={allSelected} onchange={toggleAll} />
              <span class="text-text-secondary">Select all</span>
            </label>
            <span class="text-text-muted">·</span>
            <span class="text-text-muted">{selectedCount} of {rows.length} selected</span>
          </div>

          <div class="space-y-2">
            {#each rows as row (row.s.key)}
              {@const sh = stabilityHint(row.s)}
              {@const ch = confidenceHint(row.s)}
              <div class="p-3 rounded-md border border-border-subtle bg-bg-elevated
                          {row.selected ? 'ring-1 ring-brand-500/50' : ''}">
                <div class="flex items-start gap-3">
                  <input type="checkbox" bind:checked={row.selected} class="mt-1" />

                  <button
                    type="button"
                    class="shrink-0 cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all"
                    onclick={() => openIconPicker(row.s.key)}
                    title="Pick an icon for this app"
                    aria-label="Pick icon for {row.nameOverride || row.s.name}"
                  >
                    <AppIcon icon={effectiveIcon(row)} name={row.nameOverride || row.s.name || 'App'} size="md" />
                  </button>

                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-2 flex-wrap">
                      <input
                        type="text"
                        bind:value={row.nameOverride}
                        class="font-medium text-text-primary bg-transparent border-b border-transparent hover:border-border-subtle focus:border-brand-500 focus:outline-none px-1"
                      />
                      <span class="text-xs px-1.5 py-0.5 rounded cursor-help
                                   {row.s.confidence === 'high' ? 'bg-green-500/15 text-green-300' : ''}
                                   {row.s.confidence === 'medium' ? 'bg-blue-500/15 text-blue-300' : ''}
                                   {row.s.confidence === 'low' ? 'bg-gray-500/15 text-gray-300' : ''}"
                            title={ch.tip}>
                        {ch.label}
                      </span>
                      {#if sh.tone !== 'gray'}
                        <span class="text-xs px-1.5 py-0.5 rounded
                                     {sh.tone === 'amber' ? 'bg-amber-500/15 text-amber-300' : ''}
                                     {sh.tone === 'red' ? 'bg-red-500/15 text-red-300' : ''}"
                              title={sh.tip}>
                          {row.s.stability}
                        </span>
                      {/if}
                      {#if statusFor(row.s.key)}
                        {@const st = statusFor(row.s.key)!}
                        <span class="text-xs px-1.5 py-0.5 rounded font-medium
                                     {st.status === 'created' ? 'bg-green-500/15 text-green-300' : ''}
                                     {st.status === 'skipped_exists' ? 'bg-blue-500/15 text-blue-300' : ''}
                                     {st.status === 'validation_failed' ? 'bg-red-500/15 text-red-300' : ''}
                                     {st.status === 'name_collision_in_batch' ? 'bg-red-500/15 text-red-300' : ''}
                                     {st.status === 'aborted_by_batch_failure' ? 'bg-amber-500/15 text-amber-300' : ''}"
                              title={st.error || st.status}>
                          {st.status.replace(/_/g, ' ')}
                        </span>
                      {/if}
                    </div>

                    <div class="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-text-muted">
                      <span><span class="text-text-secondary">image:</span> {row.s.image_ref}</span>
                      <span><span class="text-text-secondary">key:</span> <code>{row.s.key}</code></span>
                      <span><span class="text-text-secondary">strategy:</span> {row.s.effective_strategy}</span>
                    </div>

                    {#if row.s.url}
                      <div class="mt-1 text-xs text-text-secondary">
                        <span class="text-text-muted">URL:</span> <code>{row.s.url}</code>
                      </div>
                    {:else}
                      <div class="mt-1 text-xs text-amber-300">
                        ⚠ No URL could be built - fix port / strategy in Settings → Discovery before importing.
                      </div>
                    {/if}

                    {#if row.s.notes && row.s.notes.length > 0}
                      <details class="mt-1 text-xs text-text-muted">
                        <summary class="cursor-pointer">Notes ({row.s.notes.length})</summary>
                        <ul class="mt-1 ml-4 list-disc">
                          {#each row.s.notes as n}<li>{n}</li>{/each}
                        </ul>
                      </details>
                    {/if}

                    <div class="mt-3 space-y-2 text-xs">
                      <div class="flex flex-wrap gap-4">
                        <label class="flex items-center gap-1.5 cursor-pointer">
                          <input type="checkbox" bind:checked={row.createApp} disabled={row.s.requires_input} />
                          <span class="text-text-primary">Add to menu</span>
                        </label>
                        <label class="flex items-center gap-1.5 cursor-pointer">
                          <input
                            type="checkbox"
                            bind:checked={row.createGateway}
                            disabled={row.routing === 'gateway' && row.createApp}
                          />
                          <span class="text-text-primary">Add gateway site</span>
                          {#if row.routing === 'gateway' && row.createApp}
                            <span class="text-text-muted">(required by routing)</span>
                          {/if}
                        </label>
                        {#if row.createGateway}
                          <input
                            type="text"
                            bind:value={row.gatewayDomain}
                            placeholder="sonarr.example.com"
                            class="text-xs px-2 py-0.5 bg-bg-base border border-border-subtle rounded text-text-primary focus:outline-none focus:ring-1 focus:ring-brand-500"
                          />
                        {/if}
                      </div>
                      {#if row.createApp}
                        <fieldset class="flex flex-wrap items-center gap-3 pl-6 text-text-secondary"
                                  data-testid="row-routing">
                          <legend class="text-text-muted">Menu link:</legend>
                          <label class="flex items-center gap-1 cursor-pointer">
                            <input type="radio" bind:group={row.routing} value="direct" />
                            <span title="Menu links straight to the container's URL. Requires the dashboard machine to reach the container's IP.">Direct</span>
                          </label>
                          <label class="flex items-center gap-1 cursor-pointer">
                            <input type="radio" bind:group={row.routing} value="proxy" />
                            <span title="Menu links to /proxy/<slug>; Muximux reverse-proxies to the container.">Proxy</span>
                          </label>
                          <label class="flex items-center gap-1 cursor-pointer"
                                 title={row.gatewayDomain ? '' : 'Set a gateway domain to enable this option.'}>
                            <input
                              type="radio"
                              bind:group={row.routing}
                              value="gateway"
                              disabled={!row.gatewayDomain.trim()}
                              onchange={(e) => { if ((e.currentTarget as HTMLInputElement).value === 'gateway') row.createGateway = true; }}
                            />
                            <span>Gateway domain</span>
                          </label>
                        </fieldset>
                      {/if}
                    </div>
                  </div>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <footer class="px-5 py-3 border-t border-border flex items-center justify-between gap-2">
        <span class="text-xs">
          {#if importTopError}
            <span class="text-red-300">{importTopError}</span>
          {:else if importResult && importResult.success}
            <span class="text-green-300">Import succeeded ({importResult.items.length} items)</span>
          {:else if importResult && !importResult.success}
            <span class="text-red-300">{importResult.error || 'Import failed - see per-row status'}</span>
          {:else}
            <span class="text-text-muted">{selectedCount} of {rows.length} selected</span>
          {/if}
        </span>
        <div class="flex gap-2">
          <button class="btn btn-secondary btn-sm" onclick={load} disabled={scanning || importing} type="button">Re-scan</button>
          <button class="btn btn-primary btn-sm" onclick={runImport} disabled={importing || selectedCount === 0} type="button">
            {importing ? 'Importing…' : `Import ${selectedCount} selected`}
          </button>
        </div>
      </footer>
    </div>
  </div>

  {#if iconPickerForKey}
    {@const pickingRow = rows.find(r => r.s.key === iconPickerForKey)}
    {#if pickingRow}
      {@const eff = effectiveIcon(pickingRow)}
      <div
        class="fixed inset-0 z-[60] flex items-center justify-center bg-black/60 p-4"
        role="dialog"
        aria-modal="true"
        aria-label="Pick icon"
      >
        <div class="bg-bg-surface rounded-xl shadow-2xl w-full max-w-3xl max-h-[85vh] flex flex-col border border-border">
          <div class="flex items-center justify-between p-4 border-b border-border">
            <h3 class="text-lg font-semibold text-text-primary">
              Pick an icon for {pickingRow.nameOverride || pickingRow.s.name}
            </h3>
            <button class="btn btn-ghost btn-icon" onclick={closeIconPicker} aria-label="Close" type="button">
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          <IconBrowser
            selectedIcon={eff.type === 'dashboard' || eff.type === 'lucide' ? (eff.name ?? '') : ''}
            selectedVariant={eff.variant ?? 'svg'}
            selectedType={eff.type === 'dashboard' || eff.type === 'lucide' || eff.type === 'custom' ? eff.type : 'dashboard'}
            onselect={handleIconSelect}
            onclose={closeIconPicker}
          />
        </div>
      </div>
    {/if}
  {/if}
{/if}

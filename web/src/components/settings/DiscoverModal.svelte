<script lang="ts">
  import { focusTrap } from '$lib/focusTrap';
  import * as m from '$lib/paraglide/messages.js';
  import type { App, DiscoverySuggestion, DiscoveryImportItem, DiscoveryImportResult, GatewaySite, AppIcon as AppIconType } from '$lib/types';
  import { scanDockerContainers, importDockerSuggestions, errorText } from '$lib/api';
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
    /** Fired after a successful import: the parent refreshes the
     *  apps / gateway lists and closes this modal. */
    onimported?: () => void | Promise<void>;
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
  let optedOut = $state(0);

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
    optedOut = 0;
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
      optedOut = r.opted_out ?? 0;
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
      scanError = errorText(e, m.discovery_scanFailed());
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

  // Translate the server's auto-import skip code into a short reason.
  // 'disabled' rows are filtered by the server, so there is nothing to show.
  function skipReason(s: DiscoverySuggestion): string | null {
    const k = s.auto_import_skip;
    if (!k) return null;
    switch (k.code) {
      case 'no_port': return m.discovery_skipNoPort();
      case 'no_url': return m.discovery_skipNoUrl({ detail: k.detail ?? '' });
      case 'unlabeled': return m.discovery_skipUnlabeled();
      case 'not_enabled': return m.discovery_skipNotEnabled();
      case 'invalid': return m.discovery_skipInvalid({ detail: k.detail ?? '' });
      default: return null;
    }
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
            // Build the app from the operator's row edits plus
            // every label-derived field on the suggestion. The
            // backend's ClientAppConfig accepts any subset of App
            // fields, so passing through `r.s.color`, `r.s.proxy`,
            // etc. lets a fully-labelled container materialise
            // into a fully-configured app with no post-import
            // edit needed (the "GitOps your apps" path).
            const app: Partial<App> = {
              name: r.nameOverride.trim() || r.s.name,
              url: r.s.url,
              icon: r.iconOverride ?? { type: 'dashboard', name: r.s.icon ?? '' },
              group: r.s.group ?? '',
              health_url: r.s.health_url,
              enabled: true,
            };
            if (r.s.color) app.color = r.s.color;
            if (typeof r.s.order === 'number') app.order = r.s.order;
            if (r.s.open_mode) app.open_mode = r.s.open_mode;
            if (typeof r.s.proxy === 'boolean') app.proxy = r.s.proxy;
            if (typeof r.s.proxy_skip_tls_verify === 'boolean') app.proxy_skip_tls_verify = r.s.proxy_skip_tls_verify;
            if (r.s.min_role) app.min_role = r.s.min_role;
            if (r.s.allowed_groups && r.s.allowed_groups.length > 0) app.allowed_groups = r.s.allowed_groups;
            if (r.s.permissions && r.s.permissions.length > 0) app.permissions = r.s.permissions;
            if (typeof r.s.allow_notifications === 'boolean') app.allow_notifications = r.s.allow_notifications;
            if (typeof r.s.default === 'boolean') app.default = r.s.default;
            if (typeof r.s.shortcut === 'number') app.shortcut = r.s.shortcut;
            if (typeof r.s.health_check === 'boolean') app.health_check = r.s.health_check || undefined;
            item.app = app;
            // Routing only matters when an app is being created;
            // omit when the row is gateway-only to keep the wire
            // payload tight.
            item.routing = r.routing;
          }
          if (r.createGateway) {
            // Build the gateway-site from suggestion defaults plus
            // any muximux.gateway.* labels the operator set.
            const gw: GatewaySite = {
              domain: r.gatewayDomain.trim(),
              backend_url: r.s.backend_url || r.s.url,
              tls: r.s.suggested_gateway?.tls ?? 'auto',
            };
            const sg = r.s.suggested_gateway;
            if (sg) {
              if (typeof sg.streaming === 'boolean') gw.streaming = sg.streaming;
              if (typeof sg.strip_frame_blockers === 'boolean') gw.strip_frame_blockers = sg.strip_frame_blockers;
              if (typeof sg.forwarded_headers === 'boolean') gw.forwarded_headers = sg.forwarded_headers;
              if (typeof sg.skip_tls_verify === 'boolean') gw.backend_skip_tls_verify = sg.skip_tls_verify;
              if (typeof sg.require_auth === 'boolean') gw.require_auth = sg.require_auth;
              if (sg.min_role) gw.min_role = sg.min_role;
              if (sg.allowed_groups && sg.allowed_groups.length > 0) gw.allowed_groups = sg.allowed_groups;
            }
            item.gateway = gw;
          }
          return item;
        });
      if (items.length === 0) return;

      const res = await importDockerSuggestions({ items });
      importResult = res;
      // On success the parent refreshes its config and closes this modal;
      // awaiting keeps the Import button busy until the refresh is done.
      if (res.success) {
        await onimported?.();
      }
    } catch (e) {
      importTopError = errorText(e, 'Import failed');
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

  // Hover-help tooltip positioning. The DiscoverModal is itself a
  // scrollable overlay so a naively-positioned tooltip clips off the
  // bottom edge when the trigger is in a row near the footer; we flip
  // it above on the fly. Same pattern as AppForm.svelte; lifting to a
  // shared component is a future cleanup that touches multiple call
  // sites and isn't worth blocking 3.1.0 on.
  function positionTooltip(trigger: HTMLElement) {
    const tooltip = trigger.querySelector('.help-tooltip') as HTMLElement | null;
    if (!tooltip) return;
    const triggerRect = trigger.getBoundingClientRect();
    const scrollParent = findScrollParent(trigger);
    const containerRect = scrollParent
      ? scrollParent.getBoundingClientRect()
      : { bottom: window.innerHeight, right: window.innerWidth, left: 0 } as DOMRect;
    const prev = tooltip.style.cssText;
    tooltip.style.cssText = 'display: block; visibility: hidden;';
    const tooltipHeight = tooltip.offsetHeight;
    const tooltipWidth = tooltip.offsetWidth;
    tooltip.style.cssText = prev;
    const roomBelow = containerRect.bottom - triggerRect.bottom;
    tooltip.classList.toggle('tooltip-above', roomBelow < tooltipHeight + 12);
    const wouldOverflowRight = triggerRect.left + tooltipWidth > containerRect.right - 8;
    tooltip.classList.toggle('tooltip-right-anchored', wouldOverflowRight);
  }

  function findScrollParent(el: HTMLElement): HTMLElement | null {
    let node: HTMLElement | null = el.parentElement;
    while (node && node !== document.body) {
      const style = getComputedStyle(node);
      if (style.overflowY === 'auto' || style.overflowY === 'scroll' ||
          style.overflowX === 'auto' || style.overflowX === 'scroll') return node;
      node = node.parentElement;
    }
    return null;
  }
</script>

{#snippet helpTip(label: string, body: string)}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <span
    class="help-trigger relative ms-1 inline-block align-middle"
    onmouseenter={(e) => positionTooltip(e.currentTarget as HTMLElement)}
  >
    <svg
      class="w-3.5 h-3.5 text-text-disabled cursor-help"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      aria-label={label}
      role="img"
    >
      <circle cx="12" cy="12" r="10" />
      <path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" />
      <line x1="12" y1="17" x2="12.01" y2="17" />
    </svg>
    <span class="help-tooltip">{body}</span>
  </span>
{/snippet}

{#if open}
  <div class="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4" role="dialog" aria-modal="true" aria-labelledby="discover-modal-title" tabindex="-1" use:focusTrap={{ onEscape: onclose }}>
    <div class="bg-bg-base border border-border rounded-lg shadow-xl w-full max-w-4xl max-h-[90vh] flex flex-col">
      <header class="px-5 py-4 border-b border-border flex items-center justify-between">
        <div>
          <h2 id="discover-modal-title" class="text-lg font-semibold text-text-primary">{m.discovery_modalTitle()}</h2>
          <p class="text-xs text-text-muted mt-0.5">
            {#if mode === 'apps'}
              {m.discovery_modalIntroApps()}
            {:else}
              {m.discovery_modalIntroGateway()}
            {/if}
          </p>
        </div>
        <button class="btn btn-secondary btn-sm" onclick={onclose} type="button">{m.common_close()}</button>
      </header>

      <div class="flex-1 overflow-y-auto p-5">
        {#if scanning}
          <div class="text-text-muted text-sm">{m.discovery_scanning()}</div>
        {:else if scanBlocked}
          <div class="notice notice-warning">
            <div class="font-medium mb-1">{m.discovery_scanBlocked()}</div>
            <div>{scanBlocked}</div>
          </div>
        {:else if scanError}
          <div role="alert" class="notice notice-danger">
            <div class="font-medium mb-1">{m.discovery_scanFailed()}</div>
            <div>{scanError}</div>
          </div>
        {:else if rows.length === 0}
          <div class="text-text-muted text-sm">
            {m.discovery_noContainers()}
          </div>
        {:else}
          {#if optedOut > 0}
            <div role="status" class="notice notice-info mb-3 text-sm">{m.discovery_optedOut({ count: optedOut })}</div>
          {/if}
          <div class="mb-3 flex items-center gap-2 text-sm">
            <label class="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" checked={allSelected} onchange={toggleAll} />
              <span class="text-text-secondary">{m.discovery_selectAll()}</span>
            </label>
            <span class="text-text-muted">·</span>
            <span class="text-text-muted">{m.discovery_selectedCount({ selected: selectedCount, total: rows.length })}</span>
          </div>

          <div class="space-y-2">
            {#each rows as row (row.s.key)}
              {@const sh = stabilityHint(row.s)}
              {@const ch = confidenceHint(row.s)}
              {@const st = statusFor(row.s.key)}
              {@const skip = skipReason(row.s)}
              <div data-testid="discover-row" class="p-3 rounded-md border border-border-subtle bg-bg-elevated
                          {row.selected ? 'ring-1 ring-accent-primary/50' : ''}">
                <div class="flex items-start gap-3">
                  <input aria-label={m.discovery_selectApp({ name: row.nameOverride || row.s.name })} type="checkbox" bind:checked={row.selected} class="mt-1" />

                  <button
                    type="button"
                    class="shrink-0 cursor-pointer rounded hover:ring-2 hover:ring-accent-primary transition-all"
                    onclick={() => openIconPicker(row.s.key)}
                    title={m.discovery_pickIconTitle()}
                    aria-label={m.discovery_pickIconFor({ name: row.nameOverride || row.s.name })}
                  >
                    <AppIcon icon={effectiveIcon(row)} name={row.nameOverride || row.s.name || 'App'} size="md" />
                  </button>

                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-2 flex-wrap">
                      <input
                        aria-label={m.discovery_appNameFor({ name: row.s.name })}
                        type="text"
                        bind:value={row.nameOverride}
                        class="font-medium text-text-primary bg-transparent border-b border-transparent hover:border-border-subtle px-1"
                      />
                      <span class="text-xs px-1.5 py-0.5 rounded cursor-help
                                   {row.s.confidence === 'high' ? 'bg-success-bg text-success-text' : ''}
                                   {row.s.confidence === 'medium' ? 'bg-info-bg text-info-text' : ''}
                                   {row.s.confidence === 'low' ? 'bg-bg-active text-text-secondary' : ''}"
                            title={ch.tip}>
                        {ch.label}
                      </span>
                      {#if skip}
                        <span class="text-xs px-1.5 py-0.5 rounded bg-warning-bg text-warning-text" data-testid="not-importable">{m.discovery_notImportable({ reason: skip })}</span>
                      {/if}
                      {#if sh.tone !== 'gray'}
                        <span class="text-xs px-1.5 py-0.5 rounded
                                     {sh.tone === 'amber' ? 'bg-warning-bg text-warning-text' : ''}
                                     {sh.tone === 'red' ? 'bg-danger-bg text-danger-text' : ''}"
                              title={sh.tip}>
                          {row.s.stability}
                        </span>
                      {/if}
                      {#if st}
                        <span class="text-xs px-1.5 py-0.5 rounded font-medium
                                     {st.status === 'created' ? 'bg-success-bg text-success-text' : ''}
                                     {st.status === 'skipped_exists' ? 'bg-info-bg text-info-text' : ''}
                                     {st.status === 'validation_failed' ? 'bg-danger-bg text-danger-text' : ''}
                                     {st.status === 'name_collision_in_batch' ? 'bg-danger-bg text-danger-text' : ''}
                                     {st.status === 'aborted_by_batch_failure' ? 'bg-warning-bg text-warning-text' : ''}"
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
                        <span class="text-text-muted">{m.discovery_urlLabel()}</span> <code>{row.s.url}</code>
                      </div>
                    {:else}
                      <div class="mt-1 text-xs text-warning-text">
                        {m.discovery_noUrlWarning()}
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
                      <div class="flex flex-wrap items-center gap-4">
                        <span class="flex items-center gap-1.5">
                          <label class="flex items-center gap-1.5 cursor-pointer">
                            <input type="checkbox" bind:checked={row.createApp} disabled={row.s.requires_input} />
                            <span class="text-text-primary">{m.discovery_addToMenu()}</span>
                          </label>
                          {@render helpTip(
                            'More info about the menu option',
                            'Shows this app in Muximux\'s navigation menu. The "Menu link" radio below picks how clicks reach the container.'
                          )}
                        </span>
                        <span class="flex items-center gap-1.5">
                          <label class="flex items-center gap-1.5 cursor-pointer">
                            <input
                              type="checkbox"
                              bind:checked={row.createGateway}
                              disabled={row.routing === 'gateway' && row.createApp}
                            />
                            <span class="text-text-primary">{m.discovery_addGatewaySite()}</span>
                            {#if row.routing === 'gateway' && row.createApp}
                              <span class="text-text-muted">(required by routing)</span>
                            {/if}
                          </label>
                          {@render helpTip(
                            'More info about the gateway option',
                            'Publishes this app on its own subdomain (e.g. sonarr.example.com) with auto-HTTPS. Independent of "Add to menu" - use either, both, or neither.'
                          )}
                        </span>
                        {#if row.createGateway}
                          <input
                            aria-label={m.discovery_gatewayDomainFor({ name: row.s.name })}
                            type="text"
                            bind:value={row.gatewayDomain}
                            placeholder="sonarr.example.com"
                            class="text-xs px-2 py-0.5 bg-bg-base border border-border-subtle rounded text-text-primary"
                          />
                        {/if}
                      </div>
                      {#if row.createApp}
                        <fieldset class="flex flex-wrap items-center gap-3 pl-6 text-text-secondary"
                                  data-testid="row-routing">
                          <legend class="text-text-muted">{m.discovery_menuLink()}</legend>
                          <label class="flex items-center gap-1 cursor-pointer">
                            <input type="radio" bind:group={row.routing} value="direct" />
                            <span title="Menu links straight to the container's URL. Requires the dashboard machine to reach the container's IP.">{m.discovery_routeDirect()}</span>
                          </label>
                          <label class="flex items-center gap-1 cursor-pointer">
                            <input type="radio" bind:group={row.routing} value="proxy" />
                            <span title="Menu links to /proxy/<slug>; Muximux reverse-proxies to the container.">{m.discovery_routeProxy()}</span>
                          </label>
                          <label class="flex items-center gap-1 cursor-pointer"
                                 title={row.gatewayDomain ? '' : m.discovery_gatewayDomainNeeded()}>
                            <input
                              type="radio"
                              bind:group={row.routing}
                              value="gateway"
                              disabled={!row.gatewayDomain.trim()}
                              onchange={(e) => { if ((e.currentTarget as HTMLInputElement).value === 'gateway') row.createGateway = true; }}
                            />
                            <span>{m.discovery_routeGateway()}</span>
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
            <span class="text-danger-text" role="alert">{importTopError}</span>
          {:else if importResult && importResult.success}
            <span class="text-success-text" role="status">{m.discovery_importSucceeded({ count: importResult.items.length })}</span>
          {:else if importResult && !importResult.success}
            <span class="text-danger-text" role="alert">{importResult.error || m.discovery_importFailedRows()}</span>
          {:else}
            <span class="text-text-muted">{m.discovery_selectedCount({ selected: selectedCount, total: rows.length })}</span>
          {/if}
        </span>
        <div class="flex gap-2">
          <button class="btn btn-secondary btn-sm" onclick={load} disabled={scanning || importing} type="button">{m.discovery_rescan()}</button>
          <button class="btn btn-primary btn-sm" onclick={runImport} disabled={importing || selectedCount === 0} type="button">
            {importing ? m.discovery_importing() : m.discovery_importSelected({ count: selectedCount })}
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
        aria-label={m.discovery_pickIcon()}
      >
        <div class="bg-bg-surface rounded-xl shadow-2xl w-full max-w-3xl max-h-[85vh] flex flex-col border border-border">
          <div class="flex items-center justify-between p-4 border-b border-border">
            <h3 class="text-lg font-semibold text-text-primary">
              {m.discovery_pickIconHeading({ name: pickingRow.nameOverride || pickingRow.s.name })}
            </h3>
            <button class="btn btn-ghost btn-icon" onclick={closeIconPicker} aria-label={m.common_close()} type="button">
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

<style>
  /* Hover-help tooltip shown next to the per-row Add to menu / Add
     gateway site checkboxes. Mirrors AppForm.svelte; the flip
     classes are toggled imperatively by positionTooltip() so we need
     :global() to defeat Svelte's CSS scoping. */
  .help-tooltip {
    display: none;
    position: absolute;
    top: calc(100% + 6px);
    inset-inline-start: 0;
    width: 280px;
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

  .help-trigger:hover > .help-tooltip,
  .help-trigger:focus-within > .help-tooltip {
    display: block;
  }

  .help-tooltip:global(.tooltip-above) {
    top: auto;
    bottom: calc(100% + 6px);
  }

  .help-tooltip:global(.tooltip-right-anchored) {
    inset-inline-start: auto;
    inset-inline-end: 0;
  }
</style>

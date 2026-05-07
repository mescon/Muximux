<script lang="ts">
  import { onMount } from 'svelte';
  import { fly } from 'svelte/transition';
  import type { App, GatewaySite, DiscoveryDockerStatus } from '$lib/types';
  import {
    listGatewaySites,
    createGatewaySite,
    updateGatewaySite,
    deleteGatewaySite,
    validateGatewaySite,
    fetchApps,
    createApp,
    fetchDiscoveryDockerStatus,
  } from '$lib/api';
  import { isAdmin } from '$lib/authStore';

  let {
    ondiscoveryconfigure,
    ondiscoveryscan,
  }: {
    ondiscoveryconfigure?: () => void;
    ondiscoveryscan?: () => void;
  } = $props();

  // Sites loaded from /api/gateway/sites. The list view is sorted by
  // domain so the order stays predictable as operators add and remove
  // entries.
  let sites = $state<GatewaySite[]>([]);
  let loading = $state(false);
  let topLevelError = $state<string | null>(null);
  let restartBanner = $state(false);
  let mismatchBanner = $state(false);

  // Apps list, used to drive the "Linked app" dropdown so the
  // operator picks an existing app instead of typing a name they
  // have to remember exactly. Loaded once on mount; refreshed after
  // we create a new app from inside the gateway form.
  let apps = $state<App[]>([]);
  // appLinkChoice drives the dropdown's three modes:
  //   ""           - keep gateway site standalone (no nav menu entry)
  //   "<app name>" - link to an existing app (form.app_name = this)
  //   "__create__" - inline-create a new app from the gateway domain
  let appLinkChoice = $state<string>('');
  // newAppName backs the inline "create new app" input. Defaults to
  // a derived name from the domain so the operator only has to tweak
  // capitalisation if they care.
  let newAppName = $state<string>('');

  // Modal state. We use one modal for both create and edit; `editing`
  // tracks the prior domain for the update path so a rename works.
  let showForm = $state(false);
  let editing = $state<string | null>(null); // null => create mode
  let formError = $state<string | null>(null);
  let formSubmitting = $state(false);
  let validationError = $state<string | null>(null);

  // Form fields. blankSite() lives at the bottom; we initialise here so
  // svelte's $state proxy is happy with a concrete object on mount.
  let form = $state<GatewaySite>(blankSite());
  let proxyHeadersRaw = $state(''); // serialized form for the textarea: "key: value" per line

  // Per-row delete confirm.
  let confirmDelete = $state<string | null>(null);

  async function load() {
    loading = true;
    topLevelError = null;
    try {
      // Load gateway sites and apps in parallel so the form is
      // immediately ready with a populated dropdown.
      const [siteList, appList] = await Promise.all([
        listGatewaySites(),
        loadApps(),
      ]);
      sites = siteList;
      sites.sort((a, b) => a.domain.localeCompare(b.domain));
      apps = appList;
    } catch (e) {
      topLevelError = e instanceof Error ? e.message : 'Failed to load gateway sites';
    } finally {
      loading = false;
    }
  }

  // Wrapped so a failed apps fetch doesn't take the whole tab down.
  async function loadApps(): Promise<App[]> {
    try {
      const list = await fetchApps();
      list.sort((a, b) => a.name.localeCompare(b.name));
      return list;
    } catch {
      // Apps list is a UX nicety here; if it fails we just render
      // the dropdown with no existing-app entries and the operator
      // can still pick "create new app" or leave it standalone.
      return [];
    }
  }

  // deriveAppNameFromDomain turns "sonarr.example.com" into "Sonarr"
  // so the create-new-app input is pre-filled with a sensible default.
  function deriveAppNameFromDomain(domain: string): string {
    if (!domain) return '';
    const first = domain.split('.')[0] ?? '';
    if (!first) return '';
    return first.charAt(0).toUpperCase() + first.slice(1);
  }

  function openCreate() {
    editing = null;
    form = blankSite();
    proxyHeadersRaw = '';
    appLinkChoice = '';
    newAppName = '';
    formError = null;
    validationError = null;
    showForm = true;
  }

  function openEdit(site: GatewaySite) {
    editing = site.domain;
    form = { ...site, proxy_headers: { ...(site.proxy_headers ?? {}) } };
    proxyHeadersRaw = serializeHeaders(site.proxy_headers ?? {});
    // If the site is linked to an app that still exists, pin that
    // selection. If app_name was set but the app has since been
    // deleted, fall back to standalone so the operator notices.
    if (site.app_name && apps.some(a => a.name === site.app_name)) {
      appLinkChoice = site.app_name;
    } else {
      appLinkChoice = '';
    }
    newAppName = '';
    formError = null;
    validationError = null;
    showForm = true;
  }

  function cancelForm() {
    showForm = false;
    editing = null;
    formError = null;
    validationError = null;
  }

  async function lintCurrent() {
    // Best-effort live validation. The form save path runs the same
    // checks server-side, so a network or session failure here just
    // means the operator finds out at Save time. We surface the
    // outage as an amber notice rather than a clean form, so the
    // operator knows the live linter is offline and the visible
    // green is not authoritative.
    const candidate = formToSite();
    try {
      const result = await validateGatewaySite(candidate);
      validationError = result.valid ? null : (result.error ?? 'Invalid site configuration');
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Could not reach validator';
      validationError = `${message} - your save will still be checked server-side.`;
    }
  }

  async function submitForm() {
    formError = null;
    formSubmitting = true;
    try {
      const candidate = formToSite();
      if (!candidate.domain || !candidate.backend_url) {
        formError = 'Domain and backend URL are required.';
        return;
      }

      // Resolve the app-link choice to a concrete app_name.
      //
      //   ""           - no link, leave app_name empty
      //   "<existing>" - link directly
      //   "__create__" - create the app first, then link
      //
      // If create-new-app fails we abort the gateway save so we don't
      // end up with a gateway site that points at a non-existent app
      // (the cross-reference validator on the server would reject it
      // anyway, but failing early gives a clearer error).
      if (appLinkChoice === '__create__') {
        const wantedName = newAppName.trim() || deriveAppNameFromDomain(candidate.domain);
        if (!wantedName) {
          formError = 'Pick a name for the new app.';
          return;
        }
        // The new app derives its URL from the gateway domain so the
        // operator doesn't have to type it twice. We default to the
        // public-facing scheme implied by the gateway TLS mode.
        const scheme = candidate.tls === 'none' ? 'http' : 'https';
        try {
          const created = await createApp({
            name: wantedName,
            url: `${scheme}://${candidate.domain}`,
            enabled: true,
          });
          candidate.app_name = created.name;
          // Refresh apps so the next open of the form sees the new entry.
          apps = await loadApps();
        } catch (e) {
          formError = e instanceof Error ? `Could not create app: ${e.message}` : 'Could not create app.';
          return;
        }
      } else {
        candidate.app_name = appLinkChoice || undefined;
      }

      const result = editing
        ? await updateGatewaySite(editing, candidate)
        : await createGatewaySite(candidate);

      if (!result.success) {
        // The 503 divergence response carries `mismatch: true` to flag
        // that Caddy is serving the candidate while disk has the prior
        // config. Pin a sticky banner so the operator restarts to
        // recover; everything else is a regular form error.
        if (result.mismatch) {
          mismatchBanner = true;
        }
        formError = result.error ?? 'Save failed.';
        return;
      }
      restartBanner = restartBanner || (result.restart_required ?? false);
      showForm = false;
      editing = null;
      await load();
    } catch (e) {
      formError = e instanceof Error ? e.message : 'Save failed';
    } finally {
      formSubmitting = false;
    }
  }

  async function handleDelete(domain: string) {
    try {
      await deleteGatewaySite(domain);
      confirmDelete = null;
      await load();
    } catch (e) {
      topLevelError = e instanceof Error ? e.message : 'Delete failed';
    }
  }

  function formToSite(): GatewaySite {
    return {
      ...form,
      proxy_headers: parseHeaders(proxyHeadersRaw),
    };
  }

  // Convert "Key: Value" lines from the textarea into a header map.
  // Blank lines and lines without a colon are skipped; trailing
  // whitespace is trimmed off both name and value.
  function parseHeaders(raw: string): Record<string, string> | undefined {
    const out: Record<string, string> = {};
    for (const line of raw.split('\n')) {
      const trimmed = line.trim();
      if (!trimmed) continue;
      const colon = trimmed.indexOf(':');
      if (colon < 0) continue;
      const k = trimmed.slice(0, colon).trim();
      const v = trimmed.slice(colon + 1).trim();
      if (k) out[k] = v;
    }
    return Object.keys(out).length > 0 ? out : undefined;
  }

  function serializeHeaders(headers: Record<string, string>): string {
    return Object.entries(headers).map(([k, v]) => `${k}: ${v}`).join('\n');
  }

  function blankSite(): GatewaySite {
    return {
      domain: '',
      backend_url: '',
      tls: 'auto',
      strip_frame_blockers: false,
      streaming: false,
      forwarded_headers: true,
    };
  }

  // Lint the form whenever the operator pauses typing. 300 ms debounce
  // is enough to feel instant while avoiding a request per keystroke.
  let lintTimer: ReturnType<typeof setTimeout> | null = null;
  function scheduleLint() {
    if (lintTimer) clearTimeout(lintTimer);
    lintTimer = setTimeout(lintCurrent, 300);
  }

  // headersPlaceholder uses a real newline (not the backslash-n
  // sequence) so the textarea renders the example on two lines. We
  // cannot inline this as a plain attribute because attribute strings
  // do not interpret escape sequences.
  const headersPlaceholder = 'X-Api-Key: your-backend-api-key\nAuthorization: Bearer ...';

  // tlsLabel describes the chosen TLS mode in plain English for the
  // table view, where we don't have room for a tooltip.
  function tlsLabel(site: GatewaySite): string {
    switch (site.tls) {
      case 'custom':
        return 'Custom cert';
      case 'none':
        return 'HTTP only';
      default:
        return 'Auto (LE)';
    }
  }

  // Discovery capability state for the "Discover from Docker" button
  // mirrored from the Apps tab. The same four-state ladder applies:
  // hidden / cta / unreachable / strategy_blocked / active.
  let discoveryStatus = $state<DiscoveryDockerStatus | null>(null);
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
      case 'active':           return 'Scan the configured Docker daemon and add containers as gateway sites (and optionally as apps).';
      default:                 return '';
    }
  });

  onMount(async () => {
    if ($isAdmin) {
      load();
      try { discoveryStatus = await fetchDiscoveryDockerStatus(); } catch { /* ignore */ }
    }
  });
</script>

{#if $isAdmin}
  <div class="space-y-6">
    <div>
      <h3 class="text-lg font-semibold text-text-primary mb-1">Gateway sites</h3>
      <p class="text-sm text-text-muted">
        Subdomains hosted by Muximux's embedded Caddy. Each site reverse-proxies to a backend on your network. WebSocket upgrades, HTTP/2, and large uploads work by default. Toggle Streaming for media servers (Plex, Jellyfin, Grafana). Strip iframe blockers when you also want to embed the site in Muximux's dashboard.
      </p>
    </div>

    {#if topLevelError}
      <div class="p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm">
        {topLevelError}
      </div>
    {/if}

    {#if restartBanner}
      <div class="p-3 rounded-lg bg-amber-500/10 border border-amber-500/30 text-amber-200 text-sm">
        Caddy isn't running yet (no TLS or gateway site was configured at startup). Your changes are saved to <code>config.yaml</code> but won't serve traffic until Muximux is restarted.
      </div>
    {/if}

    {#if mismatchBanner}
      <div class="p-3 rounded-lg bg-red-500/10 border border-red-500/30 text-red-200 text-sm">
        <strong>Configuration mismatch.</strong> Muximux's running gateway disagrees with what's in <code>config.yaml</code>. This happens when a save failed mid-reload. Restart Muximux to bring the running config back in line with disk.
      </div>
    {/if}

    <div class="flex items-center justify-between">
      <div class="text-sm text-text-muted">
        {sites.length} {sites.length === 1 ? 'site' : 'sites'} configured
      </div>
      <div class="flex gap-2">
        {#if discoveryButtonState !== 'hidden'}
          <button
            class="btn btn-sm flex items-center gap-1
                   {discoveryButtonState === 'active' || discoveryButtonState === 'cta' ? 'btn-secondary' : ''}
                   {discoveryButtonState === 'unreachable' || discoveryButtonState === 'strategy_blocked' ? 'btn-secondary opacity-50 cursor-not-allowed' : ''}"
            onclick={() => {
              if (discoveryButtonState === 'active') ondiscoveryscan?.();
              else ondiscoveryconfigure?.();
            }}
            disabled={discoveryButtonState === 'unreachable' || discoveryButtonState === 'strategy_blocked'}
            title={discoveryTooltip}
            type="button"
          >
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
        <button class="btn btn-primary btn-sm" onclick={openCreate} type="button">
          Add gateway site
        </button>
      </div>
    </div>

    {#if loading}
      <div class="text-center py-4 text-text-muted">Loading...</div>
    {:else if sites.length === 0}
      <div class="p-6 rounded-lg bg-bg-surface border border-border-subtle text-center text-sm text-text-muted">
        No gateway sites yet. Click <strong>Add gateway site</strong> to expose a backend at its own subdomain.
      </div>
    {:else}
      <div class="space-y-2">
        {#each sites as site (site.domain)}
          <div class="p-3 rounded-lg bg-bg-surface border border-border">
            <div class="flex items-center gap-3">
              <div class="flex-1 min-w-0">
                <div class="text-sm font-medium text-text-primary truncate">{site.domain}</div>
                <div class="text-xs text-text-muted truncate">{site.backend_url}</div>
              </div>
              <div class="flex items-center gap-2 text-xs text-text-secondary flex-shrink-0">
                <span class="px-2 py-0.5 rounded bg-bg-elevated border border-border-subtle">
                  {tlsLabel(site)}
                </span>
                {#if site.streaming}
                  <span class="px-2 py-0.5 rounded bg-blue-500/15 border border-blue-500/30 text-blue-300">streaming</span>
                {/if}
                {#if site.strip_frame_blockers}
                  <span class="px-2 py-0.5 rounded bg-purple-500/15 border border-purple-500/30 text-purple-300">embeddable</span>
                {/if}
                {#if site.app_name}
                  <span class="px-2 py-0.5 rounded bg-green-500/15 border border-green-500/30 text-green-300">app: {site.app_name}</span>
                {/if}
              </div>
              <button class="btn btn-secondary btn-sm" onclick={() => openEdit(site)} type="button">
                Edit
              </button>
              {#if confirmDelete === site.domain}
                <div class="flex items-center gap-1.5">
                  <button class="btn btn-danger btn-sm" onclick={() => handleDelete(site.domain)} type="button">Delete</button>
                  <button class="btn btn-secondary btn-sm" onclick={() => confirmDelete = null} type="button">Cancel</button>
                </div>
              {:else}
                <button
                  class="p-1.5 text-text-disabled hover:text-red-400 rounded transition-colors"
                  onclick={() => confirmDelete = site.domain}
                  title="Delete this gateway site"
                  type="button"
                  aria-label="Delete {site.domain}"
                >
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </button>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/if}

<!-- Create/Edit modal -->
{#if showForm}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
    onclick={cancelForm}
    onkeydown={(e) => { if (e.key === 'Escape') cancelForm(); }}
    role="presentation"
  >
    <div
      class="bg-bg-base border border-border rounded-xl shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      aria-labelledby="gateway-form-title"
      tabindex="-1"
      in:fly={{ y: 8, duration: 150 }}
    >
      <div class="p-5 border-b border-border">
        <h3 id="gateway-form-title" class="text-base font-semibold text-text-primary">
          {editing ? `Edit ${editing}` : 'Add gateway site'}
        </h3>
      </div>

      <div class="p-5 space-y-4">
        {#if formError}
          <div class="p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm">
            {formError}
          </div>
        {/if}
        {#if validationError}
          <div class="p-3 rounded-lg bg-amber-500/10 border border-amber-500/30 text-amber-200 text-sm">
            {validationError}
          </div>
        {/if}

        <div>
          <label for="gw-domain" class="block text-sm font-medium text-text-secondary mb-1">Domain</label>
          <input
            id="gw-domain"
            type="text"
            bind:value={form.domain}
            oninput={scheduleLint}
            placeholder="sonarr.example.com"
            class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
          <p class="text-xs text-text-muted mt-1">Public hostname Caddy will listen for. Must point at this server's IP.</p>
        </div>

        <div>
          <label for="gw-backend" class="block text-sm font-medium text-text-secondary mb-1">Backend URL</label>
          <input
            id="gw-backend"
            type="text"
            bind:value={form.backend_url}
            oninput={scheduleLint}
            readonly={!!form.docker_key}
            placeholder="http://sonarr:8989"
            class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 {form.docker_key ? 'opacity-70 cursor-not-allowed' : ''}"
            data-testid="gw-form-backend-url"
          />
          {#if form.docker_key}
            <p class="text-xs text-amber-300 mt-1 flex items-start gap-1.5" data-testid="gw-form-docker-locked">
              <svg class="w-4 h-4 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 11v2m0 4h.01M5 11V7a7 7 0 0114 0v4M5 11h14a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2z" />
              </svg>
              <span>Docker-managed: Backend URL refreshes from container <code class="font-mono text-text-secondary">{form.docker_key}</code>. Detach via Settings → Discovery → Currently tracked.</span>
            </p>
          {:else}
            <p class="text-xs text-text-muted mt-1">
              Where Muximux forwards requests. Private IPs (10.x, 192.168.x, Docker hostnames) are fine.
            </p>
          {/if}
        </div>

        <div>
          <label for="gw-tls" class="block text-sm font-medium text-text-secondary mb-1">TLS</label>
          <select
            id="gw-tls"
            bind:value={form.tls}
            onchange={scheduleLint}
            class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            <option value="auto">Automatic (Let's Encrypt)</option>
            <option value="custom">Custom certificate</option>
            <option value="none">HTTP only (no TLS)</option>
          </select>
          {#if form.tls === 'custom'}
            <div class="mt-3 grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label for="gw-cert" class="block text-xs text-text-muted mb-1">TLS cert path</label>
                <input
                  id="gw-cert"
                  type="text"
                  bind:value={form.tls_cert}
                  oninput={scheduleLint}
                  placeholder="/etc/ssl/example.crt"
                  class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                />
              </div>
              <div>
                <label for="gw-key" class="block text-xs text-text-muted mb-1">TLS key path</label>
                <input
                  id="gw-key"
                  type="text"
                  bind:value={form.tls_key}
                  oninput={scheduleLint}
                  placeholder="/etc/ssl/example.key"
                  class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                />
              </div>
            </div>
          {/if}
        </div>

        <div class="space-y-2">
          <label class="flex items-start gap-2 cursor-pointer text-sm">
            <input type="checkbox" bind:checked={form.streaming} class="mt-0.5" />
            <span>
              <span class="text-text-primary">Streaming</span>
              <span class="block text-xs text-text-muted">For media servers and live dashboards (Plex, Jellyfin, Grafana, Home Assistant). Disables response buffering. Most apps don't need this.</span>
            </span>
          </label>

          <label class="flex items-start gap-2 cursor-pointer text-sm">
            <input type="checkbox" bind:checked={form.strip_frame_blockers} />
            <span>
              <span class="text-text-primary">Strip iframe blockers</span>
              <span class="block text-xs text-text-muted">Drop X-Frame-Options and rewrite Content-Security-Policy so the dashboard can iframe this subdomain. Only enable for self-hosted backends you trust.</span>
            </span>
          </label>

          <label class="flex items-start gap-2 cursor-pointer text-sm">
            <input type="checkbox" bind:checked={form.forwarded_headers} />
            <span>
              <span class="text-text-primary">Forward headers</span>
              <span class="block text-xs text-text-muted">Send X-Forwarded-Proto, X-Forwarded-Host, X-Real-IP. On by default; turn off for backends that reject those headers.</span>
            </span>
          </label>
        </div>

        <div>
          <label for="gw-headers" class="block text-sm font-medium text-text-secondary mb-1">Upstream headers</label>
          <textarea
            id="gw-headers"
            bind:value={proxyHeadersRaw}
            placeholder={headersPlaceholder}
            rows="3"
            class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-xs font-mono focus:outline-none focus:ring-2 focus:ring-brand-500"
          ></textarea>
          <p class="text-xs text-text-muted mt-1">
            One <code>name: value</code> per line. Injected on the upstream request, e.g. for the backend's own API key.
          </p>
        </div>

        <div>
          <label for="gw-app" class="block text-sm font-medium text-text-secondary mb-1">Show in navigation menu</label>
          <select
            id="gw-app"
            bind:value={appLinkChoice}
            class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            <option value="">Don't add to menu (standalone gateway)</option>
            <option value="__create__">Add to menu - create new app</option>
            {#if apps.length > 0}
              <optgroup label="Link to existing app">
                {#each apps as a (a.name)}
                  <option value={a.name}>{a.name}</option>
                {/each}
              </optgroup>
            {/if}
          </select>
          {#if appLinkChoice === '__create__'}
            <div class="mt-2">
              <label for="gw-new-app-name" class="block text-xs text-text-secondary mb-1">New app name</label>
              <input
                id="gw-new-app-name"
                type="text"
                bind:value={newAppName}
                placeholder={deriveAppNameFromDomain(form.domain) || 'Sonarr'}
                class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
              <p class="text-xs text-text-muted mt-1">
                A new app pointing at <code>{form.tls === 'none' ? 'http' : 'https'}://{form.domain || 'your-domain'}</code> will be created and added to the menu when you save.
              </p>
            </div>
          {:else}
            <p class="text-xs text-text-muted mt-1">
              Standalone keeps this gateway site in Caddy only - no entry in the dashboard's navigation. Linking to an app makes it appear there with the app's icon and group.
            </p>
          {/if}
        </div>
      </div>

      <div class="p-5 border-t border-border flex justify-end gap-2">
        <button class="btn btn-secondary btn-sm" onclick={cancelForm} type="button">Cancel</button>
        <button
          class="btn btn-primary btn-sm disabled:opacity-50"
          onclick={submitForm}
          disabled={formSubmitting}
          type="button"
        >
          {formSubmitting ? 'Saving...' : (editing ? 'Save changes' : 'Add site')}
        </button>
      </div>
    </div>
  </div>
{/if}

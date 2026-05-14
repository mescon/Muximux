<script lang="ts">
  import { onMount } from 'svelte';
  import type { DiscoveryDockerConfig, DiscoveryDockerStatus } from '$lib/types';
  import {
    fetchDiscoveryDockerStatus,
    updateDiscoveryDockerConfig,
    testDiscoveryDockerConfig,
    listDockerNetworks,
  } from '$lib/api';
  import DiscoveryTrackedEntries from './DiscoveryTrackedEntries.svelte';

  // Bumped after a save so the tracked-entries panel reloads (a
  // newly-changed endpoint may strand existing tracking and we want
  // the Re-link button to show up immediately).
  let trackedRefreshKey = $state(0);

  // Live status from /api/discovery/docker/status. Populated on mount
  // and after every save / test.
  let status = $state<DiscoveryDockerStatus | null>(null);
  let loading = $state(false);
  let topLevelError = $state<string | null>(null);

  // Available Docker networks, surfaced as a chip strip + datalist
  // beneath the network_filter input so operators pick from real
  // values instead of guessing at network names. Empty array means
  // "not loaded" or "daemon unreachable" -- in both cases we degrade
  // to a plain text input. Refreshed after every save so flipping the
  // endpoint to a different daemon picks up that daemon's networks.
  let availableNetworks = $state<string[]>([]);

  // Form state. Initialised from status.endpoint etc. on first load,
  // tracked separately so the operator can edit without losing the
  // baseline. Submit copies form -> server.
  let form = $state<DiscoveryDockerConfig>(blankConfig());
  let submitting = $state(false);
  let testInFlight = $state(false);
  let testResult = $state<DiscoveryDockerStatus | null>(null);
  let lastSaveError = $state<string | null>(null);

  onMount(load);

  async function load() {
    loading = true;
    topLevelError = null;
    try {
      const s = await fetchDiscoveryDockerStatus();
      status = s;
      // Seed the form from the current status. Empty / unset fields
      // get sensible defaults so a fresh-install operator sees what
      // the documented values are.
      form = {
        enabled: s.configured,
        endpoint: s.endpoint || 'unix:///var/run/docker.sock',
        tls: { enabled: false },
        network_strategy: (s.strategy as DiscoveryDockerConfig['network_strategy']) || 'container_ip',
        host_ip: '',
        network_filter: '',
        refresh_interval: '60s',
      };
      // Refresh the available-networks list in the background. We
      // intentionally don't await this on the main path so a slow
      // daemon doesn't delay the form rendering. Failures are
      // silenced -- the form still works without the chip strip.
      void refreshAvailableNetworks();
    } catch (e) {
      topLevelError = e instanceof Error ? e.message : 'Failed to load discovery status';
    } finally {
      loading = false;
    }
  }

  // Pull the network list without blocking the form. Called on
  // load() and after a successful save so the chip strip reflects
  // whichever daemon the operator just pointed Muximux at.
  async function refreshAvailableNetworks() {
    try {
      const r = await listDockerNetworks();
      availableNetworks = r.networks ?? [];
    } catch {
      // Daemon unreachable or discovery off -- keep the chip strip
      // empty so the input falls back to plain text. The live
      // status banner already surfaces the underlying error.
      availableNetworks = [];
    }
  }

  // Click a chip → fill the input. Doesn't lint/save; the operator
  // still has to hit Save like any other field change.
  function pickNetwork(name: string) {
    form.network_filter = name;
  }

  async function save() {
    submitting = true;
    lastSaveError = null;
    try {
      const updated = await updateDiscoveryDockerConfig(form);
      status = updated;
      testResult = null;
      // Endpoint may have changed; tracked entries' endpoint_matches
      // flag could flip - reload the panel. Same reasoning for the
      // available-networks chip strip: a new daemon has a different
      // set of networks, and showing the old daemon's chips would be
      // wrong.
      trackedRefreshKey += 1;
      void refreshAvailableNetworks();
    } catch (e) {
      lastSaveError = e instanceof Error ? e.message : 'Save failed';
    } finally {
      submitting = false;
    }
  }

  async function runTest() {
    testInFlight = true;
    testResult = null;
    try {
      testResult = await testDiscoveryDockerConfig(form);
    } catch (e) {
      testResult = {
        configured: form.enabled,
        reachable: false,
        strategy_ok: false,
        last_error: e instanceof Error ? e.message : 'Test failed',
      };
    } finally {
      testInFlight = false;
    }
  }

  function blankConfig(): DiscoveryDockerConfig {
    return {
      enabled: false,
      endpoint: 'unix:///var/run/docker.sock',
      tls: { enabled: false },
      network_strategy: 'container_ip',
      host_ip: '',
      network_filter: '',
      refresh_interval: '60s',
    };
  }

  // Display helpers. The status banner picks one of four shapes
  // matching the four-state UI gating ladder.
  function statusBadge(s: DiscoveryDockerStatus | null): { tone: 'red' | 'amber' | 'green' | 'gray'; text: string } {
    if (!s) return { tone: 'gray', text: 'Loading…' };
    if (!s.configured) return { tone: 'gray', text: 'Discovery is disabled' };
    if (!s.reachable) return { tone: 'red', text: `Daemon unreachable: ${s.last_error || 'unknown error'}` };
    if (!s.strategy_ok) {
      return {
        tone: 'amber',
        text: `Connected to Docker ${s.api_version || ''} but the network strategy "${s.strategy}" cannot identify Muximux's container - set network_filter or switch to host_port.`,
      };
    }
    return {
      tone: 'green',
      text: `Connected to Docker API ${s.api_version || ''} (self-detect: ${s.self_detect_method || 'n/a'})`,
    };
  }

  let statusVisual = $derived(statusBadge(status));
  let testVisual = $derived(testResult ? statusBadge(testResult) : null);

  // Divergence banner state. Three shapes:
  //  - hidden when the running counter is 0
  //  - red 'active' when a divergence happened and we have not yet
  //    seen a clean refresh tick after it
  //  - amber 'recovered' when the most recent transition was
  //    divergence -> clean tick (recovered_at populated)
  //
  // The backend bumps the counter on caddy rollback failure and
  // stamps recovered_at on the first clean tick after that. The
  // banner stays sticky in the recovered state until a new
  // divergence flips it back to red.
  let divergenceBanner = $derived.by((): { tone: 'red' | 'amber'; text: string } | null => {
    if (!status || !status.refresh_divergences || status.refresh_divergences <= 0) return null;
    if (status.recovered_at) {
      return {
        tone: 'amber',
        text: `Caddy diverged earlier (${status.refresh_divergences} time(s)). Last divergence at ${status.last_divergence_at || 'unknown'}, recovered at ${status.recovered_at}.`,
      };
    }
    return {
      tone: 'red',
      text: `Docker refresh diverged: a Caddy reload failed and the rollback also failed. Last divergence at ${status.last_divergence_at || 'unknown'}. Gateway state may be inconsistent until the next clean refresh tick.`,
    };
  });

  // tcp:// endpoint enables the TLS section. We hide it for unix://
  // sockets since cert-based auth is not meaningful there.
  let tlsRelevant = $derived(form.endpoint.startsWith('tcp://'));
</script>

<div class="space-y-6">
  <header>
    <h2 class="text-lg font-semibold text-text-primary">Docker discovery</h2>
    <p class="text-sm text-text-muted mt-1">
      Connect Muximux to a Docker daemon to discover running containers and offer them as apps. Auto-managed apps update their URL when the container restarts. Off by default - see <a href="https://github.com/mescon/Muximux/wiki/docker-discovery" target="_blank" rel="noopener noreferrer" class="text-brand-400 hover:underline">the docs</a> for the full label / strategy reference.
    </p>
  </header>

  {#if topLevelError}
    <div class="p-3 rounded-md border border-red-500/40 bg-red-500/10 text-red-300 text-sm">{topLevelError}</div>
  {/if}

  {#if loading}
    <div class="text-text-muted text-sm">Loading…</div>
  {:else}
    <!-- Divergence banner: sticky, only when the counter > 0. Sits
         above the live status banner so an operator who checks the
         page sees the divergence first. -->
    {#if divergenceBanner}
      <div class="p-3 rounded-md border text-sm
                  {divergenceBanner.tone === 'red' ? 'border-red-500/40 bg-red-500/10 text-red-300' : 'border-amber-500/40 bg-amber-500/10 text-amber-300'}">
        <strong class="font-semibold">{divergenceBanner.tone === 'red' ? 'Gateway divergence detected' : 'Gateway recovered'}</strong>
        <div class="mt-1 text-xs">{divergenceBanner.text}</div>
      </div>
    {/if}

    <!-- Live status banner -->
    <div class="p-3 rounded-md border text-sm
                {statusVisual.tone === 'red' ? 'border-red-500/40 bg-red-500/10 text-red-300' : ''}
                {statusVisual.tone === 'amber' ? 'border-amber-500/40 bg-amber-500/10 text-amber-300' : ''}
                {statusVisual.tone === 'green' ? 'border-green-500/40 bg-green-500/10 text-green-300' : ''}
                {statusVisual.tone === 'gray' ? 'border-border bg-bg-elevated text-text-secondary' : ''}">
      {statusVisual.text}
      {#if status?.tls_warning}
        <div class="mt-1 text-xs text-amber-300">⚠ {status.tls_warning}</div>
      {/if}
    </div>

    <!-- Currently tracked: lists every app + gateway site with a
         DockerKey; per-row Detach + (when endpoint mismatches)
         Re-link buttons. Hidden when discovery is disabled because
         the listing endpoint requires the capability cache to have
         loaded - the underlying API returns an empty entries
         array though, so showing it on disabled is also fine. We
         show it always so an operator can detach orphaned tracking
         even after disabling discovery. -->
    <DiscoveryTrackedEntries refreshKey={trackedRefreshKey} />

    <!-- Form -->
    <div class="space-y-4">
      <label class="flex items-start gap-2 cursor-pointer text-sm">
        <input type="checkbox" bind:checked={form.enabled} class="mt-0.5" />
        <span>
          <span class="text-text-primary">Enable Docker discovery</span>
          <span class="block text-xs text-text-muted">
            Requires read access to the Docker daemon socket or TCP endpoint. Default off because it is privileged access.
          </span>
        </span>
      </label>

      <div>
        <label for="dd-endpoint" class="block text-sm font-medium text-text-secondary mb-1">Endpoint</label>
        <input
          id="dd-endpoint"
          type="text"
          bind:value={form.endpoint}
          placeholder="unix:///var/run/docker.sock"
          class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
        />
        <p class="text-xs text-text-muted mt-1">
          <code>unix:///var/run/docker.sock</code> for local Docker, or <code>tcp://host:2376</code> for a remote daemon (TLS recommended).
        </p>
      </div>

      <div>
        <label for="dd-strategy" class="block text-sm font-medium text-text-secondary mb-1">Network strategy</label>
        <select
          id="dd-strategy"
          bind:value={form.network_strategy}
          class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
        >
          <option value="container_ip">container_ip - use the container's docker-network IP (Muximux must share a network)</option>
          <option value="container_dns">container_dns - use the container name (resolves via docker DNS in shared network)</option>
          <option value="host_port">host_port - use the host IP and the published port (works from anywhere)</option>
          <option value="host_docker_internal">host_docker_internal - use host.docker.internal (Mac/Win Desktop, modern Linux)</option>
        </select>
        <p class="text-xs text-text-muted mt-1">
          How URLs for discovered apps are constructed. <code>container_ip</code> needs Muximux to run in a container that shares a docker network with the targets - the status above tells you whether self-detection succeeded.
        </p>
      </div>

      {#if form.network_strategy === 'host_port'}
        <div>
          <label for="dd-hostip" class="block text-sm font-medium text-text-secondary mb-1">Host IP (optional)</label>
          <input
            id="dd-hostip"
            type="text"
            bind:value={form.host_ip}
            placeholder="leave blank to use 127.0.0.1"
            class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
          <p class="text-xs text-text-muted mt-1">Set when Muximux runs on a different host than Docker (or when 127.0.0.1 isn't reachable from where Muximux runs).</p>
        </div>
      {/if}

      <div>
        <label for="dd-filter" class="block text-sm font-medium text-text-secondary mb-1">Network filter (optional)</label>
        <input
          id="dd-filter"
          type="text"
          list="dd-filter-networks"
          bind:value={form.network_filter}
          placeholder={availableNetworks.length > 0
            ? availableNetworks[0]
            : 'e.g. media (name of a Docker network)'}
          class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          data-testid="dd-filter"
        />
        {#if availableNetworks.length > 0}
          <datalist id="dd-filter-networks">
            {#each availableNetworks as net (net)}
              <option value={net}></option>
            {/each}
          </datalist>
          <div class="mt-2 flex flex-wrap items-center gap-1.5 text-xs" data-testid="dd-filter-chips">
            <span class="text-text-muted">Available on this daemon:</span>
            {#each availableNetworks as net (net)}
              <button
                type="button"
                onclick={() => pickNetwork(net)}
                class="px-2 py-0.5 rounded border text-xs transition-colors
                       {form.network_filter === net
                         ? 'bg-brand-500/20 border-brand-500/50 text-brand-200'
                         : 'bg-bg-elevated border-border-subtle text-text-secondary hover:bg-bg-hover'}"
                title="Use Docker network {net}"
              >
                {net}
              </button>
            {/each}
            {#if form.network_filter}
              <button
                type="button"
                onclick={() => pickNetwork('')}
                class="text-text-muted hover:text-text-primary underline-offset-2 hover:underline"
                title="Clear network filter"
              >
                clear
              </button>
            {/if}
          </div>
        {/if}
        <p class="text-xs text-text-muted mt-2">
          When set, only containers attached to this Docker network appear in scans.
          Required for <code>container_ip</code> / <code>container_dns</code> when Muximux runs
          on the host (not in a container). Pick a network you can see your target containers
          on - typically the same one your <code>docker compose</code> stack creates.
        </p>
      </div>

      <div>
        <label for="dd-interval" class="block text-sm font-medium text-text-secondary mb-1">Refresh interval</label>
        <input
          id="dd-interval"
          type="text"
          bind:value={form.refresh_interval}
          placeholder="60s"
          class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
        />
        <p class="text-xs text-text-muted mt-1">How often the poller checks tracked containers for IP/port changes. <code>60s</code> is a good default.</p>
      </div>

      <!-- TLS section -->
      {#if tlsRelevant}
        <details class="border border-border-subtle rounded-md p-3">
          <summary class="text-sm font-medium text-text-secondary cursor-pointer">mTLS (for tcp:// endpoints)</summary>
          <div class="mt-3 space-y-3">
            <label class="flex items-start gap-2 cursor-pointer text-sm">
              <input type="checkbox" bind:checked={form.tls.enabled} class="mt-0.5" />
              <span>
                <span class="text-text-primary">Use TLS client certificate authentication</span>
                <span class="block text-xs text-text-muted">Required for production tcp://. All three paths must be readable by the Muximux process.</span>
              </span>
            </label>
            {#if form.tls.enabled}
              <div>
                <label for="dd-tls-ca" class="block text-xs text-text-secondary mb-1">CA certificate</label>
                <input id="dd-tls-ca" type="text" bind:value={form.tls.ca_cert} placeholder="/etc/docker/ca.pem" class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500" />
              </div>
              <div>
                <label for="dd-tls-cert" class="block text-xs text-text-secondary mb-1">Client certificate</label>
                <input id="dd-tls-cert" type="text" bind:value={form.tls.client_cert} placeholder="/etc/docker/cert.pem" class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500" />
              </div>
              <div>
                <label for="dd-tls-key" class="block text-xs text-text-secondary mb-1">Client key (chmod 600)</label>
                <input id="dd-tls-key" type="text" bind:value={form.tls.client_key} placeholder="/etc/docker/key.pem" class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500" />
              </div>
            {/if}
          </div>
        </details>
      {/if}

      <!-- Test result -->
      {#if testResult && testVisual}
        <div class="p-3 rounded-md border text-sm
                    {testVisual.tone === 'red' ? 'border-red-500/40 bg-red-500/10 text-red-300' : ''}
                    {testVisual.tone === 'amber' ? 'border-amber-500/40 bg-amber-500/10 text-amber-300' : ''}
                    {testVisual.tone === 'green' ? 'border-green-500/40 bg-green-500/10 text-green-300' : ''}
                    {testVisual.tone === 'gray' ? 'border-border bg-bg-elevated text-text-secondary' : ''}">
          <span class="font-medium">Test result:</span> {testVisual.text}
        </div>
      {/if}

      {#if lastSaveError}
        <div class="p-3 rounded-md border border-red-500/40 bg-red-500/10 text-red-300 text-sm">{lastSaveError}</div>
      {/if}

      <div class="flex gap-2 justify-end">
        <button class="btn btn-secondary btn-sm" onclick={runTest} disabled={testInFlight} type="button">
          {testInFlight ? 'Testing…' : 'Test connection'}
        </button>
        <button class="btn btn-primary btn-sm" onclick={save} disabled={submitting} type="button">
          {submitting ? 'Saving…' : 'Save'}
        </button>
      </div>
    </div>
  {/if}
</div>

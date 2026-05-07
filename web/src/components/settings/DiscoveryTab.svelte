<script lang="ts">
  import { onMount } from 'svelte';
  import type { DiscoveryDockerConfig, DiscoveryDockerStatus } from '$lib/types';
  import {
    fetchDiscoveryDockerStatus,
    updateDiscoveryDockerConfig,
    testDiscoveryDockerConfig,
  } from '$lib/api';

  // Live status from /api/discovery/docker/status. Populated on mount
  // and after every save / test.
  let status = $state<DiscoveryDockerStatus | null>(null);
  let loading = $state(false);
  let topLevelError = $state<string | null>(null);

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
    } catch (e) {
      topLevelError = e instanceof Error ? e.message : 'Failed to load discovery status';
    } finally {
      loading = false;
    }
  }

  async function save() {
    submitting = true;
    lastSaveError = null;
    try {
      const updated = await updateDiscoveryDockerConfig(form);
      status = updated;
      testResult = null;
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
        text: `Connected to Docker ${s.api_version || ''} but the network strategy "${s.strategy}" cannot identify Muximux's container — set network_filter or switch to host_port.`,
      };
    }
    return {
      tone: 'green',
      text: `Connected to Docker API ${s.api_version || ''} (self-detect: ${s.self_detect_method || 'n/a'})`,
    };
  }

  let statusVisual = $derived(statusBadge(status));
  let testVisual = $derived(testResult ? statusBadge(testResult) : null);

  // tcp:// endpoint enables the TLS section. We hide it for unix://
  // sockets since cert-based auth is not meaningful there.
  let tlsRelevant = $derived(form.endpoint.startsWith('tcp://'));
</script>

<div class="space-y-6">
  <header>
    <h2 class="text-lg font-semibold text-text-primary">Docker discovery</h2>
    <p class="text-sm text-text-muted mt-1">
      Connect Muximux to a Docker daemon to discover running containers and offer them as apps. Auto-managed apps update their URL when the container restarts. Off by default — see <a href="https://github.com/mescon/Muximux/wiki/docker-discovery" target="_blank" rel="noopener noreferrer" class="text-brand-400 hover:underline">the docs</a> for the full label / strategy reference.
    </p>
  </header>

  {#if topLevelError}
    <div class="p-3 rounded-md border border-red-500/40 bg-red-500/10 text-red-300 text-sm">{topLevelError}</div>
  {/if}

  {#if loading}
    <div class="text-text-muted text-sm">Loading…</div>
  {:else}
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
          <option value="container_ip">container_ip — use the container's docker-network IP (Muximux must share a network)</option>
          <option value="container_dns">container_dns — use the container name (resolves via docker DNS in shared network)</option>
          <option value="host_port">host_port — use the host IP and the published port (works from anywhere)</option>
          <option value="host_docker_internal">host_docker_internal — use host.docker.internal (Mac/Win Desktop, modern Linux)</option>
        </select>
        <p class="text-xs text-text-muted mt-1">
          How URLs for discovered apps are constructed. <code>container_ip</code> needs Muximux to run in a container that shares a docker network with the targets — the status above tells you whether self-detection succeeded.
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
          bind:value={form.network_filter}
          placeholder="e.g. media"
          class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
        />
        <p class="text-xs text-text-muted mt-1">
          When set, only containers attached to this docker network appear in scans. Substitutes for self-detection when Muximux runs on the host.
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

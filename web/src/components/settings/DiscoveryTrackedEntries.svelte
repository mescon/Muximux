<script lang="ts">
  import { onMount } from 'svelte';
  import type { DiscoveryTrackedEntry, DiscoveryTrackedListResult } from '$lib/types';
  import { listDockerTracked, detachDockerTracked, ApiError } from '$lib/api';
  import DiscoveryRelinkModal from './DiscoveryRelinkModal.svelte';

  // Refresh signal: parent bumps refreshKey to force a reload after a
  // detach / re-link from elsewhere. Defaults to 0; any change reloads.
  let { refreshKey = 0 } = $props<{ refreshKey?: number }>();

  let result = $state<DiscoveryTrackedListResult | null>(null);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let detachInFlight = $state<string | null>(null); // key currently detaching
  let relinkKey = $state<string | null>(null); // open modal for this key

  // Reload whenever the parent bumps refreshKey OR after onMount.
  // $effect re-runs whenever refreshKey changes; the initial mount
  // is covered too because the effect runs after the component
  // initialises.
  $effect(() => {
    refreshKey;
    void load();
  });

  async function load() {
    loading = true;
    loadError = null;
    try {
      result = await listDockerTracked();
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'Failed to load tracked entries';
    } finally {
      loading = false;
    }
  }

  async function detach(entry: DiscoveryTrackedEntry) {
    if (!confirm(`Detach "${entry.name}" from Docker auto-management? Its URL will stop refreshing automatically.`)) return;
    detachInFlight = entry.key;
    try {
      await detachDockerTracked(entry.key);
      await load();
    } catch (e) {
      // Treat 404 (already detached by a concurrent caller) as
      // success since the desired state was reached. Branch on the
      // HTTP status, not the server's error message string -
      // matching message text would silently break the moment the
      // backend rewrites the error copy.
      if (e instanceof ApiError && e.status === 404) {
        // idempotent re-detach; reload to refresh the panel
        await load();
      } else {
        alert(`Detach failed: ${e instanceof Error ? e.message : String(e)}`);
        await load();
      }
    } finally {
      detachInFlight = null;
    }
  }

  function startRelink(entry: DiscoveryTrackedEntry) {
    relinkKey = entry.key;
  }

  function closeRelink() {
    relinkKey = null;
    void load(); // refresh in case re-link succeeded
  }

  function ago(iso: string | undefined): string {
    if (!iso) return 'never';
    const d = new Date(iso);
    if (isNaN(d.getTime())) return iso;
    const diffMs = Date.now() - d.getTime();
    const sec = Math.round(diffMs / 1000);
    if (sec < 60) return `${sec}s ago`;
    const min = Math.round(sec / 60);
    if (min < 60) return `${min}m ago`;
    const hr = Math.round(min / 60);
    if (hr < 24) return `${hr}h ago`;
    return d.toISOString();
  }
</script>

<section class="space-y-3" data-testid="tracked-entries">
  <div class="flex items-center justify-between">
    <h3 class="text-sm font-semibold text-text-primary">Currently tracked</h3>
    <button
      type="button"
      class="text-xs text-brand-400 hover:underline disabled:text-text-muted disabled:cursor-not-allowed"
      onclick={load}
      disabled={loading}
      data-testid="tracked-refresh"
    >
      {loading ? 'Refreshing…' : 'Refresh'}
    </button>
  </div>

  {#if loadError}
    <div class="p-3 rounded-md border border-red-500/40 bg-red-500/10 text-red-300 text-sm">{loadError}</div>
  {:else if loading && !result}
    <div class="text-sm text-text-muted">Loading tracked entries…</div>
  {:else if result && result.entries.length === 0}
    <div class="text-sm text-text-muted">
      No apps or gateway sites are linked to Docker yet. Use the Discover button on the Apps or Gateway tab to import containers.
    </div>
  {:else if result}
    <ul class="divide-y divide-border-subtle border border-border-subtle rounded-md overflow-hidden">
      {#each result.entries as e (e.kind + ':' + e.name)}
        <li class="p-3 flex items-center justify-between gap-3 text-sm
                   {e.endpoint_matches ? '' : 'bg-amber-500/5'}">
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="text-text-primary font-medium truncate">{e.name}</span>
              <span class="text-xs px-1.5 py-0.5 rounded bg-bg-elevated text-text-muted uppercase tracking-wide">{e.kind}</span>
              {#if !e.endpoint_matches}
                <span class="text-xs px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-300" title="DockerEndpoint differs from the current discovery endpoint">Endpoint changed</span>
              {/if}
            </div>
            <div class="text-xs text-text-muted mt-0.5 font-mono truncate">{e.key}</div>
            <div class="text-xs text-text-muted mt-0.5 truncate">{e.url} · last seen {ago(e.last_seen_at)}</div>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            {#if !e.endpoint_matches}
              <button
                type="button"
                class="btn btn-secondary btn-xs"
                onclick={() => startRelink(e)}
                data-testid="tracked-relink-btn"
              >Re-link</button>
            {/if}
            <button
              type="button"
              class="btn btn-secondary btn-xs text-red-400"
              onclick={() => detach(e)}
              disabled={detachInFlight === e.key}
              data-testid="tracked-detach-btn"
            >{detachInFlight === e.key ? 'Detaching…' : 'Detach'}</button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</section>

{#if relinkKey}
  <DiscoveryRelinkModal trackingKey={relinkKey} onClose={closeRelink} />
{/if}

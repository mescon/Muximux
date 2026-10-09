<script lang="ts">
  import { tick } from 'svelte';
  import type { DiscoveryQuarantinedEntry, DiscoveryTrackedEntry, DiscoveryTrackedListResult } from '$lib/types';
  import { listDockerTracked, detachDockerTracked, ApiError, errorText } from '$lib/api';
  import * as m from '$lib/paraglide/messages.js';
  import { getLocale } from '$lib/paraglide/runtime.js';
  import DiscoveryRelinkModal from './DiscoveryRelinkModal.svelte';

  // Refresh signal: parent bumps refreshKey to force a reload after a
  // detach / re-link from elsewhere. Defaults to 0; any change reloads.
  // ontrackingchanged tells the parent that tracking may have changed on the
  // server (detach, re-link) so the Settings dialog can refresh its own copy.
  let { refreshKey = 0, ontrackingchanged } = $props<{ refreshKey?: number; ontrackingchanged?: () => void }>();

  let result = $state<DiscoveryTrackedListResult | null>(null);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let detachInFlight = $state<string | null>(null); // key currently detaching
  let relinkKey = $state<string | null>(null); // open modal for this key
  let actionError = $state<string | null>(null);
  let sectionEl = $state<HTMLElement>();
  let trackedHeading = $state<HTMLElement>();
  let quarantinedHeading = $state<HTMLElement>();

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
      loadError = errorText(e, 'Failed to load tracked entries');
    } finally {
      loading = false;
    }
  }

  async function detach(entry: DiscoveryTrackedEntry) {
    if (!confirm(m.discovery_confirmDetach({ name: entry.name }))) return;
    actionError = null;
    const idx = result?.entries.findIndex((x) => x.key === entry.key && x.name === entry.name) ?? 0;
    detachInFlight = entry.key;
    try {
      await detachDockerTracked(entry.key);
      ontrackingchanged?.();
      await load();
    } catch (e) {
      // Treat 404 (already detached by a concurrent caller) as
      // success since the desired state was reached. Branch on the
      // HTTP status, not the server's error message string -
      // matching message text would silently break the moment the
      // backend rewrites the error copy.
      if (e instanceof ApiError && e.status === 404) {
        // idempotent re-detach; reload to refresh the panel
        ontrackingchanged?.();
        await load();
      } else {
        actionError = m.discovery_detachFailed({ name: entry.name, error: errorText(e, String(e)) });
        await load();
      }
    } finally {
      detachInFlight = null;
    }
    // After the buttons are enabled again, so the target can take focus.
    await focusAfterRemoval('[data-testid="tracked-detach-btn"]', idx, trackedHeading);
  }

  const quarantined = $derived(result?.quarantined ?? []);
  let removeInFlight = $state(false);

  async function focusAfterRemoval(selector: string, idx: number, heading: HTMLElement | undefined) {
    await tick();
    const btns = sectionEl?.querySelectorAll<HTMLElement>(selector);
    const target = btns && btns.length > 0 ? btns[Math.min(idx, btns.length - 1)] : (heading ?? trackedHeading);
    target?.focus();
  }

  async function removeQuarantined(keys: string[], name: string, idx: number) {
    actionError = null;
    removeInFlight = true;
    try {
      for (const key of keys) {
        try {
          await detachDockerTracked(key, 'quarantined');
        } catch (e) {
          // 404 means it is already gone, which is the state we want.
          if (!(e instanceof ApiError && e.status === 404)) {
            actionError = m.discovery_removeFailed({ name, error: errorText(e, String(e)) });
            break;
          }
        }
      }
      ontrackingchanged?.();
      await load();
    } finally {
      removeInFlight = false;
    }
    // After the buttons are enabled again, so the target can take focus.
    await focusAfterRemoval('[data-testid="quarantined-remove-btn"]', idx, quarantinedHeading);
  }

  async function removeOne(q: DiscoveryQuarantinedEntry) {
    if (!confirm(m.discovery_confirmRemove({ name: q.name }))) return;
    await removeQuarantined([q.key], q.name, quarantined.indexOf(q));
  }

  async function removeAll() {
    const keys = [...new Set(quarantined.map((q) => q.key))];
    if (!confirm(m.discovery_confirmRemoveAll({ count: quarantined.length }))) return;
    await removeQuarantined(keys, m.discovery_quarantinedTitle(), 0);
  }

  function startRelink(entry: DiscoveryTrackedEntry) {
    relinkKey = entry.key;
  }

  function closeRelink() {
    relinkKey = null;
    // The modal re-links or detaches server-side; either way the parent's
    // copy may be stale.
    ontrackingchanged?.();
    void load(); // refresh in case re-link succeeded
  }

  // Localized relative time (under 24h) or an absolute date-time.
  function when(iso: string | undefined): string {
    if (!iso) return m.discovery_neverSeen();
    const d = new Date(iso);
    if (isNaN(d.getTime())) return iso;
    const sec = Math.round((d.getTime() - Date.now()) / 1000);
    const rtf = new Intl.RelativeTimeFormat(getLocale(), { numeric: 'auto' });
    if (Math.abs(sec) < 60) return rtf.format(sec, 'second');
    const min = Math.round(sec / 60);
    if (Math.abs(min) < 60) return rtf.format(min, 'minute');
    const hr = Math.round(min / 60);
    if (Math.abs(hr) < 24) return rtf.format(hr, 'hour');
    return d.toLocaleString(getLocale());
  }
</script>

<section class="space-y-3" data-testid="tracked-entries" bind:this={sectionEl}>
  <div class="flex items-center justify-between">
    <h3 class="text-sm font-semibold text-text-primary" tabindex="-1" bind:this={trackedHeading}>Currently tracked</h3>
    <button
      type="button"
      class="text-xs text-accent-text hover:underline disabled:text-text-muted disabled:cursor-not-allowed"
      onclick={load}
      disabled={loading}
      data-testid="tracked-refresh"
    >
      {loading ? 'Refreshing…' : 'Refresh'}
    </button>
  </div>

  {#if actionError}
    <div role="alert" class="notice notice-danger" data-testid="tracked-action-error">{actionError}</div>
  {/if}

  {#if loadError}
    <div role="alert" class="notice notice-danger">{loadError}</div>
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
                   {e.endpoint_matches ? '' : 'bg-warning-bg'}">
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="text-text-primary font-medium truncate">{e.name}</span>
              <span class="text-xs px-1.5 py-0.5 rounded bg-bg-elevated text-text-muted uppercase tracking-wide">{e.kind}</span>
              {#if e.missing_since}
                <span class="text-xs px-1.5 py-0.5 rounded bg-warning-bg text-warning-text">{m.discovery_missingSince({ time: when(e.missing_since) })}</span>
              {/if}
              {#if !e.endpoint_matches}
                <span class="text-xs px-1.5 py-0.5 rounded bg-warning-bg text-warning-text" title="DockerEndpoint differs from the current discovery endpoint">Endpoint changed</span>
              {/if}
            </div>
            <div class="text-xs text-text-muted mt-0.5 font-mono truncate">{e.key}</div>
            <div class="text-xs text-text-muted mt-0.5 truncate">{e.url} · {m.discovery_lastSeen({ when: when(e.last_seen_at) })}</div>
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
              class="btn btn-secondary btn-xs text-danger-text"
              onclick={() => detach(e)}
              disabled={detachInFlight === e.key}
              data-testid="tracked-detach-btn"
            >{detachInFlight === e.key ? 'Detaching…' : 'Detach'}</button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}

  {#if quarantined.length > 0}
    <div class="notice notice-warning space-y-2" role="status" data-testid="quarantined-entries">
      <div class="flex items-center justify-between gap-3">
        <h4 class="text-sm font-semibold" tabindex="-1" bind:this={quarantinedHeading}>{m.discovery_quarantinedTitle()}</h4>
        <button type="button" class="btn btn-secondary btn-xs" onclick={removeAll} disabled={removeInFlight}
          data-testid="quarantined-remove-all-btn">{m.discovery_quarantinedRemoveAll()}</button>
      </div>
      <p class="text-xs">{m.discovery_quarantinedHint()}</p>
      <ul class="space-y-1">
        {#each quarantined as q (q.kind + ':' + q.name + ':' + q.key)}
          <li class="flex items-center justify-between gap-3 text-sm">
            <div class="min-w-0 flex-1">
              <span class="font-medium">{q.name}</span>
              <span class="text-xs uppercase tracking-wide">{q.kind}</span>
              <div class="text-xs font-mono truncate">{q.key}</div>
              <div class="text-xs">{q.reason}</div>
            </div>
            <button type="button" class="btn btn-secondary btn-xs shrink-0" onclick={() => removeOne(q)}
              disabled={removeInFlight}
              aria-label={m.discovery_quarantinedRemoveNamed({ name: q.name, kind: q.kind })}
              data-testid="quarantined-remove-btn">{m.discovery_quarantinedRemove()}</button>
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</section>

{#if relinkKey}
  <DiscoveryRelinkModal trackingKey={relinkKey} onClose={closeRelink} />
{/if}

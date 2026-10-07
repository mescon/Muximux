<script lang="ts">
  import type { DiscoveryRelinkProbeResult, DiscoveryRelinkCandidate } from '$lib/types';
  import { probeDockerRelink, confirmDockerRelink, detachDockerTracked } from '$lib/api';
  import { focusTrap } from '$lib/focusTrap';

  let { trackingKey, onClose } = $props<{ trackingKey: string; onClose: () => void }>();

  let probing = $state(true);
  let probeResult = $state<DiscoveryRelinkProbeResult | null>(null);
  let probeError = $state<string | null>(null);
  let confirmInFlight = $state(false);
  let confirmError = $state<string | null>(null);

  // Run the probe immediately on mount. trackingKey is captured at
  // render time; remounting the modal with a different key kicks
  // off a fresh probe.
  $effect(() => {
    void probe();
  });

  async function probe() {
    probing = true;
    probeError = null;
    probeResult = null;
    try {
      probeResult = await probeDockerRelink(trackingKey);
    } catch (e) {
      probeError = e instanceof Error ? e.message : 'Probe failed';
    } finally {
      probing = false;
    }
  }

  async function applyRelink(c: DiscoveryRelinkCandidate) {
    confirmInFlight = true;
    confirmError = null;
    try {
      await confirmDockerRelink({ old_key: trackingKey, new_key: c.key });
      onClose();
    } catch (e) {
      confirmError = e instanceof Error ? e.message : 'Re-link failed';
    } finally {
      confirmInFlight = false;
    }
  }

  async function detachInstead() {
    if (!confirm('Stop trying to re-link and detach this entry? Its URL will stop refreshing automatically.')) return;
    confirmInFlight = true;
    confirmError = null;
    try {
      await detachDockerTracked(trackingKey);
      onClose();
    } catch (e) {
      confirmError = e instanceof Error ? e.message : 'Detach failed';
    } finally {
      confirmInFlight = false;
    }
  }
</script>

<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
  role="dialog"
  aria-modal="true"
  aria-labelledby="relink-modal-title"
  tabindex="-1"
  use:focusTrap={{ onEscape: onClose }}
  data-testid="relink-modal"
>
  <div class="bg-bg-base border border-border rounded-lg shadow-xl max-w-2xl w-full max-h-[80vh] overflow-y-auto">
    <header class="p-4 border-b border-border-subtle flex items-center justify-between">
      <h2 id="relink-modal-title" class="text-lg font-semibold text-text-primary">Re-link tracked container</h2>
      <button
        type="button"
        class="text-text-muted hover:text-text-primary"
        onclick={onClose}
        aria-label="Close"
        data-testid="relink-modal-close"
      >×</button>
    </header>

    <div class="p-4 space-y-3 text-sm">
      <div class="text-text-muted">
        Tracked key: <code class="font-mono">{trackingKey}</code>
      </div>

      {#if probing}
        <div class="text-text-muted">Probing the current Docker endpoint…</div>
      {:else if probeError}
        <div class="p-3 rounded-md border border-red-500/40 bg-red-500/10 text-red-300">{probeError}</div>
      {:else if probeResult?.found && probeResult.container}
        {@const c = probeResult.container}
        <div class="p-3 rounded-md border border-green-500/40 bg-green-500/10 text-green-300">
          Container <strong class="font-semibold">{c.name}</strong> (image <code class="font-mono">{c.image}</code>) found on the current endpoint with the same tracking key. Re-link?
        </div>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary btn-sm" onclick={onClose} disabled={confirmInFlight}>Cancel</button>
          <button type="button" class="btn btn-primary btn-sm" onclick={() => applyRelink(c)} disabled={confirmInFlight} data-testid="relink-confirm-btn">
            {confirmInFlight ? 'Re-linking…' : 'Confirm re-link'}
          </button>
        </div>
      {:else if probeResult && !probeResult.found}
        <div class="text-text-secondary">
          No container with key <code class="font-mono">{trackingKey}</code> on the current endpoint. Pick a replacement to link this entry to:
        </div>
        {#if (probeResult.candidates ?? []).length === 0}
          <div class="text-text-muted text-sm">No running containers found on the current endpoint.</div>
        {:else}
          <ul class="divide-y divide-border-subtle border border-border-subtle rounded-md overflow-hidden">
            {#each probeResult.candidates ?? [] as c (c.key)}
              <li class="p-3 flex items-center justify-between gap-3">
                <div class="min-w-0 flex-1">
                  <div class="text-text-primary truncate">{c.name}</div>
                  <div class="text-xs text-text-muted font-mono truncate">{c.image}</div>
                  <div class="text-xs text-text-muted font-mono truncate">{c.key}</div>
                </div>
                <button
                  type="button"
                  class="btn btn-primary btn-xs shrink-0"
                  onclick={() => applyRelink(c)}
                  disabled={confirmInFlight}
                  data-testid="relink-pick-btn"
                >Link</button>
              </li>
            {/each}
          </ul>
        {/if}
        <div class="flex justify-between items-center pt-2">
          <button
            type="button"
            class="text-xs text-red-400 hover:underline disabled:text-text-muted disabled:cursor-not-allowed"
            onclick={detachInstead}
            disabled={confirmInFlight}
            data-testid="relink-detach-btn"
          >Stop trying to re-link (detach)</button>
          <button type="button" class="btn btn-secondary btn-sm" onclick={onClose} disabled={confirmInFlight}>Close</button>
        </div>
      {/if}

      {#if confirmError}
        <div class="p-2 rounded border border-red-500/40 bg-red-500/10 text-red-300 text-xs">{confirmError}</div>
      {/if}
    </div>
  </div>
</div>

<script lang="ts">
  import { onMount } from 'svelte';
  import * as m from '$lib/paraglide/messages.js';

  let { appName, action, image, uptimeOrExit, loading = false, onconfirm, oncancel }: {
    appName: string;
    action: 'stop' | 'restart';
    image: string;
    uptimeOrExit: string;
    // While an action is in flight the modal stays open in a disabled,
    // non-dismissable state so the operator sees progress and cannot
    // re-fire or cancel a running start/stop/restart.
    loading?: boolean;
    onconfirm?: () => void;
    oncancel?: () => void;
  } = $props();

  // Heading text by action. Start never shows this modal (the parent
  // checks `action !== 'start'` before opening it), so only stop and
  // restart are typed in the prop union.
  let heading = $derived(
    action === 'stop'
      ? m.docker_modal_confirm_stop({ name: appName })
      : m.docker_modal_confirm_restart({ name: appName })
  );

  // Mirror ConfirmActionModal: Esc cancels, Enter confirms. Bound on
  // the window via onMount so the keyboard works without focus inside
  // the dialog, and torn down on unmount.
  onMount(() => {
    const handler = (e: KeyboardEvent) => {
      if (loading) return; // ignore keys while an action is running
      if (e.key === 'Escape') {
        e.preventDefault();
        oncancel?.();
      }
      if (e.key === 'Enter') {
        e.preventDefault();
        onconfirm?.();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  });
</script>

<div
  class="modal-backdrop"
  data-testid="docker-confirm-modal-backdrop"
  role="presentation"
  onclick={(e) => {
    if (loading) return; // don't dismiss while an action is running
    if (e.target === e.currentTarget) oncancel?.();
  }}
>
  <div
    class="modal-dialog"
    role="dialog"
    aria-modal="true"
    aria-labelledby="docker-confirm-heading"
    aria-busy={loading}
  >
    <h2 id="docker-confirm-heading" class="modal-heading">{heading}</h2>
    <p class="modal-body">{m.docker_modal_body({ image, uptimeOrExit })}</p>
    <div class="modal-actions">
      <button type="button" class="btn" disabled={loading} onclick={() => oncancel?.()}>{m.common_cancel()}</button>
      <button type="button" class="btn btn-primary" autofocus disabled={loading} onclick={() => onconfirm?.()}>
        {#if loading}
          <span class="inline-block w-4 h-4 me-2 border-2 border-white/30 border-t-white rounded-full animate-spin align-[-2px]"></span>
        {/if}
        {m.common_confirm()}
      </button>
    </div>
  </div>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 100;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .modal-dialog {
    background: var(--bg-elevated);
    border-radius: 0.75rem;
    padding: 1.5rem;
    max-width: 28rem;
    width: 90vw;
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.3);
  }
  .modal-heading {
    margin: 0 0 0.5rem 0;
    font-size: 1.125rem;
    color: var(--text-primary);
  }
  .modal-body {
    margin: 0 0 1.25rem 0;
    color: var(--text-secondary);
    font-size: 0.875rem;
  }
  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
  }
</style>

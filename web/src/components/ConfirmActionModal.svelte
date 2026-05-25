<script lang="ts">
  import { onMount } from 'svelte';
  import type { App } from '$lib/types';
  import * as m from '$lib/paraglide/messages.js';

  let {
    app,
    onConfirm,
    onCancel,
  }: {
    app: App;
    onConfirm: () => void;
    onCancel: () => void;
  } = $props();

  const method = $derived(app.http_action_method ?? 'POST');

  onMount(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        onCancel();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  });
</script>

<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
  data-testid="confirm-modal-backdrop"
  role="presentation"
  onclick={(e) => {
    if (e.target === e.currentTarget) onCancel();
  }}
>
  <div
    role="dialog"
    aria-modal="true"
    aria-labelledby="confirm-modal-title"
    class="bg-bg-elevated border border-border rounded-lg shadow-xl max-w-md w-full mx-4 p-6 space-y-4"
  >
    <h2 id="confirm-modal-title" class="text-lg font-semibold text-text-primary">
      {m.http_action_confirm_title()}
    </h2>
    <p class="text-sm text-text-muted">
      {m.http_action_confirm_body()}
    </p>
    <div class="text-sm font-mono break-all bg-bg-base border border-border-subtle rounded px-3 py-2 text-text-primary">
      <span class="font-semibold">{method}</span>
      <span class="ml-2">{app.url}</span>
    </div>
    <div class="flex justify-end gap-2">
      <button
        type="button"
        onclick={onCancel}
        class="px-4 py-2 text-sm rounded-md border border-border-subtle text-text-primary hover:bg-bg-base"
      >
        {m.common_cancel()}
      </button>
      <button
        type="button"
        onclick={onConfirm}
        autofocus
        class="px-4 py-2 text-sm rounded-md bg-brand-500 hover:bg-brand-600 text-white"
      >
        {m.common_confirm()}
      </button>
    </div>
  </div>
</div>

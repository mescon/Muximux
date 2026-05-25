<script lang="ts">
  import * as m from '$lib/paraglide/messages.js';

  let {
    value = {},
    onChange,
  }: {
    value: Record<string, string>;
    onChange?: (next: Record<string, string>) => void;
  } = $props();

  type Row = { id: number; key: string; value: string };

  let nextId = 1;
  function initRows(v: Record<string, string>): Row[] {
    const out: Row[] = [];
    for (const k of Object.keys(v)) {
      out.push({ id: nextId++, key: k, value: v[k] });
    }
    return out;
  }

  let rows = $state<Row[]>(initRows(value));

  function flush() {
    const out: Record<string, string> = {};
    for (const r of rows) {
      const k = r.key.trim();
      if (k === '') continue;
      out[k] = r.value;
    }
    onChange?.(out);
  }

  function addRow() {
    rows = [...rows, { id: nextId++, key: '', value: '' }];
    flush();
  }

  function removeRow(id: number) {
    rows = rows.filter((r) => r.id !== id);
    flush();
  }

  function updateKey(id: number, k: string) {
    rows = rows.map((r) => (r.id === id ? { ...r, key: k } : r));
    flush();
  }

  function updateValue(id: number, v: string) {
    rows = rows.map((r) => (r.id === id ? { ...r, value: v } : r));
    flush();
  }
</script>

<div class="space-y-2">
  {#each rows as row (row.id)}
    <div class="flex gap-2">
      <input
        type="text"
        value={row.key}
        placeholder={m.app_http_action_header_name()}
        oninput={(e) => updateKey(row.id, (e.currentTarget as HTMLInputElement).value)}
        class="flex-1 min-w-0 px-2 py-1 text-sm bg-bg-elevated border border-border-subtle rounded text-text-primary placeholder-text-disabled"
      />
      <input
        type="text"
        value={row.value}
        placeholder={m.app_http_action_header_value()}
        oninput={(e) => updateValue(row.id, (e.currentTarget as HTMLInputElement).value)}
        class="flex-1 min-w-0 px-2 py-1 text-sm bg-bg-elevated border border-border-subtle rounded text-text-primary placeholder-text-disabled"
      />
      <button
        type="button"
        aria-label={m.app_http_action_remove_header()}
        title={m.app_http_action_remove_header()}
        class="px-2 py-1 text-text-muted hover:text-red-400"
        onclick={() => removeRow(row.id)}
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
  {/each}
  <button
    type="button"
    aria-label={m.app_http_action_add_header()}
    onclick={addRow}
    class="text-xs text-brand-400 hover:text-brand-300"
  >
    {m.app_http_action_add_header()}
  </button>
</div>

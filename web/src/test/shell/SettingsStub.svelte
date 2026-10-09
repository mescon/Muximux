<script lang="ts">
  // Stands in for Settings.svelte in App shell tests. requestClose blocks
  // while globalThis.__settingsBlockClose is set (an unsaved-changes
  // prompt). Every config prop it receives is recorded in
  // globalThis.__settingsConfigs (the real dialog rebases on each one).
  let { config, onsave, onclose }: {
    config: unknown;
    onsave: (c: unknown, b: unknown) => Promise<void>;
    onclose: () => void;
  } = $props();

  const g = globalThis as Record<string, unknown>;
  $effect(() => {
    ((g.__settingsConfigs ??= []) as unknown[]).push($state.snapshot(config));
  });

  export function requestClose(): boolean {
    if (g.__settingsBlockClose) return false;
    onclose();
    return true;
  }
  export function handleEscape(): boolean {
    return false;
  }
  async function save() {
    try {
      await onsave(config, config);
      onclose();
    } catch {
      // stays open, like the real dialog
    }
  }
</script>
<div data-testid="settings">
  <button data-testid="settings-save" onclick={save}>save</button>
  <button data-testid="settings-discard" onclick={() => onclose()}>discard</button>
</div>

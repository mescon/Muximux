<script lang="ts">
  // Stands in for Settings.svelte in App shell tests. requestClose blocks
  // while globalThis.__settingsBlockClose is set (an unsaved-changes prompt).
  let { config, onsave, onclose }: {
    config: unknown;
    onsave: (c: unknown, b: unknown) => Promise<void>;
    onclose: () => void;
  } = $props();

  export function requestClose(): boolean {
    if ((globalThis as Record<string, unknown>).__settingsBlockClose) return false;
    onclose();
    return true;
  }
  export function handleEscape(): boolean {
    return false;
  }
  async function save() {
    await onsave(config, config);
    onclose();
  }
</script>
<div data-testid="settings">
  <button data-testid="settings-save" onclick={save}>save</button>
</div>

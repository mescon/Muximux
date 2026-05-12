<script lang="ts">
  // Test-only stub used by vi.mock to swap in for the real IconBrowser.
  // Exposes deterministic buttons so tests can fire onselect with each
  // icon-type variant + onclose, exercising the consumer's callback
  // wiring without rendering real icon grids.
  let { onselect, onclose }: {
    onselect?: (detail: { name: string; variant: string; type: string }) => void;
    onclose?: () => void;
  } = $props();
</script>

<button data-testid="iconbrowser-stub-pick-dashboard" onclick={() => onselect?.({ name: 'radarr', variant: 'svg', type: 'dashboard' })}>pick-dashboard</button>
<button data-testid="iconbrowser-stub-pick-lucide" onclick={() => onselect?.({ name: 'rocket', variant: 'svg', type: 'lucide' })}>pick-lucide</button>
<button data-testid="iconbrowser-stub-pick-custom" onclick={() => onselect?.({ name: 'mine.svg', variant: 'svg', type: 'custom' })}>pick-custom</button>
<button data-testid="iconbrowser-stub-pick-url" onclick={() => onselect?.({ name: 'http://x', variant: '', type: 'url' })}>pick-url</button>
<button data-testid="iconbrowser-stub-cancel" onclick={() => onclose?.()}>cancel</button>

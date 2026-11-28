<script lang="ts">
  import type { App } from '../../lib/types';
  let { apps, onselect, onlogout }: {
    apps?: App[];
    onselect?: (app: App, event: MouseEvent) => void;
    onlogout?: () => void;
  } = $props();
</script>
<nav data-testid="nav">
  <button data-testid="logout" onclick={() => onlogout?.()}>logout</button>
  {#each apps ?? [] as app (app.name)}
    <button data-testid={'select-' + app.name} onclick={(event) => onselect?.(app, event)}>{app.name}</button>
  {/each}
</nav>

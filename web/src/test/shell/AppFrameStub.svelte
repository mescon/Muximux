<script lang="ts">
  let { app }: { app: { name: string } } = $props();
</script>
<div data-testid="frame-{app.name}"></div>

<script lang="ts">
  let { oncomplete }: { oncomplete: (d: unknown) => void } = $props();
</script>
<button
  data-testid="onboarding-done"
  onclick={() => oncomplete({ apps: [], navigation: {}, groups: [], theme: { family: 'default', variant: 'dark' } })}
>done</button>

<script lang="ts">
  let { onlogout }: { onlogout?: () => void } = $props();
</script>
<nav data-testid="nav"><button data-testid="logout" onclick={() => onlogout?.()}>logout</button></nav>

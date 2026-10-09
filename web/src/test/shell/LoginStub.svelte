<script lang="ts">
  let { onsuccess }: { onsuccess: () => void } = $props();
</script>
<button data-testid="login-stub" onclick={() => onsuccess()}>login</button>

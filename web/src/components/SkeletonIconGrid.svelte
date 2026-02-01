<script lang="ts">
  import Skeleton from './Skeleton.svelte';

  export let count: number = 30;
</script>

<!-- Mimics the icon browser grid -->
<div class="grid grid-cols-6 sm:grid-cols-8 md:grid-cols-10 gap-2">
  {#each Array(count) as _, i}
    <div class="aspect-square p-2 rounded-lg border border-gray-700">
      <Skeleton height="100%" />
    </div>
  {/each}
</div>

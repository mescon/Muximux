<script lang="ts">
  import Skeleton from './Skeleton.svelte';
</script>

<!-- Skeleton version of the Splash screen -->
<div class="h-full overflow-auto bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900 p-8">
  <div class="max-w-6xl mx-auto">
    <!-- Header skeleton -->
    <div class="text-center mb-12">
      <div class="flex justify-center mb-4">
        <Skeleton width="200px" height="2.5rem" />
      </div>
      <div class="flex justify-center">
        <Skeleton width="250px" height="1rem" />
      </div>
    </div>

    <!-- Group skeleton -->
    <div class="mb-8">
      <div class="mb-4 pb-2 border-b border-gray-700">
        <Skeleton width="100px" height="1.25rem" />
      </div>

      <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
        {#each Array(6) as _}
          <div class="flex flex-col items-center p-4 rounded-xl bg-gray-800/50 border border-gray-700">
            <Skeleton circle height="64px" class="mb-3" />
            <Skeleton width="80%" height="0.875rem" />
          </div>
        {/each}
      </div>
    </div>

    <!-- Second group skeleton -->
    <div class="mb-8">
      <div class="mb-4 pb-2 border-b border-gray-700">
        <Skeleton width="120px" height="1.25rem" />
      </div>

      <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
        {#each Array(4) as _}
          <div class="flex flex-col items-center p-4 rounded-xl bg-gray-800/50 border border-gray-700">
            <Skeleton circle height="64px" class="mb-3" />
            <Skeleton width="80%" height="0.875rem" />
          </div>
        {/each}
      </div>
    </div>
  </div>
</div>

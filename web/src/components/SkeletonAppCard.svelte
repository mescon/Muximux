<script lang="ts">
  import Skeleton from './Skeleton.svelte';
</script>

<!-- Mimics the app card layout in Splash screen -->
<div class="p-4 bg-gray-800/50 rounded-lg border border-gray-700">
  <div class="flex items-center gap-3">
    <!-- Icon skeleton -->
    <Skeleton circle height="40px" />

    <!-- Text skeleton -->
    <div class="flex-1 space-y-2">
      <Skeleton height="16px" width="70%" />
      <Skeleton height="12px" width="40%" />
    </div>
  </div>
</div>

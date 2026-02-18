<script lang="ts">
  let { width = '100%', height = '1rem', circle = false, class: className = '' }: {
    width?: string;
    height?: string;
    circle?: boolean;
    class?: string;
  } = $props();
</script>

<div
  class="animate-pulse bg-bg-hover {circle ? 'rounded-full' : 'rounded'} {className}"
  style="width: {circle ? height : width}; height: {height};"
  aria-hidden="true"
></div>

<style>
  @keyframes shimmer {
    0% {
      background-position: -200% 0;
    }
    100% {
      background-position: 200% 0;
    }
  }

  div {
    background: linear-gradient(
      90deg,
      rgba(55, 65, 81, 0.5) 25%,
      rgba(75, 85, 99, 0.5) 50%,
      rgba(55, 65, 81, 0.5) 75%
    );
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
  }
</style>

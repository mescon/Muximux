<script lang="ts">
  /**
   * Skeleton loading placeholder component
   * Usage:
   *   <Skeleton /> - Default bar
   *   <Skeleton circle /> - Circle
   *   <Skeleton width="100px" height="20px" /> - Custom size
   *   <Skeleton class="my-custom-class" /> - Additional classes
   */

  export let width: string = '100%';
  export let height: string = '1rem';
  export let circle: boolean = false;
  let className: string = '';
  export { className as class };
</script>

<div
  class="animate-pulse bg-gray-700/50 {circle ? 'rounded-full' : 'rounded'} {className}"
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

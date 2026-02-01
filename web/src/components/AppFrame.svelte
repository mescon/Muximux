<script lang="ts">
  import { fade } from 'svelte/transition';
  import { onMount } from 'svelte';
  import type { App } from '$lib/types';
  import { slugify } from '$lib/api';
  import { isMobileViewport, isTouchDevice } from '$lib/useSwipe';

  export let app: App;

  // Compute the effective URL - use proxy if enabled
  $: effectiveUrl = app.proxy ? `/proxy/${slugify(app.name)}/` : app.url;

  $: scale = app.scale || 1;
  $: transform = scale !== 1 ? `scale(${scale})` : '';
  $: transformOrigin = scale !== 1 ? 'top left' : '';
  $: width = scale !== 1 ? `${100 / scale}%` : '100%';
  $: height = scale !== 1 ? `${100 / scale}%` : '100%';

  // Pull-to-refresh state
  let isMobile = false;
  let hasTouch = false;
  let isPulling = false;
  let pullDistance = 0;
  let isRefreshing = false;
  let startY = 0;
  let iframeRef: HTMLIFrameElement;
  let containerRef: HTMLDivElement;

  const PULL_THRESHOLD = 80;
  const RESISTANCE = 2.5;

  onMount(() => {
    isMobile = isMobileViewport();
    hasTouch = isTouchDevice();

    const handleResize = () => { isMobile = isMobileViewport(); };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  });

  function handleTouchStart(e: TouchEvent) {
    if (isRefreshing || !hasTouch || !isMobile) return;
    startY = e.touches[0].clientY;
    isPulling = true;
  }

  function handleTouchMove(e: TouchEvent) {
    if (!isPulling || isRefreshing) return;

    const currentY = e.touches[0].clientY;
    const delta = (currentY - startY) / RESISTANCE;

    // Only allow pull down
    if (delta > 0) {
      pullDistance = Math.min(delta, PULL_THRESHOLD * 1.5);
    } else {
      pullDistance = 0;
    }
  }

  async function handleTouchEnd() {
    if (!isPulling) return;

    if (pullDistance >= PULL_THRESHOLD && !isRefreshing) {
      isRefreshing = true;
      pullDistance = 60; // Hold at indicator position

      // Refresh the iframe
      if (iframeRef) {
        iframeRef.src = iframeRef.src;
      }

      // Wait a bit then reset
      setTimeout(() => {
        isRefreshing = false;
        pullDistance = 0;
      }, 1000);
    } else {
      pullDistance = 0;
    }

    isPulling = false;
  }

  $: pullProgress = Math.min(pullDistance / PULL_THRESHOLD, 1);
  $: showPullIndicator = pullDistance > 10 || isRefreshing;
</script>

<div
  bind:this={containerRef}
  class="w-full h-full overflow-hidden bg-white relative"
  in:fade={{ duration: 150, delay: 50 }}
  on:touchstart={handleTouchStart}
  on:touchmove={handleTouchMove}
  on:touchend={handleTouchEnd}
>
  <!-- Pull-to-refresh indicator -->
  {#if showPullIndicator && isMobile}
    <div
      class="absolute top-0 left-0 right-0 flex justify-center items-center z-10 bg-gray-100 transition-all overflow-hidden"
      style="height: {pullDistance}px"
    >
      <div
        class="flex items-center gap-2 text-gray-600"
        style="opacity: {pullProgress}"
      >
        {#if isRefreshing}
          <svg class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          <span class="text-sm font-medium">Refreshing...</span>
        {:else}
          <svg
            class="w-5 h-5 transition-transform"
            style="transform: rotate({pullProgress * 180}deg)"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 14l-7 7m0 0l-7-7m7 7V3" />
          </svg>
          <span class="text-sm font-medium">
            {pullProgress >= 1 ? 'Release to refresh' : 'Pull to refresh'}
          </span>
        {/if}
      </div>
    </div>
  {/if}

  <iframe
    bind:this={iframeRef}
    src={effectiveUrl}
    title={app.name}
    class="app-frame"
    style:transform="{transform} translateY({pullDistance}px)"
    style:transform-origin={transformOrigin}
    style:width
    style:height
    sandbox="allow-forms allow-same-origin allow-pointer-lock allow-scripts allow-downloads allow-popups allow-modals allow-top-navigation"
    allowfullscreen
  ></iframe>
</div>

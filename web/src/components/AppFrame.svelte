<script lang="ts">
  import { onMount } from 'svelte';
  import { getEffectiveUrl, type App } from '$lib/types';
  import { resolvePermissions } from '$lib/constants';
  import { isMobileViewport, isTouchDevice } from '$lib/useSwipe';
  import * as m from '$lib/paraglide/messages.js';

  let { app }: { app: App } = $props();

  // Allowlist the iframe src so `javascript:` / `data:` URLs in app.url
  // cannot execute in Muximux's origin via the `allow-same-origin`
  // sandbox token (findings.md H4). Same-origin paths (proxied apps,
  // which arrive as `/proxy/...`) and http/https URLs are accepted;
  // anything else falls back to `about:blank` so the iframe becomes
  // inert rather than a stored-XSS pivot.
  function safeIframeSrc(raw: string): string {
    if (!raw) return 'about:blank';
    if (raw.startsWith('/') && !raw.startsWith('//') && !raw.startsWith('/\\')) {
      return raw;
    }
    try {
      const u = new URL(raw, window.location.href);
      if (u.protocol === 'http:' || u.protocol === 'https:') {
        return u.toString();
      }
    } catch {
      /* fall through */
    }
    return 'about:blank';
  }

  let effectiveUrl = $derived(safeIframeSrc(getEffectiveUrl(app)));

  // Build the iframe allow attribute from configured permissions.
  // For proxied apps the iframe is same-origin, so 'self' is sufficient.
  // For non-proxied cross-origin apps we delegate to the app's specific origin.
  let allowAttr = $derived.by(() => {
    const perms = resolvePermissions(app.permissions);
    if (perms.length === 0) return undefined;
    let origin = "'self'";
    if (!app.proxyUrl) {
      try {
        origin = "'self' " + new URL(app.url).origin;
      } catch {
        origin = "'self'";
      }
    }
    return perms.map(p => `${p} ${origin}`).join('; ');
  });

  let scale = $derived(app.scale || 1);
  let transform = $derived(scale !== 1 ? `scale(${scale})` : '');
  let transformOrigin = $derived(scale !== 1 ? 'top left' : '');
  let width = $derived(scale !== 1 ? `${100 / scale}%` : '100%');
  let height = $derived(scale !== 1 ? `${100 / scale}%` : '100%');

  // Loading / error state
  let loadError = $state(false);
  let isLoading = $state(true);
  let iframeReady = $state(false);
  let loadTimeout: ReturnType<typeof setTimeout>;

  // Pull-to-refresh state
  let isMobile = $state(false);
  let hasTouch = $state(false);
  let isPulling = $state(false);
  let pullDistance = $state(0);
  let isRefreshing = $state(false);
  let startY = $state(0);
  let iframeRef = $state<HTMLIFrameElement | undefined>(undefined);
  let containerRef = $state<HTMLDivElement | undefined>(undefined);

  const PULL_THRESHOLD = 80;
  const RESISTANCE = 2.5;

  function handleIframeLoad() {
    clearTimeout(loadTimeout);
    loadError = false;
    // Brief delay lets the loaded page paint its own background before we
    // reveal the iframe, preventing a white flash on dark themes.
    requestAnimationFrame(() => {
      iframeReady = true;
      isLoading = false;
    });
  }

  function handleIframeError() {
    clearTimeout(loadTimeout);
    isLoading = false;
    loadError = true;
  }

  function retryLoad() {
    loadError = false;
    isLoading = true;
    iframeReady = false;
    loadTimeout = setTimeout(() => {
      if (isLoading) {
        isLoading = false;
        loadError = true;
      }
    }, 30000);
    if (iframeRef) iframeRef.src = effectiveUrl;
  }

  onMount(() => {
    isMobile = isMobileViewport();
    hasTouch = isTouchDevice();

    loadTimeout = setTimeout(() => {
      if (isLoading) {
        isLoading = false;
        loadError = true;
      }
    }, 30000);

    const handleResize = () => { isMobile = isMobileViewport(); };
    window.addEventListener('resize', handleResize);
    return () => {
      clearTimeout(loadTimeout);
      window.removeEventListener('resize', handleResize);
    };
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

      // Refresh the iframe and clear the spinner when the new load
      // actually lands (or the safety timeout fires). Previously
      // isRefreshing was cleared unconditionally after 1 s regardless
      // of whether the iframe had actually reloaded, which lied to
      // the user on slow or broken apps (findings.md L12).
      const frame = iframeRef;
      if (frame) {
        const onLoad = () => {
          isRefreshing = false;
          pullDistance = 0;
          frame.removeEventListener('load', onLoad);
          clearTimeout(safety);
        };
        const safety = setTimeout(() => {
          isRefreshing = false;
          pullDistance = 0;
          frame.removeEventListener('load', onLoad);
        }, 10_000);
        frame.addEventListener('load', onLoad);
        frame.src = frame.src;
      } else {
        // Rare: touch gesture without a frame reference. Reset the
        // overlay immediately rather than leaving it stuck.
        isRefreshing = false;
        pullDistance = 0;
      }
    } else {
      pullDistance = 0;
    }

    isPulling = false;
  }

  let pullProgress = $derived(Math.min(pullDistance / PULL_THRESHOLD, 1));
  let showPullIndicator = $derived(pullDistance > 10 || isRefreshing);
</script>

<div
  bind:this={containerRef}
  class="w-full h-full overflow-hidden bg-[var(--bg-base)] relative"
  role="application"
  ontouchstart={handleTouchStart}
  ontouchmove={handleTouchMove}
  ontouchend={handleTouchEnd}
>
  <!-- Pull-to-refresh indicator -->
  {#if showPullIndicator && isMobile}
    <div
      class="absolute top-0 left-0 right-0 flex justify-center items-center z-10 bg-bg-elevated transition-all overflow-hidden"
      style="height: {pullDistance}px"
    >
      <div
        class="flex items-center gap-2 text-text-disabled"
        style="opacity: {pullProgress}"
      >
        {#if isRefreshing}
          <svg class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          <span class="text-sm font-medium">{m.appFrame_refreshing()}</span>
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
            {pullProgress >= 1 ? m.appFrame_releaseToRefresh() : m.appFrame_pullToRefresh()}
          </span>
        {/if}
      </div>
    </div>
  {/if}

  <iframe
    data-app={app.name}
    bind:this={iframeRef}
    src={effectiveUrl}
    title={app.name}
    class="app-frame"
    class:app-frame-ready={iframeReady}
    style:transform="{transform} translateY({pullDistance}px)"
    style:transform-origin={transformOrigin}
    style:width
    style:height
    sandbox="allow-forms allow-same-origin allow-pointer-lock allow-scripts allow-downloads allow-popups allow-popups-to-escape-sandbox allow-modals"
    allow={allowAttr}
    referrerpolicy="no-referrer-when-downgrade"
    allowfullscreen
    onload={handleIframeLoad}
    onerror={handleIframeError}
  ></iframe>

  {#if loadError}
    <div class="absolute inset-0 flex flex-col items-center justify-center gap-3" style="background: var(--bg-primary);">
      <svg class="w-10 h-10" style="color: var(--text-muted); opacity: 0.4;" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
      </svg>
      <p class="text-sm font-medium" style="color: var(--text-secondary);">{m.error_appLoadFailed({ appName: app.name })}</p>
      <p class="text-xs" style="color: var(--text-muted);">{effectiveUrl}</p>
      <button
        class="mt-2 px-4 py-1.5 text-sm rounded-lg transition-colors"
        style="background: var(--bg-surface); color: var(--text-primary); border: 1px solid var(--border-default);"
        onclick={retryLoad}
      >
        {m.common_retry()}
      </button>
    </div>
  {/if}

  {#if isLoading}
    <div class="absolute inset-0 flex items-center justify-center" style="background: var(--bg-primary);">
      <svg class="w-6 h-6 animate-spin" style="color: var(--text-muted);" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
    </div>
  {/if}
</div>

<script lang="ts">
  import { healthData } from '$lib/healthStore';
  import type { HealthStatus } from '$lib/api';
  import { triggerHealthCheck } from '$lib/api';
  import * as m from '$lib/paraglide/messages';

  let { appName, showTooltip = true, size = 'sm' }: {
    appName: string;
    showTooltip?: boolean;
    size?: 'sm' | 'md' | 'lg';
  } = $props();

  let checking = $state(false);
  let tooltipVisible = $state(false);
  let tooltipX = $state(0);
  let tooltipY = $state(0);
  let dotEl = $state<HTMLElement | undefined>(undefined);
  let host = $state<HTMLElement | null>(null);
  let tipEl = $state<HTMLElement | undefined>(undefined);
  const tipId = `health-tip-${Math.random().toString(36).slice(2, 9)}`;

  // Subscribe to health data
  let health = $derived($healthData.get(appName) || null);
  let status = $derived<HealthStatus>(health?.status || 'unknown');

  const sizeClasses = {
    sm: 'w-2 h-2',
    md: 'w-3 h-3',
    lg: 'w-4 h-4',
  };

  // Shapes live in app.css: circle = healthy, diamond = unhealthy, hollow ring = unknown.
  const shapeClass: Record<string, string> = {
    healthy: 'health-dot-healthy',
    unhealthy: 'health-dot-unhealthy',
  };

  function getStatusClass(s: HealthStatus): string {
    return shapeClass[s] ?? 'health-dot-unknown';
  }

  function getStatusLabel(s: HealthStatus): string {
    if (s === 'healthy') return m.health_statusHealthy();
    if (s === 'unhealthy') return m.health_statusUnhealthy();
    return m.health_statusUnknown();
  }

  // Inside a host (nav item, splash card) the host already carries the app name, so the dot
  // names the status only; a standalone dot names the app too.
  let label = $derived(
    host
      ? m.health_statusOnly({ status: getStatusLabel(status) })
      : m.health_label({ app: appName, status: getStatusLabel(status) })
  );

  function formatResponseTime(ms: number): string {
    if (ms < 1000) {
      return `${Math.round(ms)}ms`;
    }
    return `${(ms / 1000).toFixed(1)}s`;
  }

  function formatLastCheck(timestamp: string): string {
    if (!timestamp) return m.health_never();
    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffSecs = Math.floor(diffMs / 1000);

    if (diffSecs < 60) return `${diffSecs}s ago`;
    if (diffSecs < 3600) return `${Math.floor(diffSecs / 60)}m ago`;
    if (diffSecs < 86400) return `${Math.floor(diffSecs / 3600)}h ago`;
    return date.toLocaleString();
  }

  // Pointer exits hide after a short grace period so the pointer can cross the gap onto the
  // (portaled) tooltip to reach "Check now"; keyboard blur and Escape hide at once.
  const HIDE_DELAY_MS = 80;
  let hideTimer: ReturnType<typeof setTimeout> | undefined;

  function cancelHide() {
    clearTimeout(hideTimer);
    hideTimer = undefined;
  }

  function scheduleHide() {
    cancelHide();
    hideTimer = setTimeout(hideTip, HIDE_DELAY_MS);
  }

  $effect(() => cancelHide);

  function showTip() {
    cancelHide();
    if (!showTooltip || !health || !dotEl) return;
    const rect = dotEl.getBoundingClientRect();
    tooltipX = rect.left + rect.width / 2;
    tooltipY = rect.top;
    tooltipVisible = true;
  }

  function hideTip() {
    cancelHide();
    tooltipVisible = false;
  }

  // Only keyboard focus opens the tooltip: a mouse click also focuses the host link or
  // button, and that must not pop the tooltip open (or leave it stuck there).
  function isKeyboardFocus(el: HTMLElement): boolean {
    try {
      return el.matches(':focus-visible');
    } catch {
      return false;
    }
  }

  function onHostFocus(e: Event) {
    if (isKeyboardFocus(e.currentTarget as HTMLElement)) showTip();
  }

  function onDotFocus(e: FocusEvent) {
    if (isKeyboardFocus(e.currentTarget as HTMLElement)) showTip();
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && tooltipVisible) {
      e.stopPropagation();
      hideTip();
    }
  }

  // Keyboard path: when the dot sits inside a focusable host (a nav item button), follow the
  // host's focus; otherwise (Splash) the dot is focusable itself. Pressing the host (a click
  // that switches apps) always closes the tooltip.
  $effect(() => {
    if (!dotEl) return;
    const h = dotEl.parentElement?.closest<HTMLElement>('button, a, [tabindex]:not([tabindex="-1"])') ?? null;
    host = h;
    if (!h) return;
    h.addEventListener('focus', onHostFocus);
    h.addEventListener('blur', hideTip);
    h.addEventListener('keydown', onKey);
    h.addEventListener('pointerdown', hideTip);
    h.addEventListener('click', hideTip);
    return () => {
      h.removeEventListener('focus', onHostFocus);
      h.removeEventListener('blur', hideTip);
      h.removeEventListener('keydown', onKey);
      h.removeEventListener('pointerdown', hideTip);
      h.removeEventListener('click', hideTip);
    };
  });

  // While open, anything that is not the dot or the tooltip dismisses it: a press elsewhere,
  // Escape, scrolling (the fixed tooltip would drift from its dot), or losing the window.
  $effect(() => {
    if (!tooltipVisible) return;
    const onPointerDown = (e: Event) => {
      const t = e.target as Node | null;
      if (t && (dotEl?.contains(t) || tipEl?.contains(t))) return;
      hideTip();
    };
    const onDocKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') hideTip();
    };
    document.addEventListener('pointerdown', onPointerDown, true);
    document.addEventListener('keydown', onDocKey, true);
    window.addEventListener('scroll', hideTip, true);
    window.addEventListener('blur', hideTip);
    return () => {
      document.removeEventListener('pointerdown', onPointerDown, true);
      document.removeEventListener('keydown', onDocKey, true);
      window.removeEventListener('scroll', hideTip, true);
      window.removeEventListener('blur', hideTip);
    };
  });

  // A different app (or a disabled tooltip) never inherits an open tooltip.
  $effect(() => {
    void appName;
    if (!showTooltip) hideTip();
    return hideTip;
  });

  // Focus sits on the host, so the host (not the dot) is described by the open tooltip.
  // Only presence matters here, so the effect does not re-run on every health poll.
  let hasHealth = $derived(!!health);
  $effect(() => {
    const h = host;
    if (!h || !tooltipVisible || !hasHealth) return;
    const prev = h.getAttribute('aria-describedby');
    h.setAttribute('aria-describedby', prev ? `${prev} ${tipId}` : tipId);
    return () => {
      if (prev) h.setAttribute('aria-describedby', prev);
      else h.removeAttribute('aria-describedby');
    };
  });

  // A standalone dot is a tab stop only when focusing it can open something.
  let standaloneFocusable = $derived(!host && showTooltip && !!health);

  // Render the tooltip under document.body so it never sits inside the host button: otherwise
  // the button's accessible name absorbs the tooltip text and "Check now" nests in a button.
  function portal(node: HTMLElement) {
    document.body.appendChild(node);
    return {
      destroy() {
        node.remove();
      },
    };
  }

  async function handleCheckNow(e: MouseEvent) {
    e.stopPropagation();
    if (checking) return;
    checking = true;
    try {
      const newHealth = await triggerHealthCheck(appName);
      healthData.update((data) => {
        data.set(appName, newHealth);
        return data;
      });
    } catch (e) {
      console.error('Health check failed:', e);
    } finally {
      checking = false;
    }
  }
</script>

<div class="inline-flex items-center">
  <!-- Standalone (no focusable host, as in Splash) the dot is the only keyboard path to its tooltip, so it takes focus and handles Escape. -->
  <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
  <span
    bind:this={dotEl}
    role="img"
    aria-label={label}
    aria-describedby={!host && tooltipVisible && health ? tipId : undefined}
    tabindex={standaloneFocusable ? 0 : undefined}
    onfocus={standaloneFocusable ? onDotFocus : undefined}
    onmouseenter={showTip}
    onmouseleave={scheduleHide}
    onblur={host ? undefined : hideTip}
    onkeydown={standaloneFocusable ? onKey : undefined}
    class="health-hit inline-block {sizeClasses[size]} {getStatusClass(status)}"
  ></span>
  <!-- Portal container: moved to document.body on mount; the tooltip is position: fixed. -->
  <div class="health-tooltip-portal" use:portal>
    {#if tooltipVisible && health}
      <div
        class="health-tooltip"
        role="tooltip"
        bind:this={tipEl}
        style="left: {tooltipX}px; top: {tooltipY}px;"
        onmouseenter={cancelHide}
        onmouseleave={scheduleHide}
      >
        <!-- The description covers the status rows only, not the "Check now" button text. -->
        <div id={tipId}>
          <div class="flex items-center justify-between mb-1">
            <span class="font-medium {status === 'healthy' ? 'text-success-text' : status === 'unhealthy' ? 'text-danger-text' : 'text-text-muted'}">
              {getStatusLabel(status)}
            </span>
            {#if health.check_count > 0}
              <span class="badge {status === 'unhealthy' ? 'badge-error' : status === 'healthy' ? 'badge-success' : 'badge-default'}">{health.uptime_percent.toFixed(0)}%</span>
            {/if}
          </div>

          {#if health.response_time_ms > 0}
            <div class="health-detail-row">
              {m.health_response()}: {formatResponseTime(health.response_time_ms)}
            </div>
          {/if}

          {#if health.check_count > 0}
            <div class="health-detail-row">
              {m.health_uptime()}: {health.success_count}/{health.check_count}
            </div>
          {/if}

          <div class="health-detail-row">
            {m.health_checked()}: {formatLastCheck(health.last_check)}
          </div>

          {#if health.last_error}
            <div class="health-error" title={health.last_error}>
              {health.last_error}
            </div>
          {/if}
        </div>

        <button
          class="health-check-btn"
          onclick={handleCheckNow}
          disabled={checking}
        >
          {checking ? m.health_checking() : m.health_checkNow()}
        </button>

        <!-- Arrow -->
        <div class="health-tooltip-arrow"></div>
      </div>
    {/if}
  </div>
</div>

<style>
  /* Invisible ~20px hover target around the dot. The pseudo-element belongs to the dot, so
     hover lands on it, a click still bubbles to the host link, and layout does not move. */
  .health-hit {
    position: relative;
  }
  .health-hit::before {
    content: '';
    position: absolute;
    inset: -6px;
  }
  .health-tooltip {
    position: fixed;
    z-index: 99999;
    transform: translate(-50%, -100%) translateY(-8px);
    padding: 8px 12px;
    background: var(--bg-elevated);
    border: 1px solid var(--border-default);
    border-radius: 8px;
    box-shadow: var(--shadow-lg);
    min-width: 180px;
    font-size: 12px;
    pointer-events: auto;
  }
  .health-tooltip-arrow {
    position: absolute;
    top: 100%;
    left: 50%;
    transform: translateX(-50%);
    border: 5px solid transparent;
    border-top-color: var(--border-default);
  }
  .health-detail-row {
    color: var(--text-muted);
  }
  .health-error {
    margin-top: 4px;
    font-size: 10px;
    color: var(--danger-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 200px;
  }
  .health-check-btn {
    margin-top: 8px;
    width: 100%;
    text-align: center;
    font-size: 10px;
    color: var(--accent-text);
    background: none;
    border: none;
    cursor: pointer;
    padding: 0;
  }
  .health-check-btn:disabled {
    opacity: 0.5;
    cursor: default;
  }
</style>

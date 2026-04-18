<script lang="ts">
  import { healthData } from '$lib/healthStore';
  import type { HealthStatus } from '$lib/api';
  import { triggerHealthCheck } from '$lib/api';

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

  // Subscribe to health data
  let health = $derived($healthData.get(appName) || null);

  const sizeClasses = {
    sm: 'w-2 h-2',
    md: 'w-3 h-3',
    lg: 'w-4 h-4',
  };

  function getStatusClass(status: HealthStatus): string {
    switch (status) {
      case 'healthy':
        return 'health-dot-healthy';
      case 'unhealthy':
        return 'health-dot-unhealthy';
      default:
        return 'health-dot-unknown';
    }
  }

  function getStatusLabel(status: HealthStatus): string {
    switch (status) {
      case 'healthy':
        return 'Healthy';
      case 'unhealthy':
        return 'Unhealthy';
      default:
        return 'Unknown';
    }
  }

  function formatResponseTime(ms: number): string {
    if (ms < 1000) {
      return `${Math.round(ms)}ms`;
    }
    return `${(ms / 1000).toFixed(1)}s`;
  }

  function formatLastCheck(timestamp: string): string {
    if (!timestamp) return 'Never';
    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffSecs = Math.floor(diffMs / 1000);

    if (diffSecs < 60) return `${diffSecs}s ago`;
    if (diffSecs < 3600) return `${Math.floor(diffSecs / 60)}m ago`;
    if (diffSecs < 86400) return `${Math.floor(diffSecs / 3600)}h ago`;
    return date.toLocaleString();
  }

  function showTip() {
    if (!showTooltip || !health || !dotEl) return;
    const rect = dotEl.getBoundingClientRect();
    tooltipX = rect.left + rect.width / 2;
    tooltipY = rect.top;
    tooltipVisible = true;
  }

  function hideTip() {
    tooltipVisible = false;
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

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="inline-flex items-center"
  onmouseenter={showTip}
  onmouseleave={hideTip}
>
  <!-- Status dot -->
  <span
    bind:this={dotEl}
    class="rounded-full {sizeClasses[size]} {getStatusClass(health?.status || 'unknown')}"
  ></span>
</div>

<!-- Fixed-position tooltip — rendered outside all overflow/stacking contexts -->
{#if tooltipVisible && health}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="health-tooltip"
    style="left: {tooltipX}px; top: {tooltipY}px;"
    onmouseenter={showTip}
    onmouseleave={hideTip}
  >
    <div class="flex items-center justify-between mb-1">
      <span class="font-medium health-status-label {getStatusClass(health.status)}-text">
        {getStatusLabel(health.status)}
      </span>
      {#if health.check_count > 0}
        <span class="health-uptime-badge {getStatusClass(health.status)}">
          {health.uptime_percent.toFixed(0)}%
        </span>
      {/if}
    </div>

    {#if health.response_time_ms > 0}
      <div class="health-detail-row">
        Response: {formatResponseTime(health.response_time_ms)}
      </div>
    {/if}

    {#if health.check_count > 0}
      <div class="health-detail-row">
        Uptime: {health.success_count}/{health.check_count} checks
      </div>
    {/if}

    <div class="health-detail-row">
      Checked: {formatLastCheck(health.last_check)}
    </div>

    {#if health.last_error}
      <div class="health-error" title={health.last_error}>
        {health.last_error}
      </div>
    {/if}

    <button
      class="health-check-btn"
      onclick={handleCheckNow}
      disabled={checking}
    >
      {checking ? 'Checking...' : 'Check Now'}
    </button>

    <!-- Arrow -->
    <div class="health-tooltip-arrow"></div>
  </div>
{/if}

<style>
  .health-dot-healthy {
    background: var(--status-success);
  }
  .health-dot-unhealthy {
    background: var(--status-error);
  }
  .health-dot-unknown {
    background: var(--bg-active);
  }
  .health-dot-healthy-text {
    color: var(--status-success);
  }
  .health-dot-unhealthy-text {
    color: var(--status-error);
  }
  .health-dot-unknown-text {
    color: var(--text-muted);
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
  .health-uptime-badge {
    border-radius: 9999px;
    padding: 1px 6px;
    font-size: 10px;
    color: white;
  }
  .health-error {
    margin-top: 4px;
    font-size: 10px;
    color: var(--status-error);
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
    color: var(--accent-primary);
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

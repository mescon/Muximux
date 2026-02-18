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

  // Subscribe to health data
  let health = $derived($healthData.get(appName) || null);

  const sizeClasses = {
    sm: 'w-2 h-2',
    md: 'w-3 h-3',
    lg: 'w-4 h-4',
  };

  function getStatusColor(status: HealthStatus): string {
    switch (status) {
      case 'healthy':
        return 'bg-green-500';
      case 'unhealthy':
        return 'bg-red-500';
      default:
        return 'bg-bg-active';
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

<div class="relative group/health inline-flex items-center">
  <!-- Status dot -->
  <span
    class="rounded-full {sizeClasses[size]} {getStatusColor(health?.status || 'unknown')}"
  ></span>

  <!-- Tooltip -->
  {#if showTooltip && health}
    <div
      class="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 px-3 py-2
             bg-bg-base border border-border rounded-lg shadow-lg
             opacity-0 invisible group-hover/health:opacity-100 group-hover/health:visible
             transition-all duration-200 z-50 min-w-[180px] text-xs"
    >
      <div class="flex items-center justify-between mb-1">
        <span class="font-medium text-text-primary">{getStatusLabel(health.status)}</span>
        <span class="rounded-full px-1.5 py-0.5 text-[10px] {getStatusColor(health.status)} text-white">
          {health.uptime_percent.toFixed(1)}%
        </span>
      </div>

      {#if health.response_time_ms > 0}
        <div class="text-text-muted">
          Response: {formatResponseTime(health.response_time_ms)}
        </div>
      {/if}

      <div class="text-text-muted">
        Checked: {formatLastCheck(health.last_check)}
      </div>

      {#if health.last_error}
        <div class="text-red-400 mt-1 text-[10px] truncate max-w-[200px]" title={health.last_error}>
          {health.last_error}
        </div>
      {/if}

      <button
        class="mt-2 w-full text-center text-brand-400 hover:text-brand-300 text-[10px] disabled:opacity-50"
        onclick={handleCheckNow}
        disabled={checking}
      >
        {checking ? 'Checking...' : 'Check Now'}
      </button>

      <!-- Arrow -->
      <div class="absolute top-full left-1/2 -translate-x-1/2 -mt-px">
        <div class="border-4 border-transparent border-t-gray-700"></div>
      </div>
    </div>
  {/if}
</div>

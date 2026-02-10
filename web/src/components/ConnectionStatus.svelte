<script lang="ts">
  import { connectionState, lastError } from '$lib/websocketStore';
  import type { ConnectionState } from '$lib/websocketStore';

  let { showLabel = false }: { showLabel?: boolean } = $props();

  function getStatusColor(state: ConnectionState): string {
    switch (state) {
      case 'connected':
        return 'bg-green-500';
      case 'connecting':
        return 'bg-yellow-500 animate-pulse';
      case 'disconnected':
        return 'bg-gray-500';
      case 'error':
        return 'bg-red-500';
    }
  }

  function getStatusLabel(state: ConnectionState): string {
    switch (state) {
      case 'connected':
        return 'Connected';
      case 'connecting':
        return 'Connecting...';
      case 'disconnected':
        return 'Disconnected';
      case 'error':
        return 'Connection Error';
    }
  }
</script>

<div class="inline-flex items-center gap-1.5 group relative" title={$lastError || getStatusLabel($connectionState)}>
  <span class="w-2 h-2 rounded-full {getStatusColor($connectionState)}"></span>
  {#if showLabel}
    <span class="text-xs text-gray-400">{getStatusLabel($connectionState)}</span>
  {/if}

  <!-- Tooltip on hover -->
  {#if $lastError}
    <div class="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 px-2 py-1 text-xs bg-gray-900 text-red-400 rounded
                opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all whitespace-nowrap z-50">
      {$lastError}
    </div>
  {/if}
</div>

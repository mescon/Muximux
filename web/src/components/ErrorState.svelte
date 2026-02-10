<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    title = 'Something went wrong',
    message = '',
    icon = 'error',
    showRetry = true,
    retryLabel = 'Try again',
    compact = false,
    onretry,
    children,
  }: {
    title?: string;
    message?: string;
    icon?: 'error' | 'network' | 'notfound' | 'empty';
    showRetry?: boolean;
    retryLabel?: string;
    compact?: boolean;
    onretry?: () => void;
    children?: Snippet;
  } = $props();

  const icons = {
    error: {
      path: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z',
      color: 'text-red-400',
    },
    network: {
      path: 'M8.111 16.404a5.5 5.5 0 017.778 0M12 20h.01m-7.08-7.071c3.904-3.905 10.236-3.905 14.14 0M1.394 9.393c5.857-5.857 15.355-5.857 21.213 0',
      color: 'text-yellow-400',
    },
    notfound: {
      path: 'M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
      color: 'text-gray-400',
    },
    empty: {
      path: 'M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4',
      color: 'text-gray-500',
    },
  };

  let iconData = $derived(icons[icon]);
</script>

<div class="flex flex-col items-center justify-center {compact ? 'py-6' : 'py-12'} text-center">
  <!-- Icon -->
  <svg
    class="{compact ? 'w-10 h-10' : 'w-14 h-14'} {iconData.color} mb-4"
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    stroke-width="1.5"
  >
    <path stroke-linecap="round" stroke-linejoin="round" d={iconData.path} />
  </svg>

  <!-- Title -->
  <h3 class="{compact ? 'text-base' : 'text-lg'} font-medium text-white mb-1">
    {title}
  </h3>

  <!-- Message -->
  {#if message}
    <p class="text-sm text-gray-400 max-w-md mb-4">
      {message}
    </p>
  {/if}

  <!-- Actions -->
  {#if showRetry || children}
    <div class="flex items-center gap-3 mt-2">
      {#if showRetry}
        <button
          class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md
                 transition-colors flex items-center gap-2"
          onclick={() => onretry?.()}
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          {retryLabel}
        </button>
      {/if}
      {#if children}
        {@render children()}
      {/if}
    </div>
  {/if}
</div>

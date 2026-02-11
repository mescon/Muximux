<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import type { Snippet } from 'svelte';

  let { error = null, resetError = null, children }: {
    error?: Error | null;
    resetError?: (() => void) | null;
    children?: Snippet;
  } = $props();

  let errorState = $state<Error | null>(untrack(() => error));
  let errorInfo = $state('');

  // Sync external error prop
  $effect(() => {
    errorState = error;
  });

  // Global error handler
  onMount(() => {
    const handleError = (event: ErrorEvent) => {
      errorState = event.error || new Error(event.message);
      errorInfo = `${event.filename}:${event.lineno}:${event.colno}`;
      event.preventDefault();
    };

    const handleRejection = (event: PromiseRejectionEvent) => {
      errorState = event.reason instanceof Error ? event.reason : new Error(String(event.reason));
      errorInfo = 'Unhandled Promise Rejection';
      event.preventDefault();
    };

    window.addEventListener('error', handleError);
    window.addEventListener('unhandledrejection', handleRejection);

    return () => {
      window.removeEventListener('error', handleError);
      window.removeEventListener('unhandledrejection', handleRejection);
    };
  });

  function handleReset() {
    errorState = null;
    errorInfo = '';
    if (resetError) {
      resetError();
    }
  }

  function handleReload() {
    window.location.reload();
  }

  function copyErrorDetails() {
    const details = `Error: ${errorState?.message}\n\nStack:\n${errorState?.stack}\n\nInfo: ${errorInfo}`;
    navigator.clipboard.writeText(details);
  }
</script>

{#if errorState}
  <div class="fixed inset-0 z-[100] flex items-center justify-center bg-gray-900/95 p-4">
    <div class="max-w-lg w-full bg-gray-800 rounded-lg shadow-xl border border-gray-700 overflow-hidden">
      <!-- Header -->
      <div class="bg-red-500/10 border-b border-red-500/20 px-6 py-4">
        <div class="flex items-center gap-3">
          <svg class="w-8 h-8 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          <div>
            <h2 class="text-lg font-semibold text-white">Something went wrong</h2>
            <p class="text-sm text-gray-400">An unexpected error occurred</p>
          </div>
        </div>
      </div>

      <!-- Error details -->
      <div class="px-6 py-4">
        <div class="bg-gray-900 rounded-md p-4 mb-4">
          <p class="text-red-400 font-mono text-sm break-all">{errorState.message}</p>
          {#if errorInfo}
            <p class="text-gray-500 text-xs mt-2">{errorInfo}</p>
          {/if}
        </div>

        {#if errorState.stack}
          <details class="text-sm">
            <summary class="text-gray-400 cursor-pointer hover:text-gray-300 mb-2">
              Show stack trace
            </summary>
            <pre class="bg-gray-900 rounded-md p-3 text-xs text-gray-500 overflow-x-auto max-h-40">{errorState.stack}</pre>
          </details>
        {/if}
      </div>

      <!-- Actions -->
      <div class="px-6 py-4 bg-gray-800/50 border-t border-gray-700 flex gap-3">
        <button
          onclick={handleReload}
          class="flex-1 px-4 py-2 bg-brand-600 hover:bg-brand-700 text-white rounded-md
                 transition-colors text-sm font-medium"
        >
          Reload Page
        </button>
        <button
          onclick={handleReset}
          class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded-md
                 transition-colors text-sm"
        >
          Try Again
        </button>
        <button
          onclick={copyErrorDetails}
          class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded-md
                 transition-colors text-sm"
          title="Copy error details"
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
          </svg>
        </button>
      </div>
    </div>
  </div>
{:else if children}
  {@render children()}
{/if}

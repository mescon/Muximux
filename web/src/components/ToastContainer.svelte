<script lang="ts">
  import { flip } from 'svelte/animate';
  import { fly, fade } from 'svelte/transition';
  import { toasts, type Toast } from '$lib/toastStore';

  const icons: Record<string, string> = {
    success: 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z',
    error: 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z',
    warning: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z',
    info: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
  };

  const colors: Record<string, { bg: string; border: string; icon: string; text: string }> = {
    success: {
      bg: 'bg-green-500/10',
      border: 'border-green-500/30',
      icon: 'text-green-400',
      text: 'text-green-100',
    },
    error: {
      bg: 'bg-red-500/10',
      border: 'border-red-500/30',
      icon: 'text-red-400',
      text: 'text-red-100',
    },
    warning: {
      bg: 'bg-yellow-500/10',
      border: 'border-yellow-500/30',
      icon: 'text-yellow-400',
      text: 'text-yellow-100',
    },
    info: {
      bg: 'bg-blue-500/10',
      border: 'border-blue-500/30',
      icon: 'text-blue-400',
      text: 'text-blue-100',
    },
  };

  function getColor(type: string) {
    return colors[type] || colors.info;
  }

  function getIcon(type: string) {
    return icons[type] || icons.info;
  }
</script>

<div
  class="fixed bottom-4 right-4 z-[100] flex flex-col gap-2 pointer-events-none"
  aria-live="polite"
  aria-label="Notifications"
>
  {#each $toasts as toast (toast.id)}
    <div
      class="pointer-events-auto max-w-sm w-full shadow-lg rounded-lg border backdrop-blur-sm
             {getColor(toast.type).bg} {getColor(toast.type).border}"
      in:fly={{ x: 100, duration: 200 }}
      out:fade={{ duration: 150 }}
      animate:flip={{ duration: 200 }}
      role="alert"
    >
      <div class="p-4 flex items-start gap-3">
        <!-- Icon -->
        <svg
          class="w-5 h-5 flex-shrink-0 mt-0.5 {getColor(toast.type).icon}"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d={getIcon(toast.type)} />
        </svg>

        <!-- Content -->
        <div class="flex-1 min-w-0">
          {#if toast.title}
            <p class="text-sm font-medium text-white">{toast.title}</p>
          {/if}
          <p class="text-sm {toast.title ? 'text-gray-300' : getColor(toast.type).text}">
            {toast.message}
          </p>
        </div>

        <!-- Dismiss button -->
        {#if toast.dismissible}
          <button
            class="flex-shrink-0 p-1 rounded-md text-gray-400 hover:text-white
                   hover:bg-white/10 transition-colors"
            on:click={() => toasts.dismiss(toast.id)}
            aria-label="Dismiss"
          >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        {/if}
      </div>

      <!-- Progress bar for auto-dismiss -->
      {#if toast.duration > 0}
        <div class="h-1 rounded-b-lg overflow-hidden bg-white/5">
          <div
            class="h-full {toast.type === 'success' ? 'bg-green-500' :
                          toast.type === 'error' ? 'bg-red-500' :
                          toast.type === 'warning' ? 'bg-yellow-500' : 'bg-blue-500'}"
            style="animation: shrink {toast.duration}ms linear forwards"
          ></div>
        </div>
      {/if}
    </div>
  {/each}
</div>

<style>
  @keyframes shrink {
    from {
      width: 100%;
    }
    to {
      width: 0%;
    }
  }
</style>

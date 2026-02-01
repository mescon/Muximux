<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { fade, scale } from 'svelte/transition';

  const dispatch = createEventDispatcher<{
    close: void;
  }>();

  const shortcuts = [
    {
      category: 'Navigation',
      items: [
        { keys: ['/', 'Ctrl+K'], description: 'Open search' },
        { keys: ['1-9'], description: 'Switch to app by number' },
        { keys: ['Tab'], description: 'Next app' },
        { keys: ['Shift+Tab'], description: 'Previous app' },
        { keys: ['Escape'], description: 'Close modals / Go to splash' }
      ]
    },
    {
      category: 'Actions',
      items: [
        { keys: ['r'], description: 'Refresh current app' },
        { keys: ['f'], description: 'Toggle fullscreen mode' },
        { keys: ['Ctrl+,'], description: 'Open settings' },
        { keys: ['?'], description: 'Show this help' }
      ]
    },
    {
      category: 'Search (when open)',
      items: [
        { keys: ['↑/↓'], description: 'Navigate results' },
        { keys: ['Enter'], description: 'Select highlighted app' },
        { keys: ['1-9'], description: 'Quick select result' },
        { keys: ['Escape'], description: 'Close search' }
      ]
    }
  ];

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' || event.key === '?') {
      dispatch('close');
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
  on:click|self={() => dispatch('close')}
  role="dialog"
  aria-modal="true"
  aria-label="Keyboard shortcuts"
  transition:fade={{ duration: 150 }}
>
  <div
    class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-2xl border border-gray-700 overflow-hidden"
    in:scale={{ start: 0.95, duration: 200 }}
    out:fade={{ duration: 100 }}
  >
    <!-- Header -->
    <div class="flex items-center justify-between p-4 border-b border-gray-700">
      <h2 class="text-lg font-semibold text-white">Keyboard Shortcuts</h2>
      <button
        class="p-1.5 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
        on:click={() => dispatch('close')}
      >
        <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <!-- Content -->
    <div class="p-6 max-h-[70vh] overflow-y-auto">
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
        {#each shortcuts as section}
          <div>
            <h3 class="text-sm font-semibold text-gray-400 uppercase tracking-wider mb-3">
              {section.category}
            </h3>
            <div class="space-y-2">
              {#each section.items as shortcut}
                <div class="flex items-center justify-between py-1">
                  <span class="text-gray-300">{shortcut.description}</span>
                  <div class="flex items-center gap-1">
                    {#each shortcut.keys as key, i}
                      {#if i > 0}
                        <span class="text-gray-500 text-xs">or</span>
                      {/if}
                      <kbd class="px-2 py-1 text-xs bg-gray-700 border border-gray-600 rounded text-gray-200 font-mono">
                        {key}
                      </kbd>
                    {/each}
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/each}
      </div>
    </div>

    <!-- Footer -->
    <div class="p-4 border-t border-gray-700 text-center">
      <p class="text-sm text-gray-400">
        Press <kbd class="px-1.5 py-0.5 text-xs bg-gray-700 rounded border border-gray-600">?</kbd> or
        <kbd class="px-1.5 py-0.5 text-xs bg-gray-700 rounded border border-gray-600">Escape</kbd> to close
      </p>
    </div>
  </div>
</div>

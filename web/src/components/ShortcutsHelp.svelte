<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import { fade, fly } from 'svelte/transition';
  import { isMobileViewport } from '$lib/useSwipe';
  import { keybindings, formatKeybinding, type Keybinding } from '$lib/keybindingsStore';

  const dispatch = createEventDispatcher<{
    close: void;
  }>();

  let isMobile = false;

  onMount(() => {
    isMobile = isMobileViewport();
    const handleResize = () => { isMobile = isMobileViewport(); };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  });

  // Category labels for display
  const categoryLabels: Record<string, string> = {
    navigation: 'Navigation',
    actions: 'Actions',
    apps: 'App Quick Access'
  };

  // Group keybindings by category
  $: groupedBindings = $keybindings.reduce((acc, binding) => {
    if (!acc[binding.category]) {
      acc[binding.category] = [];
    }
    acc[binding.category].push(binding);
    return acc;
  }, {} as Record<string, Keybinding[]>);

  // Additional non-customizable shortcuts
  const additionalShortcuts = [
    {
      category: 'Modal Navigation',
      items: [
        { keys: ['Escape'], description: 'Close modals / Go to home' },
        { keys: ['↑/↓'], description: 'Navigate results (in search/palette)' },
        { keys: ['Enter'], description: 'Select highlighted item' }
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
  class="shortcuts-help fixed inset-0 z-50 flex items-center justify-center bg-black/50 {isMobile ? 'p-0' : 'p-4'}"
  on:click|self={() => dispatch('close')}
  role="dialog"
  aria-modal="true"
  aria-label="Keyboard shortcuts"
  transition:fade={{ duration: 150 }}
>
  <div
    class="shortcuts-modal shadow-2xl w-full overflow-hidden
           {isMobile
             ? 'h-full max-h-full rounded-none'
             : 'rounded-xl max-w-2xl'}"
    in:fly={{ y: isMobile ? 50 : 0, duration: 200 }}
    out:fade={{ duration: 100 }}
  >
    <!-- Header -->
    <div class="flex items-center justify-between p-4 border-b" style="border-color: var(--border-subtle);">
      <h2 class="text-lg font-semibold" style="color: var(--text-primary);">Keyboard Shortcuts</h2>
      <button
        class="shortcuts-close-btn p-1.5 rounded-md"
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
        <!-- Customizable shortcuts from keybindings store -->
        {#each Object.entries(groupedBindings) as [category, bindings]}
          {#if category !== 'apps'}
            <div>
              <h3 class="text-sm font-semibold uppercase tracking-wider mb-3" style="color: var(--text-muted);">
                {categoryLabels[category] || category}
              </h3>
              <div class="space-y-2">
                {#each bindings as binding}
                  <div class="flex items-center justify-between py-1">
                    <span style="color: var(--text-secondary);">{binding.label}</span>
                    <div class="flex items-center gap-1">
                      {#each binding.combos as combo, i}
                        {#if i > 0}
                          <span class="text-xs" style="color: var(--text-disabled);">or</span>
                        {/if}
                        <kbd class="shortcuts-kbd px-2 py-1 text-xs rounded font-mono">
                          {#if combo.ctrl}Ctrl+{/if}{#if combo.alt}Alt+{/if}{#if combo.shift}Shift+{/if}{#if combo.meta}⌘{/if}{combo.key.length === 1 ? combo.key.toUpperCase() : combo.key}
                        </kbd>
                      {/each}
                    </div>
                  </div>
                {/each}
              </div>
            </div>
          {/if}
        {/each}

        <!-- App Quick Access (summarized) -->
        {#if groupedBindings.apps}
          <div>
            <h3 class="text-sm font-semibold uppercase tracking-wider mb-3" style="color: var(--text-muted);">
              App Quick Access
            </h3>
            <div class="space-y-2">
              <div class="flex items-center justify-between py-1">
                <span style="color: var(--text-secondary);">Switch to app by number</span>
                <kbd class="shortcuts-kbd px-2 py-1 text-xs rounded font-mono">
                  1-9
                </kbd>
              </div>
            </div>
          </div>
        {/if}

        <!-- Additional non-customizable shortcuts -->
        {#each additionalShortcuts as section}
          <div>
            <h3 class="text-sm font-semibold uppercase tracking-wider mb-3" style="color: var(--text-muted);">
              {section.category}
            </h3>
            <div class="space-y-2">
              {#each section.items as shortcut}
                <div class="flex items-center justify-between py-1">
                  <span style="color: var(--text-secondary);">{shortcut.description}</span>
                  <div class="flex items-center gap-1">
                    {#each shortcut.keys as key, i}
                      {#if i > 0}
                        <span class="text-xs" style="color: var(--text-disabled);">or</span>
                      {/if}
                      <kbd class="shortcuts-kbd px-2 py-1 text-xs rounded font-mono">
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

      <!-- Customization hint -->
      <div class="mt-6 p-3 rounded-lg" style="background: var(--bg-hover);">
        <p class="text-sm text-center" style="color: var(--text-muted);">
          Customize shortcuts in <span style="color: var(--accent-primary);">Settings → Keybindings</span>
        </p>
      </div>
    </div>

    <!-- Footer -->
    <div class="p-4 border-t text-center" style="border-color: var(--border-subtle);">
      <p class="text-sm" style="color: var(--text-muted);">
        Press <kbd class="shortcuts-kbd px-1.5 py-0.5 text-xs rounded">?</kbd> or
        <kbd class="shortcuts-kbd px-1.5 py-0.5 text-xs rounded">Escape</kbd> to close
      </p>
    </div>
  </div>
</div>

<style>
  .shortcuts-modal {
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
  }

  .shortcuts-close-btn {
    color: var(--text-muted);
  }

  .shortcuts-close-btn:hover {
    color: var(--text-primary);
    background: var(--bg-hover);
  }

  .shortcuts-kbd {
    background: var(--bg-elevated);
    border: 1px solid var(--border-default);
    color: var(--text-secondary);
  }
</style>

<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import {
    keybindings,
    type Keybinding,
    type KeyCombo,
    type KeyAction,
    formatKeyCombo,
    eventToKeyCombo,
    setKeybinding,
    removeKeyCombo,
    resetKeybinding,
    resetAllKeybindings,
    isCustomized,
    findConflicts,
    comboEquals,
    DEFAULT_KEYBINDINGS
  } from '$lib/keybindingsStore';

  const dispatch = createEventDispatcher<{
    change: void;
  }>();

  // State for capturing new keybinding
  let capturingAction: KeyAction | null = null;
  let capturingIndex: number | null = null; // null = adding new, number = replacing
  let capturedCombo: KeyCombo | null = null;
  let conflicts: Keybinding[] = [];

  // Category labels
  const categoryLabels: Record<string, string> = {
    navigation: 'Navigation',
    actions: 'Actions',
    apps: 'App Quick Access'
  };

  // Group bindings by category
  $: groupedBindings = $keybindings.reduce((acc, binding) => {
    if (!acc[binding.category]) {
      acc[binding.category] = [];
    }
    acc[binding.category].push(binding);
    return acc;
  }, {} as Record<string, Keybinding[]>);

  function startCapture(action: KeyAction, index: number | null = null) {
    capturingAction = action;
    capturingIndex = index;
    capturedCombo = null;
    conflicts = [];
  }

  function cancelCapture() {
    capturingAction = null;
    capturingIndex = null;
    capturedCombo = null;
    conflicts = [];
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!capturingAction) return;

    event.preventDefault();
    event.stopPropagation();

    // Ignore lone modifier keys
    if (['Control', 'Alt', 'Shift', 'Meta'].includes(event.key)) {
      return;
    }

    // Escape cancels capture
    if (event.key === 'Escape') {
      cancelCapture();
      return;
    }

    capturedCombo = eventToKeyCombo(event);
    conflicts = findConflicts(capturedCombo, capturingAction);
  }

  function confirmCapture() {
    if (!capturingAction || !capturedCombo) return;

    const binding = $keybindings.find(b => b.action === capturingAction);
    if (!binding) return;

    let newCombos: KeyCombo[];

    if (capturingIndex !== null) {
      // Replace existing combo
      newCombos = binding.combos.map((c, i) =>
        i === capturingIndex ? capturedCombo! : c
      );
    } else {
      // Add new combo
      newCombos = [...binding.combos, capturedCombo];
    }

    // Remove duplicates
    newCombos = newCombos.filter((combo, index, self) =>
      index === self.findIndex(c => comboEquals(c, combo))
    );

    setKeybinding(capturingAction, newCombos);
    dispatch('change');
    cancelCapture();
  }

  function handleRemoveCombo(action: KeyAction, index: number) {
    const binding = $keybindings.find(b => b.action === action);
    if (binding && binding.combos.length > 1) {
      removeKeyCombo(action, index);
      dispatch('change');
    }
  }

  function handleResetBinding(action: KeyAction) {
    resetKeybinding(action);
    dispatch('change');
  }

  function handleResetAll() {
    if (confirm('Reset all keybindings to defaults?')) {
      resetAllKeybindings();
      dispatch('change');
    }
  }

  function getDefaultCombos(action: KeyAction): KeyCombo[] {
    const def = DEFAULT_KEYBINDINGS.find(b => b.action === action);
    return def?.combos || [];
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="space-y-6">
  <!-- Header with reset button -->
  <div class="flex items-center justify-between">
    <p class="text-sm text-gray-400">
      Click a keybinding to change it. Press Escape to cancel.
    </p>
    <button
      type="button"
      class="text-sm text-gray-400 hover:text-white transition-colors"
      on:click={handleResetAll}
    >
      Reset All to Defaults
    </button>
  </div>

  <!-- Keybindings by category -->
  {#each Object.entries(groupedBindings) as [category, bindings]}
    <div>
      <h4 class="text-sm font-semibold text-gray-400 uppercase tracking-wider mb-3">
        {categoryLabels[category] || category}
      </h4>

      <div class="space-y-2">
        {#each bindings as binding}
          <div
            class="flex items-center justify-between p-3 bg-gray-700/50 rounded-lg
                   {isCustomized(binding.action) ? 'ring-1 ring-brand-500/30' : ''}"
          >
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <span class="text-gray-200">{binding.label}</span>
                {#if isCustomized(binding.action)}
                  <span class="text-xs text-brand-400">(customized)</span>
                {/if}
              </div>
              <p class="text-xs text-gray-500 mt-0.5">{binding.description}</p>
            </div>

            <div class="flex items-center gap-2 ml-4">
              <!-- Key combos -->
              <div class="flex flex-wrap gap-1.5 justify-end">
                {#each binding.combos as combo, i}
                  <div class="flex items-center">
                    {#if capturingAction === binding.action && capturingIndex === i}
                      <!-- Capturing mode for this combo -->
                      <div class="flex items-center gap-1">
                        {#if capturedCombo}
                          <kbd class="px-2 py-1 text-xs bg-brand-600 border border-brand-500 rounded text-white font-mono">
                            {formatKeyCombo(capturedCombo)}
                          </kbd>
                        {:else}
                          <kbd class="px-2 py-1 text-xs bg-gray-600 border border-gray-500 rounded text-gray-300 font-mono animate-pulse">
                            Press keys...
                          </kbd>
                        {/if}
                      </div>
                    {:else}
                      <button
                        type="button"
                        class="group flex items-center gap-1"
                        on:click={() => binding.editable && startCapture(binding.action, i)}
                        disabled={!binding.editable}
                      >
                        <kbd
                          class="px-2 py-1 text-xs bg-gray-700 border border-gray-600 rounded font-mono
                                 {binding.editable ? 'text-gray-200 group-hover:bg-gray-600 group-hover:border-gray-500' : 'text-gray-500'}"
                        >
                          {formatKeyCombo(combo)}
                        </kbd>
                        {#if binding.editable && binding.combos.length > 1}
                          <button
                            type="button"
                            class="p-0.5 text-gray-500 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-opacity"
                            on:click|stopPropagation={() => handleRemoveCombo(binding.action, i)}
                            title="Remove this shortcut"
                          >
                            <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                          </button>
                        {/if}
                      </button>
                    {/if}
                    {#if i < binding.combos.length - 1}
                      <span class="text-gray-500 text-xs mx-1">or</span>
                    {/if}
                  </div>
                {/each}

                <!-- Add new combo button -->
                {#if binding.editable && capturingAction !== binding.action}
                  <button
                    type="button"
                    class="p-1 text-gray-500 hover:text-gray-300 transition-colors"
                    on:click={() => startCapture(binding.action, null)}
                    title="Add alternative shortcut"
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                    </svg>
                  </button>
                {/if}

                <!-- Adding new combo -->
                {#if capturingAction === binding.action && capturingIndex === null}
                  <div class="flex items-center gap-1">
                    <span class="text-gray-500 text-xs mx-1">or</span>
                    {#if capturedCombo}
                      <kbd class="px-2 py-1 text-xs bg-brand-600 border border-brand-500 rounded text-white font-mono">
                        {formatKeyCombo(capturedCombo)}
                      </kbd>
                    {:else}
                      <kbd class="px-2 py-1 text-xs bg-gray-600 border border-gray-500 rounded text-gray-300 font-mono animate-pulse">
                        Press keys...
                      </kbd>
                    {/if}
                  </div>
                {/if}
              </div>

              <!-- Reset button (shown if customized) -->
              {#if isCustomized(binding.action)}
                <button
                  type="button"
                  class="p-1 text-gray-500 hover:text-yellow-400 transition-colors"
                  on:click={() => handleResetBinding(binding.action)}
                  title="Reset to default"
                >
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                          d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                  </svg>
                </button>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    </div>
  {/each}
</div>

<!-- Capture confirmation modal -->
{#if capturingAction && capturedCombo}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
    <div class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-md mx-4 overflow-hidden">
      <div class="p-6">
        <h3 class="text-lg font-semibold text-white mb-4">
          Set Keybinding
        </h3>

        <div class="text-center mb-6">
          <kbd class="px-4 py-2 text-lg bg-gray-700 border border-gray-600 rounded-lg text-white font-mono">
            {formatKeyCombo(capturedCombo)}
          </kbd>
        </div>

        {#if conflicts.length > 0}
          <div class="bg-yellow-900/30 border border-yellow-700/50 rounded-lg p-3 mb-4">
            <p class="text-yellow-300 text-sm font-medium mb-1">
              Conflict detected
            </p>
            <p class="text-yellow-200/80 text-xs">
              This shortcut is already used by:
              {conflicts.map(c => c.label).join(', ')}
            </p>
          </div>
        {/if}

        <div class="flex gap-3">
          <button
            type="button"
            class="flex-1 px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-200 rounded-lg transition-colors"
            on:click={cancelCapture}
          >
            Cancel
          </button>
          <button
            type="button"
            class="flex-1 px-4 py-2 bg-brand-600 hover:bg-brand-500 text-white rounded-lg transition-colors"
            on:click={confirmCapture}
          >
            {conflicts.length > 0 ? 'Use Anyway' : 'Confirm'}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

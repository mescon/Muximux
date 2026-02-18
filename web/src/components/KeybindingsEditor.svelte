<script lang="ts">
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
    comboEquals
  } from '$lib/keybindingsStore';

  // Props
  let {
    onchange
  }: {
    onchange?: () => void;
  } = $props();

  // State for capturing new keybinding
  let capturingAction = $state<KeyAction | null>(null);
  let capturingIndex = $state<number | null>(null); // null = adding new, number = replacing
  let capturedCombo = $state<KeyCombo | null>(null);
  let conflicts = $state<Keybinding[]>([]);

  // Category labels
  const categoryLabels: Record<string, string> = {
    navigation: 'Navigation',
    actions: 'Actions',
    apps: 'App Quick Access'
  };

  // Group bindings by category
  const groupedBindings = $derived($keybindings.reduce((acc, binding) => {
    if (!acc[binding.category]) {
      acc[binding.category] = [];
    }
    acc[binding.category].push(binding);
    return acc;
  }, {} as Record<string, Keybinding[]>));

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
    onchange?.();
    cancelCapture();
  }

  function handleRemoveCombo(action: KeyAction, index: number) {
    const binding = $keybindings.find(b => b.action === action);
    if (binding && binding.combos.length > 1) {
      removeKeyCombo(action, index);
      onchange?.();
    }
  }

  function handleResetBinding(action: KeyAction) {
    resetKeybinding(action);
    onchange?.();
  }

  let confirmResetAll = $state(false);

  function handleResetAll() {
    confirmResetAll = true;
  }

  function confirmResetAllAction() {
    resetAllKeybindings();
    onchange?.();
    confirmResetAll = false;
  }

</script>

<svelte:window onkeydown={handleKeydown} />

<div class="space-y-6">
  <!-- Header with reset button -->
  <div class="flex items-center justify-between">
    <p class="text-sm text-text-muted">
      Click a keybinding to change it. Press Escape to cancel.
    </p>
    {#if confirmResetAll}
      <div class="flex items-center gap-2">
        <span class="text-sm text-red-400">Reset all keybindings?</span>
        <button
          type="button"
          class="px-2 py-1 text-xs rounded bg-red-600 hover:bg-red-500 text-white"
          onclick={confirmResetAllAction}
        >Yes, Reset</button>
        <button
          type="button"
          class="px-2 py-1 text-xs rounded bg-bg-overlay hover:bg-bg-active text-text-primary"
          onclick={() => confirmResetAll = false}
        >Cancel</button>
      </div>
    {:else}
      <button
        type="button"
        class="text-sm text-text-muted hover:text-text-primary transition-colors"
        onclick={handleResetAll}
      >
        Reset All to Defaults
      </button>
    {/if}
  </div>

  <!-- Keybindings by category -->
  {#each Object.entries(groupedBindings) as [category, bindings] (category)}
    <div>
      <h4 class="text-sm font-semibold text-text-muted uppercase tracking-wider mb-3">
        {categoryLabels[category] || category}
      </h4>

      <div class="space-y-2">
        {#each bindings as binding (binding.action)}
          <div
            class="flex items-center justify-between p-3 bg-bg-hover rounded-lg
                   {isCustomized(binding.action) ? 'ring-1 ring-brand-500/30' : ''}"
          >
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <span class="text-text-primary">{binding.label}</span>
                {#if isCustomized(binding.action)}
                  <span class="text-xs text-brand-400">(customized)</span>
                {/if}
              </div>
              <p class="text-xs text-text-disabled mt-0.5">{binding.description}</p>
            </div>

            <div class="flex items-center gap-2 ml-4">
              <!-- Key combos -->
              <div class="flex flex-wrap gap-1.5 justify-end">
                {#each binding.combos as combo, i (i)}
                  <div class="flex items-center">
                    {#if capturingAction === binding.action && capturingIndex === i}
                      <!-- Capturing mode for this combo -->
                      <div class="flex items-center gap-1">
                        {#if capturedCombo}
                          <kbd class="px-2 py-1 text-xs bg-brand-600 border border-brand-500 rounded text-white font-mono">
                            {formatKeyCombo(capturedCombo)}
                          </kbd>
                        {:else}
                          <kbd class="px-2 py-1 text-xs bg-bg-overlay border border-border-strong rounded text-text-secondary font-mono animate-pulse">
                            Press keys...
                          </kbd>
                        {/if}
                      </div>
                    {:else}
                      <div class="group flex items-center gap-1">
                        <button
                          type="button"
                          onclick={() => binding.editable && startCapture(binding.action, i)}
                          disabled={!binding.editable}
                        >
                          <kbd
                            class="px-2 py-1 text-xs bg-bg-elevated border border-border-subtle rounded font-mono
                                   {binding.editable ? 'text-text-primary group-hover:bg-bg-active group-hover:border-border-strong' : 'text-text-disabled'}"
                          >
                            {formatKeyCombo(combo)}
                          </kbd>
                        </button>
                        {#if binding.editable && binding.combos.length > 1}
                          <span
                            role="button"
                            class="p-0.5 text-text-disabled hover:text-red-400 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                            onclick={(e) => { e.stopPropagation(); handleRemoveCombo(binding.action, i); }}
                            onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.stopPropagation(); handleRemoveCombo(binding.action, i); } }}
                            tabindex="0"
                            title="Remove this shortcut"
                          >
                            <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                          </span>
                        {/if}
                      </div>
                    {/if}
                    {#if i < binding.combos.length - 1}
                      <span class="text-text-disabled text-xs mx-1">or</span>
                    {/if}
                  </div>
                {/each}

                <!-- Add new combo button -->
                {#if binding.editable && capturingAction !== binding.action}
                  <button
                    type="button"
                    class="p-1 text-text-disabled hover:text-text-secondary transition-colors"
                    onclick={() => startCapture(binding.action, null)}
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
                    <span class="text-text-disabled text-xs mx-1">or</span>
                    {#if capturedCombo}
                      <kbd class="px-2 py-1 text-xs bg-brand-600 border border-brand-500 rounded text-white font-mono">
                        {formatKeyCombo(capturedCombo)}
                      </kbd>
                    {:else}
                      <kbd class="px-2 py-1 text-xs bg-bg-overlay border border-border-strong rounded text-text-secondary font-mono animate-pulse">
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
                  class="p-1 text-text-disabled hover:text-yellow-400 transition-colors"
                  onclick={() => handleResetBinding(binding.action)}
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
  <div class="fixed inset-0 z-50 flex items-center justify-center backdrop-blur-sm" style="background: rgba(0, 0, 0, 0.6);">
    <div class="keybinding-modal rounded-xl shadow-2xl w-full max-w-md mx-4 overflow-hidden">
      <div class="p-6">
        <h3 class="text-lg font-semibold mb-4" style="color: var(--text-primary);">
          Set Keybinding
        </h3>

        <div class="text-center mb-6">
          <kbd class="keybinding-kbd px-4 py-2 text-lg rounded-lg font-mono">
            {formatKeyCombo(capturedCombo)}
          </kbd>
        </div>

        {#if conflicts.length > 0}
          <div class="keybinding-conflict rounded-lg p-3 mb-4">
            <p class="text-sm font-medium mb-1" style="color: var(--status-warning);">
              Conflict detected
            </p>
            <p class="text-xs" style="color: var(--text-secondary);">
              This shortcut is already used by:
              {conflicts.map(c => c.label).join(', ')}
            </p>
          </div>
        {/if}

        <div class="flex gap-3">
          <button
            type="button"
            class="keybinding-btn-cancel flex-1 px-4 py-2 rounded-lg transition-colors"
            onclick={cancelCapture}
          >
            Cancel
          </button>
          <button
            type="button"
            class="keybinding-btn-confirm flex-1 px-4 py-2 rounded-lg transition-colors"
            onclick={confirmCapture}
          >
            {conflicts.length > 0 ? 'Use Anyway' : 'Confirm'}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .keybinding-modal {
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
  }
  .keybinding-kbd {
    background: var(--bg-elevated);
    border: 1px solid var(--border-default);
    color: var(--text-primary);
  }
  .keybinding-conflict {
    background: color-mix(in srgb, var(--status-warning) 12%, transparent);
    border: 1px solid color-mix(in srgb, var(--status-warning) 30%, transparent);
  }
  .keybinding-btn-cancel {
    background: var(--bg-elevated);
    color: var(--text-secondary);
  }
  .keybinding-btn-cancel:hover {
    background: var(--bg-active);
  }
  .keybinding-btn-confirm {
    background: var(--accent-primary);
    color: #fff;
  }
  .keybinding-btn-confirm:hover {
    filter: brightness(1.1);
  }
</style>

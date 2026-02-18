<script lang="ts">
  import { untrack } from 'svelte';
  import { resolvedTheme, allThemes, isDarkTheme, saveCustomThemeToServer, deleteCustomThemeFromServer, getCurrentThemeVariables, themeVariableGroups, sanitizeThemeId, selectedFamily, variantMode, themeFamilies, setThemeFamily, setVariantMode } from '$lib/themeStore';
  import { toasts } from '$lib/toastStore';

  // Theme delete confirmation
  let confirmDeleteTheme = $state<string | null>(null);

  // Theme editor state
  let showThemeEditor = $state(false);
  let themeEditorVars: Record<string, string> = $state({});
  let themeEditorDefaults: Record<string, string> = $state({});
  let saveThemeName = $state('');
  let saveThemeDescription = $state('');
  let saveThemeAuthor = $state('');
  let isSavingTheme = $state(false);

  function openThemeEditor() {
    themeEditorDefaults = getCurrentThemeVariables();
    themeEditorVars = { ...themeEditorDefaults };
    showThemeEditor = true;
  }

  // Refresh theme editor when the active theme changes while editor is open
  $effect(() => {
    $resolvedTheme; // track
    if (showThemeEditor) {
      // Clear any live preview overrides from the previous theme
      const varNames = untrack(() => Object.keys(themeEditorVars));
      for (const name of varNames) {
        document.documentElement.style.removeProperty(name);
      }
      // Re-read the new theme's variables
      // Use a microtask so the theme CSS has loaded
      queueMicrotask(() => {
        themeEditorDefaults = getCurrentThemeVariables();
        themeEditorVars = { ...themeEditorDefaults };
      });
    }
  });

  function closeThemeEditor() {
    // Revert live preview changes
    for (const name of Object.keys(themeEditorVars)) {
      document.documentElement.style.removeProperty(name);
    }
    showThemeEditor = false;
    saveThemeName = '';
  }

  function updateThemeVar(name: string, value: string) {
    themeEditorVars[name] = value;
    // Live preview
    document.documentElement.style.setProperty(name, value);
  }

  function resetThemeVar(name: string) {
    themeEditorVars[name] = themeEditorDefaults[name];
    document.documentElement.style.removeProperty(name);
  }

  function resetAllThemeVars() {
    for (const name of Object.keys(themeEditorVars)) {
      document.documentElement.style.removeProperty(name);
    }
    themeEditorVars = { ...themeEditorDefaults };
  }

  async function handleSaveTheme() {
    if (!saveThemeName.trim()) return;
    isSavingTheme = true;
    const success = await saveCustomThemeToServer(
      saveThemeName.trim(),
      $resolvedTheme,
      $isDarkTheme,
      themeEditorVars,
      saveThemeDescription.trim(),
      saveThemeAuthor.trim()
    );
    isSavingTheme = false;
    if (success) {
      // Clear inline overrides — the saved CSS file takes over
      for (const name of Object.keys(themeEditorVars)) {
        document.documentElement.style.removeProperty(name);
      }
      // Switch to the new theme (as a standalone family)
      const id = sanitizeThemeId(saveThemeName.trim());
      setThemeFamily(id);
      setVariantMode($isDarkTheme ? 'dark' : 'light');
      showThemeEditor = false;
      saveThemeName = '';
      saveThemeDescription = '';
      saveThemeAuthor = '';
      toasts.success('Theme saved');
    } else {
      toasts.error('Failed to save theme');
    }
  }

  function handleDeleteTheme(themeId: string) {
    confirmDeleteTheme = themeId;
  }

  async function confirmDeleteThemeAction() {
    if (!confirmDeleteTheme) return;
    const themeId = confirmDeleteTheme;
    confirmDeleteTheme = null;
    const success = await deleteCustomThemeFromServer(themeId);
    if (success) {
      toasts.success('Theme deleted');
    } else {
      toasts.error('Failed to delete theme');
    }
  }

  // Convert CSS color to hex (for color input compatibility)
  function cssColorToHex(color: string): string {
    if (!color) return '#000000';
    // Already hex
    if (color.startsWith('#')) return color.length === 4
      ? '#' + color[1] + color[1] + color[2] + color[2] + color[3] + color[3]
      : color;
    // Try rgb/rgba
    const match = color.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/);
    if (match) {
      const r = parseInt(match[1]).toString(16).padStart(2, '0');
      const g = parseInt(match[2]).toString(16).padStart(2, '0');
      const b = parseInt(match[3]).toString(16).padStart(2, '0');
      return `#${r}${g}${b}`;
    }
    return '#000000';
  }

  // Variable display names
  const varLabels: Record<string, string> = {
    '--bg-base': 'Base',
    '--bg-surface': 'Surface',
    '--bg-elevated': 'Elevated',
    '--text-primary': 'Primary',
    '--text-secondary': 'Secondary',
    '--text-muted': 'Muted',
    '--accent-primary': 'Primary',
    '--accent-secondary': 'Secondary',
    '--status-success': 'Success',
    '--status-warning': 'Warning',
    '--status-error': 'Error',
    '--status-info': 'Info',
  };
</script>

<div class="space-y-6">
  <!-- Variant Mode Selector (Dark / System / Light) -->
  <div class="p-4 rounded-lg bg-bg-elevated border border-border-subtle">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg flex items-center justify-center"
             style="background: linear-gradient(135deg, var(--bg-surface) 50%, var(--bg-overlay) 50%); border: 1px solid var(--border-default);">
          <svg class="w-5 h-5" style="color: var(--text-muted);" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
          </svg>
        </div>
        <div>
          <div class="font-medium" style="color: var(--text-primary);">Appearance</div>
          <div class="text-xs" style="color: var(--text-muted);">Choose dark, light, or follow your system</div>
        </div>
      </div>
      <!-- Three-way segmented control -->
      <div class="flex rounded-lg overflow-hidden" style="border: 1px solid var(--border-default);">
        {#each (['dark', 'system', 'light'] as const) as mode (mode)}
          <button
            class="px-3 py-1.5 text-xs font-medium transition-colors"
            style="
              background: {$variantMode === mode ? 'var(--accent-primary)' : 'var(--bg-surface)'};
              color: {$variantMode === mode ? 'white' : 'var(--text-secondary)'};
            "
            onclick={() => setVariantMode(mode)}
          >
            {#if mode === 'dark'}
              Dark
            {:else if mode === 'system'}
              System
            {:else}
              Light
            {/if}
          </button>
        {/each}
      </div>
    </div>
  </div>

  <!-- Theme Family Grid -->
  <div>
    <span class="block text-sm font-medium mb-3 text-text-secondary">
      Choose Theme
    </span>
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
      {#each $themeFamilies as family (family.id)}
        {@const isSelected = $selectedFamily === family.id}
        {@const isCustom = family.darkTheme ? !family.darkTheme.isBuiltin : family.lightTheme ? !family.lightTheme.isBuiltin : false}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="relative p-4 rounded-xl text-left transition-all group cursor-pointer"
          style="
            background: var(--bg-surface);
            border: 2px solid {isSelected ? 'var(--accent-primary)' : 'var(--border-subtle)'};
            box-shadow: {isSelected ? 'var(--shadow-glow)' : 'none'};
          "
          onclick={() => setThemeFamily(family.id)}
          onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setThemeFamily(family.id); } }}
          role="button"
          tabindex="0"
        >
          <!-- Selection indicator / delete button -->
          <div class="absolute top-3 right-3 flex items-center gap-1">
            {#if isCustom}
              <button
                class="w-5 h-5 rounded-full flex items-center justify-center opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity"
                style="background: var(--status-error); color: white;"
                tabindex="-1"
                onclick={(e: MouseEvent) => { e.stopPropagation(); handleDeleteTheme(family.darkTheme?.id || family.lightTheme?.id || ''); }}
                title="Delete theme"
                aria-label="Delete theme"
              >
                <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            {/if}
            {#if isSelected}
              <div class="w-5 h-5 rounded-full flex items-center justify-center"
                   style="background: var(--accent-primary);">
                <svg class="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                </svg>
              </div>
            {/if}
          </div>

          <!-- Dual Preview Swatches (dark left, light right) -->
          <div class="flex gap-1.5 mb-3">
            {#if family.darkTheme?.preview && family.lightTheme?.preview}
              <!-- Dark variant swatch -->
              <div class="w-10 h-12 rounded-lg overflow-hidden flex flex-col shadow-md"
                   style="border: 1px solid {family.darkTheme.preview.text}20;">
                <div class="flex-1" style="background: {family.darkTheme.preview.bg};"></div>
                <div class="h-2" style="background: {family.darkTheme.preview.accent};"></div>
              </div>
              <!-- Light variant swatch -->
              <div class="w-10 h-12 rounded-lg overflow-hidden flex flex-col shadow-md"
                   style="border: 1px solid {family.lightTheme.preview.text}20;">
                <div class="flex-1" style="background: {family.lightTheme.preview.bg};"></div>
                <div class="h-2" style="background: {family.lightTheme.preview.accent};"></div>
              </div>
            {:else}
              <!-- Single variant swatch -->
              {@const theme = family.darkTheme || family.lightTheme}
              {#if theme?.preview}
                <div class="w-12 h-12 rounded-lg overflow-hidden flex flex-col shadow-md"
                     style="border: 1px solid {theme.preview.text}20;">
                  <div class="flex-1" style="background: {theme.preview.bg};"></div>
                  <div class="h-2" style="background: {theme.preview.accent};"></div>
                </div>
                <div class="flex flex-col gap-1">
                  <div class="w-6 h-5.5 rounded" style="background: {theme.preview.surface}; border: 1px solid {theme.preview.text}15;"></div>
                  <div class="w-6 h-5.5 rounded" style="background: {theme.preview.accent};"></div>
                </div>
              {:else}
                <div class="w-12 h-12 rounded-lg flex items-center justify-center bg-bg-elevated border border-border-subtle">
                  <svg class="w-6 h-6 text-text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
                  </svg>
                </div>
              {/if}
            {/if}
          </div>

          <!-- Family Name & Badge -->
          <div class="flex items-center gap-2">
            <span class="font-medium" style="color: var(--text-primary);">{family.name}</span>
            {#if isCustom}
              <span class="text-[10px] px-1.5 py-0.5 rounded flex-shrink-0"
                    style="background: var(--accent-subtle); color: var(--accent-primary);">
                Custom
              </span>
            {/if}
          </div>
          {#if family.description}
            <div class="text-xs mt-0.5 pr-1" style="color: var(--text-muted);">{family.description}</div>
          {/if}

          <!-- Delete confirmation overlay -->
          {#if confirmDeleteTheme === (family.darkTheme?.id || family.lightTheme?.id)}
            <div class="absolute inset-0 rounded-xl flex items-center justify-center gap-3 z-10"
                 style="background: var(--bg-overlay); backdrop-filter: blur(4px);"
                 onclick={(e: MouseEvent) => e.stopPropagation()}
                 onkeydown={(e: KeyboardEvent) => e.stopPropagation()}
                 role="presentation">
                <span class="text-sm font-medium" style="color: var(--text-primary);">Delete?</span>
                <button class="px-3 py-1 rounded text-sm font-medium"
                        style="background: var(--status-error); color: white;"
                        onclick={(e: MouseEvent) => { e.stopPropagation(); confirmDeleteThemeAction(); }}>Yes</button>
                <button class="btn btn-secondary btn-sm"
                        onclick={(e: MouseEvent) => { e.stopPropagation(); confirmDeleteTheme = null; }}>No</button>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </div>

  <!-- Current Theme Info -->
  <div class="p-4 rounded-lg bg-bg-elevated border border-border-subtle">
    <div class="flex items-center gap-2 text-sm">
      <span style="color: var(--text-muted);">Currently using:</span>
      <span class="font-medium capitalize" style="color: var(--text-primary);">
        {$allThemes.find(t => t.id === $resolvedTheme)?.name || $resolvedTheme} theme
      </span>
      {#if $variantMode === 'system'}
        <span class="text-xs" style="color: var(--text-disabled);">(from system preference)</span>
      {/if}
    </div>
  </div>

  <!-- Theme Customization -->
  <div class="space-y-3">
    {#if !showThemeEditor}
      <button
        class="w-full p-4 rounded-lg text-left transition-all hover:border-brand-500/50 flex items-center gap-3"
        style="background: var(--bg-surface); border: 1px solid var(--border-subtle);"
        onclick={openThemeEditor}
      >
        <div class="w-8 h-8 rounded-lg flex-shrink-0 flex items-center justify-center"
             style="background: var(--accent-subtle);">
          <svg class="w-4 h-4" style="color: var(--accent-primary);" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
          </svg>
        </div>
        <div>
          <div class="font-medium text-sm" style="color: var(--text-primary);">Customize Current Theme</div>
          <p class="text-xs mt-0.5" style="color: var(--text-muted);">Tweak colors and save as a new custom theme</p>
        </div>
      </button>
    {:else}
      <!-- Theme Editor Panel -->
      <div class="rounded-lg overflow-hidden" style="border: 1px solid var(--border-default);">
        <div class="flex items-center justify-between p-3 bg-bg-elevated">
          <span class="text-sm font-medium text-text-primary">Theme Editor</span>
          <div class="flex items-center gap-2">
            <button
              class="px-2 py-1 text-xs rounded transition-colors"
              style="background: var(--bg-hover); color: var(--text-secondary);"
              onclick={resetAllThemeVars}
            >Reset All</button>
            <button
              class="p-1 rounded transition-colors"
              style="color: var(--text-muted);"
              onclick={closeThemeEditor}
              aria-label="Close theme editor"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        <div class="p-3 space-y-4" style="background: var(--bg-surface);">
          {#each Object.entries(themeVariableGroups) as [groupName, vars] (groupName)}
            <div>
              <div class="text-xs font-semibold uppercase tracking-wider mb-2" style="color: var(--text-muted);">{groupName}</div>
              <div class="space-y-2">
                {#each vars as varName (varName)}
                  {@const isColorVar = !themeEditorVars[varName]?.startsWith('rgba') && !themeEditorVars[varName]?.includes('px')}
                  <div class="flex items-center gap-2">
                    <span class="text-xs w-20 flex-shrink-0" style="color: var(--text-secondary);">{varLabels[varName] || varName.replace('--', '')}</span>
                    {#if isColorVar}
                      <input
                        type="color"
                        value={cssColorToHex(themeEditorVars[varName] || '#000000')}
                        oninput={(e) => updateThemeVar(varName, e.currentTarget.value)}
                        class="w-8 h-8 rounded cursor-pointer"
                      />
                    {/if}
                    <input
                      type="text"
                      value={themeEditorVars[varName] || ''}
                      oninput={(e) => updateThemeVar(varName, e.currentTarget.value)}
                      class="flex-1 px-2 py-1 text-xs rounded font-mono"
                      style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-subtle);"
                    />
                    {#if themeEditorVars[varName] !== themeEditorDefaults[varName]}
                      <button
                        class="p-1 rounded transition-colors flex-shrink-0"
                        style="color: var(--text-muted);"
                        onclick={() => resetThemeVar(varName)}
                        title="Reset to default"
                      >
                        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                        </svg>
                      </button>
                    {:else}
                      <div class="w-[22px]"></div>
                    {/if}
                  </div>
                {/each}
              </div>
            </div>
          {/each}

          <!-- Save as theme -->
          <div class="pt-3 space-y-2" style="border-top: 1px solid var(--border-subtle);">
            <input
              type="text"
              bind:value={saveThemeName}
              placeholder="Theme name..."
              class="w-full px-3 py-2 text-sm rounded"
              style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-default);"
            />
            <input
              type="text"
              bind:value={saveThemeDescription}
              placeholder="Description (optional)"
              class="w-full px-3 py-2 text-sm rounded"
              style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-default);"
            />
            <input
              type="text"
              bind:value={saveThemeAuthor}
              placeholder="Author (optional)"
              class="w-full px-3 py-2 text-sm rounded"
              style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-default);"
            />
            <button
              class="w-full px-4 py-2 text-sm rounded font-medium transition-colors disabled:opacity-50"
              style="background: var(--accent-primary); color: var(--bg-base);"
              disabled={!saveThemeName.trim() || isSavingTheme}
              onclick={handleSaveTheme}
            >
              {isSavingTheme ? 'Saving...' : 'Save Theme'}
            </button>
            <p class="text-xs" style="color: var(--text-disabled);">
              Saves as a CSS file on the server. Changes are live-previewed above.
            </p>
          </div>
        </div>
      </div>
    {/if}

  </div>
</div>

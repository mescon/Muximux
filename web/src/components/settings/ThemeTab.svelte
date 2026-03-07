<script lang="ts">
  import { untrack } from 'svelte';
  import { resolvedTheme, allThemes, isDarkTheme, saveCustomThemeToServer, deleteCustomThemeFromServer, getCurrentThemeVariables, themeVariableGroups, sanitizeThemeId, selectedFamily, variantMode, themeFamilies, setThemeFamily, setVariantMode } from '$lib/themeStore';
  import { toasts } from '$lib/toastStore';
  import * as m from '$lib/paraglide/messages.js';

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
      toasts.success(m.toast_themeSaved());
    } else {
      toasts.error(m.toast_failedSaveTheme());
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
      toasts.success(m.toast_themeDeleted());
    } else {
      toasts.error(m.toast_failedDeleteTheme());
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
  const varLabels: Record<string, { get label(): string }> = {
    '--bg-base': { get label() { return m.theme_colorBase(); } },
    '--bg-surface': { get label() { return m.theme_colorSurface(); } },
    '--bg-elevated': { get label() { return m.theme_colorElevated(); } },
    '--text-primary': { get label() { return m.theme_colorPrimary(); } },
    '--text-secondary': { get label() { return m.theme_colorSecondary(); } },
    '--text-muted': { get label() { return m.theme_colorMuted(); } },
    '--accent-primary': { get label() { return m.theme_colorPrimary(); } },
    '--accent-secondary': { get label() { return m.theme_colorSecondary(); } },
    '--status-success': { get label() { return m.theme_statusSuccess(); } },
    '--status-warning': { get label() { return m.theme_statusWarning(); } },
    '--status-error': { get label() { return m.theme_statusError(); } },
    '--status-info': { get label() { return m.theme_statusInfo(); } },
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
          <div class="font-medium" style="color: var(--text-primary);">{m.theme_appearance()}</div>
          <div class="text-xs" style="color: var(--text-muted);">{m.theme_appearanceDesc()}</div>
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
              {m.theme_dark()}
            {:else if mode === 'system'}
              {m.theme_system()}
            {:else}
              {m.theme_light()}
            {/if}
          </button>
        {/each}
      </div>
    </div>
  </div>

  <!-- Theme Family Grid -->
  <div>
    <span class="block text-sm font-medium mb-3 text-text-secondary">
      {m.theme_chooseTheme()}
    </span>
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
      {#each $themeFamilies as family (family.id)}
        {@const isSelected = $selectedFamily === family.id}
        {@const isCustom = family.darkTheme ? !family.darkTheme.isBuiltin : family.lightTheme ? !family.lightTheme.isBuiltin : false}
        <div
          class="relative p-4 rounded-xl text-start transition-all group cursor-pointer"
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
          <div class="absolute top-3 end-3 flex items-center gap-1">
            {#if isCustom}
              <button
                class="w-5 h-5 rounded-full flex items-center justify-center opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity"
                style="background: var(--status-error); color: white;"
                tabindex="-1"
                onclick={(e: MouseEvent) => { e.stopPropagation(); handleDeleteTheme(family.darkTheme?.id || family.lightTheme?.id || ''); }}
                title={m.theme_deleteTheme()}
                aria-label={m.theme_deleteTheme()}
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
                {m.theme_custom()}
              </span>
            {/if}
          </div>
          {#if family.description}
            <div class="text-xs mt-0.5 pe-1" style="color: var(--text-muted);">{family.description}</div>
          {/if}

          <!-- Delete confirmation overlay -->
          {#if confirmDeleteTheme === (family.darkTheme?.id || family.lightTheme?.id)}
            <div class="absolute inset-0 rounded-xl flex items-center justify-center gap-3 z-10"
                 style="background: var(--bg-overlay); backdrop-filter: blur(4px);"
                 onclick={(e: MouseEvent) => e.stopPropagation()}
                 onkeydown={(e: KeyboardEvent) => e.stopPropagation()}
                 role="presentation">
                <span class="text-sm font-medium" style="color: var(--text-primary);">{m.common_deleteConfirm()}</span>
                <button class="px-3 py-1 rounded text-sm font-medium"
                        style="background: var(--status-error); color: white;"
                        onclick={(e: MouseEvent) => { e.stopPropagation(); confirmDeleteThemeAction(); }}>{m.common_yes()}</button>
                <button class="btn btn-secondary btn-sm"
                        onclick={(e: MouseEvent) => { e.stopPropagation(); confirmDeleteTheme = null; }}>{m.common_no()}</button>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </div>

  <!-- Current Theme Info -->
  <div class="p-4 rounded-lg bg-bg-elevated border border-border-subtle">
    <div class="flex items-center gap-2 text-sm">
      <span style="color: var(--text-muted);">{m.theme_currentlyUsing()}</span>
      <span class="font-medium capitalize" style="color: var(--text-primary);">
        {$allThemes.find(t => t.id === $resolvedTheme)?.name || $resolvedTheme} {m.theme_theme()}
      </span>
      {#if $variantMode === 'system'}
        <span class="text-xs" style="color: var(--text-disabled);">{m.theme_fromSystemPreference()}</span>
      {/if}
    </div>
  </div>

  <!-- Theme Customization -->
  <div class="space-y-3">
    {#if !showThemeEditor}
      <button
        class="w-full p-4 rounded-lg text-start transition-all hover:border-brand-500/50 flex items-center gap-3"
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
          <div class="font-medium text-sm" style="color: var(--text-primary);">{m.theme_customizeCurrentTheme()}</div>
          <p class="text-xs mt-0.5" style="color: var(--text-muted);">{m.theme_customizeDesc()}</p>
        </div>
      </button>
    {:else}
      <!-- Theme Editor Panel -->
      <div class="rounded-lg overflow-hidden" style="border: 1px solid var(--border-default);">
        <div class="flex items-center justify-between p-3 bg-bg-elevated">
          <span class="text-sm font-medium text-text-primary">{m.theme_editor()}</span>
          <div class="flex items-center gap-2">
            <button
              class="px-2 py-1 text-xs rounded transition-colors"
              style="background: var(--bg-hover); color: var(--text-secondary);"
              onclick={resetAllThemeVars}
            >{m.theme_resetAll()}</button>
            <button
              class="p-1 rounded transition-colors"
              style="color: var(--text-muted);"
              onclick={closeThemeEditor}
              aria-label={m.theme_closeEditor()}
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
                    <span class="text-xs w-20 flex-shrink-0" style="color: var(--text-secondary);">{varLabels[varName]?.label || varName.replace('--', '')}</span>
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
                        title={m.theme_resetToDefault()}
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
              placeholder={m.theme_namePlaceholder()}
              class="w-full px-3 py-2 text-sm rounded"
              style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-default);"
            />
            <input
              type="text"
              bind:value={saveThemeDescription}
              placeholder={m.theme_descriptionPlaceholder()}
              class="w-full px-3 py-2 text-sm rounded"
              style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-default);"
            />
            <input
              type="text"
              bind:value={saveThemeAuthor}
              placeholder={m.theme_authorPlaceholder()}
              class="w-full px-3 py-2 text-sm rounded"
              style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-default);"
            />
            <button
              class="w-full px-4 py-2 text-sm rounded font-medium transition-colors disabled:opacity-50 bg-accent-primary text-accent-on-primary"
              disabled={!saveThemeName.trim() || isSavingTheme}
              onclick={handleSaveTheme}
            >
              {isSavingTheme ? m.theme_saving() : m.theme_saveTheme()}
            </button>
            <p class="text-xs" style="color: var(--text-disabled);">
              {m.theme_saveHelp()}
            </p>
          </div>
        </div>
      </div>
    {/if}

  </div>
</div>

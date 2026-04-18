import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';
import {
  sanitizeThemeId,
  builtinThemes,
  customThemes,
  allThemes,
  selectedFamily,
  variantMode,
  systemTheme,
  themeFamilies,
  resolvedTheme,
  isDarkTheme,
  themeMode,
  setThemeFamily,
  setVariantMode,
  setTheme,
  toggleDarkMode,
  cycleTheme,
  registerCustomTheme,
  loadCustomThemeCSS,
  initTheme,
  syncFromConfig,
  getThemeInfo,
  getCurrentThemeVariables,
  themeVariableNames,
  themeVariableGroups,
  type ThemeInfo,
} from './themeStore';

// Mock fetch globally for detectCustomThemes
const mockFetch = vi.fn();

describe('themeStore', () => {
  let originalFetch: typeof globalThis.fetch;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
    globalThis.fetch = mockFetch;
    mockFetch.mockReset();

    // Reset stores to default state
    selectedFamily.set('default');
    variantMode.set('system');
    systemTheme.set('dark');
    customThemes.set([]);
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  describe('sanitizeThemeId', () => {
    it('converts to lowercase', () => {
      expect(sanitizeThemeId('MyTheme')).toBe('mytheme');
    });

    it('replaces non-alphanumeric chars with hyphens', () => {
      expect(sanitizeThemeId('hello world')).toBe('hello-world');
    });

    it('collapses multiple hyphens', () => {
      expect(sanitizeThemeId('a---b')).toBe('a-b');
    });

    it('removes leading and trailing hyphens', () => {
      expect(sanitizeThemeId('-hello-')).toBe('hello');
      expect(sanitizeThemeId('--test--')).toBe('test');
    });

    it('handles empty string', () => {
      expect(sanitizeThemeId('')).toBe('');
    });

    it('keeps numbers and hyphens', () => {
      expect(sanitizeThemeId('nord-2')).toBe('nord-2');
    });

    it('handles special characters', () => {
      expect(sanitizeThemeId('Theme @#$ Name')).toBe('theme-name');
    });
  });

  describe('builtinThemes', () => {
    it('has dark and light themes', () => {
      expect(builtinThemes).toHaveLength(2);
      expect(builtinThemes[0].id).toBe('dark');
      expect(builtinThemes[1].id).toBe('light');
    });

    it('both belong to default family', () => {
      expect(builtinThemes[0].family).toBe('default');
      expect(builtinThemes[1].family).toBe('default');
    });

    it('dark theme is marked isDark, light is not', () => {
      expect(builtinThemes[0].isDark).toBe(true);
      expect(builtinThemes[1].isDark).toBe(false);
    });
  });

  describe('store initialization (default state)', () => {
    it('selectedFamily defaults to "default"', () => {
      expect(get(selectedFamily)).toBe('default');
    });

    it('variantMode defaults to "system"', () => {
      expect(get(variantMode)).toBe('system');
    });
  });

  describe('allThemes derived store', () => {
    it('includes builtinThemes when no custom themes', () => {
      const themes = get(allThemes);
      expect(themes).toHaveLength(builtinThemes.length);
      expect(themes.map(t => t.id)).toContain('dark');
      expect(themes.map(t => t.id)).toContain('light');
    });

    it('includes custom themes when added', () => {
      customThemes.set([{
        id: 'nord-dark',
        name: 'Nord Dark',
        isBuiltin: false,
        isDark: true,
        family: 'nord',
        variant: 'dark',
        familyName: 'Nord',
      }]);
      const themes = get(allThemes);
      expect(themes).toHaveLength(3);
      expect(themes.map(t => t.id)).toContain('nord-dark');
    });
  });

  describe('themeFamilies derived store', () => {
    it('groups themes into families', () => {
      const families = get(themeFamilies);
      expect(families.length).toBeGreaterThanOrEqual(1);
      const defaultFamily = families.find(f => f.id === 'default');
      expect(defaultFamily).toBeDefined();
      expect(defaultFamily!.darkTheme).toBeDefined();
      expect(defaultFamily!.lightTheme).toBeDefined();
    });

    it('sorts default family first', () => {
      const families = get(themeFamilies);
      expect(families[0].id).toBe('default');
    });

    it('sorts custom families alphabetically after default', () => {
      customThemes.set([
        {
          id: 'zen-dark', name: 'Zen Dark', isBuiltin: false, isDark: true,
          family: 'zen', variant: 'dark', familyName: 'Zen',
        },
        {
          id: 'ayu-dark', name: 'Ayu Dark', isBuiltin: false, isDark: true,
          family: 'ayu', variant: 'dark', familyName: 'Ayu',
        },
      ]);
      const families = get(themeFamilies);
      expect(families[0].id).toBe('default');
      expect(families[1].id).toBe('ayu');
      expect(families[2].id).toBe('zen');
    });
  });

  describe('resolvedTheme derived store', () => {
    it('resolves to dark when variantMode is dark and family is default', () => {
      selectedFamily.set('default');
      variantMode.set('dark');
      expect(get(resolvedTheme)).toBe('dark');
    });

    it('resolves to light when variantMode is light and family is default', () => {
      selectedFamily.set('default');
      variantMode.set('light');
      expect(get(resolvedTheme)).toBe('light');
    });

    it('resolves based on systemTheme when variantMode is system', () => {
      selectedFamily.set('default');
      variantMode.set('system');
      systemTheme.set('dark');
      expect(get(resolvedTheme)).toBe('dark');

      systemTheme.set('light');
      expect(get(resolvedTheme)).toBe('light');
    });

    it('falls back to dark/light when family not found', () => {
      selectedFamily.set('nonexistent');
      variantMode.set('dark');
      expect(get(resolvedTheme)).toBe('dark');

      variantMode.set('light');
      expect(get(resolvedTheme)).toBe('light');
    });

    it('resolves custom family theme', () => {
      customThemes.set([
        {
          id: 'nord-dark', name: 'Nord Dark', isBuiltin: false, isDark: true,
          family: 'nord', variant: 'dark', familyName: 'Nord',
        },
        {
          id: 'nord-light', name: 'Nord Light', isBuiltin: false, isDark: false,
          family: 'nord', variant: 'light', familyName: 'Nord',
        },
      ]);
      selectedFamily.set('nord');
      variantMode.set('dark');
      expect(get(resolvedTheme)).toBe('nord-dark');

      variantMode.set('light');
      expect(get(resolvedTheme)).toBe('nord-light');
    });

    it('falls back to available variant when preferred variant missing', () => {
      customThemes.set([
        {
          id: 'mono-dark', name: 'Mono Dark', isBuiltin: false, isDark: true,
          family: 'mono', variant: 'dark', familyName: 'Mono',
        },
      ]);
      selectedFamily.set('mono');
      // Request light, but only dark exists -- should fallback to dark
      variantMode.set('light');
      expect(get(resolvedTheme)).toBe('mono-dark');
    });
  });

  describe('isDarkTheme derived store', () => {
    it('returns true when resolved theme is dark', () => {
      variantMode.set('dark');
      expect(get(isDarkTheme)).toBe(true);
    });

    it('returns false when resolved theme is light', () => {
      selectedFamily.set('default');
      variantMode.set('light');
      expect(get(isDarkTheme)).toBe(false);
    });
  });

  describe('themeMode backward-compat store', () => {
    it('returns system for default family + system variant', () => {
      selectedFamily.set('default');
      variantMode.set('system');
      expect(get(themeMode)).toBe('system');
    });

    it('returns dark for default family + dark variant', () => {
      selectedFamily.set('default');
      variantMode.set('dark');
      expect(get(themeMode)).toBe('dark');
    });

    it('returns light for default family + light variant', () => {
      selectedFamily.set('default');
      variantMode.set('light');
      expect(get(themeMode)).toBe('light');
    });

    it('returns family id for custom families', () => {
      selectedFamily.set('nord');
      variantMode.set('dark');
      expect(get(themeMode)).toBe('nord');
    });
  });

  describe('setThemeFamily', () => {
    it('updates selectedFamily store and persists to localStorage', () => {
      setThemeFamily('nord');
      expect(get(selectedFamily)).toBe('nord');
      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_theme_family', 'nord');
    });
  });

  describe('setVariantMode', () => {
    it('updates variantMode store and persists to localStorage', () => {
      setVariantMode('light');
      expect(get(variantMode)).toBe('light');
      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_theme_variant', 'light');
    });
  });

  describe('setTheme (backward-compat)', () => {
    it('sets system mode for "system"', () => {
      setTheme('system');
      expect(get(selectedFamily)).toBe('default');
      expect(get(variantMode)).toBe('system');
    });

    it('sets dark variant for "dark"', () => {
      setTheme('dark');
      expect(get(variantMode)).toBe('dark');
    });

    it('sets light variant for "light"', () => {
      setTheme('light');
      expect(get(variantMode)).toBe('light');
    });

    it('finds family for known custom theme ID', () => {
      customThemes.set([
        {
          id: 'nord-dark', name: 'Nord Dark', isBuiltin: false, isDark: true,
          family: 'nord', variant: 'dark', familyName: 'Nord',
        },
      ]);
      setTheme('nord-dark');
      expect(get(selectedFamily)).toBe('nord');
      expect(get(variantMode)).toBe('dark');
    });

    it('treats unknown ID as standalone family', () => {
      setTheme('custom-standalone');
      expect(get(selectedFamily)).toBe('custom-standalone');
      expect(get(variantMode)).toBe('dark');
    });
  });

  describe('toggleDarkMode', () => {
    it('toggles from dark to light', () => {
      setVariantMode('dark');
      toggleDarkMode();
      expect(get(variantMode)).toBe('light');
    });

    it('toggles from light to dark', () => {
      setVariantMode('light');
      toggleDarkMode();
      expect(get(variantMode)).toBe('dark');
    });

    it('toggles from system to dark (system != dark)', () => {
      setVariantMode('system');
      toggleDarkMode();
      // 'system' !== 'dark', so it switches to 'dark'
      expect(get(variantMode)).toBe('dark');
    });
  });

  describe('cycleTheme', () => {
    it('cycles to next family', () => {
      const families = get(themeFamilies);
      setThemeFamily(families[0].id);
      cycleTheme();
      // With only default family, it should wrap around
      if (families.length === 1) {
        expect(get(selectedFamily)).toBe(families[0].id);
      } else {
        expect(get(selectedFamily)).toBe(families[1].id);
      }
    });

    it('wraps around from last to first', () => {
      customThemes.set([
        {
          id: 'nord-dark', name: 'Nord Dark', isBuiltin: false, isDark: true,
          family: 'nord', variant: 'dark', familyName: 'Nord',
        },
      ]);
      const families = get(themeFamilies);
      // Set to last family
      setThemeFamily(families[families.length - 1].id);
      cycleTheme();
      expect(get(selectedFamily)).toBe(families[0].id);
    });
  });

  describe('registerCustomTheme', () => {
    it('adds a new theme to customThemes', () => {
      const theme: ThemeInfo = {
        id: 'test-custom',
        name: 'Test Custom',
        isBuiltin: false,
        isDark: true,
        family: 'test',
        variant: 'dark',
      };
      registerCustomTheme(theme);
      const themes = get(customThemes);
      expect(themes.some(t => t.id === 'test-custom')).toBe(true);
    });

    it('updates existing theme by id', () => {
      const theme1: ThemeInfo = {
        id: 'update-test',
        name: 'Before',
        isBuiltin: false,
        isDark: true,
      };
      const theme2: ThemeInfo = {
        id: 'update-test',
        name: 'After',
        isBuiltin: false,
        isDark: false,
      };
      registerCustomTheme(theme1);
      registerCustomTheme(theme2);
      const themes = get(customThemes);
      const found = themes.filter(t => t.id === 'update-test');
      expect(found).toHaveLength(1);
      expect(found[0].name).toBe('After');
    });
  });

  describe('loadCustomThemeCSS', () => {
    it('returns true if link element already exists', async () => {
      const mockLink = document.createElement('link');
      mockLink.id = 'theme-existing';
      document.head.appendChild(mockLink);

      const result = await loadCustomThemeCSS('existing');
      expect(result).toBe(true);

      mockLink.remove();
    });

    it('creates a link element for new themes', async () => {
      const promise = loadCustomThemeCSS('new-theme');

      const link = document.getElementById('theme-new-theme') as HTMLLinkElement;
      expect(link).toBeDefined();
      expect(link?.getAttribute('rel')).toBe('stylesheet');
      expect(link?.getAttribute('href')).toBe('/themes/new-theme.css');

      // Simulate load
      if (link?.onload) {
        (link.onload as EventListener)(new Event('load'));
      }

      const result = await promise;
      expect(result).toBe(true);

      link?.remove();
    });

    it('removes link and returns false on error', async () => {
      const promise = loadCustomThemeCSS('error-theme');

      const link = document.getElementById('theme-error-theme') as HTMLLinkElement;
      expect(link).toBeDefined();

      // Simulate error
      if (link?.onerror) {
        (link.onerror as EventListener)(new Event('error'));
      }

      const result = await promise;
      expect(result).toBe(false);

      expect(document.getElementById('theme-error-theme')).toBeNull();
    });
  });

  describe('initTheme', () => {
    beforeEach(() => {
      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve([]),
      });
    });

    it('subscribes to resolvedTheme and sets data-theme on document', async () => {
      selectedFamily.set('default');
      variantMode.set('dark');

      initTheme();

      // Give the subscription time to fire
      await new Promise(r => setTimeout(r, 50));

      expect(document.documentElement.dataset.theme).toBe('dark');
    });

    it('sets up matchMedia listener', () => {
      initTheme();
      expect(globalThis.matchMedia).toHaveBeenCalledWith('(prefers-color-scheme: dark)');
    });
  });

  describe('syncFromConfig', () => {
    it('updates family and variant from config', () => {
      syncFromConfig({ family: 'nord', variant: 'light' });
      expect(get(selectedFamily)).toBe('nord');
      expect(get(variantMode)).toBe('light');
    });

    it('does nothing for null config', () => {
      setThemeFamily('default');
      syncFromConfig(null as unknown as { family: string; variant: string });
      expect(get(selectedFamily)).toBe('default');
    });

    it('does not update family if same value', () => {
      setThemeFamily('default');
      vi.clearAllMocks();
      syncFromConfig({ family: 'default', variant: 'dark' });
      expect(get(selectedFamily)).toBe('default');
    });

    it('does not update variant if same value', () => {
      setVariantMode('dark');
      vi.clearAllMocks();
      syncFromConfig({ family: 'default', variant: 'dark' });
      expect(get(variantMode)).toBe('dark');
    });

    it('ignores invalid variant values', () => {
      setVariantMode('dark');
      syncFromConfig({ family: 'default', variant: 'invalid' });
      expect(get(variantMode)).toBe('dark');
    });
  });

  describe('getThemeInfo', () => {
    it('returns theme info for known theme', () => {
      const info = getThemeInfo('dark');
      expect(info).toBeDefined();
      expect(info?.id).toBe('dark');
      expect(info?.isDark).toBe(true);
    });

    it('returns undefined for unknown theme', () => {
      const info = getThemeInfo('nonexistent');
      expect(info).toBeUndefined();
    });
  });

  describe('getCurrentThemeVariables', () => {
    it('reads CSS variables from computed style', () => {
      const vars = getCurrentThemeVariables();
      expect(typeof vars).toBe('object');
      expect('--bg-base' in vars).toBe(true);
      expect('--accent-primary' in vars).toBe(true);
    });
  });

  describe('themeVariableNames and themeVariableGroups', () => {
    it('themeVariableNames is a non-empty array', () => {
      expect(themeVariableNames.length).toBeGreaterThan(0);
      expect(themeVariableNames).toContain('--bg-base');
    });

    it('themeVariableGroups has expected groups', () => {
      expect(themeVariableGroups).toHaveProperty('Backgrounds');
      expect(themeVariableGroups).toHaveProperty('Text');
      expect(themeVariableGroups).toHaveProperty('Accents');
      expect(themeVariableGroups).toHaveProperty('Status');
    });
  });

  describe('detectCustomThemes', () => {
    it('handles API failure gracefully', async () => {
      const { detectCustomThemes, customThemes } = await import('./themeStore');
      customThemes.set([]);
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      await detectCustomThemes();
      expect(get(customThemes)).toEqual([]);
    });

    it('handles non-ok response', async () => {
      const { detectCustomThemes, customThemes } = await import('./themeStore');
      customThemes.set([]);
      mockFetch.mockResolvedValueOnce({ ok: false });

      await detectCustomThemes();
      expect(get(customThemes)).toEqual([]);
    });
  });

  describe('saveCustomThemeToServer', () => {
    it('returns false on API failure', async () => {
      const { saveCustomThemeToServer } = await import('./themeStore');
      mockFetch.mockResolvedValueOnce({ ok: false });

      const result = await saveCustomThemeToServer('Test', 'dark', true, {});
      expect(result).toBe(false);
    });

    it('returns false on network error', async () => {
      const { saveCustomThemeToServer } = await import('./themeStore');
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      const result = await saveCustomThemeToServer('Test', 'dark', true, {});
      expect(result).toBe(false);
    });
  });

  describe('deleteCustomThemeFromServer', () => {
    it('returns false on API failure', async () => {
      const { deleteCustomThemeFromServer } = await import('./themeStore');
      mockFetch.mockResolvedValueOnce({ ok: false });

      const result = await deleteCustomThemeFromServer('nord-dark');
      expect(result).toBe(false);
    });

    it('returns false on network error', async () => {
      const { deleteCustomThemeFromServer } = await import('./themeStore');
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      const result = await deleteCustomThemeFromServer('nord-dark');
      expect(result).toBe(false);
    });

    it('removes link element and refreshes custom themes on success', async () => {
      const { deleteCustomThemeFromServer } = await import('./themeStore');

      // Create a mock link element
      const mockLink = document.createElement('link');
      mockLink.id = 'theme-test-delete';
      document.head.appendChild(mockLink);

      // First call: DELETE succeeds
      mockFetch.mockResolvedValueOnce({ ok: true });
      // Second call: detectCustomThemes fetch
      mockFetch.mockResolvedValueOnce({ ok: true, json: () => Promise.resolve([]) });

      const result = await deleteCustomThemeFromServer('test-delete');
      expect(result).toBe(true);
      expect(document.getElementById('theme-test-delete')).toBeNull();
    });
  });

  describe('saveCustomThemeToServer - success', () => {
    it('saves theme, refreshes custom themes, and reloads CSS', async () => {
      const { saveCustomThemeToServer } = await import('./themeStore');

      // Auto-fire onload for any link elements that get created
      const origCreateElement = document.createElement.bind(document);
      vi.spyOn(document, 'createElement').mockImplementation((tag: string, opts?: ElementCreationOptions) => {
        const el = origCreateElement(tag, opts);
        if (tag === 'link') {
          // Simulate load event after appending
          setTimeout(() => {
            if ((el as HTMLLinkElement).onload) {
              (el as HTMLLinkElement).onload!(new Event('load'));
            }
          }, 0);
        }
        return el;
      });

      // First call: POST succeeds
      mockFetch.mockResolvedValueOnce({ ok: true });
      // Second call: detectCustomThemes fetch
      mockFetch.mockResolvedValueOnce({ ok: true, json: () => Promise.resolve([]) });

      const result = await saveCustomThemeToServer('My Theme', 'dark', true, {
        '--bg-base': '#000',
      });

      expect(result).toBe(true);
      expect(mockFetch).toHaveBeenCalledWith('/api/themes', expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      }));

      vi.restoreAllMocks();
    });
  });

  describe('detectCustomThemes - success', () => {
    it('loads custom themes from API and sets customThemes store', async () => {
      const { detectCustomThemes } = await import('./themeStore');

      const themes = [
        {
          id: 'nord-dark',
          name: 'Nord Dark',
          isBuiltin: false,
          isDark: true,
          family: 'nord',
          variant: 'dark',
          familyName: 'Nord',
        },
      ];

      // Auto-fire onload for any link elements
      const origCreateElement = document.createElement.bind(document);
      vi.spyOn(document, 'createElement').mockImplementation((tag: string, opts?: ElementCreationOptions) => {
        const el = origCreateElement(tag, opts);
        if (tag === 'link') {
          setTimeout(() => {
            if ((el as HTMLLinkElement).onload) {
              (el as HTMLLinkElement).onload!(new Event('load'));
            }
          }, 0);
        }
        return el;
      });

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(themes),
      });

      await detectCustomThemes();

      const loadedThemes = get(customThemes);
      expect(loadedThemes).toHaveLength(1);
      expect(loadedThemes[0].id).toBe('nord-dark');

      vi.restoreAllMocks();
    });
  });

  describe('themeFamilies - theme variant assignment', () => {
    it('assigns theme without variant based on isDark', () => {
      customThemes.set([
        {
          id: 'solo-light',
          name: 'Solo Light',
          isBuiltin: false,
          isDark: false,
          // No variant property, no family property
        },
      ]);

      const families = get(themeFamilies);
      const soloFamily = families.find(f => f.id === 'solo-light');
      expect(soloFamily).toBeDefined();
      expect(soloFamily!.lightTheme).toBeDefined();
    });

    it('assigns theme without variant and isDark=true as dark', () => {
      customThemes.set([
        {
          id: 'solo-dark',
          name: 'Solo Dark',
          isBuiltin: false,
          isDark: true,
          // No variant property
        },
      ]);

      const families = get(themeFamilies);
      const soloFamily = families.find(f => f.id === 'solo-dark');
      expect(soloFamily).toBeDefined();
      expect(soloFamily!.darkTheme).toBeDefined();
    });
  });

  describe('initTheme - migration', () => {
    it('migrates old system theme from localStorage', () => {
      // Set up old format
      localStorage.setItem('muximux_theme', 'system');

      // Clear new format keys so migration runs
      // (localStorage is already cleared in beforeEach, so we just set the old key)

      initTheme();

      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_theme_family', 'default');
      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_theme_variant', 'system');
    });

    it('migrates old dark theme from localStorage', () => {
      localStorage.setItem('muximux_theme', 'dark');

      initTheme();

      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_theme_family', 'default');
      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_theme_variant', 'dark');
    });

    it('migrates old light theme from localStorage', () => {
      localStorage.setItem('muximux_theme', 'light');

      initTheme();

      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_theme_family', 'default');
      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_theme_variant', 'light');
    });

    it('migrates old custom theme from localStorage', () => {
      localStorage.setItem('muximux_theme', 'nord');

      initTheme();

      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_theme_family', 'nord');
      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_theme_variant', 'dark');
    });

    it('does not migrate if new format already exists', () => {
      localStorage.setItem('muximux_theme_family', 'default');
      localStorage.setItem('muximux_theme', 'nord');

      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve([]),
      });

      initTheme();

      // Should not have tried to migrate
      expect(localStorage.removeItem).not.toHaveBeenCalledWith('muximux_theme');
    });
  });

  describe('syncFromConfig - edge cases', () => {
    it('does not update when family is empty string', () => {
      setThemeFamily('default');
      syncFromConfig({ family: '', variant: 'dark' });
      // Empty family should be falsy, so no update
      expect(get(selectedFamily)).toBe('default');
    });
  });
});

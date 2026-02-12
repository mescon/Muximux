/**
 * Theme Store - Manages application theme families with dark/light variants
 *
 * Themes are organized into families (e.g. "Nord"), each with dark and light variants.
 * Users pick a family and a variant mode (dark/light/system). When set to "system",
 * the OS preference determines which variant gets applied.
 *
 * Themes are applied via the data-theme attribute on <html>.
 */

import { writable, derived, get } from 'svelte/store';

// Built-in themes
export type BuiltinTheme = 'dark' | 'light';

// Theme mode includes system preference option (kept for backward compat with App.svelte keybindings)
export type ThemeMode = BuiltinTheme | 'system' | string;

// What actually gets applied
export type ResolvedTheme = string;

// Variant mode for the family system
export type VariantMode = 'dark' | 'light' | 'system';

// Theme metadata for display (matches backend ThemeInfo JSON)
export interface ThemeInfo {
  id: string;
  name: string;
  description?: string;
  isBuiltin: boolean;
  isDark: boolean;
  family?: string;
  variant?: string;
  familyName?: string;
  preview?: {
    bg: string;
    surface: string;
    accent: string;
    text: string;
  };
}

// A family groups dark + light variants together
export interface ThemeFamily {
  id: string;
  name: string;
  description?: string;
  darkTheme?: ThemeInfo;
  lightTheme?: ThemeInfo;
}

// Built-in theme definitions (CSS-only, defined in app.css)
export const builtinThemes: ThemeInfo[] = [
  {
    id: 'dark',
    name: 'Dark',
    description: 'Deep charcoals with teal accents',
    isBuiltin: true,
    isDark: true,
    family: 'default',
    variant: 'dark',
    familyName: 'Muximux',
    preview: {
      bg: '#09090b',
      surface: '#111114',
      accent: '#2dd4bf',
      text: '#fafafa'
    }
  },
  {
    id: 'light',
    name: 'Light',
    description: 'Clean and bright with subtle shadows',
    isBuiltin: true,
    isDark: false,
    family: 'default',
    variant: 'light',
    familyName: 'Muximux',
    preview: {
      bg: '#fafafa',
      surface: '#ffffff',
      accent: '#0d9488',
      text: '#18181b'
    }
  }
];

// Custom themes from API
export const customThemes = writable<ThemeInfo[]>([]);

// Combined list of all available themes
export const allThemes = derived(
  customThemes,
  ($customThemes) => [...builtinThemes, ...$customThemes]
);

// --- Storage keys ---
const FAMILY_KEY = 'muximux_theme_family';
const VARIANT_KEY = 'muximux_theme_variant';
const OLD_THEME_KEY = 'muximux_theme';

// --- Helper: read from localStorage ---
function getStoredFamily(): string {
  if (typeof window === 'undefined') return 'default';
  return localStorage.getItem(FAMILY_KEY) || 'default';
}

function getStoredVariantMode(): VariantMode {
  if (typeof window === 'undefined') return 'system';
  const stored = localStorage.getItem(VARIANT_KEY);
  if (stored === 'dark' || stored === 'light' || stored === 'system') return stored;
  return 'system';
}

function getSystemPreference(): BuiltinTheme {
  if (typeof window === 'undefined') return 'dark';
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

// --- Stores ---
export const selectedFamily = writable<string>(getStoredFamily());
export const variantMode = writable<VariantMode>(getStoredVariantMode());
export const systemTheme = writable<BuiltinTheme>(getSystemPreference());

// Group themes into families
export const themeFamilies = derived(
  allThemes,
  ($allThemes) => {
    const familyMap = new Map<string, ThemeFamily>();

    for (const theme of $allThemes) {
      const famId = theme.family || theme.id;
      const famName = theme.familyName || theme.name;

      if (!familyMap.has(famId)) {
        familyMap.set(famId, {
          id: famId,
          name: famName,
          description: theme.description,
        });
      }

      const family = familyMap.get(famId)!;
      if (theme.variant === 'light' || (!theme.variant && !theme.isDark)) {
        family.lightTheme = theme;
      } else {
        family.darkTheme = theme;
      }
    }

    // Sort: "default" first, then alphabetical
    return Array.from(familyMap.values()).sort((a, b) => {
      if (a.id === 'default') return -1;
      if (b.id === 'default') return 1;
      return a.name.localeCompare(b.name);
    });
  }
);

// Resolve the actual theme ID to apply
export const resolvedTheme = derived(
  [selectedFamily, variantMode, systemTheme, themeFamilies],
  ([$selectedFamily, $variantMode, $systemTheme, $themeFamilies]) => {
    const family = $themeFamilies.find(f => f.id === $selectedFamily);

    // Determine which variant we want
    const wantDark = $variantMode === 'system' ? $systemTheme === 'dark' : $variantMode === 'dark';

    if (family) {
      const preferred = wantDark ? family.darkTheme : family.lightTheme;
      if (preferred) return preferred.id;
      // Fallback: use whichever variant exists
      const fallback = family.darkTheme || family.lightTheme;
      if (fallback) return fallback.id;
    }

    // Ultimate fallback: built-in dark/light
    return wantDark ? 'dark' : 'light';
  }
);

// Whether the current resolved theme is dark
export const isDarkTheme = derived(
  [resolvedTheme, allThemes],
  ([$resolvedTheme, $allThemes]) => {
    const theme = $allThemes.find(t => t.id === $resolvedTheme);
    return theme?.isDark ?? true;
  }
);

// Backward-compatible themeMode (derived, read-only for external consumers)
export const themeMode = derived(
  [selectedFamily, variantMode],
  ([$selectedFamily, $variantMode]) => {
    if ($selectedFamily === 'default' && $variantMode === 'system') return 'system' as ThemeMode;
    if ($selectedFamily === 'default' && $variantMode === 'dark') return 'dark' as ThemeMode;
    if ($selectedFamily === 'default' && $variantMode === 'light') return 'light' as ThemeMode;
    // For custom families, return the resolved family id
    return $selectedFamily as ThemeMode;
  }
);

// --- Apply theme to document ---
async function applyTheme(theme: string) {
  if (typeof document === 'undefined') return;

  const root = document.documentElement;

  // Load external theme CSS if needed
  const themeInfo = get(allThemes).find(t => t.id === theme);
  if (themeInfo && !themeInfo.isBuiltin) {
    await loadCustomThemeCSS(theme);
  }

  // Set data-theme attribute
  root.setAttribute('data-theme', theme);

  // Maintain .dark class for Tailwind compatibility
  if (themeInfo?.isDark ?? theme === 'dark') {
    root.classList.add('dark');
  } else {
    root.classList.remove('dark');
  }
}

// --- Public API ---

// Set theme family and persist
export function setThemeFamily(familyId: string) {
  selectedFamily.set(familyId);
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(FAMILY_KEY, familyId);
  }
}

// Set variant mode and persist
export function setVariantMode(mode: VariantMode) {
  variantMode.set(mode);
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(VARIANT_KEY, mode);
  }
}

// Backward-compatible setTheme (used by App.svelte keybindings)
export function setTheme(mode: ThemeMode) {
  if (mode === 'system') {
    setThemeFamily('default');
    setVariantMode('system');
  } else if (mode === 'dark') {
    setVariantMode('dark');
  } else if (mode === 'light') {
    setVariantMode('light');
  } else {
    // Custom theme ID — find its family
    const themes = get(allThemes);
    const theme = themes.find(t => t.id === mode);
    if (theme?.family) {
      setThemeFamily(theme.family);
      setVariantMode(theme.isDark ? 'dark' : 'light');
    } else {
      // Standalone theme — treat it as its own family
      setThemeFamily(mode);
      setVariantMode('dark');
    }
  }
}

// Toggle between dark and light variant
export function toggleDarkMode() {
  const current = get(variantMode);
  setVariantMode(current === 'dark' ? 'light' : 'dark');
}

// Cycle through families
export function cycleTheme() {
  const families = get(themeFamilies);
  const currentFam = get(selectedFamily);
  const currentIndex = families.findIndex(f => f.id === currentFam);
  const nextIndex = (currentIndex + 1) % families.length;
  setThemeFamily(families[nextIndex].id);
}

// Register a custom theme
export function registerCustomTheme(theme: ThemeInfo) {
  customThemes.update(themes => {
    if (themes.some(t => t.id === theme.id)) {
      return themes.map(t => t.id === theme.id ? theme : t);
    }
    return [...themes, theme];
  });
}

// Load custom theme CSS (if not already in document)
export async function loadCustomThemeCSS(themeId: string): Promise<boolean> {
  if (typeof document === 'undefined') return false;

  const linkId = `theme-${themeId}`;
  if (document.getElementById(linkId)) return true;

  try {
    const link = document.createElement('link');
    link.id = linkId;
    link.rel = 'stylesheet';
    link.href = `/themes/${themeId}.css`;

    return new Promise((resolve) => {
      link.onload = () => resolve(true);
      link.onerror = () => {
        link.remove();
        resolve(false);
      };
      document.head.appendChild(link);
    });
  } catch {
    return false;
  }
}

// Pre-load CSS for both variants of a family
async function preloadFamilyCSS(familyId: string) {
  const families = get(themeFamilies);
  const family = families.find(f => f.id === familyId);
  if (!family) return;

  const loads: Promise<boolean>[] = [];
  if (family.darkTheme && !family.darkTheme.isBuiltin) {
    loads.push(loadCustomThemeCSS(family.darkTheme.id));
  }
  if (family.lightTheme && !family.lightTheme.isBuiltin) {
    loads.push(loadCustomThemeCSS(family.lightTheme.id));
  }
  await Promise.all(loads);
}

// Detect available custom themes from the API
export async function detectCustomThemes(): Promise<void> {
  if (typeof window === 'undefined') return;

  try {
    const response = await fetch('/api/themes');
    if (response.ok) {
      const themes: ThemeInfo[] = await response.json();
      customThemes.set(themes);

      // Load CSS for all themes
      await Promise.all(themes.map(t => loadCustomThemeCSS(t.id)));

      // Pre-load both variants of the selected family
      await preloadFamilyCSS(get(selectedFamily));
    }
  } catch {
    // API not available — no custom themes
  }
}

// Migrate from old localStorage format
function migrateOldThemeStorage() {
  if (typeof window === 'undefined') return;

  // Already migrated?
  if (localStorage.getItem(FAMILY_KEY)) return;

  const oldTheme = localStorage.getItem(OLD_THEME_KEY);
  if (!oldTheme) return;

  if (oldTheme === 'system') {
    localStorage.setItem(FAMILY_KEY, 'default');
    localStorage.setItem(VARIANT_KEY, 'system');
  } else if (oldTheme === 'dark') {
    localStorage.setItem(FAMILY_KEY, 'default');
    localStorage.setItem(VARIANT_KEY, 'dark');
  } else if (oldTheme === 'light') {
    localStorage.setItem(FAMILY_KEY, 'default');
    localStorage.setItem(VARIANT_KEY, 'light');
  } else {
    // Custom theme ID like 'nord' — will resolve family after themes load
    localStorage.setItem(FAMILY_KEY, oldTheme);
    localStorage.setItem(VARIANT_KEY, 'dark');
  }

  localStorage.removeItem(OLD_THEME_KEY);

  // Re-read the migrated values into stores
  selectedFamily.set(getStoredFamily());
  variantMode.set(getStoredVariantMode());
}

// Post-load migration: fix family ID after custom themes are loaded
function postLoadMigration() {
  const currentFamily = get(selectedFamily);
  const families = get(themeFamilies);

  // If the stored family doesn't match any family ID, try to find it as a theme ID
  if (!families.some(f => f.id === currentFamily)) {
    const themes = get(allThemes);
    const theme = themes.find(t => t.id === currentFamily);
    if (theme?.family) {
      setThemeFamily(theme.family);
      setVariantMode(theme.isDark ? 'dark' : 'light');
    }
  }
}

// Initialize theme system
export function initTheme() {
  if (typeof window === 'undefined') return;

  // Migrate old storage format first
  migrateOldThemeStorage();

  // Listen for system preference changes
  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
  const handleChange = (e: MediaQueryListEvent | MediaQueryList) => {
    systemTheme.set(e.matches ? 'dark' : 'light');
  };

  if (mediaQuery.addEventListener) {
    mediaQuery.addEventListener('change', handleChange);
  } else {
    mediaQuery.addListener(handleChange as (e: MediaQueryListEvent) => void);
  }

  handleChange(mediaQuery);

  // Subscribe to resolved theme changes and apply
  resolvedTheme.subscribe(applyTheme);

  // Detect and load custom themes, then fix up migration
  detectCustomThemes().then(() => {
    postLoadMigration();
    // Re-apply after themes loaded to ensure correct CSS is active
    applyTheme(get(resolvedTheme));
  });
}

// Sync theme stores from server-side config (called after config loads)
export function syncFromConfig(theme: { family: string; variant: string }) {
  if (!theme) return;
  if (theme.family && theme.family !== get(selectedFamily)) {
    setThemeFamily(theme.family);
  }
  const v = theme.variant as VariantMode;
  if (v && (v === 'dark' || v === 'light' || v === 'system') && v !== get(variantMode)) {
    setVariantMode(v);
  }
}

// Convert a theme name to a safe filesystem ID
export function sanitizeThemeId(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/-+/g, '-').replace(/^-|-$/g, '');
}

// Get theme info by ID
export function getThemeInfo(themeId: string): ThemeInfo | undefined {
  return get(allThemes).find(t => t.id === themeId);
}

// Save a custom theme via API
export async function saveCustomThemeToServer(
  name: string,
  baseTheme: string,
  isDark: boolean,
  variables: Record<string, string>,
  description?: string,
  author?: string
): Promise<boolean> {
  try {
    const response = await fetch('/api/themes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, baseTheme, isDark, variables, description, author })
    });

    if (!response.ok) return false;

    await detectCustomThemes();

    const id = sanitizeThemeId(name);
    const linkEl = document.getElementById(`theme-${id}`);
    if (linkEl) linkEl.remove();
    await loadCustomThemeCSS(id);

    return true;
  } catch {
    return false;
  }
}

// Delete a custom theme via API
export async function deleteCustomThemeFromServer(themeId: string): Promise<boolean> {
  try {
    const response = await fetch(`/api/themes/${themeId}`, {
      method: 'DELETE'
    });

    if (!response.ok) return false;

    const linkEl = document.getElementById(`theme-${themeId}`);
    if (linkEl) linkEl.remove();

    // If the deleted theme was from the current family, check if we need to fallback
    const currentResolved = get(resolvedTheme);
    if (currentResolved === themeId) {
      // resolvedTheme will auto-fallback after customThemes refreshes
    }

    await detectCustomThemes();
    return true;
  } catch {
    return false;
  }
}

// Get the current computed CSS variables for a theme
export function getCurrentThemeVariables(): Record<string, string> {
  if (typeof document === 'undefined') return {};
  const style = getComputedStyle(document.documentElement);
  const vars: Record<string, string> = {};
  for (const name of themeVariableNames) {
    vars[name] = style.getPropertyValue(name).trim();
  }
  return vars;
}

// Key CSS variable names that define a theme
export const themeVariableNames = [
  '--bg-base', '--bg-surface', '--bg-elevated', '--bg-overlay', '--bg-hover', '--bg-active',
  '--glass-bg', '--glass-border', '--glass-highlight',
  '--text-primary', '--text-secondary', '--text-muted', '--text-disabled',
  '--border-subtle', '--border-default', '--border-strong',
  '--accent-primary', '--accent-secondary', '--accent-muted', '--accent-subtle',
  '--status-success', '--status-warning', '--status-error', '--status-info',
  '--shadow-sm', '--shadow-md', '--shadow-lg', '--shadow-glow',
] as const;

// Grouped variable names for the editor UI
export const themeVariableGroups = {
  'Backgrounds': ['--bg-base', '--bg-surface', '--bg-elevated'],
  'Text': ['--text-primary', '--text-secondary', '--text-muted'],
  'Accents': ['--accent-primary', '--accent-secondary'],
  'Status': ['--status-success', '--status-warning', '--status-error', '--status-info'],
} as const;

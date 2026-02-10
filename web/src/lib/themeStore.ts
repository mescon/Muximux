/**
 * Theme Store - Manages application theme (dark/light/system/custom)
 *
 * Themes are applied via the data-theme attribute on <html>
 * Custom themes can be loaded from CSS files that define the same CSS custom properties
 */

import { writable, derived, get } from 'svelte/store';

// Built-in themes
export type BuiltinTheme = 'dark' | 'light';

// Theme mode includes system preference option
export type ThemeMode = BuiltinTheme | 'system' | string; // string for custom theme names

// What actually gets applied
export type ResolvedTheme = string;

// Theme metadata for display
export interface ThemeInfo {
  id: string;
  name: string;
  description?: string;
  isBuiltin: boolean;
  isDark: boolean; // For determining icon colors, etc.
  preview?: {
    bg: string;
    surface: string;
    accent: string;
    text: string;
  };
}

// Built-in theme definitions (CSS-only, no hardcoded data)
// Dark and Light are defined in app.css; all others come from /api/themes
export const builtinThemes: ThemeInfo[] = [
  {
    id: 'dark',
    name: 'Dark',
    description: 'Deep charcoals with teal accents',
    isBuiltin: true,
    isDark: true,
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
    preview: {
      bg: '#fafafa',
      surface: '#ffffff',
      accent: '#0d9488',
      text: '#18181b'
    }
  }
];

// Custom themes that can be detected/loaded
// Users can add their own by creating CSS files
export const customThemes = writable<ThemeInfo[]>([]);

// Combined list of all available themes
export const allThemes = derived(
  customThemes,
  ($customThemes) => [...builtinThemes, ...$customThemes]
);

const THEME_KEY = 'muximux_theme';

// Get stored theme or default to 'dark'
function getStoredTheme(): ThemeMode {
  if (typeof window === 'undefined') return 'dark';
  const stored = localStorage.getItem(THEME_KEY);
  if (stored) return stored;
  return 'dark'; // Default to dark mode
}

// Get system preference
function getSystemPreference(): BuiltinTheme {
  if (typeof window === 'undefined') return 'dark';
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

// Create the writable store for user's theme choice
export const themeMode = writable<ThemeMode>(getStoredTheme());

// Track system theme preference
export const systemTheme = writable<BuiltinTheme>(getSystemPreference());

// Resolved theme (actual theme being applied)
export const resolvedTheme = derived(
  [themeMode, systemTheme],
  ([$themeMode, $systemTheme]) => {
    if ($themeMode === 'system') {
      return $systemTheme;
    }
    return $themeMode;
  }
);

// Whether the current resolved theme is dark (for UI elements that need to know)
export const isDarkTheme = derived(
  [resolvedTheme, allThemes],
  ([$resolvedTheme, $allThemes]) => {
    const theme = $allThemes.find(t => t.id === $resolvedTheme);
    return theme?.isDark ?? true; // Default to dark if unknown
  }
);

// Apply theme to document
async function applyTheme(theme: string) {
  if (typeof document === 'undefined') return;

  const root = document.documentElement;

  // Load external theme CSS if needed
  const themeInfo = get(allThemes).find(t => t.id === theme);
  if (themeInfo && !themeInfo.isBuiltin) {
    await loadCustomThemeCSS(theme);
  }

  // Set data-theme attribute (used by CSS)
  root.setAttribute('data-theme', theme);

  // Also maintain the .dark class for Tailwind compatibility
  if (themeInfo?.isDark ?? theme === 'dark') {
    root.classList.add('dark');
  } else {
    root.classList.remove('dark');
  }
}

// Set theme and persist
export function setTheme(mode: ThemeMode) {
  themeMode.set(mode);
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(THEME_KEY, mode);
  }
}

// Cycle through themes (for quick toggle)
export function cycleTheme() {
  const current = get(themeMode);
  const themes = get(allThemes);
  const currentIndex = themes.findIndex(t => t.id === current);
  const nextIndex = (currentIndex + 1) % themes.length;
  setTheme(themes[nextIndex].id);
}

// Toggle between dark and light (ignores custom themes)
export function toggleDarkMode() {
  const current = get(resolvedTheme);
  setTheme(current === 'dark' ? 'light' : 'dark');
}

// Register a custom theme
export function registerCustomTheme(theme: ThemeInfo) {
  customThemes.update(themes => {
    // Don't add duplicates
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
    // Try to load from /themes/{themeId}.css
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

// Detect available custom themes by checking for CSS files
export async function detectCustomThemes(): Promise<void> {
  if (typeof window === 'undefined') return;

  try {
    // Fetch list of custom themes from server
    const response = await fetch('/api/themes');
    if (response.ok) {
      const themes: ThemeInfo[] = await response.json();
      customThemes.set(themes);

      // Load CSS for each custom theme
      for (const theme of themes) {
        await loadCustomThemeCSS(theme.id);
      }
    }
  } catch {
    // API not available, that's fine - no custom themes
  }
}

// Initialize theme system
export function initTheme() {
  if (typeof window === 'undefined') return;

  // Listen for system preference changes
  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
  const handleChange = (e: MediaQueryListEvent | MediaQueryList) => {
    systemTheme.set(e.matches ? 'dark' : 'light');
  };

  // Modern browsers
  if (mediaQuery.addEventListener) {
    mediaQuery.addEventListener('change', handleChange);
  } else {
    // Fallback for older browsers
    mediaQuery.addListener(handleChange as (e: MediaQueryListEvent) => void);
  }

  // Set initial system theme
  handleChange(mediaQuery);

  // Subscribe to resolved theme changes and apply
  resolvedTheme.subscribe(applyTheme);

  // Detect and load custom themes
  detectCustomThemes();
}

// Get theme info by ID
export function getThemeInfo(themeId: string): ThemeInfo | undefined {
  return get(allThemes).find(t => t.id === themeId);
}

// Save a custom theme via API (generates + saves CSS on server)
export async function saveCustomThemeToServer(
  name: string,
  baseTheme: string,
  isDark: boolean,
  variables: Record<string, string>
): Promise<boolean> {
  try {
    const response = await fetch('/api/themes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, baseTheme, isDark, variables })
    });

    if (!response.ok) return false;

    // Refresh custom themes list
    await detectCustomThemes();

    // Force reload the CSS for this theme (remove old link tag to get fresh version)
    const id = name.toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/-+/g, '-').replace(/^-|-$/g, '');
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

    // Remove CSS link if loaded
    const linkEl = document.getElementById(`theme-${themeId}`);
    if (linkEl) linkEl.remove();

    // If current theme was deleted, switch to dark
    if (get(themeMode) === themeId) {
      setTheme('dark');
    }

    // Refresh custom themes list
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

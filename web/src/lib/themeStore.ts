/**
 * Theme Store - Manages application theme (dark/light/system)
 */

import { writable, derived } from 'svelte/store';

export type ThemeMode = 'dark' | 'light' | 'system';
export type ResolvedTheme = 'dark' | 'light';

const THEME_KEY = 'muximux_theme';

// Get stored theme or default to 'dark'
function getStoredTheme(): ThemeMode {
  if (typeof window === 'undefined') return 'dark';
  const stored = localStorage.getItem(THEME_KEY);
  if (stored === 'dark' || stored === 'light' || stored === 'system') {
    return stored;
  }
  return 'dark'; // Default to dark mode
}

// Get system preference
function getSystemTheme(): ResolvedTheme {
  if (typeof window === 'undefined') return 'dark';
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

// Create the writable store
export const themeMode = writable<ThemeMode>(getStoredTheme());

// Track system theme preference
export const systemTheme = writable<ResolvedTheme>(getSystemTheme());

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

// Apply theme to document
function applyTheme(theme: ResolvedTheme) {
  if (typeof document === 'undefined') return;

  const root = document.documentElement;
  if (theme === 'dark') {
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
}

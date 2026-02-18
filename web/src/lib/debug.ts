const STORAGE_KEY = 'muximux_debug';

let enabled = typeof localStorage !== 'undefined' && localStorage.getItem(STORAGE_KEY) === '1';

export function initDebug(): void {
  if (typeof window === 'undefined') return;
  const params = new URLSearchParams(window.location.search);
  if (params.get('debug') === 'true') {
    localStorage.setItem(STORAGE_KEY, '1');
    enabled = true;
  } else if (params.get('debug') === 'false') {
    localStorage.removeItem(STORAGE_KEY);
    enabled = false;
  }
  if (enabled) {
    console.debug('[muximux] debug logging enabled');
  }
}

export function debug(category: string, ...args: unknown[]): void {
  if (!enabled) return;
  console.debug(`[muximux:${category}]`, ...args);
}

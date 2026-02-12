import { writable } from 'svelte/store';

const stored = typeof localStorage === 'undefined'
  ? null : localStorage.getItem('muximux_capture_keybindings');

export const captureKeybindings = writable<boolean>(stored === null ? true : stored === 'true');

captureKeybindings.subscribe(v => {
  if (typeof localStorage !== 'undefined')
    localStorage.setItem('muximux_capture_keybindings', String(v));
});

export function toggleCaptureKeybindings() {
  captureKeybindings.update(v => !v);
}

export function isProtectedKey(event: KeyboardEvent): boolean {
  if (event.key === 'Escape') return true;
  if (event.key === 'k' && (event.ctrlKey || event.metaKey) && !event.shiftKey && !event.altKey) return true;
  return false;
}

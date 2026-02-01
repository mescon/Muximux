/**
 * Fullscreen Store - Manages fullscreen/kiosk mode
 */

import { writable, derived } from 'svelte/store';

// Whether fullscreen (navigation hidden) mode is active
export const isFullscreen = writable(false);

// Toggle fullscreen mode
export function toggleFullscreen() {
  isFullscreen.update(v => !v);
}

// Enter fullscreen mode
export function enterFullscreen() {
  isFullscreen.set(true);
}

// Exit fullscreen mode
export function exitFullscreen() {
  isFullscreen.set(false);
}

// Request browser native fullscreen API
export function requestBrowserFullscreen() {
  const elem = document.documentElement;
  if (elem.requestFullscreen) {
    elem.requestFullscreen();
  } else if ((elem as any).webkitRequestFullscreen) {
    // Safari
    (elem as any).webkitRequestFullscreen();
  } else if ((elem as any).msRequestFullscreen) {
    // IE11
    (elem as any).msRequestFullscreen();
  }
}

// Exit browser native fullscreen
export function exitBrowserFullscreen() {
  if (document.exitFullscreen) {
    document.exitFullscreen();
  } else if ((document as any).webkitExitFullscreen) {
    (document as any).webkitExitFullscreen();
  } else if ((document as any).msExitFullscreen) {
    (document as any).msExitFullscreen();
  }
}

// Check if browser is in fullscreen mode
export function isBrowserFullscreen(): boolean {
  return !!(
    document.fullscreenElement ||
    (document as any).webkitFullscreenElement ||
    (document as any).msFullscreenElement
  );
}

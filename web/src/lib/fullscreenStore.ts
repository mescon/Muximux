/**
 * Fullscreen Store - Manages fullscreen/kiosk mode
 */

import { writable } from 'svelte/store';

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

// Vendor-prefixed fullscreen types for Safari and legacy IE
interface VendorHTMLElement extends HTMLElement {
  webkitRequestFullscreen?: () => Promise<void>;
  msRequestFullscreen?: () => void;
}

interface VendorDocument extends Document {
  webkitExitFullscreen?: () => Promise<void>;
  msExitFullscreen?: () => void;
  webkitFullscreenElement?: Element | null;
  msFullscreenElement?: Element | null;
}

// Request browser native fullscreen API
export function requestBrowserFullscreen() {
  const elem = document.documentElement as VendorHTMLElement;
  if (elem.requestFullscreen) {
    elem.requestFullscreen();
  } else if (elem.webkitRequestFullscreen) {
    // Safari
    elem.webkitRequestFullscreen();
  } else if (elem.msRequestFullscreen) {
    // IE11
    elem.msRequestFullscreen();
  }
}

// Exit browser native fullscreen
export function exitBrowserFullscreen() {
  const doc = document as VendorDocument;
  if (doc.exitFullscreen) {
    doc.exitFullscreen();
  } else if (doc.webkitExitFullscreen) {
    doc.webkitExitFullscreen();
  } else if (doc.msExitFullscreen) {
    doc.msExitFullscreen();
  }
}

// Check if browser is in fullscreen mode
export function isBrowserFullscreen(): boolean {
  const doc = document as VendorDocument;
  return !!(
    doc.fullscreenElement ||
    doc.webkitFullscreenElement ||
    doc.msFullscreenElement
  );
}

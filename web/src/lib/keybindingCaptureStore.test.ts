import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import {
  captureKeybindings,
  toggleCaptureKeybindings,
  isProtectedKey,
} from './keybindingCaptureStore';

describe('keybindingCaptureStore', () => {
  beforeEach(() => {
    // Reset store to default state (true)
    captureKeybindings.set(true);
  });

  describe('store default state', () => {
    it('defaults to true', () => {
      expect(get(captureKeybindings)).toBe(true);
    });
  });

  describe('toggleCaptureKeybindings', () => {
    it('toggles from true to false', () => {
      expect(get(captureKeybindings)).toBe(true);
      toggleCaptureKeybindings();
      expect(get(captureKeybindings)).toBe(false);
    });

    it('toggles from false to true', () => {
      captureKeybindings.set(false);
      expect(get(captureKeybindings)).toBe(false);
      toggleCaptureKeybindings();
      expect(get(captureKeybindings)).toBe(true);
    });

    it('persists state to localStorage on toggle', () => {
      toggleCaptureKeybindings();
      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_capture_keybindings', 'false');
    });

    it('toggles multiple times correctly', () => {
      expect(get(captureKeybindings)).toBe(true);
      toggleCaptureKeybindings();
      expect(get(captureKeybindings)).toBe(false);
      toggleCaptureKeybindings();
      expect(get(captureKeybindings)).toBe(true);
    });
  });

  describe('subscription persists to localStorage', () => {
    it('writes to localStorage when store value changes', () => {
      captureKeybindings.set(false);
      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_capture_keybindings', 'false');
      captureKeybindings.set(true);
      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_capture_keybindings', 'true');
    });
  });
});

describe('isProtectedKey', () => {
  // Helper to create a mock KeyboardEvent
  function makeKeyEvent(overrides: Partial<KeyboardEvent> = {}): KeyboardEvent {
    return {
      key: 'a',
      ctrlKey: false,
      metaKey: false,
      shiftKey: false,
      altKey: false,
      ...overrides,
    } as KeyboardEvent;
  }

  it('returns true for Escape key', () => {
    expect(isProtectedKey(makeKeyEvent({ key: 'Escape' }))).toBe(true);
  });

  it('returns true for Ctrl+K', () => {
    expect(isProtectedKey(makeKeyEvent({ key: 'k', ctrlKey: true }))).toBe(true);
  });

  it('returns true for Meta+K (macOS Cmd+K)', () => {
    expect(isProtectedKey(makeKeyEvent({ key: 'k', metaKey: true }))).toBe(true);
  });

  it('returns false for Ctrl+Shift+K (shift modifier present)', () => {
    expect(isProtectedKey(makeKeyEvent({ key: 'k', ctrlKey: true, shiftKey: true }))).toBe(false);
  });

  it('returns false for Ctrl+Alt+K (alt modifier present)', () => {
    expect(isProtectedKey(makeKeyEvent({ key: 'k', ctrlKey: true, altKey: true }))).toBe(false);
  });

  it('returns false for regular letter keys', () => {
    expect(isProtectedKey(makeKeyEvent({ key: 'a' }))).toBe(false);
    expect(isProtectedKey(makeKeyEvent({ key: 'z' }))).toBe(false);
  });

  it('returns false for Tab key', () => {
    expect(isProtectedKey(makeKeyEvent({ key: 'Tab' }))).toBe(false);
  });

  it('returns false for Enter key', () => {
    expect(isProtectedKey(makeKeyEvent({ key: 'Enter' }))).toBe(false);
  });

  it('returns false for K without modifiers', () => {
    expect(isProtectedKey(makeKeyEvent({ key: 'k' }))).toBe(false);
  });

  it('returns false for number keys', () => {
    expect(isProtectedKey(makeKeyEvent({ key: '1' }))).toBe(false);
    expect(isProtectedKey(makeKeyEvent({ key: '9' }))).toBe(false);
  });

  it('returns false for space', () => {
    expect(isProtectedKey(makeKeyEvent({ key: ' ' }))).toBe(false);
  });
});

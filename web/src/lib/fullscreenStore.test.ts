import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import {
  isFullscreen,
  toggleFullscreen,
  enterFullscreen,
  exitFullscreen,
  requestBrowserFullscreen,
  exitBrowserFullscreen,
  isBrowserFullscreen,
} from './fullscreenStore';

describe('fullscreenStore', () => {
  beforeEach(() => {
    isFullscreen.set(false);
  });

  describe('isFullscreen store', () => {
    it('defaults to false', () => {
      expect(get(isFullscreen)).toBe(false);
    });
  });

  describe('toggleFullscreen', () => {
    it('toggles from false to true', () => {
      toggleFullscreen();
      expect(get(isFullscreen)).toBe(true);
    });

    it('toggles from true to false', () => {
      isFullscreen.set(true);
      toggleFullscreen();
      expect(get(isFullscreen)).toBe(false);
    });

    it('toggles multiple times', () => {
      toggleFullscreen(); // true
      toggleFullscreen(); // false
      toggleFullscreen(); // true
      expect(get(isFullscreen)).toBe(true);
    });
  });

  describe('enterFullscreen', () => {
    it('sets isFullscreen to true', () => {
      enterFullscreen();
      expect(get(isFullscreen)).toBe(true);
    });

    it('stays true if already in fullscreen', () => {
      isFullscreen.set(true);
      enterFullscreen();
      expect(get(isFullscreen)).toBe(true);
    });
  });

  describe('exitFullscreen', () => {
    it('sets isFullscreen to false', () => {
      isFullscreen.set(true);
      exitFullscreen();
      expect(get(isFullscreen)).toBe(false);
    });

    it('stays false if not in fullscreen', () => {
      exitFullscreen();
      expect(get(isFullscreen)).toBe(false);
    });
  });

  describe('requestBrowserFullscreen', () => {
    it('calls requestFullscreen on document.documentElement', () => {
      const mockRequestFullscreen = vi.fn();
      Object.defineProperty(document.documentElement, 'requestFullscreen', {
        value: mockRequestFullscreen,
        configurable: true,
      });

      requestBrowserFullscreen();

      expect(mockRequestFullscreen).toHaveBeenCalled();
    });

    it('falls back to webkitRequestFullscreen', () => {
      const mockWebkitFullscreen = vi.fn();
      const elem = document.documentElement as unknown as Record<string, unknown>;

      // Remove standard method
      const origRequestFullscreen = elem.requestFullscreen;
      Object.defineProperty(document.documentElement, 'requestFullscreen', {
        value: undefined,
        configurable: true,
      });
      Object.defineProperty(document.documentElement, 'webkitRequestFullscreen', {
        value: mockWebkitFullscreen,
        configurable: true,
      });

      requestBrowserFullscreen();

      expect(mockWebkitFullscreen).toHaveBeenCalled();

      // Restore
      Object.defineProperty(document.documentElement, 'requestFullscreen', {
        value: origRequestFullscreen,
        configurable: true,
      });
      delete elem.webkitRequestFullscreen;
    });

    it('falls back to msRequestFullscreen', () => {
      const mockMsFullscreen = vi.fn();
      const elem = document.documentElement as unknown as Record<string, unknown>;

      const origRequestFullscreen = elem.requestFullscreen;
      Object.defineProperty(document.documentElement, 'requestFullscreen', {
        value: undefined,
        configurable: true,
      });
      Object.defineProperty(document.documentElement, 'webkitRequestFullscreen', {
        value: undefined,
        configurable: true,
      });
      Object.defineProperty(document.documentElement, 'msRequestFullscreen', {
        value: mockMsFullscreen,
        configurable: true,
      });

      requestBrowserFullscreen();

      expect(mockMsFullscreen).toHaveBeenCalled();

      // Restore
      Object.defineProperty(document.documentElement, 'requestFullscreen', {
        value: origRequestFullscreen,
        configurable: true,
      });
      delete elem.webkitRequestFullscreen;
      delete elem.msRequestFullscreen;
    });
  });

  describe('exitBrowserFullscreen', () => {
    it('calls exitFullscreen on document', () => {
      const mockExitFullscreen = vi.fn();
      Object.defineProperty(document, 'exitFullscreen', {
        value: mockExitFullscreen,
        configurable: true,
      });

      exitBrowserFullscreen();

      expect(mockExitFullscreen).toHaveBeenCalled();
    });

    it('falls back to webkitExitFullscreen', () => {
      const mockWebkitExit = vi.fn();
      const doc = document as unknown as Record<string, unknown>;

      const origExitFullscreen = doc.exitFullscreen;
      Object.defineProperty(document, 'exitFullscreen', {
        value: undefined,
        configurable: true,
      });
      Object.defineProperty(document, 'webkitExitFullscreen', {
        value: mockWebkitExit,
        configurable: true,
      });

      exitBrowserFullscreen();

      expect(mockWebkitExit).toHaveBeenCalled();

      // Restore
      Object.defineProperty(document, 'exitFullscreen', {
        value: origExitFullscreen,
        configurable: true,
      });
      delete doc.webkitExitFullscreen;
    });

    it('falls back to msExitFullscreen', () => {
      const mockMsExit = vi.fn();
      const doc = document as unknown as Record<string, unknown>;

      const origExitFullscreen = doc.exitFullscreen;
      Object.defineProperty(document, 'exitFullscreen', {
        value: undefined,
        configurable: true,
      });
      Object.defineProperty(document, 'webkitExitFullscreen', {
        value: undefined,
        configurable: true,
      });
      Object.defineProperty(document, 'msExitFullscreen', {
        value: mockMsExit,
        configurable: true,
      });

      exitBrowserFullscreen();

      expect(mockMsExit).toHaveBeenCalled();

      // Restore
      Object.defineProperty(document, 'exitFullscreen', {
        value: origExitFullscreen,
        configurable: true,
      });
      delete doc.webkitExitFullscreen;
      delete doc.msExitFullscreen;
    });
  });

  describe('isBrowserFullscreen', () => {
    it('returns false when no fullscreen element', () => {
      expect(isBrowserFullscreen()).toBe(false);
    });

    it('returns true when fullscreenElement is set', () => {
      Object.defineProperty(document, 'fullscreenElement', {
        value: document.createElement('div'),
        configurable: true,
      });

      expect(isBrowserFullscreen()).toBe(true);

      // Restore
      Object.defineProperty(document, 'fullscreenElement', {
        value: null,
        configurable: true,
      });
    });

    it('returns true when webkitFullscreenElement is set', () => {
      Object.defineProperty(document, 'fullscreenElement', {
        value: null,
        configurable: true,
      });
      Object.defineProperty(document, 'webkitFullscreenElement', {
        value: document.createElement('div'),
        configurable: true,
      });

      expect(isBrowserFullscreen()).toBe(true);

      // Restore
      Object.defineProperty(document, 'webkitFullscreenElement', {
        value: null,
        configurable: true,
      });
    });

    it('returns true when msFullscreenElement is set', () => {
      Object.defineProperty(document, 'fullscreenElement', {
        value: null,
        configurable: true,
      });
      Object.defineProperty(document, 'msFullscreenElement', {
        value: document.createElement('div'),
        configurable: true,
      });

      expect(isBrowserFullscreen()).toBe(true);

      // Restore
      Object.defineProperty(document, 'msFullscreenElement', {
        value: null,
        configurable: true,
      });
    });
  });
});

import { describe, it, expect, vi, beforeEach } from 'vitest';

describe('debug module', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    // Reset location.search to default
    Object.defineProperty(globalThis, 'location', {
      value: { search: '' },
      writable: true,
      configurable: true,
    });
  });

  // We need to re-import the module fresh for each test group since it has
  // module-level state (`enabled`). Use dynamic imports with cache busting
  // via vi.resetModules().
  function freshImport() {
    vi.resetModules();
    return import('./debug');
  }

  describe('initDebug', () => {
    it('should enable debug when URL has ?debug=true and persist to localStorage', async () => {
      Object.defineProperty(globalThis, 'location', {
        value: { search: '?debug=true' },
        writable: true,
        configurable: true,
      });

      const { initDebug } = await freshImport();
      const debugSpy = vi.spyOn(console, 'debug').mockImplementation(() => {});

      initDebug();

      expect(localStorage.setItem).toHaveBeenCalledWith('muximux_debug', '1');
      expect(debugSpy).toHaveBeenCalledWith('[muximux] debug logging enabled');

      debugSpy.mockRestore();
    });

    it('should disable debug when URL has ?debug=false and remove from localStorage', async () => {
      // Pre-set localStorage so debug is initially enabled
      localStorage.setItem('muximux_debug', '1');

      Object.defineProperty(globalThis, 'location', {
        value: { search: '?debug=false' },
        writable: true,
        configurable: true,
      });

      const { initDebug } = await freshImport();
      const debugSpy = vi.spyOn(console, 'debug').mockImplementation(() => {});

      initDebug();

      expect(localStorage.removeItem).toHaveBeenCalledWith('muximux_debug');
      // Should NOT log since debug is now disabled
      expect(debugSpy).not.toHaveBeenCalledWith('[muximux] debug logging enabled');

      debugSpy.mockRestore();
    });

    it('should read enabled state from localStorage on module init', async () => {
      localStorage.setItem('muximux_debug', '1');

      // No debug param in URL
      Object.defineProperty(globalThis, 'location', {
        value: { search: '' },
        writable: true,
        configurable: true,
      });

      const { initDebug } = await freshImport();
      const debugSpy = vi.spyOn(console, 'debug').mockImplementation(() => {});

      initDebug();

      // Should log since localStorage had it enabled
      expect(debugSpy).toHaveBeenCalledWith('[muximux] debug logging enabled');

      debugSpy.mockRestore();
    });
  });

  describe('debug()', () => {
    it('should log when enabled', async () => {
      localStorage.setItem('muximux_debug', '1');

      Object.defineProperty(globalThis, 'location', {
        value: { search: '?debug=true' },
        writable: true,
        configurable: true,
      });

      const { initDebug, debug } = await freshImport();
      const debugSpy = vi.spyOn(console, 'debug').mockImplementation(() => {});

      initDebug();
      debugSpy.mockClear();

      debug('test', 'hello', 42);

      expect(debugSpy).toHaveBeenCalledWith('[muximux:test]', 'hello', 42);

      debugSpy.mockRestore();
    });

    it('should do nothing when disabled', async () => {
      // No localStorage entry, no URL param => disabled
      Object.defineProperty(globalThis, 'location', {
        value: { search: '' },
        writable: true,
        configurable: true,
      });

      const { initDebug, debug } = await freshImport();
      const debugSpy = vi.spyOn(console, 'debug').mockImplementation(() => {});

      initDebug();
      debugSpy.mockClear();

      debug('test', 'should not appear');

      expect(debugSpy).not.toHaveBeenCalled();

      debugSpy.mockRestore();
    });

    it('should format category correctly as [muximux:category]', async () => {
      Object.defineProperty(globalThis, 'location', {
        value: { search: '?debug=true' },
        writable: true,
        configurable: true,
      });

      const { initDebug, debug } = await freshImport();
      const debugSpy = vi.spyOn(console, 'debug').mockImplementation(() => {});

      initDebug();
      debugSpy.mockClear();

      debug('ws', 'connected');
      expect(debugSpy).toHaveBeenCalledWith('[muximux:ws]', 'connected');

      debug('theme', 'changed', { dark: true });
      expect(debugSpy).toHaveBeenCalledWith('[muximux:theme]', 'changed', { dark: true });

      debugSpy.mockRestore();
    });
  });
});

import { describe, it, expect, vi, beforeEach } from 'vitest';

describe('favicon module', () => {
  // Track elements created by querySelector mocks
  let linkElements: Record<string, { href: string }>;
  let metaElements: Record<string, { content: string }>;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.resetModules();

    linkElements = {
      'link[rel="icon"][type="image/x-icon"]': { href: '' },
      'link[rel="icon"][sizes="32x32"]': { href: '' },
      'link[rel="icon"][sizes="16x16"]': { href: '' },
      'link[rel="mask-icon"]': { href: '' },
      'link[rel="apple-touch-icon"]': { href: '' },
      'link[rel="manifest"]': { href: '' },
    };

    metaElements = {
      'meta[name="theme-color"]': { content: '' },
      'meta[name="msapplication-TileColor"]': { content: '' },
    };

    // Mock document.querySelector to return our tracked elements
    vi.spyOn(document, 'querySelector').mockImplementation((selector: string) => {
      if (selector in linkElements) {
        return linkElements[selector] as unknown as Element;
      }
      if (selector in metaElements) {
        return metaElements[selector] as unknown as Element;
      }
      return null;
    });

    // Mock URL.createObjectURL for manifest blob
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:mock-url');

    // Mock Image as a proper class that vitest can use as constructor
    class MockImage {
      onload: (() => void) | null = null;
      private _src = '';
      get src() {
        return this._src;
      }
      set src(value: string) {
        this._src = value;
        // Trigger onload asynchronously
        setTimeout(() => this.onload?.(), 0);
      }
    }
    vi.stubGlobal('Image', MockImage);

    // Mock canvas for PNG rendering - save reference to real createElement
    const realCreateElement = document.createElement.bind(document);
    const mockCtx = {
      drawImage: vi.fn(),
    };
    const mockCanvas = {
      width: 0,
      height: 0,
      getContext: vi.fn().mockReturnValue(mockCtx),
      toDataURL: vi.fn().mockReturnValue('data:image/png;base64,mock'),
    };
    vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      if (tag === 'canvas') {
        return mockCanvas as unknown as HTMLCanvasElement;
      }
      return realCreateElement(tag);
    });
  });

  async function freshImport() {
    return import('./favicon');
  }

  describe('updateFavicons', () => {
    it('should set SVG favicon link hrefs', async () => {
      const { updateFavicons } = await freshImport();

      await updateFavicons('#ff0000');

      // All SVG icon links should have been set to a data URI
      expect(linkElements['link[rel="icon"][type="image/x-icon"]'].href).toMatch(
        /^data:image\/svg\+xml,/,
      );
      expect(linkElements['link[rel="icon"][sizes="32x32"]'].href).toMatch(
        /^data:image\/svg\+xml,/,
      );
      expect(linkElements['link[rel="icon"][sizes="16x16"]'].href).toMatch(
        /^data:image\/svg\+xml,/,
      );
      expect(linkElements['link[rel="mask-icon"]'].href).toMatch(
        /^data:image\/svg\+xml,/,
      );
    });

    it('should update theme-color meta tag', async () => {
      const { updateFavicons } = await freshImport();

      await updateFavicons('#3b82f6');

      expect(metaElements['meta[name="theme-color"]'].content).toBe('#3b82f6');
      expect(metaElements['meta[name="msapplication-TileColor"]'].content).toBe('#3b82f6');
    });

    it('should update apple-touch-icon with PNG data URL', async () => {
      const { updateFavicons } = await freshImport();

      await updateFavicons('#00ff00');

      expect(linkElements['link[rel="apple-touch-icon"]'].href).toBe(
        'data:image/png;base64,mock',
      );
    });

    it('should update manifest link with blob URL', async () => {
      const { updateFavicons } = await freshImport();

      await updateFavicons('#00ff00');

      expect(linkElements['link[rel="manifest"]'].href).toBe('blob:mock-url');
      expect(URL.createObjectURL).toHaveBeenCalled();
    });

    it('should return early if document is undefined', async () => {
      const origDoc = globalThis.document;
      Object.defineProperty(globalThis, 'document', {
        value: undefined,
        writable: true,
        configurable: true,
      });

      const { updateFavicons } = await freshImport();

      // Should not throw
      await updateFavicons('#ff0000');

      Object.defineProperty(globalThis, 'document', {
        value: origDoc,
        writable: true,
        configurable: true,
      });
    });
  });

  describe('syncFaviconsWithTheme', () => {
    it('should read --accent-primary CSS variable from computed styles', async () => {
      const getPropertyValueFn = vi.fn().mockReturnValue('#e11d48');
      const mockGetComputedStyle = vi.fn().mockReturnValue({
        getPropertyValue: getPropertyValueFn,
      });
      vi.stubGlobal('getComputedStyle', mockGetComputedStyle);

      const { syncFaviconsWithTheme } = await freshImport();

      syncFaviconsWithTheme();

      expect(mockGetComputedStyle).toHaveBeenCalledWith(document.documentElement);
      expect(getPropertyValueFn).toHaveBeenCalledWith('--accent-primary');

      vi.unstubAllGlobals();
    });

    it('should skip update if color has not changed', async () => {
      const getPropertyValue = vi.fn().mockReturnValue('#e11d48');
      const mockGetComputedStyle = vi.fn().mockReturnValue({ getPropertyValue });
      vi.stubGlobal('getComputedStyle', mockGetComputedStyle);

      const { syncFaviconsWithTheme } = await freshImport();

      // First call sets the color and triggers updateFavicons
      syncFaviconsWithTheme();
      const querySelectorCallCount = (document.querySelector as ReturnType<typeof vi.fn>).mock.calls.length;

      // Second call with same color should skip (no new querySelector calls)
      syncFaviconsWithTheme();
      const querySelectorCallCountAfter = (document.querySelector as ReturnType<typeof vi.fn>).mock.calls.length;
      expect(querySelectorCallCountAfter).toBe(querySelectorCallCount);

      vi.unstubAllGlobals();
    });

    it('should skip if color is empty', async () => {
      const mockGetComputedStyle = vi.fn().mockReturnValue({
        getPropertyValue: vi.fn().mockReturnValue(''),
      });
      vi.stubGlobal('getComputedStyle', mockGetComputedStyle);

      const { syncFaviconsWithTheme } = await freshImport();

      syncFaviconsWithTheme();

      // Should not have tried to set any link hrefs via querySelector
      expect(document.querySelector).not.toHaveBeenCalled();

      vi.unstubAllGlobals();
    });

    it('should skip if document is undefined', async () => {
      const origDoc = globalThis.document;
      Object.defineProperty(globalThis, 'document', {
        value: undefined,
        writable: true,
        configurable: true,
      });

      const { syncFaviconsWithTheme } = await freshImport();

      // Should not throw
      expect(() => syncFaviconsWithTheme()).not.toThrow();

      Object.defineProperty(globalThis, 'document', {
        value: origDoc,
        writable: true,
        configurable: true,
      });
    });
  });
});

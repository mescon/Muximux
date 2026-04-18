import { describe, it, expect, beforeEach, vi } from 'vitest';

const { mockSetLocale, mockGetLocale, mockLocales } = vi.hoisted(() => ({
  mockSetLocale: vi.fn(),
  mockGetLocale: vi.fn().mockReturnValue('en'),
  mockLocales: ['en', 'sv', 'ar', 'de', 'fr'],
}));

vi.mock('$lib/paraglide/runtime.js', () => ({
  setLocale: mockSetLocale,
  getLocale: mockGetLocale,
  locales: mockLocales,
  localStorageKey: 'PARAGLIDE_LOCALE',
}));

import { applyLocaleToDocument, syncLocaleFromConfig, getAvailableLocales, localeNames, localeFlags } from './localeStore';

describe('localeStore', () => {
  beforeEach(() => {
    document.documentElement.lang = '';
    document.documentElement.dir = '';
    vi.clearAllMocks();
    mockGetLocale.mockReturnValue('en');
  });

  describe('applyLocaleToDocument', () => {
    it('sets lang attribute on html element', () => {
      applyLocaleToDocument('sv');
      expect(document.documentElement.lang).toBe('sv');
    });

    it('sets dir="ltr" for LTR locales', () => {
      applyLocaleToDocument('en');
      expect(document.documentElement.dir).toBe('ltr');
    });

    it('sets dir="rtl" for Arabic', () => {
      applyLocaleToDocument('ar');
      expect(document.documentElement.dir).toBe('rtl');
    });

    it('sets dir="ltr" for non-RTL locales', () => {
      for (const locale of ['en', 'sv', 'uk', 'zh', 'ja', 'de', 'fr']) {
        applyLocaleToDocument(locale);
        expect(document.documentElement.dir).toBe('ltr');
      }
    });
  });

  describe('getAvailableLocales', () => {
    it('returns an array of locale objects with tag, name, and flag', () => {
      const locales = getAvailableLocales();
      expect(locales.length).toBeGreaterThan(0);
      for (const locale of locales) {
        expect(locale).toHaveProperty('tag');
        expect(locale).toHaveProperty('name');
        expect(locale).toHaveProperty('flag');
        expect(typeof locale.tag).toBe('string');
        expect(typeof locale.name).toBe('string');
        expect(typeof locale.flag).toBe('string');
      }
    });

    it('includes English', () => {
      const locales = getAvailableLocales();
      expect(locales.find(l => l.tag === 'en')).toBeDefined();
    });

    it('is sorted alphabetically by display name', () => {
      const locales = getAvailableLocales();
      const names = locales.map(l => l.name);
      const sorted = [...names].sort((a, b) => a.localeCompare(b));
      expect(names).toEqual(sorted);
    });
  });

  describe('localeNames', () => {
    it('has native name for every supported locale', () => {
      const locales = getAvailableLocales();
      for (const locale of locales) {
        expect(localeNames[locale.tag]).toBeDefined();
        expect(localeNames[locale.tag].length).toBeGreaterThan(0);
      }
    });
  });

  describe('localeFlags', () => {
    it('has a flag emoji for every supported locale', () => {
      const locales = getAvailableLocales();
      for (const locale of locales) {
        expect(localeFlags[locale.tag]).toBeDefined();
        expect(localeFlags[locale.tag].length).toBeGreaterThan(0);
      }
    });
  });

  describe('syncLocaleFromConfig', () => {
    it('returns early for empty locale string', () => {
      syncLocaleFromConfig('');
      expect(mockSetLocale).not.toHaveBeenCalled();
      expect(document.documentElement.lang).toBe('');
    });

    it('returns early for invalid/unsupported locale', () => {
      syncLocaleFromConfig('xx');
      expect(mockSetLocale).not.toHaveBeenCalled();
      expect(document.documentElement.lang).toBe('');
    });

    it('calls applyLocaleToDocument when config locale matches current locale', () => {
      mockGetLocale.mockReturnValue('sv');
      syncLocaleFromConfig('sv');
      expect(document.documentElement.lang).toBe('sv');
      expect(document.documentElement.dir).toBe('ltr');
      expect(mockSetLocale).not.toHaveBeenCalled();
    });

    it('sets RTL direction when config locale matches current RTL locale', () => {
      mockGetLocale.mockReturnValue('ar');
      syncLocaleFromConfig('ar');
      expect(document.documentElement.lang).toBe('ar');
      expect(document.documentElement.dir).toBe('rtl');
      expect(mockSetLocale).not.toHaveBeenCalled();
    });

    it('calls setLocale when config locale differs from current locale', () => {
      mockGetLocale.mockReturnValue('en');
      syncLocaleFromConfig('de');
      expect(mockSetLocale).toHaveBeenCalledWith('de');
    });

    it('does not call applyLocaleToDocument when locale differs', () => {
      mockGetLocale.mockReturnValue('en');
      syncLocaleFromConfig('fr');
      // setLocale should be called, but document should not be updated directly
      expect(mockSetLocale).toHaveBeenCalledWith('fr');
      expect(document.documentElement.lang).toBe('');
    });
  });
});

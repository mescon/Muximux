import { describe, it, expect, beforeEach } from 'vitest';
import { applyLocaleToDocument, getAvailableLocales, localeNames } from './localeStore';

describe('localeStore', () => {
  beforeEach(() => {
    document.documentElement.lang = '';
    document.documentElement.dir = '';
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
    it('returns an array of locale objects with tag and name', () => {
      const locales = getAvailableLocales();
      expect(locales.length).toBeGreaterThan(0);
      for (const locale of locales) {
        expect(locale).toHaveProperty('tag');
        expect(locale).toHaveProperty('name');
        expect(typeof locale.tag).toBe('string');
        expect(typeof locale.name).toBe('string');
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
});

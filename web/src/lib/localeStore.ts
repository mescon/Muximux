import { setLocale, getLocale, locales, localStorageKey } from '$lib/paraglide/runtime.js';

const RTL_LOCALES = new Set(['ar']);

/** Set lang and dir attributes on the document root element. */
export function applyLocaleToDocument(locale: string): void {
  document.documentElement.lang = locale;
  document.documentElement.dir = RTL_LOCALES.has(locale) ? 'rtl' : 'ltr';
}

/**
 * Sync the active locale from the server config.
 * If the config locale differs from the current Paraglide locale,
 * persist to localStorage and trigger a page reload.
 */
export function syncLocaleFromConfig(configLocale: string): void {
  if (!configLocale || !locales.includes(configLocale as typeof locales[number])) return;
  if (configLocale === getLocale()) {
    applyLocaleToDocument(configLocale);
    return;
  }
  // setLocale writes to localStorage and triggers window.location.reload()
  // Do NOT write localStorage before setLocale — it checks getLocale() internally
  // and would see the new value, think nothing changed, and skip the reload.
  setLocale(configLocale as typeof locales[number]);
}

/** Flag emoji for each supported locale. */
export const localeFlags: Record<string, string> = {
  en: '\u{1F1EC}\u{1F1E7}', sv: '\u{1F1F8}\u{1F1EA}', uk: '\u{1F1FA}\u{1F1E6}',
  zh: '\u{1F1E8}\u{1F1F3}', es: '\u{1F1EA}\u{1F1F8}', hi: '\u{1F1EE}\u{1F1F3}',
  pt: '\u{1F1F5}\u{1F1F9}', bn: '\u{1F1E7}\u{1F1E9}', ru: '\u{1F1F7}\u{1F1FA}',
  ja: '\u{1F1EF}\u{1F1F5}', vi: '\u{1F1FB}\u{1F1F3}', yue: '\u{1F1ED}\u{1F1F0}',
  tr: '\u{1F1F9}\u{1F1F7}', ar: '\u{1F1F8}\u{1F1E6}', wuu: '\u{1F1E8}\u{1F1F3}',
  mr: '\u{1F1EE}\u{1F1F3}', nb: '\u{1F1F3}\u{1F1F4}', fi: '\u{1F1EB}\u{1F1EE}',
  da: '\u{1F1E9}\u{1F1F0}', et: '\u{1F1EA}\u{1F1EA}', lv: '\u{1F1F1}\u{1F1FB}',
  lt: '\u{1F1F1}\u{1F1F9}', pl: '\u{1F1F5}\u{1F1F1}', de: '\u{1F1E9}\u{1F1EA}',
  nl: '\u{1F1F3}\u{1F1F1}', fr: '\u{1F1EB}\u{1F1F7}', it: '\u{1F1EE}\u{1F1F9}',
  hu: '\u{1F1ED}\u{1F1FA}', cs: '\u{1F1E8}\u{1F1FF}', ro: '\u{1F1F7}\u{1F1F4}',
  el: '\u{1F1EC}\u{1F1F7}', bg: '\u{1F1E7}\u{1F1EC}', hr: '\u{1F1ED}\u{1F1F7}',
  sr: '\u{1F1F7}\u{1F1F8}', sk: '\u{1F1F8}\u{1F1F0}', sl: '\u{1F1F8}\u{1F1EE}',
};

/** Native display names for each supported locale. */
export const localeNames: Record<string, string> = {
  en: 'English',
  sv: 'Svenska',
  uk: '\u0423\u043a\u0440\u0430\u0457\u043d\u0441\u044c\u043a\u0430',
  zh: '\u4e2d\u6587',
  es: 'Espa\u00f1ol',
  hi: '\u0939\u093f\u0928\u094d\u0926\u0940',
  pt: 'Portugu\u00eas',
  bn: '\u09ac\u09be\u0982\u09b2\u09be',
  ru: '\u0420\u0443\u0441\u0441\u043a\u0438\u0439',
  ja: '\u65e5\u672c\u8a9e',
  vi: 'Ti\u1ebfng Vi\u1ec7t',
  yue: '\u7cb5\u8a9e',
  tr: 'T\u00fcrk\u00e7e',
  ar: '\u0627\u0644\u0639\u0631\u0628\u064a\u0629',
  wuu: '\u5434\u8bed',
  mr: '\u092e\u0930\u093e\u0920\u0940',
  nb: 'Norsk Bokm\u00e5l',
  fi: 'Suomi',
  da: 'Dansk',
  et: 'Eesti',
  lv: 'Latvie\u0161u',
  lt: 'Lietuvi\u0173',
  pl: 'Polski',
  de: 'Deutsch',
  nl: 'Nederlands',
  fr: 'Fran\u00e7ais',
  it: 'Italiano',
  hu: 'Magyar',
  cs: '\u010ce\u0161tina',
  ro: 'Rom\u00e2n\u0103',
  el: '\u0395\u03bb\u03bb\u03b7\u03bd\u03b9\u03ba\u03ac',
  bg: '\u0411\u044a\u043b\u0433\u0430\u0440\u0441\u043a\u0438',
  hr: 'Hrvatski',
  sr: '\u0421\u0440\u043f\u0441\u043a\u0438',
  sk: 'Sloven\u010dina',
  sl: 'Sloven\u0161\u010dina',
};

/** Returns the list of available locales with their display names and flags, sorted alphabetically by name. */
export function getAvailableLocales(): Array<{ tag: string; name: string; flag: string }> {
  return locales
    .map(tag => ({ tag, name: localeNames[tag] || tag, flag: localeFlags[tag] || '' }))
    .sort((a, b) => a.name.localeCompare(b.name));
}

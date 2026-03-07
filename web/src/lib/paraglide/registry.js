/* eslint-disable */

/**
 * @param {import("./runtime.js").Locale} locale
 * @param {unknown} input
 * @param {Intl.PluralRulesOptions} [options]
 * @returns {string}
 */
export function plural(locale, input, options) { 
	return new Intl.PluralRules(locale, options).select(Number(input))
};

/**
 * @param {import("./runtime.js").Locale} locale
 * @param {unknown} input
 * @param {Intl.NumberFormatOptions} [options]
 * @returns {string}
 */
export function number(locale, input, options) {
	return new Intl.NumberFormat(locale, options).format(Number(input))
};

/**
 * @param {import("./runtime.js").Locale} locale
 * @param {unknown} input
 * @param {Intl.DateTimeFormatOptions} [options]
 * @returns {string}
 */
export function datetime(locale, input, options) {
	return new Intl.DateTimeFormat(locale, options).format(new Date(/** @type {string} */ (input)))
};
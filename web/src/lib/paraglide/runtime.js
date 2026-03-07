/* eslint-disable */

/** @type {any} */
const URLPattern = {}

/**
 * The project's base locale.
 *
 * @example
 *   if (locale === baseLocale) {
 *     // do something
 *   }
 */
export const baseLocale = "en";
/**
 * The project's locales that have been specified in the settings.
 *
 * @example
 *   if (locales.includes(userSelectedLocale) === false) {
 *     throw new Error('Locale is not available');
 *   }
 */
export const locales = /** @type {const} */ (["en", "sv", "uk", "zh", "es", "hi", "pt", "bn", "ru", "ja", "vi", "yue", "tr", "ar", "wuu", "mr", "nb", "fi", "da", "et", "lv", "lt", "pl", "de", "nl", "fr", "it", "hu", "cs", "ro", "el", "bg", "hr", "sr", "sk", "sl"]);
/** @type {string} */
export const cookieName = "PARAGLIDE_LOCALE";
/** @type {number} */
export const cookieMaxAge = 34560000;
/** @type {string} */
export const cookieDomain = "";
/** @type {string} */
export const localStorageKey = "PARAGLIDE_LOCALE";
/**
 * @type {Array<"cookie" | "baseLocale" | "globalVariable" | "url" | "preferredLanguage" | "localStorage" | `custom-${string}`>}
 */
export const strategy = [
  "localStorage",
  "baseLocale"
];
/**
 * Route-level strategy overrides.
 *
 * `match` uses URLPattern syntax.
 *
 * @type {Array<{
 *   match: string;
 *   strategy?: Array<"cookie" | "baseLocale" | "globalVariable" | "url" | "preferredLanguage" | "localStorage" | `custom-${string}`>;
 *   exclude?: boolean;
 * }>}
 */
export const routeStrategies = [];
/**
 * The used URL patterns.
 *
 * @type {Array<{ pattern: string, localized: Array<[Locale, string]> }> }
 */
export const urlPatterns = [
  {
    "pattern": ":protocol://:domain(.*)::port?/:path(.*)?",
    "localized": [
      [
        "sv",
        ":protocol://:domain(.*)::port?/sv/:path(.*)?"
      ],
      [
        "uk",
        ":protocol://:domain(.*)::port?/uk/:path(.*)?"
      ],
      [
        "zh",
        ":protocol://:domain(.*)::port?/zh/:path(.*)?"
      ],
      [
        "es",
        ":protocol://:domain(.*)::port?/es/:path(.*)?"
      ],
      [
        "hi",
        ":protocol://:domain(.*)::port?/hi/:path(.*)?"
      ],
      [
        "pt",
        ":protocol://:domain(.*)::port?/pt/:path(.*)?"
      ],
      [
        "bn",
        ":protocol://:domain(.*)::port?/bn/:path(.*)?"
      ],
      [
        "ru",
        ":protocol://:domain(.*)::port?/ru/:path(.*)?"
      ],
      [
        "ja",
        ":protocol://:domain(.*)::port?/ja/:path(.*)?"
      ],
      [
        "vi",
        ":protocol://:domain(.*)::port?/vi/:path(.*)?"
      ],
      [
        "yue",
        ":protocol://:domain(.*)::port?/yue/:path(.*)?"
      ],
      [
        "tr",
        ":protocol://:domain(.*)::port?/tr/:path(.*)?"
      ],
      [
        "ar",
        ":protocol://:domain(.*)::port?/ar/:path(.*)?"
      ],
      [
        "wuu",
        ":protocol://:domain(.*)::port?/wuu/:path(.*)?"
      ],
      [
        "mr",
        ":protocol://:domain(.*)::port?/mr/:path(.*)?"
      ],
      [
        "nb",
        ":protocol://:domain(.*)::port?/nb/:path(.*)?"
      ],
      [
        "fi",
        ":protocol://:domain(.*)::port?/fi/:path(.*)?"
      ],
      [
        "da",
        ":protocol://:domain(.*)::port?/da/:path(.*)?"
      ],
      [
        "et",
        ":protocol://:domain(.*)::port?/et/:path(.*)?"
      ],
      [
        "lv",
        ":protocol://:domain(.*)::port?/lv/:path(.*)?"
      ],
      [
        "lt",
        ":protocol://:domain(.*)::port?/lt/:path(.*)?"
      ],
      [
        "pl",
        ":protocol://:domain(.*)::port?/pl/:path(.*)?"
      ],
      [
        "de",
        ":protocol://:domain(.*)::port?/de/:path(.*)?"
      ],
      [
        "nl",
        ":protocol://:domain(.*)::port?/nl/:path(.*)?"
      ],
      [
        "fr",
        ":protocol://:domain(.*)::port?/fr/:path(.*)?"
      ],
      [
        "it",
        ":protocol://:domain(.*)::port?/it/:path(.*)?"
      ],
      [
        "hu",
        ":protocol://:domain(.*)::port?/hu/:path(.*)?"
      ],
      [
        "cs",
        ":protocol://:domain(.*)::port?/cs/:path(.*)?"
      ],
      [
        "ro",
        ":protocol://:domain(.*)::port?/ro/:path(.*)?"
      ],
      [
        "el",
        ":protocol://:domain(.*)::port?/el/:path(.*)?"
      ],
      [
        "bg",
        ":protocol://:domain(.*)::port?/bg/:path(.*)?"
      ],
      [
        "hr",
        ":protocol://:domain(.*)::port?/hr/:path(.*)?"
      ],
      [
        "sr",
        ":protocol://:domain(.*)::port?/sr/:path(.*)?"
      ],
      [
        "sk",
        ":protocol://:domain(.*)::port?/sk/:path(.*)?"
      ],
      [
        "sl",
        ":protocol://:domain(.*)::port?/sl/:path(.*)?"
      ],
      [
        "en",
        ":protocol://:domain(.*)::port?/:path(.*)?"
      ]
    ]
  }
];
/** @type {string | undefined} */
let cachedRouteStrategyUrl;
/** @type {{ match: string; strategy?: Array<string>; exclude?: boolean } | undefined} */
let cachedRouteStrategy;
/**
 * @param {string | URL} url
 * @returns {{ match: string; strategy?: Array<string>; exclude?: boolean } | undefined}
 */
function findMatchingRouteStrategy(url) {
    if (routeStrategies.length === 0) {
        return undefined;
    }
    const urlString = typeof url === "string" ? url : url.href;
    if (cachedRouteStrategyUrl === urlString) {
        return cachedRouteStrategy;
    }
    const urlObject = new URL(urlString, "http://dummy.com");
    let match;
    for (const routeStrategy of routeStrategies) {
        const pattern = new URLPattern(routeStrategy.match, urlObject.href);
        if (pattern.exec(urlObject.href)) {
            match = routeStrategy;
            break;
        }
    }
    cachedRouteStrategyUrl = urlString;
    cachedRouteStrategy = match;
    return match;
}
/**
 * Returns the strategy to use for a specific URL.
 *
 * If no route strategy matches (or the matching rule is `exclude: true`),
 * the global strategy is returned.
 *
 * @param {string | URL} url
 * @returns {typeof strategy}
 */
export function getStrategyForUrl(url) {
    const routeStrategy = findMatchingRouteStrategy(url);
    if (routeStrategy &&
        routeStrategy.exclude !== true &&
        Array.isArray(routeStrategy.strategy)) {
        // @ts-ignore - runtime value is injected and validated by compiler types.
        return routeStrategy.strategy;
    }
    return strategy;
}
/**
 * Returns whether the given URL is excluded from middleware i18n processing.
 *
 * @param {string | URL} url
 * @returns {boolean}
 */
export function isExcludedByRouteStrategy(url) {
    return findMatchingRouteStrategy(url)?.exclude === true;
}
/**
 * @typedef {{
 * 		getStore(): {
 *   		locale?: Locale,
 * 			origin?: string,
 * 			messageCalls?: Set<string>
 *   	} | undefined,
 * 		run: (store: { locale?: Locale, origin?: string, messageCalls?: Set<string>},
 *    cb: any) => any
 * }} ParaglideAsyncLocalStorage
 */
/**
 * Server side async local storage that is set by `serverMiddleware()`.
 *
 * The variable is used to retrieve the locale and origin in a server-side
 * rendering context without effecting other requests.
 *
 * @type {ParaglideAsyncLocalStorage | undefined}
 */
export let serverAsyncLocalStorage = undefined;
export const disableAsyncLocalStorage = false;
export const experimentalMiddlewareLocaleSplitting = false;
export const isServer = import.meta.env?.SSR ?? typeof window === 'undefined';
/** @type {Locale | undefined} */
// @ts-ignore - injected by bundlers at compile time
export const experimentalStaticLocale = undefined;
/**
 * Sets the server side async local storage.
 *
 * The function is needed because the `runtime.js` file
 * must define the `serverAsyncLocalStorage` variable to
 * avoid a circular import between `runtime.js` and
 * `server.js` files.
 *
 * @param {ParaglideAsyncLocalStorage | undefined} value
 */
export function overwriteServerAsyncLocalStorage(value) {
    serverAsyncLocalStorage = value;
}
const TREE_SHAKE_COOKIE_STRATEGY_USED = false;
const TREE_SHAKE_URL_STRATEGY_USED = false;
const TREE_SHAKE_GLOBAL_VARIABLE_STRATEGY_USED = false;
const TREE_SHAKE_PREFERRED_LANGUAGE_STRATEGY_USED = false;
const TREE_SHAKE_DEFAULT_URL_PATTERN_USED = true;
const TREE_SHAKE_LOCAL_STORAGE_STRATEGY_USED = true;

globalThis.__paraglide = {}

/**
 * This is a fallback to get started with a custom
 * strategy and avoid type errors.
 *
 * The implementation is overwritten
 * by \`overwriteGetLocale()\` and \`defineSetLocale()\`.
 *
 * @type {Locale|undefined}
 */
let _locale;
let localeInitiallySet = false;
/**
 * Get the current locale.
 *
 * The locale is resolved using your configured strategies (URL, cookie, localStorage, etc.)
 * in the order they are defined. In SSR contexts, the locale is retrieved from AsyncLocalStorage
 * which is set by the `paraglideMiddleware()`.
 *
 * @see https://inlang.com/m/gerre34r/library-inlang-paraglideJs/strategy - Configure locale detection strategies
 *
 * @example
 *   if (getLocale() === 'de') {
 *     console.log('Germany 🇩🇪');
 *   } else if (getLocale() === 'nl') {
 *     console.log('Netherlands 🇳🇱');
 *   }
 *
 * @type {() => Locale}
 */
export let getLocale = () => {
    if (experimentalStaticLocale !== undefined) {
        return assertIsLocale(experimentalStaticLocale);
    }
    // if running in a server-side rendering context
    // retrieve the locale from the async local storage
    if (serverAsyncLocalStorage) {
        const locale = serverAsyncLocalStorage?.getStore()?.locale;
        if (locale) {
            return locale;
        }
    }
    let strategyToUse = strategy;
    if (!isServer && typeof window !== "undefined" && window.location?.href) {
        strategyToUse = getStrategyForUrl(window.location.href);
    }
    const resolved = resolveLocaleWithStrategies(strategyToUse, typeof window !== "undefined" ? window.location?.href : undefined);
    if (resolved) {
        if (!localeInitiallySet) {
            _locale = resolved;
            // https://github.com/opral/inlang-paraglide-js/issues/455
            localeInitiallySet = true;
            setLocale(resolved, { reload: false });
        }
        return resolved;
    }
    throw new Error("No locale found. Read the docs https://inlang.com/m/gerre34r/library-inlang-paraglideJs/errors#no-locale-found");
};
/**
 * Resolve locale for a given URL using route-aware strategies.
 *
 * @param {string | URL} url
 * @returns {Locale}
 */
export function getLocaleForUrl(url) {
    if (experimentalStaticLocale !== undefined) {
        return assertIsLocale(experimentalStaticLocale);
    }
    const strategyToUse = getStrategyForUrl(url);
    const resolved = resolveLocaleWithStrategies(strategyToUse, typeof url === "string" ? url : url.href);
    if (resolved) {
        return resolved;
    }
    throw new Error("No locale found. Read the docs https://inlang.com/m/gerre34r/library-inlang-paraglideJs/errors#no-locale-found");
}
/**
 * @param {typeof strategy} strategyToUse
 * @param {string | undefined} urlForUrlStrategy
 * @returns {Locale | undefined}
 */
function resolveLocaleWithStrategies(strategyToUse, urlForUrlStrategy) {
    /** @type {string | undefined} */
    let locale;
    for (const strat of strategyToUse) {
        if (TREE_SHAKE_COOKIE_STRATEGY_USED && strat === "cookie") {
            locale = extractLocaleFromCookie();
        }
        else if (strat === "baseLocale") {
            locale = baseLocale;
        }
        else if (TREE_SHAKE_URL_STRATEGY_USED &&
            strat === "url" &&
            !isServer &&
            typeof urlForUrlStrategy === "string") {
            locale = extractLocaleFromUrl(urlForUrlStrategy);
        }
        else if (TREE_SHAKE_GLOBAL_VARIABLE_STRATEGY_USED &&
            strat === "globalVariable" &&
            _locale !== undefined) {
            locale = _locale;
        }
        else if (TREE_SHAKE_PREFERRED_LANGUAGE_STRATEGY_USED &&
            strat === "preferredLanguage" &&
            !isServer) {
            locale = extractLocaleFromNavigator();
        }
        else if (TREE_SHAKE_LOCAL_STORAGE_STRATEGY_USED &&
            strat === "localStorage" &&
            !isServer) {
            locale = localStorage.getItem(localStorageKey) ?? undefined;
        }
        else if (isCustomStrategy(strat) && customClientStrategies.has(strat)) {
            const handler = customClientStrategies.get(strat);
            if (handler) {
                const result = handler.getLocale();
                // Handle both sync and async results - skip async in sync getLocale
                if (result instanceof Promise) {
                    // Can't await in sync function, skip async strategies
                    continue;
                }
                locale = result;
            }
        }
        if (locale !== undefined) {
            return assertIsLocale(locale);
        }
    }
    return undefined;
}
/**
 * Overwrite the `getLocale()` function.
 *
 * Use this function to overwrite how the locale is resolved. This is useful
 * for custom locale resolution or advanced use cases like SSG with concurrent rendering.
 *
 * @see https://inlang.com/m/gerre34r/library-inlang-paraglideJs/strategy
 *
 * @example
 *   overwriteGetLocale(() => {
 *     return Cookies.get('locale') ?? baseLocale
 *   });
 *
 * @type {(fn: () => Locale) => void}
 */
export const overwriteGetLocale = (fn) => {
    getLocale = fn;
};

const rtlLanguages = new Set([
    "ar",
    "dv",
    "fa",
    "he",
    "ks",
    "ku",
    "ps",
    "sd",
    "ug",
    "ur",
    "yi",
]);
/**
 * Get writing direction for a locale.
 *
 * Uses `Intl.Locale` text info when available and falls back to a
 * language-based RTL check for runtimes without `getTextInfo()`.
 *
 * @example
 *   getTextDirection(); // "ltr" or "rtl" for current locale
 *   getTextDirection("ar"); // "rtl"
 *   getTextDirection("en"); // "ltr"
 *
 * @param {string} [locale] - Target locale. If not provided, uses `getLocale()`
 * @returns {"ltr" | "rtl"}
 */
export function getTextDirection(locale = getLocale()) {
    try {
        const intlLocale = /** @type {Intl.Locale & {
            getTextInfo?: () => { direction?: string };
            textInfo?: { direction?: string };
        }} */ (new Intl.Locale(locale));
        const direction = intlLocale.getTextInfo?.().direction ?? intlLocale.textInfo?.direction;
        if (direction === "ltr" || direction === "rtl") {
            return direction;
        }
    }
    catch {
        // Ignore Intl.Locale parsing/runtime errors and use fallback below.
    }
    const language = locale.split("-")[0]?.toLowerCase();
    return rtlLanguages.has(language ?? "") ? "rtl" : "ltr";
}

/**
 * Navigates to the localized URL, or reloads the current page
 *
 * @param {string} [newLocation] The new location
 * @return {undefined}
 */
const navigateOrReload = (newLocation) => {
    if (newLocation) {
        // reload the page by navigating to the new url
        window.location.href = newLocation;
    }
    else {
        // reload the page to reflect the new locale
        window.location.reload();
    }
};
/**
 * @typedef {(newLocale: Locale, options?: { reload?: boolean }) => void | Promise<void>} SetLocaleFn
 */
/**
 * Set the locale.
 *
 * Updates the locale using your configured strategies (cookie, localStorage, URL, etc.).
 * By default, this reloads the page on the client to reflect the new locale. Reloading
 * can be disabled by passing `reload: false` as an option, but you'll need to ensure
 * the UI updates to reflect the new locale.
 *
 * If any custom strategy's `setLocale` function is async, then this function
 * will become async as well.
 *
 * @see https://inlang.com/m/gerre34r/library-inlang-paraglideJs/strategy
 *
 * @example
 *   setLocale('en');
 *
 * @example
 *   setLocale('en', { reload: false });
 *
 * @type {SetLocaleFn}
 */
export let setLocale = (newLocale, options) => {
    const optionsWithDefaults = {
        reload: true,
        ...options,
    };
    // locale is already set
    // https://github.com/opral/inlang-paraglide-js/issues/430
    /** @type {Locale | undefined} */
    let currentLocale;
    try {
        currentLocale = getLocale();
    }
    catch {
        // do nothing, no locale has been set yet.
    }
    /** @type {Array<Promise<any>>} */
    const customSetLocalePromises = [];
    /** @type {string | undefined} */
    let newLocation = undefined;
    let strategyToUse = strategy;
    if (!isServer && typeof window !== "undefined" && window.location?.href) {
        strategyToUse = getStrategyForUrl(window.location.href);
    }
    for (const strat of strategyToUse) {
        if (TREE_SHAKE_GLOBAL_VARIABLE_STRATEGY_USED &&
            strat === "globalVariable") {
            // a default for a custom strategy to get started quickly
            // is likely overwritten by `defineSetLocale()`
            _locale = newLocale;
        }
        else if (TREE_SHAKE_COOKIE_STRATEGY_USED && strat === "cookie") {
            if (isServer ||
                typeof document === "undefined" ||
                typeof window === "undefined") {
                continue;
            }
            // set the cookie
            const cookieString = `${cookieName}=${newLocale}; path=/; max-age=${cookieMaxAge}`;
            document.cookie = cookieDomain
                ? `${cookieString}; domain=${cookieDomain}`
                : cookieString;
        }
        else if (strat === "baseLocale") {
            // nothing to be set here. baseLocale is only a fallback
            continue;
        }
        else if (TREE_SHAKE_URL_STRATEGY_USED &&
            strat === "url" &&
            typeof window !== "undefined") {
            // route to the new url
            //
            // this triggers a page reload but a user rarely
            // switches locales, so this should be fine.
            //
            // if the behavior is not desired, the implementation
            // can be overwritten by `defineSetLocale()` to avoid
            // a full page reload.
            newLocation = localizeUrl(window.location.href, {
                locale: newLocale,
            }).href;
        }
        else if (TREE_SHAKE_LOCAL_STORAGE_STRATEGY_USED &&
            strat === "localStorage" &&
            typeof window !== "undefined") {
            // set the localStorage
            localStorage.setItem(localStorageKey, newLocale);
        }
        else if (isCustomStrategy(strat) && customClientStrategies.has(strat)) {
            const handler = customClientStrategies.get(strat);
            if (handler) {
                let result = handler.setLocale(newLocale);
                // Handle async setLocale
                if (result instanceof Promise) {
                    result = result.catch((error) => {
                        throw new Error(`Custom strategy "${strat}" setLocale failed.`, {
                            cause: error,
                        });
                    });
                    customSetLocalePromises.push(result);
                }
            }
        }
    }
    const runReload = () => {
        if (!isServer &&
            optionsWithDefaults.reload &&
            window.location &&
            newLocale !== currentLocale) {
            navigateOrReload(newLocation);
        }
    };
    if (customSetLocalePromises.length) {
        return Promise.all(customSetLocalePromises).then(() => {
            runReload();
        });
    }
    runReload();
    return;
};
/**
 * Overwrite the \`setLocale()\` function.
 *
 * Use this function to overwrite how the locale is set. For example,
 * modify a cookie, env variable, or a user's preference.
 *
 * @example
 *   overwriteSetLocale((newLocale) => {
 *     // set the locale in a cookie
 *     return Cookies.set('locale', newLocale)
 *   });
 *
 * @param {SetLocaleFn} fn
 */
export const overwriteSetLocale = (fn) => {
    setLocale = /** @type {SetLocaleFn} */ (fn);
};

/**
 * The origin of the current URL.
 *
 * Defaults to "http://y.com" in non-browser environments. If this
 * behavior is not desired, the implementation can be overwritten
 * by `overwriteGetUrlOrigin()`.
 *
 * @type {() => string}
 */
export let getUrlOrigin = () => {
    if (serverAsyncLocalStorage) {
        return serverAsyncLocalStorage.getStore()?.origin ?? "http://fallback.com";
    }
    else if (typeof window !== "undefined") {
        return window.location.origin;
    }
    return "http://fallback.com";
};
/**
 * Overwrite the getUrlOrigin function.
 *
 * Use this function in server environments to
 * define how the URL origin is resolved.
 *
 * @type {(fn: () => string) => void}
 */
export let overwriteGetUrlOrigin = (fn) => {
    getUrlOrigin = fn;
};

/**
 * Check if something is an available locale.
 *
 * @example
 *   if (isLocale(params.locale)) {
 *     setLocale(params.locale);
 *   } else {
 *     setLocale('en');
 *   }
 *
 * @param {any} locale
 * @returns {locale is Locale}
 */
export function isLocale(locale) {
    if (typeof locale !== "string")
        return false;
    return !locale
        ? false
        : locales.some((item) => item.toLowerCase() === locale.toLowerCase());
}

/**
 * Asserts that the input is a locale.
 *
 * @param {any} input - The input to check.
 * @returns {Locale} The input if it is a locale.
 * @throws {Error} If the input is not a locale.
 */
export function assertIsLocale(input) {
    if (typeof input !== "string") {
        throw new Error(`Invalid locale: ${input}. Expected a string.`);
    }
    const lowerInput = input.toLowerCase();
    const matchedLocale = locales.find((item) => item.toLowerCase() === lowerInput);
    if (!matchedLocale) {
        throw new Error(`Invalid locale: ${input}. Expected one of: ${locales.join(", ")}`);
    }
    return matchedLocale;
}

/**
 * Extracts a locale from a request.
 *
 * Use the function on the server to extract the locale
 * from a request.
 *
 * The function goes through the strategies in the order
 * they are defined. If a strategy returns an invalid locale,
 * it will fall back to the next strategy.
 *
 * Note: Custom server strategies are not supported in this synchronous version.
 * Use `extractLocaleFromRequestAsync` if you need custom server strategies with async getLocale methods.
 *
 * @example
 *   const locale = extractLocaleFromRequest(request);
 *
 * @type {(request: Request) => Locale}
 */
export const extractLocaleFromRequest = (request) => {
    return extractLocaleFromRequestWithStrategies(request, getStrategyForUrl(request.url));
};
/**
 * Extracts a locale from a request using the provided strategy order.
 *
 * @param {Request} request
 * @param {typeof strategy} strategies
 * @returns {Locale}
 */
export const extractLocaleFromRequestWithStrategies = (request, strategies) => {
    /** @type {string|undefined} */
    let locale;
    for (const strat of strategies) {
        if (TREE_SHAKE_COOKIE_STRATEGY_USED && strat === "cookie") {
            locale = request.headers
                .get("cookie")
                ?.split("; ")
                .find((c) => c.startsWith(cookieName + "="))
                ?.split("=")[1];
        }
        else if (TREE_SHAKE_URL_STRATEGY_USED && strat === "url") {
            locale = extractLocaleFromUrl(request.url);
        }
        else if (TREE_SHAKE_PREFERRED_LANGUAGE_STRATEGY_USED &&
            strat === "preferredLanguage") {
            locale = extractLocaleFromHeader(request);
        }
        else if (strat === "globalVariable") {
            locale = _locale;
        }
        else if (strat === "baseLocale") {
            return baseLocale;
        }
        else if (strat === "localStorage") {
            continue;
        }
        else if (isCustomStrategy(strat)) {
            // Custom strategies are not supported in sync version
            // Use extractLocaleFromRequestAsync for custom server strategies
            continue;
        }
        if (locale !== undefined) {
            if (!isLocale(locale)) {
                locale = undefined;
            }
            else {
                return assertIsLocale(locale);
            }
        }
    }
    throw new Error("No locale found. There is an error in your strategy. Try adding 'baseLocale' as the very last strategy. Read more here https://inlang.com/m/gerre34r/library-inlang-paraglideJs/errors#no-locale-found");
};

/**
 * Asynchronously extracts a locale from a request.
 *
 * This function supports async custom server strategies, unlike the synchronous
 * `extractLocaleFromRequest`. Use this function when you have custom server strategies
 * that need to perform asynchronous operations (like database calls) in their getLocale method.
 *
 * The function first processes any custom server strategies asynchronously, then falls back
 * to the synchronous `extractLocaleFromRequest` for all other strategies.
 *
 * @see {@link https://github.com/opral/inlang-paraglide-js/issues/527#issuecomment-2978151022}
 *
 * @example
 *   // Basic usage
 *   const locale = await extractLocaleFromRequestAsync(request);
 *
 * @example
 *   // With custom async server strategy
 *   defineCustomServerStrategy("custom-database", {
 *     getLocale: async (request) => {
 *       const userId = extractUserIdFromRequest(request);
 *       return await getUserLocaleFromDatabase(userId);
 *     }
 *   });
 *
 *   const locale = await extractLocaleFromRequestAsync(request);
 *
 * @type {(request: Request) => Promise<Locale>}
 */
export const extractLocaleFromRequestAsync = async (request) => {
    /** @type {string|undefined} */
    let locale;
    const strategy = getStrategyForUrl(request.url);
    // Process custom strategies first, in order
    for (const strat of strategy) {
        if (isCustomStrategy(strat) && customServerStrategies.has(strat)) {
            const handler = customServerStrategies.get(strat);
            if (handler) {
                /** @type {string|undefined} */
                locale = await handler.getLocale(request);
            }
            // If we got a valid locale from this custom strategy, use it
            if (locale !== undefined && isLocale(locale)) {
                return assertIsLocale(locale);
            }
        }
    }
    // If no custom strategy provided a valid locale, fall back to sync version
    locale = extractLocaleFromRequestWithStrategies(request, strategy);
    return assertIsLocale(locale);
};

/**
 * Extracts a cookie from the document.
 *
 * Will return undefined if the document is not available or if the cookie is not set.
 * The `document` object is not available in server-side rendering, so this function should not be called in that context.
 *
 * @returns {string | undefined}
 */
export function extractLocaleFromCookie() {
    if (typeof document === "undefined" || !document.cookie) {
        return;
    }
    const match = document.cookie.match(new RegExp(`(^| )${cookieName}=([^;]+)`));
    const locale = match?.[2];
    if (isLocale(locale)) {
        return locale;
    }
    return undefined;
}

/**
 * Extracts a locale from the accept-language header.
 *
 * Use the function on the server to extract the locale
 * from the accept-language header that is sent by the client.
 *
 * @example
 *   const locale = extractLocaleFromHeader(request);
 *
 * @type {(request: Request) => Locale}
 * @param {Request} request - The request object to extract the locale from.
 * @returns {string|undefined} The negotiated preferred language.
 */
export function extractLocaleFromHeader(request) {
    const acceptLanguageHeader = request.headers.get("accept-language");
    if (acceptLanguageHeader) {
        // Parse language preferences with their q-values and base language codes
        const languages = acceptLanguageHeader
            .split(",")
            .map((lang) => {
            const [tag, q = "1"] = lang.trim().split(";q=");
            // Get both the full tag and base language code
            const baseTag = tag?.split("-")[0]?.toLowerCase();
            return {
                fullTag: tag?.toLowerCase(),
                baseTag,
                q: Number(q),
            };
        })
            .sort((a, b) => b.q - a.q);
        for (const lang of languages) {
            if (isLocale(lang.fullTag)) {
                return lang.fullTag;
            }
            else if (isLocale(lang.baseTag)) {
                return lang.baseTag;
            }
        }
        return undefined;
    }
    return undefined;
}

/**
 * Negotiates a preferred language from navigator.languages.
 *
 * Use the function on the client to extract the locale
 * from the navigator.languages array.
 *
 * @example
 *   const locale = extractLocaleFromNavigator();
 *
 * @type {() => Locale | undefined}
 * @returns {string | undefined}
 */
export function extractLocaleFromNavigator() {
    if (!navigator?.languages?.length) {
        return undefined;
    }
    const languages = navigator.languages.map((lang) => ({
        fullTag: lang.toLowerCase(),
        baseTag: lang.split("-")[0]?.toLowerCase(),
    }));
    for (const lang of languages) {
        if (isLocale(lang.fullTag)) {
            return lang.fullTag;
        }
        else if (isLocale(lang.baseTag)) {
            return lang.baseTag;
        }
    }
    return undefined;
}

/**
 * If extractLocaleFromUrl is called many times on the same page and the URL
 * hasn't changed, we don't need to recompute it every time which can get expensive.
 * We might use a LRU cache if needed, but for now storing only the last result is enough.
 * https://github.com/opral/monorepo/pull/3575#discussion_r2066731243
 */
/** @type {string|undefined} */
let cachedUrl;
/** @type {Locale|undefined} */
let cachedLocale;
/**
 * Extracts the locale from a given URL using native URLPattern.
 *
 * @param {URL|string} url - The full URL from which to extract the locale.
 * @returns {Locale|undefined} The extracted locale, or undefined if no locale is found.
 */
export function extractLocaleFromUrl(url) {
    const urlString = typeof url === "string" ? url : url.href;
    if (cachedUrl === urlString) {
        return cachedLocale;
    }
    let result;
    if (TREE_SHAKE_DEFAULT_URL_PATTERN_USED) {
        result = defaultUrlPatternExtractLocale(url);
    }
    else {
        const urlObj = typeof url === "string" ? new URL(url) : url;
        // Iterate over URL patterns
        for (const element of urlPatterns) {
            for (const [locale, localizedPattern] of element.localized) {
                const match = new URLPattern(localizedPattern, urlObj.href).exec(urlObj.href);
                if (!match) {
                    continue;
                }
                // Check if the locale is valid
                if (assertIsLocale(locale)) {
                    result = locale;
                    break;
                }
            }
            if (result)
                break;
        }
    }
    cachedUrl = urlString;
    cachedLocale = result;
    return result;
}
/**
 * https://github.com/opral/inlang-paraglide-js/issues/381
 *
 * @param {URL|string} url - The full URL from which to extract the locale.
 * @returns {Locale|undefined} The extracted locale, or undefined if no locale is found.
 */
function defaultUrlPatternExtractLocale(url) {
    const urlObj = new URL(url, "http://dummy.com");
    const pathSegments = urlObj.pathname.split("/").filter(Boolean);
    if (pathSegments.length > 0) {
        const potentialLocale = pathSegments[0];
        if (isLocale(potentialLocale)) {
            return potentialLocale;
        }
    }
    // everything else has to be the base locale
    return baseLocale;
}

/**
 * Lower-level URL localization function, primarily used in server contexts.
 *
 * This function is designed for server-side usage where you need precise control
 * over URL localization, such as in middleware or request handlers. It works with
 * URL objects and always returns absolute URLs.
 *
 * For client-side UI components, use `localizeHref()` instead, which provides
 * a more convenient API with relative paths and automatic locale detection.
 *
 * @see https://inlang.com/m/gerre34r/library-inlang-paraglideJs/i18n-routing
 *
 * @example
 * ```typescript
 * // Server middleware example
 * app.use((req, res, next) => {
 *   const url = new URL(req.url, `${req.protocol}://${req.headers.host}`);
 *   const localized = localizeUrl(url, { locale: "de" });
 *
 *   if (localized.href !== url.href) {
 *     return res.redirect(localized.href);
 *   }
 *   next();
 * });
 * ```
 *
 * @example
 * ```typescript
 * // Using with URL patterns
 * const url = new URL("https://example.com/about");
 * localizeUrl(url, { locale: "de" });
 * // => URL("https://example.com/de/about")
 *
 * // Using with domain-based localization
 * const url = new URL("https://example.com/store");
 * localizeUrl(url, { locale: "de" });
 * // => URL("https://de.example.com/store")
 * ```
 *
 * @param {string | URL} url - The URL to localize. If string, must be absolute.
 * @param {Object} [options] - Options for localization
 * @param {string} [options.locale] - Target locale. If not provided, uses getLocale()
 * @returns {URL} The localized URL, always absolute
 */
export function localizeUrl(url, options) {
    if (TREE_SHAKE_DEFAULT_URL_PATTERN_USED) {
        return localizeUrlDefaultPattern(url, options);
    }
    const targetLocale = options?.locale ?? getLocale();
    const urlObj = typeof url === "string" ? new URL(url) : url;
    // Iterate over URL patterns
    for (const element of urlPatterns) {
        // match localized patterns
        for (const [, localizedPattern] of element.localized) {
            const match = new URLPattern(localizedPattern, urlObj.href).exec(urlObj.href);
            if (!match) {
                continue;
            }
            const targetPattern = element.localized.find(([locale]) => locale === targetLocale)?.[1];
            if (!targetPattern) {
                continue;
            }
            const localizedUrl = fillPattern(targetPattern, aggregateGroups(match), urlObj.origin);
            return fillMissingUrlParts(localizedUrl, match);
        }
        const unlocalizedMatch = new URLPattern(element.pattern, urlObj.href).exec(urlObj.href);
        if (unlocalizedMatch) {
            const targetPattern = element.localized.find(([locale]) => locale === targetLocale)?.[1];
            if (targetPattern) {
                const localizedUrl = fillPattern(targetPattern, aggregateGroups(unlocalizedMatch), urlObj.origin);
                return fillMissingUrlParts(localizedUrl, unlocalizedMatch);
            }
        }
    }
    // If no match found, return the original URL
    return urlObj;
}
/**
 * https://github.com/opral/inlang-paraglide-js/issues/381
 *
 * @param {string | URL} url
 * @param {Object} [options]
 * @param {string} [options.locale]
 * @returns {URL}
 */
function localizeUrlDefaultPattern(url, options) {
    const urlObj = typeof url === "string" ? new URL(url, getUrlOrigin()) : new URL(url);
    const locale = options?.locale ?? getLocale();
    const currentLocale = extractLocaleFromUrl(urlObj);
    // If current locale matches target locale, no change needed
    if (currentLocale === locale) {
        return urlObj;
    }
    const pathSegments = urlObj.pathname.split("/").filter(Boolean);
    // If current path starts with a locale, remove it
    if (pathSegments.length > 0 && isLocale(pathSegments[0])) {
        pathSegments.shift();
    }
    // For base locale, don't add prefix
    if (locale === baseLocale) {
        urlObj.pathname = "/" + pathSegments.join("/");
    }
    else {
        // For other locales, add prefix
        urlObj.pathname = "/" + locale + "/" + pathSegments.join("/");
    }
    return urlObj;
}
/**
 * Low-level URL de-localization function, primarily used in server contexts.
 *
 * This function is designed for server-side usage where you need precise control
 * over URL de-localization, such as in middleware or request handlers. It works with
 * URL objects and always returns absolute URLs.
 *
 * For client-side UI components, use `deLocalizeHref()` instead, which provides
 * a more convenient API with relative paths.
 *
 * @see https://inlang.com/m/gerre34r/library-inlang-paraglideJs/i18n-routing
 *
 * @example
 * ```typescript
 * // Server middleware example
 * app.use((req, res, next) => {
 *   const url = new URL(req.url, `${req.protocol}://${req.headers.host}`);
 *   const baseUrl = deLocalizeUrl(url);
 *
 *   // Store the base URL for later use
 *   req.baseUrl = baseUrl;
 *   next();
 * });
 * ```
 *
 * @example
 * ```typescript
 * // Using with URL patterns
 * const url = new URL("https://example.com/de/about");
 * deLocalizeUrl(url); // => URL("https://example.com/about")
 *
 * // Using with domain-based localization
 * const url = new URL("https://de.example.com/store");
 * deLocalizeUrl(url); // => URL("https://example.com/store")
 * ```
 *
 * @param {string | URL} url - The URL to de-localize. If string, must be absolute.
 * @returns {URL} The de-localized URL, always absolute
 */
export function deLocalizeUrl(url) {
    if (TREE_SHAKE_DEFAULT_URL_PATTERN_USED) {
        return deLocalizeUrlDefaultPattern(url);
    }
    const urlObj = typeof url === "string" ? new URL(url) : url;
    // Iterate over URL patterns
    for (const element of urlPatterns) {
        // Iterate over localized versions
        for (const [, localizedPattern] of element.localized) {
            const match = new URLPattern(localizedPattern, urlObj.href).exec(urlObj.href);
            if (match) {
                // Convert localized URL back to the base pattern
                const groups = aggregateGroups(match);
                const baseUrl = fillPattern(element.pattern, groups, urlObj.origin);
                return fillMissingUrlParts(baseUrl, match);
            }
        }
        // match unlocalized pattern
        const unlocalizedMatch = new URLPattern(element.pattern, urlObj.href).exec(urlObj.href);
        if (unlocalizedMatch) {
            const baseUrl = fillPattern(element.pattern, aggregateGroups(unlocalizedMatch), urlObj.origin);
            return fillMissingUrlParts(baseUrl, unlocalizedMatch);
        }
    }
    // no match found return the original url
    return urlObj;
}
/**
 * De-localizes a URL using the default pattern (/:locale/*)
 * @param {string|URL} url
 * @returns {URL}
 */
function deLocalizeUrlDefaultPattern(url) {
    const urlObj = typeof url === "string" ? new URL(url, getUrlOrigin()) : new URL(url);
    const pathSegments = urlObj.pathname.split("/").filter(Boolean);
    // If first segment is a locale, remove it
    if (pathSegments.length > 0 && isLocale(pathSegments[0])) {
        urlObj.pathname = "/" + pathSegments.slice(1).join("/");
    }
    return urlObj;
}
/**
 * Takes matches of implicit wildcards in the UrlPattern (when a part is missing
 * it is equal to '*') and adds them back to the result of fillPattern.
 *
 * At least protocol and hostname are required to create a valid URL inside fillPattern.
 *
 * @param {URL} url
 * @param {any} match
 * @returns {URL}
 */
function fillMissingUrlParts(url, match) {
    if (match.protocol.groups["0"]) {
        url.protocol = match.protocol.groups["0"] ?? "";
    }
    if (match.hostname.groups["0"]) {
        url.hostname = match.hostname.groups["0"] ?? "";
    }
    if (match.username.groups["0"]) {
        url.username = match.username.groups["0"] ?? "";
    }
    if (match.password.groups["0"]) {
        url.password = match.password.groups["0"] ?? "";
    }
    if (match.port.groups["0"]) {
        url.port = match.port.groups["0"] ?? "";
    }
    if (match.pathname.groups["0"]) {
        url.pathname = match.pathname.groups["0"] ?? "";
    }
    if (match.search.groups["0"]) {
        url.search = match.search.groups["0"] ?? "";
    }
    if (match.hash.groups["0"]) {
        url.hash = match.hash.groups["0"] ?? "";
    }
    return url;
}
/**
 * Fills a URL pattern with values for named groups, supporting all URLPattern-style modifiers.
 *
 * This function will eventually be replaced by https://github.com/whatwg/urlpattern/issues/73
 *
 * Matches:
 * - :name        -> Simple
 * - :name?       -> Optional
 * - :name+       -> One or more
 * - :name*       -> Zero or more
 * - :name(...)   -> Regex group
 * - {text}       -> Group delimiter
 * - {text}?      -> Optional group delimiter
 *
 * If the value is `null`, the segment is removed.
 *
 * @param {string} pattern - The URL pattern containing named groups.
 * @param {Record<string, string | null | undefined>} values - Object of values for named groups.
 * @param {string} origin - Base URL to use for URL construction.
 * @returns {URL} - The constructed URL with named groups filled.
 */
function fillPattern(pattern, values, origin) {
    // Pre-process the pattern to handle explicit port numbers
    // This detects patterns like "http://localhost:5173" and protects the port number
    // from being interpreted as a parameter
    let processedPattern = pattern.replace(/(https?:\/\/[^:/]+):(\d+)(\/|$)/g, (_, protocol, port, slash) => {
        // Replace ":5173" with "#PORT-5173#" to protect it from parameter replacement
        return `${protocol}#PORT-${port}#${slash}`;
    });
    // First, handle group delimiters with curly braces
    let processedGroupDelimiters = processedPattern.replace(/\{([^{}]*)\}([?+*]?)/g, (_, content, modifier) => {
        // For optional group delimiters
        if (modifier === "?") {
            // For optional groups, we'll include the content
            return content;
        }
        // For non-optional group delimiters, always include the content
        return content;
    });
    // Then handle named groups
    let filled = processedGroupDelimiters.replace(/(\/?):([a-zA-Z0-9_]+)(\([^)]*\))?([?+*]?)/g, (_, slash, name, __, modifier) => {
        const value = values[name];
        if (value === null) {
            // If value is null, remove the entire segment including the preceding slash
            return "";
        }
        if (modifier === "?") {
            // Optional segment
            return value !== undefined ? `${slash}${value}` : "";
        }
        if (modifier === "+" || modifier === "*") {
            // Repeatable segments
            if (value === undefined && modifier === "+") {
                throw new Error(`Missing value for "${name}" (one or more required)`);
            }
            return value ? `${slash}${value}` : "";
        }
        // Simple named group (no modifier)
        if (value === undefined) {
            throw new Error(`Missing value for "${name}"`);
        }
        return `${slash}${value}`;
    });
    // Restore port numbers
    filled = filled.replace(/#PORT-(\d+)#/g, ":$1");
    return new URL(filled, origin);
}
/**
 * Aggregates named groups from various parts of the URLPattern match result.
 *
 *
 * @type {(match: any) => Record<string, string | null | undefined>}
 */
export function aggregateGroups(match) {
    return {
        ...match.hash.groups,
        ...match.hostname.groups,
        ...match.password.groups,
        ...match.pathname.groups,
        ...match.port.groups,
        ...match.protocol.groups,
        ...match.search.groups,
        ...match.username.groups,
    };
}

/**
 * @typedef {object} ShouldRedirectServerInput
 * @property {Request} request
 * @property {string | URL} [url]
 * @property {ReturnType<typeof assertIsLocale>} [locale]
 *
 * @typedef {object} ShouldRedirectClientInput
 * @property {undefined} [request]
 * @property {string | URL} [url]
 * @property {ReturnType<typeof assertIsLocale>} [locale]
 *
 * @typedef {ShouldRedirectServerInput | ShouldRedirectClientInput} ShouldRedirectInput
 *
 * @typedef {object} ShouldRedirectResult
 * @property {boolean} shouldRedirect - Indicates whether the consumer should perform a redirect.
 * @property {ReturnType<typeof assertIsLocale>} locale - Locale resolved using the configured strategies.
 * @property {URL | undefined} redirectUrl - Destination URL when a redirect is required.
 */
/**
 * Determines whether a redirect is required to align the current URL with the active locale.
 *
 * This helper mirrors the logic that powers `paraglideMiddleware`, but works in both server
 * and client environments. It evaluates the configured strategies in order, computes the
 * canonical localized URL, and reports when the current URL does not match.
 *
 * When called in the browser without arguments, the current `window.location.href` is used.
 *
 * @see https://inlang.com/m/gerre34r/library-inlang-paraglideJs/i18n-routing#client-side-redirects
 *
 * @example
 * // Client side usage (e.g. TanStack Router beforeLoad hook)
 * async function beforeLoad({ location }) {
 *   const decision = await shouldRedirect({ url: location.href });
 *
 *   if (decision.shouldRedirect) {
 *     throw redirect({ to: decision.redirectUrl.href });
 *   }
 * }
 *
 * @example
 * // Server side usage with a Request
 * export async function handle(request) {
 *   const decision = await shouldRedirect({ request });
 *
 *   if (decision.shouldRedirect) {
 *     return Response.redirect(decision.redirectUrl, 307);
 *   }
 *
 *   return render(request, decision.locale);
 * }
 *
 * @param {ShouldRedirectInput} [input]
 * @returns {Promise<ShouldRedirectResult>}
 */
export async function shouldRedirect(input = {}) {
    const currentUrl = resolveUrl(input);
    const locale = /** @type {ReturnType<typeof assertIsLocale>} */ (await resolveLocale(input, currentUrl));
    const strategy = getStrategyForUrl(currentUrl.href);
    if (isExcludedByRouteStrategy(currentUrl.href) || !strategy.includes("url")) {
        return { shouldRedirect: false, locale, redirectUrl: undefined };
    }
    const localizedUrl = localizeUrl(currentUrl.href, { locale });
    const shouldRedirectToLocalizedUrl = normalizeUrl(localizedUrl.href) !== normalizeUrl(currentUrl.href);
    return {
        shouldRedirect: shouldRedirectToLocalizedUrl,
        locale,
        redirectUrl: shouldRedirectToLocalizedUrl ? localizedUrl : undefined,
    };
}
/**
 * Resolves the locale either from the provided input or by using the configured strategies.
 *
 * @param {ShouldRedirectInput} input
 * @param {URL} currentUrl
 * @returns {Promise<ReturnType<typeof assertIsLocale>>}
 */
async function resolveLocale(input, currentUrl) {
    if (input.locale) {
        return assertIsLocale(input.locale);
    }
    if (input.request) {
        return extractLocaleFromRequestAsync(input.request);
    }
    if (typeof input.url !== "undefined") {
        return getLocaleForUrl(currentUrl.href);
    }
    return getLocale();
}
/**
 * Resolves the current URL from the provided input or runtime context.
 *
 * @param {ShouldRedirectInput} input
 * @returns {URL}
 */
function resolveUrl(input) {
    if (input.request) {
        return new URL(input.request.url);
    }
    if (input.url instanceof URL) {
        return new URL(input.url.href);
    }
    if (typeof input.url === "string") {
        return new URL(input.url, getUrlOrigin());
    }
    if (typeof window !== "undefined" && window?.location?.href) {
        return new URL(window.location.href);
    }
    throw new Error("shouldRedirect() requires either a request, an absolute URL, or must run in a browser environment.");
}
/**
 * Normalize url for comparison by stripping the trailing slash.
 *
 * @param {string} url
 * @returns {string}
 */
function normalizeUrl(url) {
    const urlObj = new URL(url);
    urlObj.pathname = urlObj.pathname.replace(/\/$/, "");
    return urlObj.href;
}

/**
 * High-level URL localization function optimized for client-side UI usage.
 *
 * This is a convenience wrapper around `localizeUrl()` that provides features
 * needed in UI:
 *
 * - Accepts relative paths (e.g., "/about")
 * - Returns relative paths when possible
 * - Automatically detects current locale if not specified
 * - Handles string input/output instead of URL objects
 *
 * @see https://inlang.com/m/gerre34r/library-inlang-paraglideJs/i18n-routing
 *
 * @example
 * ```typescript
 * // In a React/Vue/Svelte component
 * const NavLink = ({ href }) => {
 *   // Automatically uses current locale, keeps path relative
 *   return <a href={localizeHref(href)}>...</a>;
 * };
 *
 * // Examples:
 * localizeHref("/about")
 * // => "/de/about" (if current locale is "de")
 * localizeHref("/store", { locale: "fr" })
 * // => "/fr/store" (explicit locale)
 *
 * // Cross-origin links remain absolute
 * localizeHref("https://other-site.com/about")
 * // => "https://other-site.com/de/about"
 * ```
 *
 * For server-side URL localization (e.g., in middleware), use `localizeUrl()`
 * which provides more precise control over URL handling.
 *
 * @param {string} href - The href to localize (can be relative or absolute)
 * @param {Object} [options] - Options for localization
 * @param {string} [options.locale] - Target locale. If not provided, uses `getLocale()`
 * @returns {string} The localized href, relative if input was relative
 */
export function localizeHref(href, options) {
    const currentLocale = getLocale();
    const locale = options?.locale ?? currentLocale;
    const url = new URL(href, getUrlOrigin());
    const localized = localizeUrl(url, { locale });
    // if the origin is identical and the href is relative,
    // return the relative path
    if (href.startsWith("/") && url.origin === localized.origin) {
        // check for cross origin localization in which case an absolute URL must be returned.
        if (locale !== currentLocale) {
            const localizedCurrentLocale = localizeUrl(url, {
                locale: currentLocale,
            });
            if (localizedCurrentLocale.origin !== localized.origin) {
                return localized.href;
            }
        }
        return localized.pathname + localized.search + localized.hash;
    }
    return localized.href;
}
/**
 * High-level URL de-localization function optimized for client-side UI usage.
 *
 * This is a convenience wrapper around `deLocalizeUrl()` that provides features
 * needed in the UI:
 *
 * - Accepts relative paths (e.g., "/de/about")
 * - Returns relative paths when possible
 * - Handles string input/output instead of URL objects
 *
 * @see https://inlang.com/m/gerre34r/library-inlang-paraglideJs/i18n-routing
 *
 * @example
 * ```typescript
 * // In a React/Vue/Svelte component
 * const LocaleSwitcher = ({ href }) => {
 *   // Remove locale prefix before switching
 *   const baseHref = deLocalizeHref(href);
 *   return locales.map(locale =>
 *     <a href={localizeHref(baseHref, { locale })}>
 *       Switch to {locale}
 *     </a>
 *   );
 * };
 *
 * // Examples:
 * deLocalizeHref("/de/about")  // => "/about"
 * deLocalizeHref("/fr/store")  // => "/store"
 *
 * // Cross-origin links remain absolute
 * deLocalizeHref("https://example.com/de/about")
 * // => "https://example.com/about"
 * ```
 *
 * For server-side URL de-localization (e.g., in middleware), use `deLocalizeUrl()`
 * which provides more precise control over URL handling.
 *
 * @param {string} href - The href to de-localize (can be relative or absolute)
 * @returns {string} The de-localized href, relative if input was relative
 */
export function deLocalizeHref(href) {
    const url = new URL(href, getUrlOrigin());
    const deLocalized = deLocalizeUrl(url);
    // If the origin is identical and the href is relative,
    // return the relative path instead of the full URL.
    if (href.startsWith("/") && url.origin === deLocalized.origin) {
        return deLocalized.pathname + deLocalized.search + deLocalized.hash;
    }
    return deLocalized.href;
}

/**
 * @param {string} safeModuleId
 * @param {Locale} locale
 */
export function trackMessageCall(safeModuleId, locale) {
    if (isServer === false)
        return;
    const store = serverAsyncLocalStorage?.getStore();
    if (store) {
        store.messageCalls?.add(`${safeModuleId}:${locale}`);
    }
}

/**
 * Generates localized URL variants for all provided URLs based on your configured locales and URL patterns.
 *
 * This function is essential for Static Site Generation (SSG) where you need to tell your framework
 * which pages to pre-render at build time. It's also useful for generating sitemaps and
 * `<link rel="alternate" hreflang>` tags for SEO.
 *
 * The function respects your `urlPatterns` configuration - if you have translated pathnames
 * (e.g., `/about` → `/ueber-uns` for German), it will generate the correct localized paths.
 *
 * @see https://inlang.com/m/gerre34r/library-inlang-paraglideJs/static-site-generation
 *
 * @example
 * // Basic usage - generate all locale variants for a list of paths
 * const localizedUrls = generateStaticLocalizedUrls([
 *   "/",
 *   "/about",
 *   "/blog/post-1",
 * ]);
 * // Returns URL objects for each locale:
 * // ["/en/", "/de/", "/en/about", "/de/about", "/en/blog/post-1", "/de/blog/post-1"]
 *
 * @example
 * // Use with framework SSG APIs
 * // SvelteKit
 * export function entries() {
 *   const paths = ["/", "/about", "/contact"];
 *   return generateStaticLocalizedUrls(paths).map(url => ({
 *     locale: extractLocaleFromUrl(url)
 *   }));
 * }
 *
 * @example
 * // Sitemap generation
 * const allPages = ["/", "/about", "/blog"];
 * const sitemapUrls = generateStaticLocalizedUrls(allPages);
 *
 * @param {(string | URL)[]} urls - List of canonical URLs or paths to generate localized versions for.
 *   Can be absolute URLs (`https://example.com/about`) or paths (`/about`).
 *   Paths are resolved against `http://localhost` internally.
 * @returns {URL[]} Array of URL objects representing all localized variants.
 *   The order follows each input URL with all its locale variants before moving to the next URL.
 */
export function generateStaticLocalizedUrls(urls) {
    const localizedUrls = new Set();
    // For default URL pattern, we can optimize the generation
    if (TREE_SHAKE_DEFAULT_URL_PATTERN_USED) {
        for (const urlInput of urls) {
            const url = urlInput instanceof URL
                ? urlInput
                : new URL(urlInput, "http://localhost");
            // Base locale doesn't get a prefix
            localizedUrls.add(url);
            // Other locales get their code as prefix
            for (const locale of locales) {
                if (locale !== baseLocale) {
                    const localizedPath = `/${locale}${url.pathname}${url.search}${url.hash}`;
                    const localizedUrl = new URL(localizedPath, url.origin);
                    localizedUrls.add(localizedUrl);
                }
            }
        }
        return Array.from(localizedUrls);
    }
    // For custom URL patterns, we need to use localizeUrl for each URL and locale
    for (const urlInput of urls) {
        const url = urlInput instanceof URL
            ? urlInput
            : new URL(urlInput, "http://localhost");
        // Try each URL pattern to find one that matches
        let patternFound = false;
        for (const pattern of urlPatterns) {
            try {
                // Try to match the unlocalized pattern
                const unlocalizedMatch = new URLPattern(pattern.pattern, url.href).exec(url.href);
                if (!unlocalizedMatch)
                    continue;
                patternFound = true;
                // Track unique localized URLs to avoid duplicates when patterns are the same
                const seenUrls = new Set();
                // Generate localized URL for each locale
                for (const [locale] of pattern.localized) {
                    try {
                        const localizedUrl = localizeUrl(url, { locale });
                        const urlString = localizedUrl.href;
                        // Only add if we haven't seen this exact URL before
                        if (!seenUrls.has(urlString)) {
                            seenUrls.add(urlString);
                            localizedUrls.add(localizedUrl);
                        }
                    }
                    catch {
                        // Skip if localization fails for this locale
                        continue;
                    }
                }
                break;
            }
            catch {
                // Skip if pattern matching fails
                continue;
            }
        }
        // If no pattern matched, use the URL as is
        if (!patternFound) {
            localizedUrls.add(url);
        }
    }
    return Array.from(localizedUrls);
}

/**
 * @typedef {"cookie" | "baseLocale" | "globalVariable" | "url" | "preferredLanguage" | "localStorage"} BuiltInStrategy
 */
/**
 * @typedef {`custom_${string}`} CustomStrategy
 */
/**
 * @typedef {BuiltInStrategy | CustomStrategy} Strategy
 */
/**
 * @typedef {Array<Strategy>} Strategies
 */
/**
 * @typedef {{ getLocale: (request?: Request) => Promise<string | undefined> | (string | undefined) }} CustomServerStrategyHandler
 */
/**
 * @typedef {{ getLocale: () => Promise<string|undefined> | (string | undefined), setLocale: (locale: string) => Promise<void> | void }} CustomClientStrategyHandler
 */
/** @type {Map<string, CustomServerStrategyHandler>} */
export const customServerStrategies = new Map();
/** @type {Map<string, CustomClientStrategyHandler>} */
export const customClientStrategies = new Map();
/**
 * Checks if the given strategy is a custom strategy.
 *
 * @param {any} strategy The name of the custom strategy to validate.
 * Must be a string that starts with "custom-" followed by alphanumeric characters, hyphens, or underscores.
 * @returns {boolean} Returns true if it is a custom strategy, false otherwise.
 */
export function isCustomStrategy(strategy) {
    return (typeof strategy === "string" && /^custom-[A-Za-z0-9_-]+$/.test(strategy));
}
/**
 * Defines a custom strategy that is executed on the server.
 *
 * @see https://inlang.com/m/gerre34r/library-inlang-paraglideJs/strategy#write-your-own-strategy
 *
 * @param {any} strategy The name of the custom strategy to define. Must follow the pattern custom-name with alphanumeric characters, hyphens, or underscores.
 * @param {CustomServerStrategyHandler} handler The handler for the custom strategy, which should implement
 * the method getLocale.
 * @returns {void}
 */
export function defineCustomServerStrategy(strategy, handler) {
    if (!isCustomStrategy(strategy)) {
        throw new Error(`Invalid custom strategy: "${strategy}". Must be a custom strategy following the pattern custom-name.`);
    }
    customServerStrategies.set(strategy, handler);
}
/**
 * Defines a custom strategy that is executed on the client.
 *
 * @see https://inlang.com/m/gerre34r/library-inlang-paraglideJs/strategy#write-your-own-strategy
 *
 * @param {any} strategy The name of the custom strategy to define. Must follow the pattern custom-name with alphanumeric characters, hyphens, or underscores.
 * @param {CustomClientStrategyHandler} handler The handler for the custom strategy, which should implement the
 * methods getLocale and setLocale.
 * @returns {void}
 */
export function defineCustomClientStrategy(strategy, handler) {
    if (!isCustomStrategy(strategy)) {
        throw new Error(`Invalid custom strategy: "${strategy}". Must be a custom strategy following the pattern custom-name.`);
    }
    customClientStrategies.set(strategy, handler);
}

// ------ TYPES ------

/**
 * A locale that is available in the project.
 *
 * @example
 *   setLocale(request.locale as Locale)
 *
 * @typedef {(typeof locales)[number]} Locale
 */

/**
 * A branded type representing a localized string.
 *
 * Message functions return this type instead of `string`, enabling TypeScript
 * to distinguish translated strings from regular strings at compile time.
 * This allows you to enforce that only properly localized content is used
 * in your UI components.
 *
 * Since `LocalizedString` is a branded subtype of `string`, it remains fully
 * backward compatible—you can pass it anywhere a `string` is expected.
 *
 * @example
 *   // Enforce localized strings in your components
 *   function PageTitle(props: { title: LocalizedString }) {
 *     return <h1>{props.title}</h1>
 *   }
 *
 *   // ✅ Correct: using a message function
 *   <PageTitle title={m.welcome_title()} />
 *
 *   // ❌ Type error: raw strings are not LocalizedString
 *   <PageTitle title="Welcome" />
 *
 * @example
 *   // LocalizedString is assignable to string (backward compatible)
 *   const localized: LocalizedString = m.greeting()
 *   const str: string = localized  // ✅ works fine
 *
 *   // But string is not assignable to LocalizedString
 *   const raw: LocalizedString = "Hello"  // ❌ Type error
 *
 * @example
 *   // Catches accidental string concatenation
 *   function showMessage(msg: LocalizedString) { ... }
 *
 *   showMessage(m.hello())                    // ✅
 *   showMessage("Hello " + userName)          // ❌ Type error
 *   showMessage(m.hello_user({ name: userName }))  // ✅ use params instead
 *
 * @typedef {string & { readonly __brand: 'LocalizedString' }} LocalizedString
 */

/**
 * Record of markup options for a tag instance.
 *
 * @typedef {Record<string, unknown>} MessageMarkupOptions
 */

/**
 * Record of markup attributes for a tag instance.
 *
 * @typedef {Record<string, string | true>} MessageMarkupAttributes
 */

/**
 * Type-level schema for a single markup tag.
 *
 * @typedef {{
 *   options: MessageMarkupOptions;
 *   attributes: MessageMarkupAttributes;
 *   children: boolean;
 * }} MessageMarkupTag
 */

/**
 * Type-level schema for all markup tags in a message.
 *
 * @typedef {Record<string, MessageMarkupTag>} MessageMarkupSchema
 */

/**
 * Type-only metadata attached to compiled message functions.
 *
 * @template Inputs
 * @template Options
 * @template {MessageMarkupSchema} Markup
 * @typedef {{
 *   readonly __paraglide?: {
 *     inputs: Inputs;
 *     options: Options;
 *     markup: Markup;
 *   };
 * }} MessageMetadata
 */

/**
 * A compiled, framework-neutral message part.
 *
 * @typedef {{
 *   type: "text";
 *   value: string;
 * } | {
 *   type: "markup-start";
 *   name: string;
 *   options: MessageMarkupOptions;
 *   attributes: MessageMarkupAttributes;
 * } | {
 *   type: "markup-end";
 *   name: string;
 *   options: MessageMarkupOptions;
 *   attributes: MessageMarkupAttributes;
 * } | {
 *   type: "markup-standalone";
 *   name: string;
 *   options: MessageMarkupOptions;
 *   attributes: MessageMarkupAttributes;
 * }} MessagePart
 */


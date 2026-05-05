/* eslint-disable */

import * as runtime from "./runtime.js";

/**
 * Server middleware that handles locale-based routing and request processing.
 *
 * Configure `disableAsyncLocalStorage` when generating Paraglide with
 * `paraglideVitePlugin()` or `compile()`, not when calling
 * `paraglideMiddleware()`. Keep AsyncLocalStorage enabled by default and
 * only disable it for runtimes that lack `AsyncLocalStorage` support and
 * guarantee request isolation.
 *
 * This middleware performs several key functions:
 *
 * 1. Determines the locale for the incoming request using configured strategies
 * 2. Handles URL localization and redirects (only for document requests)
 * 3. Maintains locale state using AsyncLocalStorage to prevent request interference
 *
 * When URL strategy is used:
 *
 * - The locale is extracted from the URL for all request types
 * - If URL doesn't match the determined locale, redirects to localized URL (only for document requests)
 * - De-localizes URLs before passing to server (e.g., `/fr/about` → `/about`)
 *
 * @see https://inlang.com/m/gerre34r/library-inlang-paraglideJs/middleware
 *
 * @template T - The return type of the resolve function
 *
 * @param {Request} request - The incoming request object
 * @param {(args: { request: Request, locale: import("./runtime.js").Locale }) => T | Promise<T>} resolve - Function to handle the request. The callback receives:
 *   - `request`: A modified request with a delocalized URL when the URL strategy is used (e.g., `/fr/about` → `/about`).
 *      If your framework handles URL localization itself (e.g., TanStack Router's `rewrite` option), use the original
 *      request instead to avoid redirect loops.
 *   - `locale`: The determined locale for this request.
 * @param {{
 *   effectiveRequestUrl?: string | URL | ((request: Request) => string | URL),
 *   onRedirect?: (response: Response) => void
 * }} [options] - Options to control middleware behavior. `effectiveRequestUrl` sets the effective request URL used for route matching, URL-based locale detection, redirects, and `getUrlOrigin()`.
 * @returns {Promise<Response>}
 *
 * @example
 * ```typescript
 * // Basic usage in metaframeworks like NextJS, SvelteKit, Astro, Nuxt, etc.
 * export const handle = async ({ event, resolve }) => {
 *   return paraglideMiddleware(event.request, ({ request, locale }) => {
 *     // let the framework further resolve the request
 *     return resolve(request);
 *   });
 * };
 * ```
 *
 * @example
 * ```typescript
 * // Usage in a framework like Express JS or Hono
 * app.use(async (req, res, next) => {
 *   const result = await paraglideMiddleware(req, ({ request, locale }) => {
 *     // If a redirect happens this won't be called
 *     return next(request);
 *   });
 * });
 * ```
 *
 * @example
 * ```typescript
 * // Usage with frameworks that handle URL localization/delocalization themselves
 * //
 * // Some frameworks like TanStack Router handle URL localization and delocalization
 * // themselves via their own rewrite APIs (e.g., `rewrite.input`/`rewrite.output`).
 * //
 * // When the framework handles this, the middleware's URL delocalization is not needed.
 * // Using the modified `request` from the callback would cause a redirect loop because
 * // both the middleware and the framework would attempt to delocalize the URL.
 * //
 * // Solution: Pass the original request to the handler instead of the modified one.
 * // The middleware still handles locale detection, cookies, and AsyncLocalStorage context.
 * //
 * // ❌ WRONG - causes redirect loop when framework handles URL rewriting:
 * // paraglideMiddleware(req, ({ request }) => handler.fetch(request))
 * //
 * // ✅ CORRECT - use original request when framework handles URL localization:
 * // paraglideMiddleware(req, () => handler.fetch(req))
 *
 * * *
 * export default {
 *   fetch(req: Request): Promise<Response> {
 *     // TanStack Router handles URL rewriting via deLocalizeUrl/localizeUrl
 *     // so we pass the original `req` instead of the modified `request`
 *     return paraglideMiddleware(req, () => handler.fetch(req))
 *   },
 * }
 * ```
 */
export async function paraglideMiddleware(request, resolve, options) {
    if (!runtime.disableAsyncLocalStorage && !runtime.serverAsyncLocalStorage) {
      const { AsyncLocalStorage } = await import("async_hooks");
      runtime.overwriteServerAsyncLocalStorage(new AsyncLocalStorage());
    } else if (!runtime.serverAsyncLocalStorage) {
      runtime.overwriteServerAsyncLocalStorage(createMockAsyncLocalStorage());
    }
    const url = resolveMiddlewareUrl(request, options?.effectiveRequestUrl);
    const origin = url.origin;
    if (runtime.isExcludedByRouteStrategy(url.href)) {
        const locale = runtime.baseLocale;
        const newRequest = cloneRequestWithFallback(request, url);
        /** @type {Set<string>} */
        const messageCalls = new Set();
        return /** @type {Response} */ (await runtime.serverAsyncLocalStorage?.run({ locale, origin, messageCalls }, () => resolve({ locale, request: newRequest })));
    }
    const strategy = runtime.getStrategyForUrl(url.href);
    const decision = await runtime.shouldRedirect({ request, effectiveRequestUrl: url });
    const locale = decision.locale;
    // if the client makes a request to a URL that doesn't match
    // the localizedUrl, redirect the client to the localized URL
    if (request.headers.get("Sec-Fetch-Dest") === "document" &&
        decision.shouldRedirect &&
        decision.redirectUrl) {
        // Create headers object with Vary header if preferredLanguage strategy is used
        /** @type {Record<string, string>} */
        const headers = {};
        if (strategy.includes("preferredLanguage")) {
            headers["Vary"] = "Accept-Language";
        }
        const response = new Response(null, {
            status: 307,
            headers: {
                Location: decision.redirectUrl.href,
                ...headers,
            },
        });
        options?.onRedirect?.(response);
        return response;
    }
    // If the strategy includes "url", we need to de-localize the URL
    // before passing it to the server middleware.
    //
    // The middleware is responsible for mapping a localized URL to the
    // de-localized URL e.g. `/en/about` to `/about`. Otherwise,
    // the server can't render the correct page.
    let newRequest;
    if (strategy.includes("url")) {
        newRequest = cloneRequestWithFallback(request, runtime.deLocalizeUrl(url));
    }
    else {
        newRequest = cloneRequestWithFallback(request, url);
    }
    // the message functions that have been called in this request
    /** @type {Set<string>} */
    const messageCalls = new Set();
    const response = await runtime.serverAsyncLocalStorage?.run({ locale, origin, messageCalls }, () => resolve({ locale, request: newRequest }));
    // Only modify HTML responses
    if (runtime.experimentalMiddlewareLocaleSplitting &&
        response.headers.get("Content-Type")?.includes("html")) {
        const body = await response.text();
        const messages = [];
        // using .values() to avoid polyfilling in older projects. else the following error is thrown
        // Type 'Set<string>' can only be iterated through when using the '--downlevelIteration' flag or with a '--target' of 'es2015' or higher.
        for (const messageCall of Array.from(messageCalls)) {
            const [id, locale] = 
            /** @type {[string, import("./runtime.js").Locale]} */ (messageCall.split(":"));
            messages.push(`${id}: ${compiledBundles[id]?.[locale]}`);
        }
        // Prevent translated content from terminating the inline script tag.
        const escapedMessages = messages
            .join(",")
            .replace(/<\/(script)/gi, "<\\/$1");
        const script = `<script>globalThis.__paraglide = globalThis.__paraglide ?? {}; globalThis.__paraglide.ssr = { ${escapedMessages} }</script>`;
        // Insert the script before the closing head tag
        const newBody = body.replace("</head>", `${script}</head>`);
        // Create a new response with the modified body
        // Clone all headers except Content-Length which will be set automatically
        const newHeaders = new Headers(response.headers);
        newHeaders.delete("Content-Length"); // Let the browser calculate the correct length
        return new Response(newBody, {
            status: response.status,
            statusText: response.statusText,
            headers: newHeaders,
        });
    }
    return response;
}
/**
 * @param {Request} request
 * @param {string | URL | ((request: Request) => string | URL) | undefined} effectiveRequestUrl
 * @returns {URL}
 */
function resolveMiddlewareUrl(request, effectiveRequestUrl) {
    if (typeof effectiveRequestUrl === "function") {
        return new URL(effectiveRequestUrl(request), request.url);
    }
    if (typeof effectiveRequestUrl === "string" || effectiveRequestUrl instanceof URL) {
        return new URL(effectiveRequestUrl, request.url);
    }
    return new URL(request.url);
}
/**
 * Some metaframeworks (NextJS) require a new Request object.
 * https://github.com/opral/inlang-paraglide-js/issues/411
 *
 * However, some frameworks (TanStack Start 1.143+) use custom Request
 * implementations that cannot be cloned with `new Request(request)`.
 * https://github.com/opral/paraglide-js/issues/573
 *
 * Effective request URL overrides behind proxies:
 * https://github.com/opral/paraglide-js/issues/652
 *
 * @param {Request} request
 * @param {string | URL} [url]
 * @returns {Request}
 */
function cloneRequestWithFallback(request, url = request.url) {
    const targetUrl = typeof url === "string" ? url : url.href;
    if (targetUrl === request.url) {
        try {
            // Clone first so building a new Request does not consume the original body stream.
            return new Request(request.clone());
        }
        catch {
            try {
                return new Request(request);
            }
            catch {
                return request;
            }
        }
    }
    try {
        // Clone first so building a new Request does not consume the original body stream.
        return new Request(targetUrl, request.clone());
    }
    catch {
        try {
            return new Request(targetUrl, request);
        }
        catch {
            return request;
        }
    }
}
/**
 * Creates a mock AsyncLocalStorage implementation for environments where
 * native AsyncLocalStorage is not available or disabled.
 *
 * This mock implementation mimics the behavior of the native AsyncLocalStorage
 * but doesn't require the async_hooks module. It's used as a fallback when
 * the runtime does not expose AsyncLocalStorage or when it has been disabled.
 *
 * @returns {import("./runtime.js").ParaglideAsyncLocalStorage}
 */
function createMockAsyncLocalStorage() {
    /** @type {any} */
    let currentStore = undefined;
    return {
        getStore() {
            return currentStore;
        },
        async run(store, callback) {
            currentStore = store;
            try {
                return await callback();
            }
            finally {
                currentStore = undefined;
            }
        },
    };
}
// Used in generated server.js when async local storage is disabled.
void createMockAsyncLocalStorage;
/**
 * The compiled messages for the server middleware.
 *
 * Only populated if `enableMiddlewareOptimizations` is set to `true`.
 *
 * @type {Record<string, Record<import("./runtime.js").Locale, string>>}
 */
const compiledBundles = {};

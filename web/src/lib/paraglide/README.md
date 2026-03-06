# Paraglide JS Compiled Output

> Auto-generated i18n message functions. Import `messages.js` to use translated strings.

Compiled from: `./project.inlang`


## What is this folder?

This folder contains compiled [Paraglide JS](https://github.com/opral/paraglide-js) output. Paraglide JS compiles your translation messages into tree-shakeable JavaScript functions.

## At a glance

Purpose:
- This folder stores compiled i18n message functions.
- Source translations live outside this folder in your inlang project.

Safe to import:
- `messages.js` — all message functions
- `runtime.js` — locale utilities
- `server.js` — server-side middleware

Do not edit:
- All files in this folder are auto-generated.
- Changes will be overwritten on next compilation.

```
paraglide/
├── messages.js      # Message exports (import this)
├── messages/        # Individual message functions
├── runtime.js       # Locale detection & configuration
├── registry.js      # Formatting utilities (plural, number, datetime)
├── server.js        # Server-side middleware
└── .gitignore       # Marks folder as generated
```

## Usage

```js
import * as m from "./paraglide/messages.js";

// Messages are functions that return localized strings
m.hello_world();             // "Hello, World!" (in current locale)
m.greeting({ name: "Sam" }); // "Hello, Sam!"

// Override locale per-call
m.hello_world({}, { locale: "de" });           // "Hallo, Welt!"
m.greeting({ name: "Sam" }, { locale: "de" }); // "Hallo, Sam!"
```

## Runtime API

```js
import { getLocale, getTextDirection, setLocale, locales, baseLocale } from "./paraglide/runtime.js";

getLocale();    // Current locale, e.g., "en"
getTextDirection(); // "ltr" | "rtl" for current locale
setLocale("de"); // Set locale
locales;        // Available locales, e.g., ["en", "de", "fr"]
baseLocale;     // Default locale, e.g., "en"
```

## Strategy

The strategy determines how the current locale is detected and persisted:

- **Cookie**: Stores locale preference in a cookie.
- **URL**: Derives locale from URL patterns (e.g., `/en/about`, `en.example.com`).
- **GlobalVariable**: Uses a global variable (client-side only).
- **BaseLocale**: Always returns the base locale.

Strategies can be combined. The order defines precedence:

```js
await compile({
  project: "./project.inlang",
  outdir: "./src/paraglide",
  strategy: ["url", "cookie", "baseLocale"],
});
```

See the [strategy documentation](https://inlang.com/m/gerre34r/library-inlang-paraglideJs/strategy) for details.

## Key concepts

- **Tree-shakeable**: Each message is a function, enabling [up to 70% smaller i18n bundle sizes](https://inlang.com/m/gerre34r/library-inlang-paraglideJs/benchmark) than traditional i18n libraries.
- **Typesafe**: Full TypeScript/JSDoc support with autocomplete.
- **Variants**: Messages can have variants for pluralization, gender, etc.
- **Fallbacks**: Missing translations fall back to the base locale.

## Links

- [Paraglide JS Documentation](https://inlang.com/m/gerre34r/library-inlang-paraglideJs)
- [Source Repository](https://github.com/opral/paraglide-js)

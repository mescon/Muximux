# Translations

Muximux supports 36 languages out of the box. The interface language is set per-installation in **Settings > General > Language** and syncs across all devices.

## Supported Languages

| Language | Code | Language | Code |
|----------|------|----------|------|
| Arabic | `ar` | Italian | `it` |
| Bengali | `bn` | Japanese | `ja` |
| Bulgarian | `bg` | Latvian | `lv` |
| Cantonese | `yue` | Lithuanian | `lt` |
| Chinese (Simplified) | `zh` | Marathi | `mr` |
| Croatian | `hr` | Norwegian Bokmål | `nb` |
| Czech | `cs` | Dutch | `nl` |
| Danish | `da` | Polish | `pl` |
| English | `en` | Portuguese | `pt` |
| Estonian | `et` | Romanian | `ro` |
| Finnish | `fi` | Russian | `ru` |
| French | `fr` | Serbian (Latin) | `sr` |
| German | `de` | Slovak | `sk` |
| Greek | `el` | Slovenian | `sl` |
| Hindi | `hi` | Spanish | `es` |
| Hungarian | `hu` | Swedish | `sv` |
| Turkish | `tr` | Ukrainian | `uk` |
| Vietnamese | `vi` | Wu Chinese | `wuu` |

English is the base locale and the fallback for any missing translations.

## Changing the Language

1. Open **Settings** (click the gear icon or press `S`).
2. Go to the **General** tab.
3. Select a language from the **Language** dropdown.
4. Click **Save**. The page reloads automatically in the new language.

The setting is stored in `config.yaml` under `server.language` and applies to all users and devices.

## Contributing a Translation

Adding a new language requires no Go code, no Svelte changes, and no CSS changes. The entire process happens in the `web/` directory.

### Prerequisites

- Node.js 18+
- A clone of the Muximux repository

### Step-by-step

**1. Choose a BCP 47 language tag**

Pick the standard tag for your language (e.g., `ko` for Korean, `th` for Thai). Use the shortest ISO 639-1 code where one exists.

**2. Register the locale**

Add your tag to the `locales` array in `web/project.inlang/settings.json`:

```json
{
  "locales": [
    "en", "sv", "uk", ..., "your-tag"
  ]
}
```

**3. Create the message file**

Copy the English source file to your new locale:

```bash
cp web/messages/en.json web/messages/your-tag.json
```

**4. Translate the values**

Open `web/messages/your-tag.json` and translate every value. Rules:

- **Keys stay identical** -- only change the values.
- **Keep `{paramName}` placeholders** exactly as-is. Example: `"Failed to load {appName}"` → `"Kunde inte ladda {appName}"`.
- **Keep HTML tags** (`<b>`, `<code>`, `<br/>`) in place.
- **Keep technical terms** in English: URL, CORS, API, DNS, CIDR, IP, TLS, CSS, iframe, proxy, Docker, Git, WebSocket, SSO, OIDC.
- **Keep product names** untranslated: Muximux, Plex, Jellyfin, Sonarr, Radarr, Dashboard Icons, Lucide, Caddy, and all other app names in the `popularApps_*` section.
- **Keep log level names** in English: Debug, Info, Warning, Error.
- **Keep keyboard labels** as-is: Escape, Enter, esc, ↑↓, ⏎, ⌘, 1-9.
- **Translate contextually** -- these are short UI strings for a dashboard app. Prefer concise, natural phrasing over literal word-for-word translation.

**5. Add a display name**

In `web/src/lib/localeStore.ts`, add an entry to the `localeNames` map with a flag emoji and the language's native name:

```typescript
export const localeNames: Record<string, string> = {
  // ... existing entries ...
  ko: '🇰🇷 한국어',
};
```

**6. If RTL: register the direction**

If your language is right-to-left, add its tag to the `RTL_LOCALES` set in the same file:

```typescript
const RTL_LOCALES = new Set(['ar', 'your-tag']);
```

**7. Build and verify**

```bash
cd web
npx @inlang/paraglide-js compile --project ./project.inlang --outdir ./src/lib/paraglide
npm run build
npx vitest run
```

All tests should pass. Start the dev server and switch to your language in Settings to verify the UI looks correct.

**8. Submit a pull request**

Commit your changes and open a PR. The diff should include:

- `web/messages/your-tag.json` (new file)
- `web/project.inlang/settings.json` (tag added to array)
- `web/src/lib/localeStore.ts` (display name added)
- `web/src/lib/paraglide/` (regenerated output)

### Tips

- The `popularApps_*Desc` keys are short app descriptions like "Stream your media library". Translate the description, but keep the app name (Plex, Jellyfin, etc.) untranslated.
- Some values contain `{count}` with separate singular/plural keys (e.g., `splash_appSingular` / `splash_appPlural`). Translate both forms for your language.
- Test with a long language (like German) to check for text overflow, and with a short one to check for awkward whitespace.
- The `appForm_helpOpenMode` and similar `help*` keys contain HTML. Make sure tags are balanced after translation.

## Technical Details

Muximux uses [Paraglide.js](https://inlang.com/m/gerre34r/library-inlang-paraglideJs) for compile-time i18n. Message functions are generated from the JSON files and tree-shaken per page -- only the active locale's strings are loaded at runtime.

The locale is stored in three places that stay in sync:
- **Server config** (`config.yaml` → `server.language`) -- source of truth, synced across devices.
- **localStorage** (`PARAGLIDE_LOCALE`) -- used by Paraglide on page load before the server config is fetched.
- **`<html lang="..." dir="...">` attributes** -- set on every page load for accessibility and CSS logical properties (RTL support).

When the user changes language in Settings and saves, the config is written to the server, and the page reloads. On reload, Paraglide reads the locale from localStorage and renders the new language immediately, then the server config confirms it.

// safeColor validates a user-supplied colour before it is interpolated into
// an inline `style` attribute. App / group / icon colours can be set by an
// admin or imported from Docker labels (attacker-influenceable), and Svelte
// only HTML-escapes an attribute value -- it does not stop a value like
// `red; background-image: url(https://evil/x)` from injecting extra CSS
// declarations once inside the style string. This returns the colour only
// when it matches a known-safe CSS colour form, and an empty string
// otherwise so the caller falls back to its default.
//
// Accepted forms: #hex (3/4/6/8), rgb()/rgba(), hsl()/hsla(), var(--token),
// and bare CSS keyword names (letters only, e.g. "red", "transparent").
const SAFE_COLOR = new RegExp(
  [
    '^#[0-9a-fA-F]{3,8}$', // #rgb .. #rrggbbaa
    '^rgba?\\([0-9.,%/\\s]+\\)$', // rgb() / rgba()
    '^hsla?\\([0-9.,%/\\sdeg]+\\)$', // hsl() / hsla()
    '^var\\(--[a-zA-Z0-9-]+\\)$', // var(--token)
    '^[a-zA-Z]+$', // keyword: red, transparent, currentColor, ...
  ].join('|'),
);

export function safeColor(color: string | null | undefined, fallback = ''): string {
  if (!color) return fallback;
  const trimmed = color.trim();
  return SAFE_COLOR.test(trimmed) ? trimmed : fallback;
}

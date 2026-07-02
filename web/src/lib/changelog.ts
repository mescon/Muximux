// Renders release-notes markdown to HTML for the About tab's changelog.
//
// The changelog arrives from the remote update check (checkForUpdates),
// so it is untrusted input that ends up in an {@html ...} block. `marked`
// passes raw HTML through and can emit javascript: URLs from markdown
// links, so the parsed output MUST be sanitized before it reaches the
// DOM. DOMPurify strips scripts, event-handler attributes, and dangerous
// URL schemes while leaving ordinary formatting intact.
//
// Both `marked` and `dompurify` are dynamically imported so their weight
// stays out of the Settings chunk until a changelog actually renders.
export async function renderChangelog(markdown: string): Promise<string> {
  if (!markdown) return '';
  const [{ marked }, { default: DOMPurify }] = await Promise.all([
    import('marked'),
    import('dompurify'),
  ]);
  const rawHtml = String(marked.parse(markdown));
  return DOMPurify.sanitize(rawHtml);
}

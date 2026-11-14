#!/usr/bin/python3
"""Per-app favicon resolution (M-357, parent M-237).

Pure resolver: given an app's HTML page, pick its <link rel=icon> href
(relative hrefs resolved against the page URL), preferring the light
variant, else fall back to /favicon.ico, else None (Muximux letter).

Rules (documented in docs/ADDING-A-SERVICE.md):
  1. <link> tags whose rel token list contains "icon" (covers icon and
     shortcut icon), excluding mask-icon (monochrome Safari pinned tab).
  2. Within those, prefer light: no media, or media mentioning "light",
     over media mentioning "dark". Stable order otherwise (first wins).
  3. Next tier: apple-touch-icon (usually a PNG, fine as a tab icon).
  4. Last tier: mask-icon (better than a letter).
  5. No link at all -> /favicon.ico on the same origin.
  6. data: URIs pass through unchanged.
"""
from html.parser import HTMLParser
from urllib.parse import urljoin


class _LinkCollector(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.links = []

    def handle_starttag(self, tag, attrs):
        if tag.lower() != "link":
            return
        d = {}
        for k, v in attrs:
            d[k.lower()] = v if v is not None else ""
        self.links.append(d)


def _rel_tokens(link):
    return link.get("rel", "").lower().split()


def _light_score(link):
    """Lower is better: 0 = light or unconditioned, 2 = dark-only."""
    media = link.get("media", "").lower()
    if "dark" in media and "light" not in media:
        return 2
    return 0


def _tier(link):
    tokens = _rel_tokens(link)
    if "mask-icon" in tokens:
        return 2
    if "apple-touch-icon" in tokens:
        return 1
    if "icon" in tokens:
        return 0
    return 3


def iter_icon_links(html):
    """All <link> tags carrying any icon rel, in document order."""
    collector = _LinkCollector()
    collector.feed(html)
    out = []
    for link in collector.links:
        tokens = _rel_tokens(link)
        if "icon" in tokens or any(t.endswith("-icon") for t in tokens):
            if link.get("href"):
                out.append(link)
    return out


def pick_icon_href(html, page_url):
    """Absolute icon URL from the page's <link rel=icon> tags, or None.

    Relative hrefs (./favicon.svg, favicon.svg, /abs, //host) resolve
    against page_url. data: URIs pass through unchanged.
    """
    links = iter_icon_links(html)
    if not links:
        return None
    ranked = sorted(range(len(links)),
                    key=lambda i: (_tier(links[i]),
                                   _light_score(links[i]), i))
    href = links[ranked[0]]["href"].strip()
    if not href:
        return None
    if href.startswith("data:"):
        return href
    return urljoin(page_url, href)


def favicon_candidates(page_url, html):
    """Ordered fetch chain: picked <link> href, then /favicon.ico."""
    out = []
    picked = pick_icon_href(html, page_url) if html else None
    if picked:
        out.append(picked)
    base = page_url.split("?", 1)[0].split("#", 1)[0]
    if not base.endswith("/"):
        base = base.rsplit("/", 1)[0] + "/"
    ico = urljoin(base, "/favicon.ico")
    if ico not in out:
        out.append(ico)
    return out


EXTENSION_CONTENT_TYPE = {
    ".svg": "image/svg+xml",
    ".png": "image/png",
    ".ico": "image/x-icon",
    ".jpg": "image/jpeg",
    ".jpeg": "image/jpeg",
    ".gif": "image/gif",
    ".webp": "image/webp",
}


def content_type_for(url, upstream):
    """Upstream Content-Type when image-ish, else sniffed from extension."""
    ct = (upstream or "").split(";", 1)[0].strip().lower()
    if ct.startswith("image/"):
        return ct
    if ct in ("application/xml", "text/xml") and url.lower().endswith(".svg"):
        return "image/svg+xml"
    path = url.split("?", 1)[0].split("#", 1)[0].lower()
    for ext, mapped in EXTENSION_CONTENT_TYPE.items():
        if path.endswith(ext):
            return mapped
    return ct or "application/octet-stream"

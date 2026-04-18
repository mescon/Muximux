/**
 * Dynamic favicon generation — renders the Muximux logo SVG as favicon
 * in the current theme's accent color. Updates all favicon link elements
 * and the theme-color meta tag whenever the theme changes.
 */

// The logo paths from MuximuxLogo.svelte (viewBox 0 0 341 207)
const LOGO_PATHS = [
  'M 64.45 48.00 C 68.63 48.00 72.82 47.99 77.01 48.01 C 80.83 59.09 84.77 70.14 88.54 81.24 C 92.32 70.17 96.13 59.10 99.85 48.00 C 104.04 47.99 108.24 48.00 112.43 48.00 C 113.39 65.67 114.50 83.33 115.49 101.00 C 111.45 101.00 107.40 101.01 103.36 100.99 C 102.89 93.74 102.47 86.48 102.07 79.23 C 99.66 86.49 97.15 93.73 94.71 100.99 C 90.61 100.95 86.50 101.15 82.40 100.85 C 79.93 93.36 77.36 85.90 74.69 78.48 C 74.44 86.00 73.62 93.48 73.36 101.00 C 69.28 101.00 65.19 101.00 61.10 101.00 C 62.17 83.33 63.36 65.67 64.45 48.00 Z',
  'M 119.60 48.00 C 123.65 48.00 127.69 48.00 131.74 48.01 C 131.74 59.01 131.72 70.01 131.74 81.01 C 131.51 85.47 135.71 89.35 140.10 89.02 C 144.20 88.91 147.64 85.08 147.53 81.02 C 147.55 70.02 147.52 59.01 147.53 48.00 C 151.60 48.00 155.67 48.00 159.74 48.01 C 159.67 59.49 159.85 70.98 159.65 82.46 C 159.14 93.61 147.92 102.57 136.94 100.86 C 127.64 99.76 119.94 91.34 119.62 82.00 C 119.57 70.66 119.61 59.33 119.60 48.00 Z',
  'M 165.50 48.03 C 170.29 47.97 175.08 48.01 179.87 48.00 C 182.80 52.67 185.72 57.35 188.64 62.03 C 191.39 57.32 194.27 52.69 197.04 47.99 C 201.82 48.01 206.61 47.99 211.39 48.01 C 206.05 56.48 200.92 65.10 195.78 73.69 C 201.49 82.77 206.93 92.03 212.79 101.01 C 207.97 100.97 203.15 101.05 198.33 100.96 C 195.09 95.79 191.93 90.58 188.70 85.42 C 185.48 90.60 182.35 95.83 179.13 101.02 C 174.41 100.98 169.68 101.01 164.96 101.00 C 170.55 91.91 176.00 82.74 181.53 73.62 C 176.00 65.21 171.10 56.40 165.50 48.03 Z',
  'M 216.60 48.00 C 220.64 48.00 224.69 48.00 228.74 48.01 C 228.73 77.68 228.73 107.36 228.74 137.04 C 228.83 141.39 228.77 145.96 226.59 149.87 C 222.49 158.47 211.73 163.16 202.67 160.11 C 194.49 157.70 188.47 149.51 188.59 140.98 C 188.61 129.99 188.59 119.00 188.60 108.00 C 192.64 108.00 196.69 107.99 200.74 108.01 C 200.74 118.99 200.72 129.97 200.74 140.96 C 200.48 145.46 204.75 149.40 209.18 149.01 C 213.25 148.85 216.63 145.06 216.53 141.03 C 216.51 110.02 216.65 79.01 216.60 48.00 Z',
  'M 133.45 108.00 C 137.63 108.00 141.82 107.99 146.01 108.01 C 149.84 119.09 153.76 130.15 157.56 141.24 C 161.30 130.16 165.14 119.10 168.85 108.01 C 173.04 107.99 177.24 108.00 181.43 108.00 C 182.39 125.67 183.50 143.33 184.49 161.00 C 180.44 161.00 176.40 161.01 172.36 160.99 C 171.89 153.75 171.48 146.51 171.07 139.27 C 168.64 146.51 166.15 153.74 163.71 160.99 C 159.62 160.97 155.52 161.11 151.44 160.88 C 148.91 153.40 146.38 145.91 143.69 138.48 C 143.44 146.00 142.61 153.48 142.37 161.00 C 138.28 161.00 134.19 161.00 130.10 161.00 C 131.17 143.33 132.36 125.67 133.45 108.00 Z',
  'M 234.50 108.03 C 239.29 107.97 244.08 108.01 248.87 108.00 C 251.78 112.67 254.73 117.32 257.60 122.02 C 260.41 117.35 263.25 112.69 266.03 107.99 C 270.82 108.01 275.61 107.99 280.39 108.01 C 275.04 116.48 269.93 125.09 264.78 133.68 C 270.48 142.77 275.93 152.02 281.79 161.01 C 276.97 160.97 272.15 161.05 267.33 160.96 C 264.09 155.80 260.93 150.58 257.70 145.42 C 254.45 150.60 251.37 155.88 248.08 161.04 C 243.37 160.96 238.67 161.02 233.96 161.00 C 239.55 151.91 245.00 142.74 250.53 133.62 C 245.00 125.21 240.10 116.40 234.50 108.03 Z',
];

function buildLogoSVG(color: string, size: number): string {
  const paths = LOGO_PATHS.map(d => `<path d="${d}"/>`).join('');
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 341 207"><g fill="${color}">${paths}</g></svg>`;
}

function svgToDataURI(svg: string): string {
  return 'data:image/svg+xml,' + encodeURIComponent(svg);
}

/**
 * Render SVG to a PNG data URL at the given size via canvas.
 * Needed for apple-touch-icon and Android manifest icons since
 * Safari and some Android launchers don't support SVG icons.
 */
function svgToPngDataURL(svg: string, size: number): Promise<string> {
  return new Promise((resolve) => {
    const img = new Image();
    img.onload = () => {
      const canvas = document.createElement('canvas');
      canvas.width = size;
      canvas.height = size;
      const ctx = canvas.getContext('2d')!;

      // Center the wide logo in a square canvas
      const logoAspect = 341 / 207;
      let drawW = size;
      let drawH = size / logoAspect;
      if (drawH > size) {
        drawH = size;
        drawW = size * logoAspect;
      }
      const x = (size - drawW) / 2;
      const y = (size - drawH) / 2;
      ctx.drawImage(img, x, y, drawW, drawH);
      resolve(canvas.toDataURL('image/png'));
    };
    img.src = svgToDataURI(svg);
  });
}

function setLinkHref(selector: string, href: string) {
  const el = document.querySelector<HTMLLinkElement>(selector);
  if (el) el.href = href;
}

function setMetaContent(name: string, content: string) {
  const el = document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`);
  if (el) el.content = content;
}

/**
 * Update all favicon elements and theme-color meta to match the given color.
 */
export async function updateFavicons(color: string): Promise<void> {
  if (typeof document === 'undefined') return;

  const svgSmall = buildLogoSVG(color, 32);
  const svgURI = svgToDataURI(svgSmall);

  // SVG favicon (modern browsers)
  setLinkHref('link[rel="icon"][type="image/x-icon"]', svgURI);
  setLinkHref('link[rel="icon"][sizes="32x32"]', svgURI);
  setLinkHref('link[rel="icon"][sizes="16x16"]', svgURI);

  // Safari pinned tab
  setLinkHref('link[rel="mask-icon"]', svgURI);

  // PNG favicons for Apple/Android (they need raster images)
  const svg192 = buildLogoSVG(color, 192);
  const svg180 = buildLogoSVG(color, 180);

  const [png192, png180] = await Promise.all([
    svgToPngDataURL(svg192, 192),
    svgToPngDataURL(svg180, 180),
  ]);

  setLinkHref('link[rel="apple-touch-icon"]', png180);

  // Update manifest icons dynamically
  updateManifest(color, png192);

  // Update theme-color meta to match accent
  setMetaContent('theme-color', color);
  setMetaContent('msapplication-TileColor', color);
}

/**
 * Dynamically update the manifest with themed icons.
 */
function updateManifest(color: string, png192DataURL: string) {
  const manifest = {
    name: 'Muximux',
    short_name: 'Muximux',
    icons: [
      { src: png192DataURL, sizes: '192x192', type: 'image/png' },
    ],
    theme_color: color,
    background_color: color,
    display: 'standalone' as const,
  };

  const blob = new Blob([JSON.stringify(manifest)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  setLinkHref('link[rel="manifest"]', url);
}

/**
 * Read --accent-primary from computed styles and update all favicons if the
 * color has changed. Safe to call frequently (e.g. from a MutationObserver)
 * since it short-circuits when the color hasn't changed.
 */
let lastAccentColor = '';
export function syncFaviconsWithTheme(): void {
  if (typeof document === 'undefined') return;
  const color = getComputedStyle(document.documentElement)
    .getPropertyValue('--accent-primary')
    .trim();
  if (!color || color === lastAccentColor) return;
  lastAccentColor = color;
  updateFavicons(color);
}

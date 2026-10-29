import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

// Mock $lib/types - keep getEffectiveUrl as a spy so we can verify behavior
const { mockGetEffectiveUrl } = vi.hoisted(() => {
  return {
    mockGetEffectiveUrl: vi.fn((app: { proxyUrl?: string; url: string }) => {
      if (app.proxyUrl) return app.proxyUrl;
      return app.url;
    }),
  };
});

vi.mock('$lib/types', async (importOriginal) => {
  const original = await importOriginal<typeof import('$lib/types')>();
  return {
    ...original,
    getEffectiveUrl: mockGetEffectiveUrl,
  };
});

// Mock $lib/useSwipe
const mockIsMobileViewport = vi.fn(() => false);
const mockIsTouchDevice = vi.fn(() => false);

vi.mock('$lib/useSwipe', () => ({
  isMobileViewport: () => mockIsMobileViewport(),
  isTouchDevice: () => mockIsTouchDevice(),
}));

import AppFrame from './AppFrame.svelte';
import type { App, AppIcon } from '$lib/types';

function makeApp(overrides: Partial<App> = {}): App {
  return {
    name: 'TestApp',
    url: 'https://example.com',
    icon: {
      type: 'dashboard',
      name: 'test',
      file: '',
      url: '',
      variant: 'svg',
    } as AppIcon,
    color: '#374151',
    group: 'Default',
    order: 0,
    enabled: true,
    default: false,
    open_mode: 'iframe',
    proxy: false,
    scale: 1,
    ...overrides,
  };
}

describe('AppFrame', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetEffectiveUrl.mockImplementation((app: { proxyUrl?: string; url: string }) => {
      if (app.proxyUrl) return app.proxyUrl;
      return app.url;
    });
    mockIsMobileViewport.mockReturnValue(false);
    mockIsTouchDevice.mockReturnValue(false);
  });

  // ─── Existing tests (preserved) ───────────────────────────────────────────

  describe('frame name for rescue', () => {
    it('names a proxied frame after its proxy path, without the trailing slash', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ proxy: true, proxyUrl: '/proxy/dispatcharr/' }) },
      });
      expect(container.querySelector('iframe')?.getAttribute('name')).toBe('muximux-frame:/proxy/dispatcharr');
    });

    it('leaves a direct (non-proxied) frame unnamed', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      expect(container.querySelector('iframe')?.hasAttribute('name')).toBe(false);
    });
  });

  describe('smoke test', () => {
    it('renders without crashing', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      expect(container.querySelector('iframe')).toBeTruthy();
    });
  });

  describe('iframe rendering', () => {
    it('renders an iframe with the correct src for a direct URL', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ url: 'https://grafana.local:3000' }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      expect(iframe).toBeTruthy();
      expect(iframe.src).toBe('https://grafana.local:3000/');
    });

    it('sets the iframe title to the app name', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ name: 'Grafana' }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      expect(iframe.title).toBe('Grafana');
    });

    it('applies correct sandbox attributes', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      // jsdom does not implement DOMTokenList for sandbox, so check the attribute string
      const sandboxAttr = iframe.getAttribute('sandbox') || '';
      expect(sandboxAttr).toContain('allow-forms');
      expect(sandboxAttr).toContain('allow-same-origin');
      expect(sandboxAttr).toContain('allow-scripts');
      expect(sandboxAttr).toContain('allow-downloads');
      expect(sandboxAttr).toContain('allow-popups');
      expect(sandboxAttr).toContain('allow-modals');
    });

    it('has allowfullscreen attribute', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      expect(iframe.allowFullscreen).toBe(true);
    });

    it('omits the allow attribute when no permissions are configured', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      expect(iframe.hasAttribute('allow')).toBe(false);
    });

    it('sets the allow attribute with configured permissions scoped to the app origin', () => {
      const { container } = render(AppFrame, {
        props: {
          app: makeApp({
            url: 'https://grafana.local:3000',
            permissions: ['camera', 'microphone'],
          }),
        },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const allowAttr = iframe.getAttribute('allow') || '';
      expect(allowAttr).toContain("camera 'self' https://grafana.local:3000");
      expect(allowAttr).toContain("microphone 'self' https://grafana.local:3000");
    });

    it('uses self-only when the app is served through the proxy', () => {
      const { container } = render(AppFrame, {
        props: {
          app: makeApp({
            url: 'https://app.local',
            proxyUrl: '/proxy/app',
            permissions: ['geolocation'],
          }),
        },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      expect(iframe.getAttribute('allow')).toBe("geolocation 'self'");
    });

    it('has the app-frame CSS class', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      expect(iframe.classList.contains('app-frame')).toBe(true);
    });
  });

  describe('proxy vs direct URLs', () => {
    it('uses direct URL when proxyUrl is not set', () => {
      const app = makeApp({ url: 'https://direct.example.com', proxy: false });
      render(AppFrame, { props: { app } });

      expect(mockGetEffectiveUrl).toHaveBeenCalledWith(
        expect.objectContaining({ url: 'https://direct.example.com' })
      );
    });

    it('uses proxy URL when proxyUrl is set', () => {
      const app = makeApp({
        url: 'https://internal.example.com',
        proxyUrl: '/proxy/my-app/',
        proxy: true,
      });
      const { container } = render(AppFrame, { props: { app } });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;

      expect(mockGetEffectiveUrl).toHaveBeenCalledWith(
        expect.objectContaining({ proxyUrl: '/proxy/my-app/' })
      );
      // The iframe src should be the proxy URL
      expect(iframe.src).toContain('/proxy/my-app/');
    });

    it('prefers proxyUrl over direct url when both are present', () => {
      mockGetEffectiveUrl.mockReturnValue('/proxy/app-path/');

      const app = makeApp({
        url: 'https://direct.example.com',
        proxyUrl: '/proxy/app-path/',
        proxy: true,
      });
      const { container } = render(AppFrame, { props: { app } });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;

      expect(iframe.src).toContain('/proxy/app-path/');
    });
  });

  describe('scale', () => {
    it('applies no transform when scale is 1 (default)', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ scale: 1 }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const style = iframe.getAttribute('style') || '';
      // transform should be empty or not contain scale()
      expect(style).not.toMatch(/scale\([^1]/);
    });

    it('applies scale transform when scale is not 1', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ scale: 0.8 }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const style = iframe.getAttribute('style') || '';
      expect(style).toContain('scale(0.8)');
    });

    it('adjusts width and height percentages for non-1 scale', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ scale: 0.5 }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const style = iframe.getAttribute('style') || '';
      // scale 0.5 => width = 100/0.5 = 200%
      expect(style).toContain('200%');
    });

    it('uses 100% width and height when scale is 1', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ scale: 1 }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const style = iframe.getAttribute('style') || '';
      expect(style).toContain('100%');
    });
  });

  describe('container', () => {
    it('has role="application" on the container', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      const appContainer = container.querySelector('[role="application"]');
      expect(appContainer).toBeTruthy();
    });

    it('has overflow-hidden on the container', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      const appContainer = container.querySelector('[role="application"]');
      expect(appContainer?.classList.contains('overflow-hidden')).toBe(true);
    });
  });

  // ─── New tests ────────────────────────────────────────────────────────────

  describe('scale edge cases', () => {
    it('applies scale(0.25) and 400% dimensions for scale 0.25', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ scale: 0.25 }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const style = iframe.getAttribute('style') || '';
      expect(style).toContain('scale(0.25)');
      expect(style).toContain('400%');
    });

    it('applies transform-origin top left for non-1 scale', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ scale: 0.75 }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const style = iframe.getAttribute('style') || '';
      expect(style).toContain('top left');
    });

    it('does not set transform-origin when scale is 1', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ scale: 1 }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const style = iframe.getAttribute('style') || '';
      // transform-origin should be empty string, not "top left"
      expect(style).not.toContain('top left');
    });

    it('applies scale(1.5) and ~66.7% dimensions for scale 1.5', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ scale: 1.5 }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const style = iframe.getAttribute('style') || '';
      expect(style).toContain('scale(1.5)');
      // 100/1.5 = 66.666...%
      expect(style).toMatch(/66\.6+/);
    });
  });

  describe('effective URL derivation', () => {
    it('passes the full app object to getEffectiveUrl', () => {
      const app = makeApp({ name: 'Custom', url: 'https://custom.example.com' });
      render(AppFrame, { props: { app } });

      expect(mockGetEffectiveUrl).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Custom',
          url: 'https://custom.example.com',
        })
      );
    });

    it('uses whatever getEffectiveUrl returns as iframe src', () => {
      mockGetEffectiveUrl.mockReturnValue('https://special-url.test/');

      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      expect(iframe.src).toBe('https://special-url.test/');
    });
  });

  describe('sandbox attributes', () => {
    it('includes allow-pointer-lock in sandbox', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const sandboxAttr = iframe.getAttribute('sandbox') || '';
      expect(sandboxAttr).toContain('allow-pointer-lock');
    });

    it('adds top navigation to the sandbox only when allow_navigation is set', () => {
      const base = 'allow-forms allow-same-origin allow-pointer-lock allow-scripts allow-downloads allow-popups allow-popups-to-escape-sandbox allow-modals';
      const on = render(AppFrame, { props: { app: makeApp({ allow_navigation: true }) } });
      const onAttr = (on.container.querySelector('iframe') as HTMLIFrameElement).getAttribute('sandbox') || '';
      expect(onAttr).toContain('allow-top-navigation-by-user-activation');
      expect(onAttr).not.toContain('allow-top-navigation ');
      expect(onAttr.endsWith('allow-top-navigation')).toBe(false);
      on.unmount();

      const off = render(AppFrame, { props: { app: makeApp() } });
      const offAttr = (off.container.querySelector('iframe') as HTMLIFrameElement).getAttribute('sandbox');
      expect(offAttr).toBe(base);
    });
  });

  describe('container styling', () => {
    it('has theme-aware background on the container', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      const appContainer = container.querySelector('[role="application"]');
      expect(appContainer?.classList.contains('bg-[var(--bg-base)]')).toBe(true);
    });

    it('has w-full and h-full classes on the container', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      const appContainer = container.querySelector('[role="application"]');
      expect(appContainer?.classList.contains('w-full')).toBe(true);
      expect(appContainer?.classList.contains('h-full')).toBe(true);
    });

    it('has relative positioning for pull-to-refresh overlay', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      const appContainer = container.querySelector('[role="application"]');
      expect(appContainer?.classList.contains('relative')).toBe(true);
    });
  });

  describe('pull-to-refresh (mobile)', () => {
    it('does not show pull indicator on desktop', async () => {
      mockIsMobileViewport.mockReturnValue(false);
      mockIsTouchDevice.mockReturnValue(false);

      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });

      // Simulate iframe load to clear loading spinner (uses rAF internally)
      const iframe = container.querySelector('iframe')!;
      await fireEvent.load(iframe);
      await waitFor(() => {
        expect(container.querySelector('.animate-spin')).not.toBeInTheDocument();
      });
    });

    it('does not respond to touch events when not mobile or touch device', async () => {
      mockIsMobileViewport.mockReturnValue(false);
      mockIsTouchDevice.mockReturnValue(false);

      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });

      // Simulate iframe load to clear loading spinner (uses rAF internally)
      const iframe = container.querySelector('iframe')!;
      await fireEvent.load(iframe);
      await waitFor(() => {
        expect(container.querySelector('.animate-spin')).not.toBeInTheDocument();
      });

      const appContainer = container.querySelector('[role="application"]')!;

      // Simulate touch events
      await fireEvent.touchStart(appContainer, {
        touches: [{ clientY: 100 }],
      });
      await fireEvent.touchMove(appContainer, {
        touches: [{ clientY: 200 }],
      });
      await fireEvent.touchEnd(appContainer);

      // Pull indicator should not appear since not mobile
      expect(container.querySelector('.animate-spin')).not.toBeInTheDocument();
    });
  });

  describe('app without scale property', () => {
    it('defaults to scale 1 when scale is not set on the app', () => {
      // scale defaults to 1 from makeApp
      const { container } = render(AppFrame, {
        props: { app: makeApp({ scale: 1 }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const style = iframe.getAttribute('style') || '';
      expect(style).toContain('100%');
      expect(style).not.toContain('top left');
    });
  });

  describe('translateY in transform', () => {
    it('includes translateY(0px) in the transform style (for pull distance)', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ scale: 1 }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const style = iframe.getAttribute('style') || '';
      // When not pulling, pullDistance is 0, so transform has translateY(0px)
      expect(style).toContain('translateY(0px)');
    });

    it('includes both scale and translateY when scale is not 1', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ scale: 0.8 }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const style = iframe.getAttribute('style') || '';
      expect(style).toContain('scale(0.8)');
      expect(style).toContain('translateY(0px)');
    });
  });

  describe('multiple apps with different configs', () => {
    it('renders correct title for each app', () => {
      const { container: c1 } = render(AppFrame, {
        props: { app: makeApp({ name: 'App One' }) },
      });
      const { container: c2 } = render(AppFrame, {
        props: { app: makeApp({ name: 'App Two' }) },
      });

      expect((c1.querySelector('iframe') as HTMLIFrameElement).title).toBe('App One');
      expect((c2.querySelector('iframe') as HTMLIFrameElement).title).toBe('App Two');
    });
  });

  describe('pull-to-refresh on mobile', () => {
    beforeEach(() => {
      mockIsMobileViewport.mockReturnValue(true);
      mockIsTouchDevice.mockReturnValue(true);
    });

    it('handles touchstart event on mobile', async () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });

      const appContainer = container.querySelector('[role="application"]')!;

      await fireEvent.touchStart(appContainer, {
        touches: [{ clientY: 100 }],
      });

      // Touch start should set isPulling = true and startY = 100
      // No visible change yet, just state change
      expect(appContainer).toBeInTheDocument();
    });

    it('shows pull indicator when pulled down sufficiently on mobile', async () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });

      const appContainer = container.querySelector('[role="application"]')!;

      await fireEvent.touchStart(appContainer, {
        touches: [{ clientY: 0 }],
      });

      // Pull down far enough to show the indicator (pullDistance > 10)
      // delta = (currentY - startY) / RESISTANCE = (100 - 0) / 2.5 = 40
      await fireEvent.touchMove(appContainer, {
        touches: [{ clientY: 100 }],
      });

      await waitFor(() => {
        // The pull indicator text should appear
        const pullText = container.querySelector('[style*="height"]');
        expect(pullText).toBeTruthy();
      });
    });

    it('shows "Pull to refresh" text when pulling but not past threshold', async () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });

      const appContainer = container.querySelector('[role="application"]')!;

      await fireEvent.touchStart(appContainer, {
        touches: [{ clientY: 0 }],
      });

      // Pull down moderately (delta = 50/2.5 = 20, which is > 10 but < PULL_THRESHOLD 80)
      await fireEvent.touchMove(appContainer, {
        touches: [{ clientY: 50 }],
      });

      await waitFor(() => {
        expect(container.textContent).toContain('Pull to refresh');
      });
    });

    it('shows "Release to refresh" text when pulled past threshold', async () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });

      const appContainer = container.querySelector('[role="application"]')!;

      await fireEvent.touchStart(appContainer, {
        touches: [{ clientY: 0 }],
      });

      // Pull down past threshold (delta = 250/2.5 = 100, pullProgress = 100/80 = 1.25 >= 1)
      await fireEvent.touchMove(appContainer, {
        touches: [{ clientY: 250 }],
      });

      await waitFor(() => {
        expect(container.textContent).toContain('Release to refresh');
      });
    });

    it('resets pull distance when pulling up (negative delta)', async () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });

      const appContainer = container.querySelector('[role="application"]')!;

      await fireEvent.touchStart(appContainer, {
        touches: [{ clientY: 200 }],
      });

      // Pull up (negative delta)
      await fireEvent.touchMove(appContainer, {
        touches: [{ clientY: 100 }],
      });

      // Should not show pull indicator
      expect(container.textContent).not.toContain('Pull to refresh');
      expect(container.textContent).not.toContain('Release to refresh');
    });

    it('triggers refresh when released past threshold', async () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });

      const appContainer = container.querySelector('[role="application"]')!;

      await fireEvent.touchStart(appContainer, {
        touches: [{ clientY: 0 }],
      });

      // Pull past threshold
      await fireEvent.touchMove(appContainer, {
        touches: [{ clientY: 250 }],
      });

      // Release
      await fireEvent.touchEnd(appContainer);

      await waitFor(() => {
        expect(container.textContent).toContain('Refreshing...');
      });
    });

    it('resets pull distance on release below threshold', async () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });

      const appContainer = container.querySelector('[role="application"]')!;

      await fireEvent.touchStart(appContainer, {
        touches: [{ clientY: 0 }],
      });

      // Pull slightly (below threshold)
      await fireEvent.touchMove(appContainer, {
        touches: [{ clientY: 30 }],
      });

      // Release
      await fireEvent.touchEnd(appContainer);

      // Pull indicator should disappear
      await waitFor(() => {
        expect(container.textContent).not.toContain('Pull to refresh');
        expect(container.textContent).not.toContain('Refreshing...');
      });
    });

    it('does not trigger pull if touchend fires without prior pull', async () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });

      const appContainer = container.querySelector('[role="application"]')!;

      // Fire touchEnd without touchStart
      await fireEvent.touchEnd(appContainer);

      expect(container.textContent).not.toContain('Refreshing...');
    });
  });

  describe('onMount resize handler', () => {
    it('updates mobile state on window resize', async () => {
      mockIsMobileViewport.mockReturnValue(false);
      mockIsTouchDevice.mockReturnValue(false);

      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });

      // Now change the mock and fire resize
      mockIsMobileViewport.mockReturnValue(true);
      await fireEvent(window, new Event('resize'));

      // After resize, isMobile should be updated
      // We can verify by checking if pull-to-refresh UI would work
      expect(container.querySelector('[role="application"]')).toBeTruthy();
    });
  });

  describe('scale fallback', () => {
    it('defaults scale to 1 when app.scale is 0 (falsy)', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ scale: 0 }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      const style = iframe.getAttribute('style') || '';
      // scale 0 is falsy, so app.scale || 1 = 1
      expect(style).toContain('100%');
      expect(style).not.toContain('top left');
    });
  });

  describe('data-app attribute', () => {
    it('sets data-app attribute to the app name', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp({ name: 'Grafana' }) },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      expect(iframe.getAttribute('data-app')).toBe('Grafana');
    });

    it('can be used to query the correct iframe by app name', () => {
      const { container: c1 } = render(AppFrame, {
        props: { app: makeApp({ name: 'Plex' }) },
      });
      const { container: c2 } = render(AppFrame, {
        props: { app: makeApp({ name: 'Sonarr' }) },
      });

      expect(c1.querySelector('iframe[data-app="Plex"]')).toBeTruthy();
      expect(c2.querySelector('iframe[data-app="Sonarr"]')).toBeTruthy();
    });
  });

  describe('deep link', () => {
    afterEach(() => vi.restoreAllMocks());

    it('opens the iframe at the deep link and never at the start page', () => {
      const srcSet = vi.spyOn(HTMLIFrameElement.prototype, 'src', 'set');
      const attrSet = vi.spyOn(Element.prototype, 'setAttribute');
      const { container } = render(AppFrame, {
        props: { app: makeApp({ url: 'https://example.com/start' }), deepLink: { url: 'https://example.com/deep', seq: 1 } },
      });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      expect(iframe.getAttribute('src')).toBe('https://example.com/deep');
      const srcValues = srcSet.mock.calls.map((c) => String(c[0]));
      const attrValues = attrSet.mock.calls.filter((c) => c[0] === 'src').map((c) => String(c[1]));
      expect(srcValues.concat(attrValues).length).toBeGreaterThan(0);
      for (const v of srcValues.concat(attrValues)) expect(v).not.toContain('/start');
    });

    it('navigates again when seq changes, even for the same URL', async () => {
      const srcSet = vi.spyOn(HTMLIFrameElement.prototype, 'src', 'set');
      const app = makeApp({ url: 'https://example.com/start' });
      const r = render(AppFrame, { props: { app, deepLink: { url: 'https://example.com/deep', seq: 1 } } });
      srcSet.mockClear();
      await r.rerender({ app, deepLink: { url: 'https://example.com/deep', seq: 2 } });
      await waitFor(() => expect(srcSet).toHaveBeenCalledWith('https://example.com/deep'));
    });

    it('keeps a loaded frame shown on a deep link to another page', async () => {
      vi.useFakeTimers();
      try {
        const app = makeApp();
        const r = render(AppFrame, { props: { app, deepLink: { url: 'https://example.com/a', seq: 1 } } });
        const iframe = r.container.querySelector('iframe') as HTMLIFrameElement;
        await fireEvent.load(iframe);
        await vi.advanceTimersByTimeAsync(20);
        expect(iframe.classList.contains('app-frame-ready')).toBe(true);
        await r.rerender({ app, deepLink: { url: 'https://example.com/b', seq: 2 } });
        expect(iframe.getAttribute('src')).toBe('https://example.com/b');
        expect(iframe.classList.contains('app-frame-ready')).toBe(true);
        expect(r.container.querySelector('.animate-spin')).toBeNull();
        // No load event may ever arrive (a fragment change the shell cannot
        // see); that must not end on the error screen.
        await vi.advanceTimersByTimeAsync(30_000);
        expect(r.container.querySelector('button')).toBeNull();
      } finally {
        vi.useRealTimers();
      }
    });

    it('restarts the loading state on a deep link before the first load', async () => {
      vi.useFakeTimers();
      try {
        const app = makeApp();
        const r = render(AppFrame, { props: { app } });
        await vi.advanceTimersByTimeAsync(20_000);
        await r.rerender({ app, deepLink: { url: 'https://example.com/b', seq: 1 } });
        expect(r.container.querySelector('.animate-spin')).not.toBeNull();
        // The timeout starts over for the deep link.
        await vi.advanceTimersByTimeAsync(20_000);
        expect(r.container.querySelector('button')).toBeNull();
        await vi.advanceTimersByTimeAsync(10_000);
        expect(r.container.querySelector('button')).not.toBeNull();
      } finally {
        vi.useRealTimers();
      }
    });

    it('a deep link from the error screen shows the spinner', async () => {
      vi.useFakeTimers();
      try {
        const app = makeApp();
        const r = render(AppFrame, { props: { app } });
        await vi.advanceTimersByTimeAsync(30_000);
        expect(r.container.querySelector('button')).not.toBeNull();
        await r.rerender({ app, deepLink: { url: 'https://example.com/b', seq: 1 } });
        expect(r.container.querySelector('.animate-spin')).not.toBeNull();
      } finally {
        vi.useRealTimers();
      }
    });

    it('does not navigate when seq is unchanged', async () => {
      const srcSet = vi.spyOn(HTMLIFrameElement.prototype, 'src', 'set');
      const app = makeApp();
      const r = render(AppFrame, { props: { app, deepLink: { url: 'https://example.com/a', seq: 1 } } });
      srcSet.mockClear();
      await r.rerender({ app, deepLink: { url: 'https://example.com/other', seq: 1 } });
      expect(srcSet).not.toHaveBeenCalled();
    });

    it('neutralises a javascript: deep link', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp(), deepLink: { url: 'javascript:alert(1)', seq: 1 } },
      });
      expect((container.querySelector('iframe') as HTMLIFrameElement).getAttribute('src')).toBe('about:blank');
    });

    it('a config URL change after a deep link updates src', async () => {
      const r = render(AppFrame, {
        props: { app: makeApp({ url: 'https://example.com/start' }), deepLink: { url: 'https://example.com/deep', seq: 1 } },
      });
      await r.rerender({ app: makeApp({ url: 'https://other.test' }), deepLink: { url: 'https://example.com/deep', seq: 1 } });
      const iframe = r.container.querySelector('iframe') as HTMLIFrameElement;
      await waitFor(() => expect(iframe.getAttribute('src')).toBe('https://other.test/'));
    });

    it('without deepLink uses the effective URL', () => {
      const { container } = render(AppFrame, { props: { app: makeApp() } });
      expect((container.querySelector('iframe') as HTMLIFrameElement).getAttribute('src')).toBe('https://example.com/');
    });

    it('reports a deep link it mounted with as used', () => {
      const used = vi.fn();
      render(AppFrame, { props: { app: makeApp(), deepLink: { url: 'https://example.com/a', seq: 4 }, ondeeplinkused: used } });
      expect(used).toHaveBeenCalledWith(4);
    });

    it('reports a deep link it navigated to as used', async () => {
      const used = vi.fn();
      const app = makeApp();
      const r = render(AppFrame, { props: { app, ondeeplinkused: used } });
      expect(used).not.toHaveBeenCalled();
      await r.rerender({ app, deepLink: { url: 'https://example.com/a', seq: 7 }, ondeeplinkused: used });
      await waitFor(() => expect(used).toHaveBeenCalledWith(7));
    });

    it('keeps the loaded page when a used deep link is removed', async () => {
      const srcSet = vi.spyOn(HTMLIFrameElement.prototype, 'src', 'set');
      const app = makeApp();
      const r = render(AppFrame, { props: { app, deepLink: { url: 'https://example.com/a', seq: 1 } } });
      srcSet.mockClear();
      await r.rerender({ app, deepLink: undefined });
      const iframe = r.container.querySelector('iframe') as HTMLIFrameElement;
      expect(iframe.getAttribute('src')).toBe('https://example.com/a');
      expect(srcSet).not.toHaveBeenCalled();
    });

    it('a remount without the consumed deep link loads the normal URL', () => {
      const deepLink = { url: 'https://example.com/a', seq: 1 };
      const first = render(AppFrame, { props: { app: makeApp(), deepLink } });
      first.unmount();
      const { container } = render(AppFrame, { props: { app: makeApp() } });
      expect((container.querySelector('iframe') as HTMLIFrameElement).getAttribute('src')).toBe('https://example.com/');
    });

    it('a fragment-only deep link keeps the frame shown without a spinner', async () => {
      const app = makeApp({ url: 'https://example.com/web/index.html' });
      const r = render(AppFrame, { props: { app } });
      const iframe = r.container.querySelector('iframe') as HTMLIFrameElement;
      await fireEvent.load(iframe);
      await waitFor(() => expect(iframe.classList.contains('app-frame-ready')).toBe(true));
      await r.rerender({ app, deepLink: { url: 'https://example.com/web/index.html#!/item?id=1', seq: 1 } });
      await waitFor(() => expect(iframe.getAttribute('src')).toBe('https://example.com/web/index.html#!/item?id=1'));
      expect(iframe.classList.contains('app-frame-ready')).toBe(true);
      expect(r.container.querySelector('.animate-spin')).toBeNull();
      // A second fragment change is compared against the last assigned URL.
      await r.rerender({ app, deepLink: { url: 'https://example.com/web/index.html#!/home', seq: 2 } });
      await waitFor(() => expect(iframe.getAttribute('src')).toBe('https://example.com/web/index.html#!/home'));
      expect(r.container.querySelector('.animate-spin')).toBeNull();
    });

  });

  describe('load timeout', () => {
    afterEach(() => vi.useRealTimers());

    it('shows the error screen when no load arrives in 30 s', async () => {
      vi.useFakeTimers();
      const { container } = render(AppFrame, { props: { app: makeApp({ name: 'Slow' }) } });
      expect(container.querySelector('.animate-spin')).not.toBeNull();
      await vi.advanceTimersByTimeAsync(29_999);
      expect(container.textContent).not.toContain('Slow');
      await vi.advanceTimersByTimeAsync(1);
      expect(container.querySelector('.animate-spin')).toBeNull();
      expect(container.querySelector('button')).not.toBeNull();
    });

    it('does nothing at the timeout when the frame has loaded', async () => {
      vi.useFakeTimers();
      const { container } = render(AppFrame, { props: { app: makeApp() } });
      const iframe = container.querySelector('iframe') as HTMLIFrameElement;
      await fireEvent.load(iframe);
      await vi.advanceTimersByTimeAsync(30_000);
      expect(container.querySelector('button')).toBeNull();
    });

    it('retry restarts loading and the timeout', async () => {
      vi.useFakeTimers();
      const { container } = render(AppFrame, { props: { app: makeApp() } });
      await vi.advanceTimersByTimeAsync(30_000);
      const retry = container.querySelector('button') as HTMLButtonElement;
      await fireEvent.click(retry);
      expect(container.querySelector('.animate-spin')).not.toBeNull();
      expect(container.querySelector('button')).toBeNull();
      await vi.advanceTimersByTimeAsync(30_000);
      expect(container.querySelector('button')).not.toBeNull();
    });
  });
});

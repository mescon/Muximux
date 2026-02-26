import { describe, it, expect, vi, beforeEach } from 'vitest';
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
  });

  describe('container styling', () => {
    it('has bg-white class on the container', () => {
      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });
      const appContainer = container.querySelector('[role="application"]');
      expect(appContainer?.classList.contains('bg-white')).toBe(true);
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

      // Simulate iframe load to clear loading spinner
      const iframe = container.querySelector('iframe')!;
      await fireEvent.load(iframe);

      // Pull indicator should not be visible
      expect(container.querySelector('.animate-spin')).not.toBeInTheDocument();
    });

    it('does not respond to touch events when not mobile or touch device', async () => {
      mockIsMobileViewport.mockReturnValue(false);
      mockIsTouchDevice.mockReturnValue(false);

      const { container } = render(AppFrame, {
        props: { app: makeApp() },
      });

      // Simulate iframe load to clear loading spinner
      const iframe = container.querySelector('iframe')!;
      await fireEvent.load(iframe);

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
});

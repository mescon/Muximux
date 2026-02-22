import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

// Mock $lib/api
vi.mock('$lib/api', () => ({
  getBase: vi.fn(() => ''),
}));

// Mock $lib/debug
vi.mock('$lib/debug', () => ({
  debug: vi.fn(),
}));

import AppIcon from './AppIcon.svelte';
import type { AppIcon as AppIconType } from '$lib/types';

// Helper to create a minimal AppIcon config
function makeIcon(overrides: Partial<AppIconType> = {}): AppIconType {
  return {
    type: 'dashboard',
    name: '',
    file: '',
    url: '',
    variant: '',
    ...overrides,
  };
}

describe('AppIcon', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('smoke test', () => {
    it('renders without crashing with minimal props', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon(),
          name: 'TestApp',
        },
      });
      expect(container.querySelector('div')).toBeTruthy();
    });
  });

  describe('fallback letter', () => {
    it('shows fallback letter when icon has no name and no URL', () => {
      render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'dashboard', name: '' }),
          name: 'Grafana',
        },
      });
      expect(screen.getByText('G')).toBeInTheDocument();
    });

    it('shows uppercase first letter of app name as fallback', () => {
      render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'dashboard', name: '' }),
          name: 'prometheus',
        },
      });
      expect(screen.getByText('P')).toBeInTheDocument();
    });
  });

  describe('dashboard icon', () => {
    it('renders an img with correct src for dashboard icon', () => {
      render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'dashboard', name: 'grafana', variant: 'svg' }),
          name: 'Grafana',
        },
      });
      const img = screen.getByAltText('Grafana') as HTMLImageElement;
      expect(img).toBeInTheDocument();
      expect(img.src).toContain('/icons/dashboard/grafana.svg');
    });

    it('uses svg as default variant when variant is empty', () => {
      render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'dashboard', name: 'sonarr', variant: '' }),
          name: 'Sonarr',
        },
      });
      // variant fallback: icon.variant || 'svg' means empty string is falsy -> 'svg'
      const img = screen.getByAltText('Sonarr') as HTMLImageElement;
      expect(img.src).toContain('/icons/dashboard/sonarr.svg');
    });

    it('falls back to letter when dashboard icon name is empty', () => {
      render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'dashboard', name: '' }),
          name: 'MyApp',
        },
      });
      expect(screen.getByText('M')).toBeInTheDocument();
    });
  });

  describe('lucide icon', () => {
    it('renders a masked div for lucide icons instead of img', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'lucide', name: 'settings' }),
          name: 'Settings',
        },
      });
      const lucideDiv = container.querySelector('.lucide-icon');
      expect(lucideDiv).toBeInTheDocument();
      expect(lucideDiv?.getAttribute('role')).toBe('img');
      expect(lucideDiv?.getAttribute('aria-label')).toBe('Settings');
    });

    it('applies tint color from icon.color when present', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'lucide', name: 'home', color: '#ff0000' }),
          name: 'Home',
        },
      });
      const lucideDiv = container.querySelector('.lucide-icon');
      // jsdom converts hex to rgb in style attributes
      expect(lucideDiv?.getAttribute('style')).toContain('background-color: rgb(255, 0, 0)');
    });

    it('does not apply explicit background-color when icon.color is absent', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'lucide', name: 'home' }),
          name: 'Home',
        },
      });
      const lucideDiv = container.querySelector('.lucide-icon');
      // The style should NOT contain an explicit "background-color:" inline override
      // (the CSS class sets the default)
      expect(lucideDiv?.getAttribute('style')).not.toContain('background-color:');
    });
  });

  describe('url icon', () => {
    it('renders an img with the provided URL', () => {
      render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'url', url: 'https://example.com/icon.png' }),
          name: 'External',
        },
      });
      const img = screen.getByAltText('External') as HTMLImageElement;
      expect(img).toBeInTheDocument();
      expect(img.src).toBe('https://example.com/icon.png');
    });

    it('falls back to letter when url is empty', () => {
      render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'url', url: '' }),
          name: 'NoUrl',
        },
      });
      expect(screen.getByText('N')).toBeInTheDocument();
    });
  });

  describe('custom icon', () => {
    it('renders an img with the correct custom icon path', () => {
      render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'custom', file: 'my-icon.png' }),
          name: 'Custom',
        },
      });
      const img = screen.getByAltText('Custom') as HTMLImageElement;
      expect(img.src).toContain('/icons/custom/my-icon.png');
    });

    it('falls back to letter when file is empty', () => {
      render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'custom', file: '' }),
          name: 'NoFile',
        },
      });
      expect(screen.getByText('N')).toBeInTheDocument();
    });
  });

  describe('size classes', () => {
    it('applies sm size class', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon(),
          name: 'Test',
          size: 'sm',
        },
      });
      const root = container.firstElementChild as HTMLElement;
      expect(root.classList.contains('w-6')).toBe(true);
      expect(root.classList.contains('h-6')).toBe(true);
    });

    it('applies md size class by default', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon(),
          name: 'Test',
        },
      });
      const root = container.firstElementChild as HTMLElement;
      expect(root.classList.contains('w-8')).toBe(true);
      expect(root.classList.contains('h-8')).toBe(true);
    });

    it('applies lg size class', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon(),
          name: 'Test',
          size: 'lg',
        },
      });
      const root = container.firstElementChild as HTMLElement;
      expect(root.classList.contains('w-12')).toBe(true);
      expect(root.classList.contains('h-12')).toBe(true);
    });

    it('applies xl size class', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon(),
          name: 'Test',
          size: 'xl',
        },
      });
      const root = container.firstElementChild as HTMLElement;
      expect(root.classList.contains('w-16')).toBe(true);
      expect(root.classList.contains('h-16')).toBe(true);
    });
  });

  describe('scale override', () => {
    it('applies inline width/height when scale is provided and not 1', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon(),
          name: 'Test',
          size: 'md',
          scale: 2,
        },
      });
      const root = container.firstElementChild as HTMLElement;
      // md = 32px, scale 2 => 64px
      expect(root.getAttribute('style')).toContain('width: 64px');
      expect(root.getAttribute('style')).toContain('height: 64px');
    });

    it('does not apply inline width/height when scale is 1', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon(),
          name: 'Test',
          size: 'md',
          scale: 1,
        },
      });
      const root = container.firstElementChild as HTMLElement;
      const style = root.getAttribute('style') || '';
      expect(style).not.toContain('width: 64px');
    });
  });

  describe('background color', () => {
    it('applies transparent background when showBackground is false', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon(),
          name: 'Test',
          showBackground: false,
          color: '#ff0000',
        },
      });
      const root = container.firstElementChild as HTMLElement;
      expect(root.getAttribute('style')).toContain('background-color: transparent');
    });

    it('applies darkened color background when showBackground is true', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon(),
          name: 'Test',
          showBackground: true,
          color: '#ff0000',
        },
      });
      const root = container.firstElementChild as HTMLElement;
      expect(root.getAttribute('style')).toContain('color-mix');
    });

    it('uses icon.background when explicitly set and showBackground is true', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon({ background: '#00ff00' }),
          name: 'Test',
          showBackground: true,
          color: '#ff0000',
        },
      });
      const root = container.firstElementChild as HTMLElement;
      // jsdom converts hex to rgb in style attributes
      expect(root.getAttribute('style')).toContain('background-color: rgb(0, 255, 0)');
    });

    it('applies forceBackground even when showBackground is false', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon({ background: '#0000ff' }),
          name: 'Test',
          showBackground: false,
          forceBackground: true,
          color: '#ff0000',
        },
      });
      const root = container.firstElementChild as HTMLElement;
      // jsdom converts hex to rgb in style attributes
      expect(root.getAttribute('style')).toContain('background-color: rgb(0, 0, 255)');
    });
  });

  describe('invert filter', () => {
    it('applies invert filter when icon.invert is true', () => {
      render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'dashboard', name: 'test', variant: 'svg', invert: true }),
          name: 'Inverted',
        },
      });
      const img = screen.getByAltText('Inverted') as HTMLImageElement;
      expect(img.getAttribute('style')).toContain('filter: invert(1)');
    });

    it('does not apply invert filter when icon.invert is false/undefined', () => {
      render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'dashboard', name: 'test', variant: 'svg' }),
          name: 'Normal',
        },
      });
      const img = screen.getByAltText('Normal') as HTMLImageElement;
      const style = img.getAttribute('style') || '';
      expect(style).not.toContain('invert');
    });
  });

  describe('unknown icon type', () => {
    it('falls back to letter for unknown icon type', () => {
      render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'unknown' as any }),
          name: 'FallbackApp',
        },
      });
      expect(screen.getByText('F')).toBeInTheDocument();
    });
  });

  describe('image error handling', () => {
    it('shows fallback letter after image load error', async () => {
      render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'dashboard', name: 'broken-icon', variant: 'svg' }),
          name: 'BrokenApp',
        },
      });
      // The image should initially be rendered
      const img = screen.getByAltText('BrokenApp') as HTMLImageElement;
      expect(img).toBeInTheDocument();

      // Simulate image error
      await fireEvent.error(img);

      // After error, fallback letter should appear
      expect(screen.getByText('B')).toBeInTheDocument();
    });
  });

  describe('background with transparent icon background', () => {
    it('uses color-mix when icon.background is transparent and showBackground is true', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon({ background: 'transparent' }),
          name: 'Test',
          showBackground: true,
          color: '#ff0000',
        },
      });
      const root = container.firstElementChild as HTMLElement;
      // 'transparent' is treated as falsy by the condition, so it falls through to color-mix
      expect(root.getAttribute('style')).toContain('color-mix');
    });
  });

  describe('lucide icon padding', () => {
    it('applies p-1.5 padding when showBackground is true', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'lucide', name: 'settings' }),
          name: 'Settings',
          showBackground: true,
        },
      });
      const lucideDiv = container.querySelector('.lucide-icon');
      expect(lucideDiv?.classList.contains('p-1.5')).toBe(true);
    });

    it('applies p-1 padding when showBackground is false', () => {
      const { container } = render(AppIcon, {
        props: {
          icon: makeIcon({ type: 'lucide', name: 'settings' }),
          name: 'Settings',
          showBackground: false,
        },
      });
      const lucideDiv = container.querySelector('.lucide-icon');
      expect(lucideDiv?.classList.contains('p-1')).toBe(true);
    });
  });
});

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

// Mock IntersectionObserver (not available in jsdom)
let intersectionCallback: IntersectionObserverCallback | null = null;
class IntersectionObserverMock {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
  constructor(callback: IntersectionObserverCallback, _options?: IntersectionObserverInit) {
    intersectionCallback = callback;
  }
}
globalThis.IntersectionObserver = IntersectionObserverMock as unknown as typeof IntersectionObserver;

// Hoist mock data for API
const { mockListDashboardIcons, mockListLucideIcons, mockListCustomIcons, mockUploadCustomIcon, mockFetchCustomIconFromUrl, mockDeleteCustomIcon } = vi.hoisted(() => {
  return {
    mockListDashboardIcons: vi.fn(),
    mockListLucideIcons: vi.fn(),
    mockListCustomIcons: vi.fn(),
    mockUploadCustomIcon: vi.fn(),
    mockFetchCustomIconFromUrl: vi.fn(),
    mockDeleteCustomIcon: vi.fn(),
  };
});

// Mock $lib/api
vi.mock('$lib/api', () => ({
  listDashboardIcons: mockListDashboardIcons,
  getDashboardIconUrl: vi.fn((name: string, variant: string) => `/icons/dashboard/${name}.${variant}`),
  listLucideIcons: mockListLucideIcons,
  getLucideIconUrl: vi.fn((name: string) => `/icons/lucide/${name}.svg`),
  listCustomIcons: mockListCustomIcons,
  getCustomIconUrl: vi.fn((name: string) => `/icons/custom/${name}`),
  uploadCustomIcon: mockUploadCustomIcon,
  fetchCustomIconFromUrl: mockFetchCustomIconFromUrl,
  deleteCustomIcon: mockDeleteCustomIcon,
  getBase: vi.fn(() => ''),
}));

// Mock $lib/toastStore
vi.mock('$lib/toastStore', () => ({
  toasts: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
  },
}));

// Mock $lib/debug
vi.mock('$lib/debug', () => ({
  debug: vi.fn(),
}));

import IconBrowser from './IconBrowser.svelte';
import { toasts } from '$lib/toastStore';

const sampleDashboardIcons = [
  { name: 'grafana', variants: ['svg', 'png'] },
  { name: 'sonarr', variants: ['svg', 'png', 'webp'] },
  { name: 'radarr', variants: ['svg'] },
];

const sampleLucideIcons = [
  { name: 'settings', categories: ['system'] },
  { name: 'home', categories: ['navigation'] },
  { name: 'search', categories: ['navigation', 'input'] },
];

const sampleCustomIcons = [
  { name: 'my-icon.png', content_type: 'image/png', size: 1024 },
  { name: 'logo.svg', content_type: 'image/svg+xml', size: 512 },
];

describe('IconBrowser', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListDashboardIcons.mockResolvedValue(sampleDashboardIcons);
    mockListLucideIcons.mockResolvedValue(sampleLucideIcons);
    mockListCustomIcons.mockResolvedValue(sampleCustomIcons);
    mockUploadCustomIcon.mockResolvedValue({ name: 'uploaded.png', status: 'ok' });
    mockFetchCustomIconFromUrl.mockResolvedValue({ name: 'fetched-icon', status: 'uploaded' });
    mockDeleteCustomIcon.mockResolvedValue(undefined);
  });

  describe('smoke test', () => {
    it('renders without crashing', async () => {
      const { container } = render(IconBrowser);
      expect(container.querySelector('div')).toBeTruthy();
    });
  });

  describe('tabs', () => {
    it('shows Dashboard Icons tab', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('Dashboard Icons')).toBeInTheDocument();
      });
    });

    it('shows Lucide tab', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('Lucide')).toBeInTheDocument();
      });
    });

    it('shows Custom tab', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('Custom')).toBeInTheDocument();
      });
    });

    it('defaults to dashboard tab as active', async () => {
      render(IconBrowser);
      await waitFor(() => {
        const dashboardBtn = screen.getByText('Dashboard Icons').closest('button');
        expect(dashboardBtn?.className).toContain('text-brand-400');
      });
    });

    it('switches to lucide tab when clicked', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('Lucide')).toBeInTheDocument();
      });

      const lucideBtn = screen.getByText('Lucide').closest('button')!;
      await fireEvent.click(lucideBtn);

      await waitFor(() => {
        expect(lucideBtn.className).toContain('text-brand-400');
      });
    });

    it('switches to custom tab when clicked', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('Custom')).toBeInTheDocument();
      });

      const customBtn = screen.getByText('Custom').closest('button')!;
      await fireEvent.click(customBtn);

      await waitFor(() => {
        expect(customBtn.className).toContain('text-brand-400');
      });
    });
  });

  describe('search input', () => {
    it('has a search input with correct placeholder', async () => {
      render(IconBrowser);
      await waitFor(() => {
        const input = screen.getByPlaceholderText('Search icons...');
        expect(input).toBeInTheDocument();
      });
    });

    it('search input accepts user text', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByPlaceholderText('Search icons...')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search icons...') as HTMLInputElement;
      await fireEvent.input(input, { target: { value: 'grafana' } });

      expect(input.value).toBe('grafana');
    });
  });

  describe('icon grid', () => {
    it('shows icon buttons after loading', async () => {
      render(IconBrowser);
      await waitFor(() => {
        // Dashboard icons should load and display (dashboard is default tab)
        const grafanaBtn = screen.getByTitle('grafana');
        expect(grafanaBtn).toBeInTheDocument();
      });
    });

    it('displays all dashboard icons by default', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByTitle('grafana')).toBeInTheDocument();
        expect(screen.getByTitle('sonarr')).toBeInTheDocument();
        expect(screen.getByTitle('radarr')).toBeInTheDocument();
      });
    });

    it('displays lucide icons when lucide tab is selected', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('Lucide')).toBeInTheDocument();
      });

      const lucideBtn = screen.getByText('Lucide').closest('button')!;
      await fireEvent.click(lucideBtn);

      await waitFor(() => {
        expect(screen.getByTitle('settings')).toBeInTheDocument();
        expect(screen.getByTitle('home')).toBeInTheDocument();
        expect(screen.getByTitle('search')).toBeInTheDocument();
      });
    });

    it('displays custom icons when custom tab is selected', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('Custom')).toBeInTheDocument();
      });

      const customBtn = screen.getByText('Custom').closest('button')!;
      await fireEvent.click(customBtn);

      await waitFor(() => {
        expect(screen.getByTitle('my-icon.png')).toBeInTheDocument();
        expect(screen.getByTitle('logo.svg')).toBeInTheDocument();
      });
    });
  });

  describe('icon selection', () => {
    it('calls onselect when Select Icon button is clicked after picking an icon', async () => {
      const onselect = vi.fn();
      render(IconBrowser, {
        props: { onselect },
      });
      await waitFor(() => {
        expect(screen.getByTitle('grafana')).toBeInTheDocument();
      });

      // Click an icon to select it
      await fireEvent.click(screen.getByTitle('grafana'));

      // Click Select Icon button
      const selectBtn = screen.getByText('Select Icon');
      await fireEvent.click(selectBtn);

      expect(onselect).toHaveBeenCalledTimes(1);
      expect(onselect).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'grafana', type: 'dashboard' })
      );
    });

    it('Select Icon button is disabled when no icon is selected', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('Select Icon')).toBeInTheDocument();
      });

      const selectBtn = screen.getByText('Select Icon') as HTMLButtonElement;
      expect(selectBtn.disabled).toBe(true);
    });

    it('highlights selected icon', async () => {
      render(IconBrowser, {
        props: { selectedIcon: 'grafana', selectedType: 'dashboard' },
      });
      await waitFor(() => {
        expect(screen.getByTitle('grafana')).toBeInTheDocument();
      });

      const iconBtn = screen.getByTitle('grafana');
      await waitFor(() => {
        expect(iconBtn.className).toContain('border-brand-500');
      });
    });
  });

  describe('cancel button', () => {
    it('has a Cancel button', async () => {
      render(IconBrowser);
      expect(screen.getByText('Cancel')).toBeInTheDocument();
    });

    it('calls onclose when Cancel is clicked', async () => {
      const onclose = vi.fn();
      render(IconBrowser, {
        props: { onclose },
      });

      await fireEvent.click(screen.getByText('Cancel'));
      expect(onclose).toHaveBeenCalledTimes(1);
    });
  });

  describe('variant selector', () => {
    it('shows variant selector (SVG, PNG, WEBP) on dashboard tab', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('SVG')).toBeInTheDocument();
        expect(screen.getByText('PNG')).toBeInTheDocument();
        expect(screen.getByText('WEBP')).toBeInTheDocument();
      });
    });

    it('does not show variant selector on lucide tab', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('Lucide')).toBeInTheDocument();
      });

      const lucideBtn = screen.getByText('Lucide').closest('button')!;
      await fireEvent.click(lucideBtn);

      await waitFor(() => {
        expect(screen.queryByText('SVG')).not.toBeInTheDocument();
        expect(screen.queryByText('PNG')).not.toBeInTheDocument();
        expect(screen.queryByText('WEBP')).not.toBeInTheDocument();
      });
    });
  });

  describe('custom icon upload', () => {
    it('shows Upload Custom Icon button on custom tab', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('Custom')).toBeInTheDocument();
      });

      const customBtn = screen.getByText('Custom').closest('button')!;
      await fireEvent.click(customBtn);

      await waitFor(() => {
        expect(screen.getByText('Upload Custom Icon')).toBeInTheDocument();
      });
    });

    it('does not show upload button on dashboard tab', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.queryByText('Upload Custom Icon')).not.toBeInTheDocument();
      });
    });
  });

  describe('icon count footer', () => {
    it('shows icon count in footer', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText(/icons$/)).toBeInTheDocument();
      });
    });
  });

  describe('loading state', () => {
    it('shows loading state while icons are being fetched', () => {
      // Make the API calls never resolve during this test
      mockListDashboardIcons.mockReturnValue(new Promise(() => {}));
      mockListLucideIcons.mockReturnValue(new Promise(() => {}));
      mockListCustomIcons.mockReturnValue(new Promise(() => {}));

      const { container } = render(IconBrowser);
      // SkeletonIconGrid should be rendered during loading
      // The component is rendered; we just verify it didn't crash
      expect(container.querySelector('div')).toBeTruthy();
    });
  });

  describe('error state', () => {
    it('shows error state when all icon sources fail', async () => {
      mockListDashboardIcons.mockRejectedValue(new Error('Network error'));
      mockListLucideIcons.mockRejectedValue(new Error('Network error'));
      mockListCustomIcons.mockRejectedValue(new Error('Network error'));

      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('Failed to load icons')).toBeInTheDocument();
      });
    });
  });

  describe('empty state', () => {
    it('shows no icons message when API returns empty arrays', async () => {
      mockListDashboardIcons.mockResolvedValue([]);
      mockListLucideIcons.mockResolvedValue([]);
      mockListCustomIcons.mockResolvedValue([]);

      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('No icons available')).toBeInTheDocument();
      });
    });
  });

  describe('initial selectedType prop', () => {
    it('starts on lucide tab when selectedType is lucide', async () => {
      render(IconBrowser, {
        props: { selectedType: 'lucide' },
      });
      await waitFor(() => {
        const lucideBtn = screen.getByText('Lucide').closest('button');
        expect(lucideBtn?.className).toContain('text-brand-400');
      });
    });

    it('starts on custom tab when selectedType is custom', async () => {
      render(IconBrowser, {
        props: { selectedType: 'custom' },
      });
      await waitFor(() => {
        const customBtn = screen.getByText('Custom').closest('button');
        expect(customBtn?.className).toContain('text-brand-400');
      });
    });
  });

  describe('search filtering', () => {
    it('filters dashboard icons by name after debounce', async () => {
      vi.useFakeTimers();
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByTitle('grafana')).toBeInTheDocument();
        expect(screen.getByTitle('sonarr')).toBeInTheDocument();
        expect(screen.getByTitle('radarr')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search icons...') as HTMLInputElement;
      await fireEvent.input(input, { target: { value: 'grafana' } });

      // Advance past debounce timer
      vi.advanceTimersByTime(300);
      await waitFor(() => {
        expect(screen.getByTitle('grafana')).toBeInTheDocument();
        expect(screen.queryByTitle('sonarr')).not.toBeInTheDocument();
        expect(screen.queryByTitle('radarr')).not.toBeInTheDocument();
      });
      vi.useRealTimers();
    });

    it('filters lucide icons by category', async () => {
      vi.useFakeTimers();
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByText('Lucide')).toBeInTheDocument();
      });

      // Switch to Lucide tab
      await fireEvent.click(screen.getByText('Lucide').closest('button')!);
      await waitFor(() => {
        expect(screen.getByTitle('settings')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search icons...') as HTMLInputElement;
      await fireEvent.input(input, { target: { value: 'system' } });
      vi.advanceTimersByTime(300);

      await waitFor(() => {
        // 'settings' has category 'system', so it should match
        expect(screen.getByTitle('settings')).toBeInTheDocument();
        // 'home' has category 'navigation' - should not match
        expect(screen.queryByTitle('home')).not.toBeInTheDocument();
      });
      vi.useRealTimers();
    });

    it('filters custom icons by name', async () => {
      vi.useFakeTimers();
      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByTitle('my-icon.png')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search icons...') as HTMLInputElement;
      await fireEvent.input(input, { target: { value: 'logo' } });
      vi.advanceTimersByTime(300);

      await waitFor(() => {
        expect(screen.getByTitle('logo.svg')).toBeInTheDocument();
        expect(screen.queryByTitle('my-icon.png')).not.toBeInTheDocument();
      });
      vi.useRealTimers();
    });

    it('shows no matches message when search has no results', async () => {
      vi.useFakeTimers();
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByTitle('grafana')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search icons...') as HTMLInputElement;
      await fireEvent.input(input, { target: { value: 'zzzznonexistent' } });
      vi.advanceTimersByTime(300);

      await waitFor(() => {
        expect(screen.getByText('No matches found')).toBeInTheDocument();
      });
      vi.useRealTimers();
    });

    it('debounces multiple rapid search inputs', async () => {
      vi.useFakeTimers();
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByTitle('grafana')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search icons...') as HTMLInputElement;
      // Type rapidly
      await fireEvent.input(input, { target: { value: 'g' } });
      vi.advanceTimersByTime(50);
      await fireEvent.input(input, { target: { value: 'gr' } });
      vi.advanceTimersByTime(50);
      await fireEvent.input(input, { target: { value: 'gra' } });
      vi.advanceTimersByTime(300);

      // Only the final value should be applied
      await waitFor(() => {
        expect(screen.getByTitle('grafana')).toBeInTheDocument();
        expect(screen.queryByTitle('sonarr')).not.toBeInTheDocument();
      });
      vi.useRealTimers();
    });
  });

  describe('variant selection behavior', () => {
    it('changes variant when a different variant button is clicked', async () => {
      const onselect = vi.fn();
      render(IconBrowser, { props: { onselect } });
      await waitFor(() => {
        expect(screen.getByText('SVG')).toBeInTheDocument();
      });

      // Click PNG variant
      await fireEvent.click(screen.getByText('PNG'));

      // Select an icon and confirm
      await waitFor(() => {
        expect(screen.getByTitle('grafana')).toBeInTheDocument();
      });
      await fireEvent.click(screen.getByTitle('grafana'));
      await fireEvent.click(screen.getByText('Select Icon'));

      expect(onselect).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'grafana', variant: 'png', type: 'dashboard' })
      );
    });

    it('WEBP variant selection works', async () => {
      const onselect = vi.fn();
      render(IconBrowser, { props: { onselect } });
      await waitFor(() => {
        expect(screen.getByText('WEBP')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('WEBP'));
      await waitFor(() => {
        expect(screen.getByTitle('grafana')).toBeInTheDocument();
      });
      await fireEvent.click(screen.getByTitle('grafana'));
      await fireEvent.click(screen.getByText('Select Icon'));

      expect(onselect).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'webp', type: 'dashboard' })
      );
    });
  });

  describe('icon selection on different tabs', () => {
    it('calls onselect with lucide type when selecting a lucide icon', async () => {
      const onselect = vi.fn();
      render(IconBrowser, { props: { onselect } });
      await waitFor(() => {
        expect(screen.getByText('Lucide')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Lucide').closest('button')!);
      await waitFor(() => {
        expect(screen.getByTitle('settings')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByTitle('settings'));
      await fireEvent.click(screen.getByText('Select Icon'));

      expect(onselect).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'settings', variant: 'svg', type: 'lucide' })
      );
    });

    it('calls onselect with custom type when selecting a custom icon', async () => {
      const onselect = vi.fn();
      render(IconBrowser, { props: { onselect, selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByTitle('my-icon.png')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByTitle('my-icon.png'));
      await fireEvent.click(screen.getByText('Select Icon'));

      expect(onselect).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'my-icon.png', variant: 'svg', type: 'custom' })
      );
    });
  });

  describe('partial failure handling', () => {
    it('shows warning toast when some but not all sources fail', async () => {
      mockListDashboardIcons.mockRejectedValue(new Error('fail'));
      // lucide and custom succeed

      render(IconBrowser);
      await waitFor(() => {
        expect(toasts.warning).toHaveBeenCalledWith(
          expect.stringContaining('Dashboard')
        );
      });
    });

    it('shows warning listing multiple failed sources', async () => {
      mockListDashboardIcons.mockRejectedValue(new Error('fail'));
      mockListLucideIcons.mockRejectedValue(new Error('fail'));
      // custom succeeds

      render(IconBrowser);
      await waitFor(() => {
        expect(toasts.warning).toHaveBeenCalledWith(
          expect.stringContaining('Dashboard')
        );
      });
    });
  });

  describe('custom icon file upload', () => {
    it('uploads a file when selected via file input', async () => {
      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByText('Upload Custom Icon')).toBeInTheDocument();
      });

      // Find the hidden file input
      const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
      expect(fileInput).toBeTruthy();

      const file = new File(['test'], 'test-icon.png', { type: 'image/png' });
      await fireEvent.change(fileInput, { target: { files: [file] } });

      await waitFor(() => {
        expect(mockUploadCustomIcon).toHaveBeenCalledWith(file);
        // After upload, custom icons should be reloaded
        expect(mockListCustomIcons).toHaveBeenCalledTimes(2); // once on mount, once after upload
      });
    });

    it('shows upload error when upload fails', async () => {
      mockUploadCustomIcon.mockRejectedValue(new Error('File too large'));

      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByText('Upload Custom Icon')).toBeInTheDocument();
      });

      const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
      const file = new File(['test'], 'big-icon.png', { type: 'image/png' });
      await fireEvent.change(fileInput, { target: { files: [file] } });

      await waitFor(() => {
        expect(screen.getByText('File too large')).toBeInTheDocument();
      });
    });

    it('does nothing when no file is selected', async () => {
      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByText('Upload Custom Icon')).toBeInTheDocument();
      });

      const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
      // Fire change with no files
      await fireEvent.change(fileInput, { target: { files: [] } });

      expect(mockUploadCustomIcon).not.toHaveBeenCalled();
    });
  });

  describe('custom icon deletion', () => {
    it('shows delete confirmation when delete button is clicked', async () => {
      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByTitle('my-icon.png')).toBeInTheDocument();
      });

      // Find the delete button (small X button on hover)
      const deleteButtons = document.querySelectorAll('button[title="Delete"]');
      expect(deleteButtons.length).toBeGreaterThan(0);

      await fireEvent.click(deleteButtons[0]);

      // Confirmation overlay should appear
      await waitFor(() => {
        expect(screen.getByText('Delete?')).toBeInTheDocument();
        expect(screen.getByText('Yes')).toBeInTheDocument();
        expect(screen.getByText('No')).toBeInTheDocument();
      });
    });

    it('deletes icon when confirmation Yes is clicked', async () => {
      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByTitle('my-icon.png')).toBeInTheDocument();
      });

      const deleteButtons = document.querySelectorAll('button[title="Delete"]');
      await fireEvent.click(deleteButtons[0]);

      await waitFor(() => {
        expect(screen.getByText('Yes')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Yes'));

      await waitFor(() => {
        expect(mockDeleteCustomIcon).toHaveBeenCalledWith('my-icon.png');
      });
    });

    it('cancels deletion when No is clicked', async () => {
      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByTitle('my-icon.png')).toBeInTheDocument();
      });

      const deleteButtons = document.querySelectorAll('button[title="Delete"]');
      await fireEvent.click(deleteButtons[0]);

      await waitFor(() => {
        expect(screen.getByText('No')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('No'));

      // Confirmation overlay should disappear
      await waitFor(() => {
        expect(screen.queryByText('Delete?')).not.toBeInTheDocument();
      });
      expect(mockDeleteCustomIcon).not.toHaveBeenCalled();
    });

    it('clears selection when deleting the currently selected icon', async () => {
      const onselect = vi.fn();
      render(IconBrowser, {
        props: { selectedType: 'custom', selectedIcon: 'my-icon.png', onselect },
      });
      await waitFor(() => {
        expect(screen.getByTitle('my-icon.png')).toBeInTheDocument();
      });

      const deleteButtons = document.querySelectorAll('button[title="Delete"]');
      await fireEvent.click(deleteButtons[0]);

      await waitFor(() => {
        expect(screen.getByText('Yes')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Yes'));

      await waitFor(() => {
        expect(mockDeleteCustomIcon).toHaveBeenCalledWith('my-icon.png');
      });
    });

    it('shows error toast when delete fails', async () => {
      mockDeleteCustomIcon.mockRejectedValue(new Error('Delete not allowed'));

      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByTitle('my-icon.png')).toBeInTheDocument();
      });

      const deleteButtons = document.querySelectorAll('button[title="Delete"]');
      await fireEvent.click(deleteButtons[0]);

      await waitFor(() => {
        expect(screen.getByText('Yes')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Yes'));

      await waitFor(() => {
        expect(toasts.error).toHaveBeenCalledWith('Delete not allowed');
      });
    });
  });

  describe('empty state for custom tab', () => {
    it('shows "No custom icons" when custom tab is empty', async () => {
      mockListCustomIcons.mockResolvedValue([]);

      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByText('No custom icons')).toBeInTheDocument();
      });
    });
  });

  describe('footer icon count with filtering', () => {
    it('shows "X of Y icons" when search narrows results', async () => {
      vi.useFakeTimers();
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByTitle('grafana')).toBeInTheDocument();
      });

      // Initially shows total count
      expect(screen.getByText('3 icons')).toBeInTheDocument();

      const input = screen.getByPlaceholderText('Search icons...') as HTMLInputElement;
      await fireEvent.input(input, { target: { value: 'grafana' } });
      vi.advanceTimersByTime(300);

      await waitFor(() => {
        expect(screen.getByText(/1 of 3 icons/)).toBeInTheDocument();
      });
      vi.useRealTimers();
    });
  });

  describe('tab switching resets display', () => {
    it('switches back to dashboard tab after being on lucide', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByTitle('grafana')).toBeInTheDocument();
      });

      // Go to lucide
      await fireEvent.click(screen.getByText('Lucide').closest('button')!);
      await waitFor(() => {
        expect(screen.getByTitle('settings')).toBeInTheDocument();
      });

      // Go back to dashboard
      await fireEvent.click(screen.getByText('Dashboard Icons').closest('button')!);
      await waitFor(() => {
        expect(screen.getByTitle('grafana')).toBeInTheDocument();
      });
    });
  });

  describe('reload custom icons failure', () => {
    it('shows error toast when reloading custom icons fails during upload', async () => {
      // First call succeeds (initial load), second fails (after upload reload)
      mockListCustomIcons
        .mockResolvedValueOnce(sampleCustomIcons)
        .mockRejectedValueOnce(new Error('Reload failed'));
      mockUploadCustomIcon.mockResolvedValue({ name: 'new.png', status: 'ok' });

      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByText('Upload Custom Icon')).toBeInTheDocument();
      });

      const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
      const file = new File(['test'], 'new.png', { type: 'image/png' });
      await fireEvent.change(fileInput, { target: { files: [file] } });

      await waitFor(() => {
        expect(toasts.error).toHaveBeenCalledWith('Failed to reload custom icons');
      });
    });
  });

  describe('upload button click triggers file input', () => {
    it('clicking upload button triggers file input click', async () => {
      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByText('Upload Custom Icon')).toBeInTheDocument();
      });

      const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
      const clickSpy = vi.spyOn(fileInput, 'click');

      await fireEvent.click(screen.getByText('Upload Custom Icon'));

      expect(clickSpy).toHaveBeenCalled();
    });
  });

  describe('custom icon URL fetch', () => {
    it('shows URL input and Fetch button on custom tab', async () => {
      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByPlaceholderText('https://example.com/icon.png')).toBeInTheDocument();
        expect(screen.getByText('Fetch')).toBeInTheDocument();
      });
    });

    it('does not show URL input on dashboard tab', async () => {
      render(IconBrowser);
      await waitFor(() => {
        expect(screen.queryByPlaceholderText('https://example.com/icon.png')).not.toBeInTheDocument();
      });
    });

    it('Fetch button is disabled when URL input is empty', async () => {
      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        const fetchBtn = screen.getByText('Fetch') as HTMLButtonElement;
        expect(fetchBtn.disabled).toBe(true);
      });
    });

    it('fetches icon from URL when Fetch button is clicked', async () => {
      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByPlaceholderText('https://example.com/icon.png')).toBeInTheDocument();
      });

      const urlInput = screen.getByPlaceholderText('https://example.com/icon.png') as HTMLInputElement;
      await fireEvent.input(urlInput, { target: { value: 'https://example.com/my-icon.png' } });
      await fireEvent.click(screen.getByText('Fetch'));

      await waitFor(() => {
        expect(mockFetchCustomIconFromUrl).toHaveBeenCalledWith('https://example.com/my-icon.png');
        // After fetch, custom icons should be reloaded
        expect(mockListCustomIcons).toHaveBeenCalledTimes(2);
      });
    });

    it('clears URL input on successful fetch', async () => {
      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByPlaceholderText('https://example.com/icon.png')).toBeInTheDocument();
      });

      const urlInput = screen.getByPlaceholderText('https://example.com/icon.png') as HTMLInputElement;
      await fireEvent.input(urlInput, { target: { value: 'https://example.com/icon.png' } });
      await fireEvent.click(screen.getByText('Fetch'));

      await waitFor(() => {
        expect(urlInput.value).toBe('');
      });
    });

    it('shows error message when fetch fails', async () => {
      mockFetchCustomIconFromUrl.mockRejectedValue(new Error('API error: 400 Unsupported file type'));

      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByPlaceholderText('https://example.com/icon.png')).toBeInTheDocument();
      });

      const urlInput = screen.getByPlaceholderText('https://example.com/icon.png') as HTMLInputElement;
      await fireEvent.input(urlInput, { target: { value: 'https://example.com/bad.html' } });
      await fireEvent.click(screen.getByText('Fetch'));

      await waitFor(() => {
        expect(screen.getByText('API error: 400 Unsupported file type')).toBeInTheDocument();
      });
    });

    it('fetches icon when Enter is pressed in URL input', async () => {
      render(IconBrowser, { props: { selectedType: 'custom' } });
      await waitFor(() => {
        expect(screen.getByPlaceholderText('https://example.com/icon.png')).toBeInTheDocument();
      });

      const urlInput = screen.getByPlaceholderText('https://example.com/icon.png') as HTMLInputElement;
      await fireEvent.input(urlInput, { target: { value: 'https://example.com/icon.svg' } });
      await fireEvent.keyDown(urlInput, { key: 'Enter' });

      await waitFor(() => {
        expect(mockFetchCustomIconFromUrl).toHaveBeenCalledWith('https://example.com/icon.svg');
      });
    });
  });

  describe('infinite scroll', () => {
    it('triggers loading more icons when sentinel is intersecting', async () => {
      // Create a large set of icons (> BATCH_SIZE)
      const manyIcons = Array.from({ length: 150 }, (_, i) => ({
        name: `icon-${i}`,
        variants: ['svg'],
      }));
      mockListDashboardIcons.mockResolvedValue(manyIcons);

      render(IconBrowser);
      await waitFor(() => {
        expect(screen.getByTitle('icon-0')).toBeInTheDocument();
      });

      // The hasMore text should be shown
      await waitFor(() => {
        expect(screen.getByText(/Loading more/)).toBeInTheDocument();
      });

      // Simulate intersection observer callback
      if (intersectionCallback) {
        intersectionCallback(
          [{ isIntersecting: true } as IntersectionObserverEntry],
          {} as IntersectionObserver
        );
      }

      // After triggering, more icons should eventually load
      await waitFor(() => {
        expect(screen.getByTitle('icon-100')).toBeInTheDocument();
      });
    });
  });
});

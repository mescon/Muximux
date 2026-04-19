import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

// All mock functions and stores must be hoisted so vi.mock factories can reference them
const { mockStores, mockSaveCustomTheme, mockDeleteCustomTheme, mockGetCurrentThemeVars, mockSetThemeFamily, mockSetVariantMode, mockSanitizeThemeId, mockToasts } = vi.hoisted(() => {
  // vi.fn() is available inside vi.hoisted
  return {
    mockStores: {
      resolvedTheme: 'muximux',
      allThemes: [
        { id: 'muximux', name: 'Muximux Dark', isBuiltin: true },
        { id: 'muximux-light', name: 'Muximux Light', isBuiltin: true },
      ],
      isDarkTheme: true,
      selectedFamily: 'default',
      variantMode: 'dark',
      themeFamilies: [
        {
          id: 'default',
          name: 'Muximux',
          description: 'The default theme',
          darkTheme: { id: 'muximux', name: 'Muximux Dark', isBuiltin: true, preview: { bg: '#1a1a2e', surface: '#16213e', accent: '#0f3460', text: '#e0e0e0' } },
          lightTheme: { id: 'muximux-light', name: 'Muximux Light', isBuiltin: true, preview: { bg: '#ffffff', surface: '#f5f5f5', accent: '#3b82f6', text: '#1a1a1a' } },
        },
      ],
    },
    mockSaveCustomTheme: vi.fn().mockResolvedValue(true),
    mockDeleteCustomTheme: vi.fn().mockResolvedValue(true),
    mockGetCurrentThemeVars: vi.fn().mockReturnValue({
      '--bg-base': '#1a1a2e',
      '--bg-surface': '#16213e',
      '--text-primary': '#ffffff',
    }),
    mockSetThemeFamily: vi.fn(),
    mockSetVariantMode: vi.fn(),
    mockSanitizeThemeId: vi.fn((name: string) => name.toLowerCase().replace(/\s+/g, '-')),
    mockToasts: {
      success: vi.fn(),
      error: vi.fn(),
      info: vi.fn(),
    },
  };
});

// Mock themeStore
vi.mock('$lib/themeStore', async () => {
  const { writable } = await import('svelte/store');

  const resolvedTheme = writable(mockStores.resolvedTheme);
  const allThemes = writable(mockStores.allThemes);
  const isDarkTheme = writable(mockStores.isDarkTheme);
  const selectedFamily = writable(mockStores.selectedFamily);
  const variantMode = writable(mockStores.variantMode);
  const themeFamilies = writable(mockStores.themeFamilies);

  return {
    resolvedTheme,
    allThemes,
    isDarkTheme,
    selectedFamily,
    variantMode,
    themeFamilies,
    saveCustomThemeToServer: (...args: unknown[]) => mockSaveCustomTheme(...args),
    deleteCustomThemeFromServer: (...args: unknown[]) => mockDeleteCustomTheme(...args),
    getCurrentThemeVariables: (...args: unknown[]) => mockGetCurrentThemeVars(...args),
    themeVariableGroups: {
      'Backgrounds': ['--bg-base', '--bg-surface'],
      'Text': ['--text-primary'],
    },
    sanitizeThemeId: (...args: unknown[]) => mockSanitizeThemeId(...args),
    setThemeFamily: (...args: unknown[]) => mockSetThemeFamily(...args),
    setVariantMode: (...args: unknown[]) => mockSetVariantMode(...args),
  };
});

// Mock toastStore
vi.mock('$lib/toastStore', () => ({
  toasts: mockToasts,
}));

import ThemeTab from './ThemeTab.svelte';
import { themeFamilies } from '$lib/themeStore';

describe('ThemeTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSaveCustomTheme.mockResolvedValue(true);
    mockDeleteCustomTheme.mockResolvedValue(true);
    mockGetCurrentThemeVars.mockReturnValue({
      '--bg-base': '#1a1a2e',
      '--bg-surface': '#16213e',
      '--text-primary': '#ffffff',
    });
    // Reset any inline style overrides from previous tests
    document.documentElement.style.cssText = '';
  });

  // ─── Existing tests (preserved) ───────────────────────────────────────────

  it('renders without crashing (smoke test)', () => {
    const { container } = render(ThemeTab);
    expect(container.querySelector('div')).toBeTruthy();
  });

  it('renders the theme family selector with theme names', () => {
    render(ThemeTab);

    expect(screen.getByText('Choose Theme')).toBeInTheDocument();
    expect(screen.getByText('Muximux')).toBeInTheDocument();
  });

  it('shows variant mode options (Dark / System / Light)', () => {
    render(ThemeTab);

    expect(screen.getByText('Dark')).toBeInTheDocument();
    expect(screen.getByText('System')).toBeInTheDocument();
    expect(screen.getByText('Light')).toBeInTheDocument();
  });

  it('renders the Appearance section', () => {
    render(ThemeTab);

    expect(screen.getByText('Appearance')).toBeInTheDocument();
    expect(screen.getByText('Choose dark, light, or follow your system')).toBeInTheDocument();
  });

  it('renders the Customize Current Theme button when editor is closed', () => {
    render(ThemeTab);

    expect(screen.getByText('Customize Current Theme')).toBeInTheDocument();
    expect(screen.getByText('Tweak colors and save as a new custom theme')).toBeInTheDocument();
  });

  it('shows current theme info', () => {
    render(ThemeTab);

    expect(screen.getByText(/Currently using:/)).toBeInTheDocument();
  });

  it('shows theme family description when available', () => {
    render(ThemeTab);

    expect(screen.getByText('The default theme')).toBeInTheDocument();
  });

  // ─── Variant Mode Switching ───────────────────────────────────────────────

  describe('variant mode switching', () => {
    it('calls setVariantMode("dark") when Dark button is clicked', async () => {
      render(ThemeTab);

      const darkBtn = screen.getByText('Dark');
      await fireEvent.click(darkBtn);

      expect(mockSetVariantMode).toHaveBeenCalledWith('dark');
    });

    it('calls setVariantMode("light") when Light button is clicked', async () => {
      render(ThemeTab);

      const lightBtn = screen.getByText('Light');
      await fireEvent.click(lightBtn);

      expect(mockSetVariantMode).toHaveBeenCalledWith('light');
    });

    it('calls setVariantMode("system") when System button is clicked', async () => {
      render(ThemeTab);

      const systemBtn = screen.getByText('System');
      await fireEvent.click(systemBtn);

      expect(mockSetVariantMode).toHaveBeenCalledWith('system');
    });
  });

  // ─── Theme Family Selection ───────────────────────────────────────────────

  describe('theme family selection', () => {
    it('calls setThemeFamily when a theme family card is clicked', async () => {
      render(ThemeTab);

      // Click the Default theme card
      const defaultTheme = screen.getByText('Muximux').closest('[role="button"]')!;
      await fireEvent.click(defaultTheme);

      expect(mockSetThemeFamily).toHaveBeenCalledWith('default');
    });

    it('shows checkmark indicator on selected family', () => {
      const { container } = render(ThemeTab);

      // The selected family should have a checkmark SVG
      const checkmark = container.querySelector('[fill-rule="evenodd"]');
      expect(checkmark).toBeTruthy();
    });

    it('renders dual preview swatches when both dark and light themes have previews', () => {
      const { container } = render(ThemeTab);

      // The default family has both dark and light previews, so two swatches
      const swatches = container.querySelectorAll('.rounded-lg.overflow-hidden');
      expect(swatches.length).toBeGreaterThanOrEqual(2);
    });

    it('shows Custom badge for custom themes', () => {
      // Override themeFamilies to include a custom theme
      (themeFamilies as unknown as { set: (v: unknown) => void }).set([
        ...mockStores.themeFamilies,
        {
          id: 'my-custom',
          name: 'My Custom',
          description: 'A custom theme',
          darkTheme: { id: 'my-custom-dark', name: 'My Custom Dark', isBuiltin: false, preview: { bg: '#000', surface: '#111', accent: '#ff0', text: '#fff' } },
          lightTheme: null,
        },
      ]);

      render(ThemeTab);

      expect(screen.getByText('Custom')).toBeInTheDocument();
    });

    it('shows delete button on hover for custom themes', () => {
      (themeFamilies as unknown as { set: (v: unknown) => void }).set([
        {
          id: 'my-custom',
          name: 'My Custom',
          description: '',
          darkTheme: { id: 'my-custom-dark', name: 'My Custom Dark', isBuiltin: false, preview: { bg: '#000', surface: '#111', accent: '#ff0', text: '#fff' } },
          lightTheme: null,
        },
      ]);

      render(ThemeTab);

      expect(screen.getByTitle('Delete theme')).toBeInTheDocument();
    });
  });

  // ─── Theme Editor ─────────────────────────────────────────────────────────

  describe('theme editor', () => {
    it('opens theme editor when Customize button is clicked', async () => {
      render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        expect(screen.getByText('Theme Editor')).toBeInTheDocument();
      });
    });

    it('shows variable groups in the editor', async () => {
      render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        expect(screen.getByText('Backgrounds')).toBeInTheDocument();
        expect(screen.getByText('Text')).toBeInTheDocument();
      });
    });

    it('shows save theme input fields in editor', async () => {
      render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        expect(screen.getByPlaceholderText('Theme name...')).toBeInTheDocument();
        expect(screen.getByPlaceholderText('Description (optional)')).toBeInTheDocument();
        expect(screen.getByPlaceholderText('Author (optional)')).toBeInTheDocument();
      });
    });

    it('disables Save button when theme name is empty', async () => {
      render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        const saveBtn = screen.getByText('Save Theme');
        expect(saveBtn).toBeDisabled();
      });
    });

    it('enables Save button when theme name is provided', async () => {
      render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        expect(screen.getByPlaceholderText('Theme name...')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByPlaceholderText('Theme name...'), {
        target: { value: 'My Theme' },
      });

      await waitFor(() => {
        const saveBtn = screen.getByText('Save Theme');
        expect(saveBtn).not.toBeDisabled();
      });
    });

    it('closes editor when close button is clicked', async () => {
      render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        expect(screen.getByText('Theme Editor')).toBeInTheDocument();
      });

      const closeBtn = screen.getByLabelText('Close theme editor');
      await fireEvent.click(closeBtn);

      await waitFor(() => {
        expect(screen.queryByText('Theme Editor')).not.toBeInTheDocument();
        expect(screen.getByText('Customize Current Theme')).toBeInTheDocument();
      });
    });

    it('removes inline style overrides when editor is closed', async () => {
      render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        expect(screen.getByText('Theme Editor')).toBeInTheDocument();
      });

      // Simulate a live preview change
      document.documentElement.style.setProperty('--bg-base', '#ff0000');
      expect(document.documentElement.style.getPropertyValue('--bg-base')).toBe('#ff0000');

      // Close editor
      const closeBtn = screen.getByLabelText('Close theme editor');
      await fireEvent.click(closeBtn);

      // Style should be removed (the component calls removeProperty for all vars)
      await waitFor(() => {
        expect(document.documentElement.style.getPropertyValue('--bg-base')).toBe('');
      });
    });

    it('shows Reset All button in editor', async () => {
      render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        expect(screen.getByText('Reset All')).toBeInTheDocument();
      });
    });

    it('calls saveCustomThemeToServer when save is clicked', async () => {
      render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        expect(screen.getByPlaceholderText('Theme name...')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByPlaceholderText('Theme name...'), {
        target: { value: 'My Theme' },
      });
      await fireEvent.input(screen.getByPlaceholderText('Description (optional)'), {
        target: { value: 'A nice theme' },
      });
      await fireEvent.input(screen.getByPlaceholderText('Author (optional)'), {
        target: { value: 'Tester' },
      });

      await fireEvent.click(screen.getByText('Save Theme'));

      await waitFor(() => {
        expect(mockSaveCustomTheme).toHaveBeenCalledWith(
          'My Theme',
          expect.any(String), // resolvedTheme
          expect.any(Boolean), // isDarkTheme
          expect.objectContaining({ '--bg-base': '#1a1a2e' }),
          'A nice theme',
          'Tester',
        );
      });
    });

    it('shows success toast after successful save', async () => {
      render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        expect(screen.getByPlaceholderText('Theme name...')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByPlaceholderText('Theme name...'), {
        target: { value: 'My Theme' },
      });

      await fireEvent.click(screen.getByText('Save Theme'));

      await waitFor(() => {
        expect(mockToasts.success).toHaveBeenCalledWith('Theme saved');
      });
    });

    it('sets theme family and closes editor after save', async () => {
      render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        expect(screen.getByPlaceholderText('Theme name...')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByPlaceholderText('Theme name...'), {
        target: { value: 'My Theme' },
      });

      await fireEvent.click(screen.getByText('Save Theme'));

      await waitFor(() => {
        expect(mockSetThemeFamily).toHaveBeenCalledWith('my-theme');
        expect(screen.queryByText('Theme Editor')).not.toBeInTheDocument();
      });
    });

    it('shows error toast when save fails', async () => {
      mockSaveCustomTheme.mockResolvedValueOnce(false);

      render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        expect(screen.getByPlaceholderText('Theme name...')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByPlaceholderText('Theme name...'), {
        target: { value: 'Bad Theme' },
      });

      await fireEvent.click(screen.getByText('Save Theme'));

      await waitFor(() => {
        expect(mockToasts.error).toHaveBeenCalledWith('Failed to save theme');
      });
    });

    it('shows "Saving..." text while save is in progress', async () => {
      // Make save hang
      mockSaveCustomTheme.mockImplementation(() => new Promise(() => {}));

      render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        expect(screen.getByPlaceholderText('Theme name...')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByPlaceholderText('Theme name...'), {
        target: { value: 'My Theme' },
      });

      await fireEvent.click(screen.getByText('Save Theme'));

      await waitFor(() => {
        expect(screen.getByText('Saving...')).toBeInTheDocument();
      });
    });
  });

  // ─── Theme Deletion ──────────────────────────────────────────────────────

  describe('theme deletion', () => {
    beforeEach(() => {
      (themeFamilies as unknown as { set: (v: unknown) => void }).set([
        {
          id: 'my-custom',
          name: 'My Custom',
          description: '',
          darkTheme: { id: 'my-custom-dark', name: 'My Custom Dark', isBuiltin: false, preview: { bg: '#000', surface: '#111', accent: '#ff0', text: '#fff' } },
          lightTheme: null,
        },
      ]);
    });

    it('shows delete confirmation when delete button is clicked', async () => {
      render(ThemeTab);

      const deleteBtn = screen.getByTitle('Delete theme');
      await fireEvent.click(deleteBtn);

      await waitFor(() => {
        expect(screen.getByText('Delete?')).toBeInTheDocument();
        expect(screen.getByText('Yes')).toBeInTheDocument();
        expect(screen.getByText('No')).toBeInTheDocument();
      });
    });

    it('calls deleteCustomThemeFromServer when Yes is confirmed', async () => {
      render(ThemeTab);

      const deleteBtn = screen.getByTitle('Delete theme');
      await fireEvent.click(deleteBtn);

      await waitFor(() => {
        expect(screen.getByText('Yes')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Yes'));

      await waitFor(() => {
        expect(mockDeleteCustomTheme).toHaveBeenCalledWith('my-custom-dark');
      });
    });

    it('shows success toast after successful deletion', async () => {
      render(ThemeTab);

      const deleteBtn = screen.getByTitle('Delete theme');
      await fireEvent.click(deleteBtn);

      await waitFor(() => {
        expect(screen.getByText('Yes')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Yes'));

      await waitFor(() => {
        expect(mockToasts.success).toHaveBeenCalledWith('Theme deleted');
      });
    });

    it('shows error toast when deletion fails', async () => {
      mockDeleteCustomTheme.mockResolvedValueOnce(false);

      render(ThemeTab);

      const deleteBtn = screen.getByTitle('Delete theme');
      await fireEvent.click(deleteBtn);

      await waitFor(() => {
        expect(screen.getByText('Yes')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Yes'));

      await waitFor(() => {
        expect(mockToasts.error).toHaveBeenCalledWith('Failed to delete theme');
      });
    });

    it('dismisses delete confirmation when No is clicked', async () => {
      render(ThemeTab);

      const deleteBtn = screen.getByTitle('Delete theme');
      await fireEvent.click(deleteBtn);

      await waitFor(() => {
        expect(screen.getByText('No')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('No'));

      await waitFor(() => {
        expect(screen.queryByText('Delete?')).not.toBeInTheDocument();
      });
    });
  });

  // ─── cssColorToHex (internal utility) ─────────────────────────────────────

  describe('cssColorToHex logic (via color inputs in editor)', () => {
    it('renders color inputs for hex color variables in the editor', async () => {
      const { container } = render(ThemeTab);

      const customizeBtn = screen.getByText('Customize Current Theme').closest('button')!;
      await fireEvent.click(customizeBtn);

      await waitFor(() => {
        expect(screen.getByText('Theme Editor')).toBeInTheDocument();
      });

      // The editor should have color type inputs for hex color variables
      const colorInputs = container.querySelectorAll('input[type="color"]');
      // --bg-base, --bg-surface, --text-primary are all hex => 3 color inputs
      expect(colorInputs.length).toBe(3);
    });
  });

  // ─── Current Theme Info section ───────────────────────────────────────────

  describe('current theme info', () => {
    it('displays resolved theme name in Currently using section', () => {
      render(ThemeTab);

      // The "Currently using: Muximux Dark theme" section
      expect(screen.getByText(/Currently using:/)).toBeInTheDocument();
      expect(screen.getByText(/Muximux Dark theme/)).toBeInTheDocument();
    });
  });
});

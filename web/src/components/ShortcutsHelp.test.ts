import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

const { mockKeybindings } = vi.hoisted(() => {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const { writable } = require('svelte/store');
  const store = writable([
    {
      action: 'search',
      label: 'Command Palette',
      description: 'Open the command palette',
      category: 'navigation',
      editable: true,
      combos: [{ key: '/', ctrl: false, alt: false, shift: false, meta: false }],
    },
    {
      action: 'settings',
      label: 'Settings',
      description: 'Open settings',
      category: 'actions',
      editable: true,
      combos: [{ key: ',', ctrl: true, alt: false, shift: false, meta: false }],
    },
  ]);
  return { mockKeybindings: store };
});

vi.mock('$lib/keybindingsStore', () => ({
  keybindings: { subscribe: mockKeybindings.subscribe },
}));
vi.mock('$lib/useSwipe', () => ({
  isMobileViewport: vi.fn().mockReturnValue(false),
}));

import ShortcutsHelp from './ShortcutsHelp.svelte';

describe('ShortcutsHelp', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders "Keyboard Shortcuts" heading', () => {
    render(ShortcutsHelp);
    expect(screen.getByText('Keyboard Shortcuts')).toBeInTheDocument();
  });

  it('displays keybindings grouped by category', () => {
    render(ShortcutsHelp);
    // Navigation category with the search keybinding
    expect(screen.getByText('Navigation')).toBeInTheDocument();
    expect(screen.getByText('Command Palette')).toBeInTheDocument();
    // Actions category with the settings keybinding
    expect(screen.getByText('Actions')).toBeInTheDocument();
    expect(screen.getByText('Settings')).toBeInTheDocument();
  });

  it('has close button', () => {
    render(ShortcutsHelp);
    const closeButton = screen.getByRole('button', { name: 'Close' });
    expect(closeButton).toBeInTheDocument();
  });

  it('calls onclose when close button clicked', async () => {
    const onclose = vi.fn();
    render(ShortcutsHelp, { props: { onclose } });

    const closeButton = screen.getByRole('button', { name: 'Close' });
    await fireEvent.click(closeButton);

    expect(onclose).toHaveBeenCalled();
  });

  it('shows "Modal Navigation" section with Escape shortcut', () => {
    render(ShortcutsHelp);
    expect(screen.getByText('Modal Navigation')).toBeInTheDocument();
    expect(screen.getByText('Close modals / Go to home')).toBeInTheDocument();
  });

  it('shows customization hint', () => {
    render(ShortcutsHelp);
    expect(screen.getByText(/Customize shortcuts in/)).toBeInTheDocument();
    expect(screen.getByText(/Settings \u2192 Keybindings/)).toBeInTheDocument();
  });
});

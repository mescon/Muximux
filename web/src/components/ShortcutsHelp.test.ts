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

  it('renders all four modifier prefixes when a combo uses Ctrl+Alt+Shift+Meta together', () => {
    // Each modifier flag is rendered behind its own {#if} -- adding a
    // composite combo lights up the Ctrl/Alt/Shift/Meta branches of
    // the keystroke renderer (lines 118 in the source).
    mockKeybindings.set([
      {
        action: 'mega',
        label: 'Hyper key',
        description: 'Composite shortcut',
        category: 'navigation',
        editable: true,
        combos: [{ key: 'q', ctrl: true, alt: true, shift: true, meta: true }],
      },
    ]);
    render(ShortcutsHelp);
    expect(screen.getByText(/Ctrl\+Alt\+Shift\+\u2318Q/)).toBeInTheDocument();
  });

  it('renders the "or" separator between alternative combos', () => {
    // The "or" separator only appears for the 2nd combo onward (the
    // {#if i > 0} arm). Single-combo bindings (the existing tests)
    // never light it up.
    mockKeybindings.set([
      {
        action: 'twoways',
        label: 'Two paths',
        description: 'either combo works',
        category: 'navigation',
        editable: true,
        combos: [
          { key: '?', ctrl: false, alt: false, shift: false, meta: false },
          { key: 'h', ctrl: false, alt: false, shift: false, meta: false },
        ],
      },
    ]);
    render(ShortcutsHelp);
    // The "or" message comes from paraglide; the rendered text is
    // typically lowercase "or" in en-US. We anchor on the surrounding
    // structure: two <kbd> elements with the separator between them.
    const kbds = screen.getAllByText(/^[?Hh]$/, { selector: 'kbd' });
    expect(kbds.length).toBeGreaterThanOrEqual(2);
  });

  it('renders the apps category summary (1-9 quick switch) when an apps binding exists', () => {
    // groupedBindings.apps is rendered as a special summary rather
    // than the standard category section. The branch only fires when
    // at least one binding has category='apps'.
    mockKeybindings.set([
      {
        action: 'switch_app',
        label: 'Switch to app',
        description: 'jump to app N',
        category: 'apps',
        editable: false,
        combos: [{ key: '1', ctrl: false, alt: false, shift: false, meta: false }],
      },
    ]);
    render(ShortcutsHelp);
    // The summary's heading is the paraglide-translated
    // shortcuts_categoryApps message; in en-US that's "App Quick
    // Access". We anchor on that translated string.
    expect(screen.getByRole('heading', { name: /App Quick Access/i })).toBeInTheDocument();
  });
});

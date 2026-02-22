import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

// Hoist mock store and functions
const { mockKeybindings, mockIsCustomized, mockSetKeybinding, mockResetKeybinding, mockResetAllKeybindings, mockRemoveKeyCombo, mockFindConflicts } = vi.hoisted(() => {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const { writable } = require('svelte/store');

  return {
    mockKeybindings: writable([]),
    mockIsCustomized: vi.fn(() => false),
    mockSetKeybinding: vi.fn(),
    mockResetKeybinding: vi.fn(),
    mockResetAllKeybindings: vi.fn(),
    mockRemoveKeyCombo: vi.fn(),
    mockFindConflicts: vi.fn(() => []),
  };
});

vi.mock('$lib/keybindingsStore', () => ({
  keybindings: { subscribe: mockKeybindings.subscribe },
  formatKeyCombo: vi.fn((combo: { key: string; ctrl?: boolean; alt?: boolean; shift?: boolean; meta?: boolean }) => {
    const parts: string[] = [];
    if (combo.ctrl) parts.push('Ctrl');
    if (combo.alt) parts.push('Alt');
    if (combo.shift) parts.push('Shift');
    if (combo.meta) parts.push('\u2318');
    let key = combo.key;
    if (key.length === 1) key = key.toUpperCase();
    parts.push(key);
    return parts.join('+');
  }),
  eventToKeyCombo: vi.fn((event: KeyboardEvent) => ({
    key: event.key,
    ctrl: event.ctrlKey,
    alt: event.altKey,
    shift: event.shiftKey,
    meta: event.metaKey,
  })),
  setKeybinding: mockSetKeybinding,
  removeKeyCombo: mockRemoveKeyCombo,
  resetKeybinding: mockResetKeybinding,
  resetAllKeybindings: mockResetAllKeybindings,
  isCustomized: mockIsCustomized,
  findConflicts: mockFindConflicts,
  comboEquals: vi.fn((a: { key: string }, b: { key: string }) => a.key === b.key),
}));

import KeybindingsEditor from './KeybindingsEditor.svelte';

describe('KeybindingsEditor', () => {
  const defaultSampleBindings = [
    {
      action: 'search',
      label: 'Command Palette',
      description: 'Open the command palette',
      category: 'navigation',
      editable: true,
      combos: [{ key: 'k', ctrl: true }],
    },
    {
      action: 'settings',
      label: 'Open Settings',
      description: 'Open the settings panel',
      category: 'navigation',
      editable: true,
      combos: [{ key: 's' }],
    },
    {
      action: 'refresh',
      label: 'Refresh App',
      description: 'Refresh the current app iframe',
      category: 'actions',
      editable: true,
      combos: [{ key: 'r' }],
    },
    {
      action: 'fullscreen',
      label: 'Toggle Fullscreen',
      description: 'Hide/show navigation for fullscreen mode',
      category: 'actions',
      editable: true,
      combos: [{ key: 'f' }],
    },
    {
      action: 'app1',
      label: 'App 1',
      description: 'Switch to app #1',
      category: 'apps',
      editable: true,
      combos: [{ key: '1' }],
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    mockIsCustomized.mockReturnValue(false);
    mockFindConflicts.mockReturnValue([]);
    mockKeybindings.set(defaultSampleBindings);
  });

  describe('smoke test', () => {
    it('renders without crashing', () => {
      const { container } = render(KeybindingsEditor);
      expect(container.querySelector('div')).toBeTruthy();
    });
  });

  describe('instruction text', () => {
    it('shows instruction text about how to change keybindings', () => {
      render(KeybindingsEditor);
      expect(screen.getByText(/Click a keybinding to change it/)).toBeInTheDocument();
      expect(screen.getByText(/Press Escape to cancel/)).toBeInTheDocument();
    });
  });

  describe('binding categories', () => {
    it('shows Navigation category heading', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('Navigation')).toBeInTheDocument();
    });

    it('shows Actions category heading', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('Actions')).toBeInTheDocument();
    });

    it('shows App Quick Access category heading', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('App Quick Access')).toBeInTheDocument();
    });
  });

  describe('keybinding actions list', () => {
    it('shows Command Palette action in navigation category', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('Command Palette')).toBeInTheDocument();
    });

    it('shows Open Settings action in navigation category', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('Open Settings')).toBeInTheDocument();
    });

    it('shows Refresh App action in actions category', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('Refresh App')).toBeInTheDocument();
    });

    it('shows Toggle Fullscreen action in actions category', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('Toggle Fullscreen')).toBeInTheDocument();
    });

    it('shows App 1 action in apps category', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('App 1')).toBeInTheDocument();
    });
  });

  describe('key combo display', () => {
    it('shows formatted key combo for Command Palette (Ctrl+K)', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('Ctrl+K')).toBeInTheDocument();
    });

    it('shows formatted key combo for Open Settings (S)', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('S')).toBeInTheDocument();
    });

    it('shows formatted key combo for Refresh App (R)', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('R')).toBeInTheDocument();
    });

    it('shows formatted key combo for Toggle Fullscreen (F)', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('F')).toBeInTheDocument();
    });

    it('shows formatted key combo for App 1 (1)', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('1')).toBeInTheDocument();
    });
  });

  describe('action descriptions', () => {
    it('shows description for each binding', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('Open the command palette')).toBeInTheDocument();
      expect(screen.getByText('Open the settings panel')).toBeInTheDocument();
      expect(screen.getByText('Refresh the current app iframe')).toBeInTheDocument();
    });
  });

  describe('reset all button', () => {
    it('shows Reset All to Defaults button', () => {
      render(KeybindingsEditor);
      expect(screen.getByText('Reset All to Defaults')).toBeInTheDocument();
    });

    it('shows confirmation prompt when Reset All is clicked', async () => {
      render(KeybindingsEditor);
      const resetBtn = screen.getByText('Reset All to Defaults');
      await fireEvent.click(resetBtn);

      expect(screen.getByText('Reset all keybindings?')).toBeInTheDocument();
      expect(screen.getByText('Yes, Reset')).toBeInTheDocument();
      expect(screen.getByText('Cancel')).toBeInTheDocument();
    });

    it('calls resetAllKeybindings when confirmed', async () => {
      const onchange = vi.fn();
      render(KeybindingsEditor, { props: { onchange } });

      const resetBtn = screen.getByText('Reset All to Defaults');
      await fireEvent.click(resetBtn);

      const confirmBtn = screen.getByText('Yes, Reset');
      await fireEvent.click(confirmBtn);

      expect(mockResetAllKeybindings).toHaveBeenCalledTimes(1);
      expect(onchange).toHaveBeenCalledTimes(1);
    });

    it('hides confirmation prompt when Cancel is clicked', async () => {
      render(KeybindingsEditor);

      const resetBtn = screen.getByText('Reset All to Defaults');
      await fireEvent.click(resetBtn);

      expect(screen.getByText('Reset all keybindings?')).toBeInTheDocument();

      const cancelBtn = screen.getByText('Cancel');
      await fireEvent.click(cancelBtn);

      await waitFor(() => {
        expect(screen.queryByText('Reset all keybindings?')).not.toBeInTheDocument();
      });
    });
  });

  describe('customized indicator', () => {
    it('shows (customized) tag when a binding is customized', () => {
      mockIsCustomized.mockReturnValue(true);
      render(KeybindingsEditor);

      const customizedTags = screen.getAllByText('(customized)');
      expect(customizedTags.length).toBeGreaterThan(0);
    });

    it('does not show (customized) tag when no bindings are customized', () => {
      mockIsCustomized.mockReturnValue(false);
      render(KeybindingsEditor);

      expect(screen.queryByText('(customized)')).not.toBeInTheDocument();
    });
  });

  describe('add shortcut button', () => {
    it('shows add alternative shortcut button for editable bindings', () => {
      render(KeybindingsEditor);
      const addButtons = screen.getAllByTitle('Add alternative shortcut');
      expect(addButtons.length).toBeGreaterThan(0);
    });
  });

  describe('key capture mode', () => {
    it('shows "Press keys..." when a combo button is clicked to start capture', async () => {
      render(KeybindingsEditor);

      // Click the "S" key combo button for Open Settings to start capture
      const sKeyButton = screen.getByText('S').closest('button') as HTMLButtonElement;
      await fireEvent.click(sKeyButton);

      await waitFor(() => {
        expect(screen.getByText('Press keys...')).toBeInTheDocument();
      });
    });

    it('shows "Press keys..." when adding a new alternative shortcut', async () => {
      render(KeybindingsEditor);

      // Click the "Add alternative shortcut" button for the first binding
      const addButtons = screen.getAllByTitle('Add alternative shortcut');
      await fireEvent.click(addButtons[0]);

      await waitFor(() => {
        expect(screen.getByText('Press keys...')).toBeInTheDocument();
      });
    });

    it('captures a key press and shows the confirmation modal', async () => {
      render(KeybindingsEditor);

      // Start capture on Open Settings (S key)
      const sKeyButton = screen.getByText('S').closest('button') as HTMLButtonElement;
      await fireEvent.click(sKeyButton);

      // Press a key (e.g., "a")
      await fireEvent.keyDown(window, { key: 'a', ctrlKey: false, altKey: false, shiftKey: false, metaKey: false });

      await waitFor(() => {
        // The confirmation modal should appear with the captured key
        expect(screen.getByText('Set Keybinding')).toBeInTheDocument();
        // The captured key appears in both inline and modal, use getAllByText
        const aElements = screen.getAllByText('A');
        expect(aElements.length).toBeGreaterThanOrEqual(1);
      });
    });

    it('cancels capture when Escape is pressed', async () => {
      render(KeybindingsEditor);

      // Start capture
      const sKeyButton = screen.getByText('S').closest('button') as HTMLButtonElement;
      await fireEvent.click(sKeyButton);

      expect(screen.getByText('Press keys...')).toBeInTheDocument();

      // Press Escape
      await fireEvent.keyDown(window, { key: 'Escape' });

      await waitFor(() => {
        expect(screen.queryByText('Press keys...')).not.toBeInTheDocument();
      });
    });

    it('ignores lone modifier keys during capture', async () => {
      render(KeybindingsEditor);

      // Start capture
      const sKeyButton = screen.getByText('S').closest('button') as HTMLButtonElement;
      await fireEvent.click(sKeyButton);

      // Press a lone modifier key
      await fireEvent.keyDown(window, { key: 'Control' });

      // Should still show "Press keys..." (not captured)
      expect(screen.getByText('Press keys...')).toBeInTheDocument();
      expect(screen.queryByText('Set Keybinding')).not.toBeInTheDocument();
    });

    it('ignores lone Alt modifier key during capture', async () => {
      render(KeybindingsEditor);

      const sKeyButton = screen.getByText('S').closest('button') as HTMLButtonElement;
      await fireEvent.click(sKeyButton);

      await fireEvent.keyDown(window, { key: 'Alt' });

      expect(screen.getByText('Press keys...')).toBeInTheDocument();
    });

    it('ignores lone Shift modifier key during capture', async () => {
      render(KeybindingsEditor);

      const sKeyButton = screen.getByText('S').closest('button') as HTMLButtonElement;
      await fireEvent.click(sKeyButton);

      await fireEvent.keyDown(window, { key: 'Shift' });

      expect(screen.getByText('Press keys...')).toBeInTheDocument();
    });

    it('ignores lone Meta modifier key during capture', async () => {
      render(KeybindingsEditor);

      const sKeyButton = screen.getByText('S').closest('button') as HTMLButtonElement;
      await fireEvent.click(sKeyButton);

      await fireEvent.keyDown(window, { key: 'Meta' });

      expect(screen.getByText('Press keys...')).toBeInTheDocument();
    });

    it('does not respond to keydown when not in capture mode', async () => {
      render(KeybindingsEditor);

      // Press a key without being in capture mode
      await fireEvent.keyDown(window, { key: 'a' });

      // No modal should appear
      expect(screen.queryByText('Set Keybinding')).not.toBeInTheDocument();
    });

    it('captures a key combo with modifiers', async () => {
      render(KeybindingsEditor);

      // Start capture on Refresh App (R key)
      const rKeyButton = screen.getByText('R').closest('button') as HTMLButtonElement;
      await fireEvent.click(rKeyButton);

      // Press Ctrl+Shift+X
      await fireEvent.keyDown(window, { key: 'x', ctrlKey: true, shiftKey: true, altKey: false, metaKey: false });

      await waitFor(() => {
        expect(screen.getByText('Set Keybinding')).toBeInTheDocument();
        // The formatted combo appears in both inline and modal
        const comboElements = screen.getAllByText('Ctrl+Shift+X');
        expect(comboElements.length).toBeGreaterThanOrEqual(1);
      });
    });
  });

  describe('confirmation modal', () => {
    it('shows Confirm button when there are no conflicts', async () => {
      mockFindConflicts.mockReturnValue([]);
      render(KeybindingsEditor);

      // Start capture and press a key
      const sKeyButton = screen.getByText('S').closest('button') as HTMLButtonElement;
      await fireEvent.click(sKeyButton);
      await fireEvent.keyDown(window, { key: 'a' });

      await waitFor(() => {
        expect(screen.getByText('Confirm')).toBeInTheDocument();
      });
    });

    it('shows "Use Anyway" button when there are conflicts', async () => {
      // Reset store to ensure clean state
      const sampleBindings = [
        {
          action: 'settings',
          label: 'Open Settings',
          description: 'Open the settings panel',
          category: 'navigation',
          editable: true,
          combos: [{ key: 's' }],
        },
        {
          action: 'refresh',
          label: 'Refresh App',
          description: 'Refresh the current app iframe',
          category: 'actions',
          editable: true,
          combos: [{ key: 'r' }],
        },
      ];
      mockKeybindings.set(sampleBindings);
      mockFindConflicts.mockReturnValue([
        { action: 'refresh', label: 'Refresh App', description: 'Refresh', category: 'actions', editable: true, combos: [{ key: 'a' }] }
      ]);

      render(KeybindingsEditor);

      // Start capture and press a key
      const sKeyButton = screen.getByText('S').closest('button') as HTMLButtonElement;
      await fireEvent.click(sKeyButton);
      await fireEvent.keyDown(window, { key: 'a' });

      await waitFor(() => {
        expect(screen.getByText('Use Anyway')).toBeInTheDocument();
        expect(screen.getByText('Conflict detected')).toBeInTheDocument();
      });
    });

    it('calls setKeybinding and onchange when Confirm is clicked', async () => {
      const onchange = vi.fn();
      render(KeybindingsEditor, { props: { onchange } });

      // Start capture on settings
      const sKeyButton = screen.getByText('S').closest('button') as HTMLButtonElement;
      await fireEvent.click(sKeyButton);
      await fireEvent.keyDown(window, { key: 'a' });

      await waitFor(() => {
        expect(screen.getByText('Confirm')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Confirm'));

      expect(mockSetKeybinding).toHaveBeenCalledTimes(1);
      expect(mockSetKeybinding).toHaveBeenCalledWith('settings', expect.any(Array));
      expect(onchange).toHaveBeenCalledTimes(1);
    });

    it('closes the modal when Cancel is clicked in the confirmation dialog', async () => {
      render(KeybindingsEditor);

      // Start capture and press a key
      const sKeyButton = screen.getByText('S').closest('button') as HTMLButtonElement;
      await fireEvent.click(sKeyButton);
      await fireEvent.keyDown(window, { key: 'a' });

      await waitFor(() => {
        expect(screen.getByText('Set Keybinding')).toBeInTheDocument();
      });

      // Click Cancel in the modal
      const cancelButtons = screen.getAllByText('Cancel');
      // The modal Cancel button is the last one
      await fireEvent.click(cancelButtons[cancelButtons.length - 1]);

      await waitFor(() => {
        expect(screen.queryByText('Set Keybinding')).not.toBeInTheDocument();
      });
    });

    it('adds a new combo when confirming from "Add alternative shortcut"', async () => {
      const onchange = vi.fn();
      render(KeybindingsEditor, { props: { onchange } });

      // Click "Add alternative shortcut" for the first binding (search)
      const addButtons = screen.getAllByTitle('Add alternative shortcut');
      await fireEvent.click(addButtons[0]);

      // Press a key
      await fireEvent.keyDown(window, { key: 'j' });

      await waitFor(() => {
        expect(screen.getByText('Set Keybinding')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Confirm'));

      expect(mockSetKeybinding).toHaveBeenCalledTimes(1);
      // Should include original combo plus the new one
      expect(mockSetKeybinding).toHaveBeenCalledWith('search', expect.arrayContaining([
        expect.objectContaining({ key: 'k', ctrl: true }),
        expect.objectContaining({ key: 'j' }),
      ]));
      expect(onchange).toHaveBeenCalledTimes(1);
    });
  });

  describe('remove combo', () => {
    it('shows remove button when binding has multiple combos', () => {
      const multiComboBindings = [
        {
          action: 'search',
          label: 'Command Palette',
          description: 'Open the command palette',
          category: 'navigation',
          editable: true,
          combos: [{ key: 'k', ctrl: true }, { key: 'p', ctrl: true }],
        },
      ];
      mockKeybindings.set(multiComboBindings);

      render(KeybindingsEditor);

      const removeButtons = screen.getAllByTitle('Remove this shortcut');
      expect(removeButtons.length).toBe(2);
    });

    it('calls removeKeyCombo when remove button is clicked', async () => {
      const onchange = vi.fn();
      const multiComboBindings = [
        {
          action: 'search',
          label: 'Command Palette',
          description: 'Open the command palette',
          category: 'navigation',
          editable: true,
          combos: [{ key: 'k', ctrl: true }, { key: 'p', ctrl: true }],
        },
      ];
      mockKeybindings.set(multiComboBindings);

      render(KeybindingsEditor, { props: { onchange } });

      const removeButtons = screen.getAllByTitle('Remove this shortcut');
      await fireEvent.click(removeButtons[0]);

      expect(mockRemoveKeyCombo).toHaveBeenCalledWith('search', 0);
      expect(onchange).toHaveBeenCalledTimes(1);
    });

    it('shows "or" separator between multiple combos', () => {
      const multiComboBindings = [
        {
          action: 'search',
          label: 'Command Palette',
          description: 'Open the command palette',
          category: 'navigation',
          editable: true,
          combos: [{ key: 'k', ctrl: true }, { key: 'p', ctrl: true }],
        },
      ];
      mockKeybindings.set(multiComboBindings);

      render(KeybindingsEditor);

      expect(screen.getByText('or')).toBeInTheDocument();
    });
  });

  describe('reset individual binding', () => {
    it('shows reset button when a binding is customized', () => {
      mockIsCustomized.mockReturnValue(true);
      render(KeybindingsEditor);

      const resetButtons = screen.getAllByTitle('Reset to default');
      expect(resetButtons.length).toBeGreaterThan(0);
    });

    it('calls resetKeybinding and onchange when reset button is clicked', async () => {
      const onchange = vi.fn();
      mockIsCustomized.mockReturnValue(true);
      render(KeybindingsEditor, { props: { onchange } });

      const resetButtons = screen.getAllByTitle('Reset to default');
      await fireEvent.click(resetButtons[0]);

      expect(mockResetKeybinding).toHaveBeenCalledTimes(1);
      expect(onchange).toHaveBeenCalledTimes(1);
    });

    it('does not show reset button when binding is not customized', () => {
      mockIsCustomized.mockReturnValue(false);
      render(KeybindingsEditor);

      expect(screen.queryByTitle('Reset to default')).not.toBeInTheDocument();
    });
  });

  describe('non-editable bindings', () => {
    it('disables the key combo button for non-editable bindings', () => {
      const bindingsWithNonEditable = [
        {
          action: 'escape',
          label: 'Close Dialog',
          description: 'Close any open dialog',
          category: 'navigation',
          editable: false,
          combos: [{ key: 'Escape' }],
        },
      ];
      mockKeybindings.set(bindingsWithNonEditable);

      render(KeybindingsEditor);

      const escapeButton = screen.getByText('Escape').closest('button') as HTMLButtonElement;
      expect(escapeButton.disabled).toBe(true);
    });

    it('does not show add alternative shortcut button for non-editable bindings', () => {
      const bindingsWithNonEditable = [
        {
          action: 'escape',
          label: 'Close Dialog',
          description: 'Close any open dialog',
          category: 'navigation',
          editable: false,
          combos: [{ key: 'Escape' }],
        },
      ];
      mockKeybindings.set(bindingsWithNonEditable);

      render(KeybindingsEditor);

      expect(screen.queryByTitle('Add alternative shortcut')).not.toBeInTheDocument();
    });
  });

  describe('remove combo via keyboard', () => {
    it('calls removeKeyCombo when Enter is pressed on the remove button', async () => {
      const onchange = vi.fn();
      const multiComboBindings = [
        {
          action: 'search',
          label: 'Command Palette',
          description: 'Open the command palette',
          category: 'navigation',
          editable: true,
          combos: [{ key: 'k', ctrl: true }, { key: 'p', ctrl: true }],
        },
      ];
      mockKeybindings.set(multiComboBindings);

      render(KeybindingsEditor, { props: { onchange } });

      const removeButtons = screen.getAllByTitle('Remove this shortcut');
      await fireEvent.keyDown(removeButtons[0], { key: 'Enter' });

      expect(mockRemoveKeyCombo).toHaveBeenCalledWith('search', 0);
      expect(onchange).toHaveBeenCalledTimes(1);
    });

    it('calls removeKeyCombo when Space is pressed on the remove button', async () => {
      const onchange = vi.fn();
      const multiComboBindings = [
        {
          action: 'search',
          label: 'Command Palette',
          description: 'Open the command palette',
          category: 'navigation',
          editable: true,
          combos: [{ key: 'k', ctrl: true }, { key: 'p', ctrl: true }],
        },
      ];
      mockKeybindings.set(multiComboBindings);

      render(KeybindingsEditor, { props: { onchange } });

      const removeButtons = screen.getAllByTitle('Remove this shortcut');
      await fireEvent.keyDown(removeButtons[0], { key: ' ' });

      expect(mockRemoveKeyCombo).toHaveBeenCalledWith('search', 0);
      expect(onchange).toHaveBeenCalledTimes(1);
    });
  });

  describe('adding new combo UI', () => {
    it('shows "or" separator when adding a new combo', async () => {
      render(KeybindingsEditor);

      // Click "Add alternative shortcut" for the first binding
      const addButtons = screen.getAllByTitle('Add alternative shortcut');
      await fireEvent.click(addButtons[0]);

      await waitFor(() => {
        // Should show the "or" text before the "Press keys..." placeholder
        const orTexts = screen.getAllByText('or');
        expect(orTexts.length).toBeGreaterThan(0);
      });
    });

    it('hides add button during capture for that binding', async () => {
      render(KeybindingsEditor);

      const addButtonsBefore = screen.getAllByTitle('Add alternative shortcut');
      const initialCount = addButtonsBefore.length;

      // Start capture on first binding
      await fireEvent.click(addButtonsBefore[0]);

      await waitFor(() => {
        // One fewer add button should be shown
        const addButtonsAfter = screen.queryAllByTitle('Add alternative shortcut');
        expect(addButtonsAfter.length).toBe(initialCount - 1);
      });
    });
  });

  describe('confirm capture with replacement', () => {
    it('replaces existing combo when confirming replace mode', async () => {
      const onchange = vi.fn();

      // Reset store to standard single-combo bindings
      const sampleBindings = [
        {
          action: 'settings',
          label: 'Open Settings',
          description: 'Open the settings panel',
          category: 'navigation',
          editable: true,
          combos: [{ key: 's' }],
        },
      ];
      mockKeybindings.set(sampleBindings);

      render(KeybindingsEditor, { props: { onchange } });

      // Click on the existing "S" combo to replace it
      const sKeyButton = screen.getByText('S').closest('button') as HTMLButtonElement;
      await fireEvent.click(sKeyButton);

      // Press a new key
      await fireEvent.keyDown(window, { key: 'x' });

      await waitFor(() => {
        expect(screen.getByText('Set Keybinding')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Confirm'));

      expect(mockSetKeybinding).toHaveBeenCalledWith('settings', [
        expect.objectContaining({ key: 'x' }),
      ]);
      expect(onchange).toHaveBeenCalledTimes(1);
    });
  });
});

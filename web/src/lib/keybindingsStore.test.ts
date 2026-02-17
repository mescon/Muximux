import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import {
  keybindings,
  DEFAULT_KEYBINDINGS,
  formatKeyCombo,
  formatKeybinding,
  eventToKeyCombo,
  matchesCombo,
  matchesKeybinding,
  findAction,
  setKeybinding,
  addKeyCombo,
  removeKeyCombo,
  resetKeybinding,
  resetAllKeybindings,
  isCustomized,
  findConflicts,
  comboEquals,
  exportKeybindings,
  importKeybindings,
  initKeybindings,
  getKeybindingsForConfig,
  getKeybindingsByCategory,
} from './keybindingsStore';

describe('keybindingsStore', () => {
  beforeEach(() => {
    // Reset all keybindings before each test
    resetAllKeybindings();
  });

  describe('DEFAULT_KEYBINDINGS', () => {
    it('should have all expected actions', () => {
      const actions = DEFAULT_KEYBINDINGS.map(b => b.action);
      expect(actions).toContain('search');
      expect(actions).toContain('settings');
      expect(actions).toContain('shortcuts');
      expect(actions).toContain('refresh');
      expect(actions).toContain('fullscreen');
      expect(actions).toContain('nextApp');
      expect(actions).toContain('prevApp');
      expect(actions).toContain('app1');
    });

    it('should have combos for each binding', () => {
      for (const binding of DEFAULT_KEYBINDINGS) {
        expect(binding.combos.length).toBeGreaterThan(0);
      }
    });
  });

  describe('formatKeyCombo', () => {
    it('should format simple key', () => {
      expect(formatKeyCombo({ key: 'a' })).toBe('A');
      expect(formatKeyCombo({ key: '/' })).toBe('/');
    });

    it('should format key with Ctrl', () => {
      expect(formatKeyCombo({ key: 'k', ctrl: true })).toBe('Ctrl+K');
    });

    it('should format key with multiple modifiers', () => {
      expect(formatKeyCombo({ key: 'p', ctrl: true, shift: true })).toBe('Ctrl+Shift+P');
    });

    it('should format special keys', () => {
      expect(formatKeyCombo({ key: 'Tab' })).toBe('Tab');
      expect(formatKeyCombo({ key: ' ' })).toBe('Space');
      expect(formatKeyCombo({ key: 'ArrowUp' })).toBe('↑');
      expect(formatKeyCombo({ key: 'ArrowDown' })).toBe('↓');
    });

    it('should format meta key', () => {
      expect(formatKeyCombo({ key: 's', meta: true })).toBe('⌘+S');
    });
  });

  describe('formatKeybinding', () => {
    it('should format single combo binding', () => {
      const binding = DEFAULT_KEYBINDINGS.find(b => b.action === 'refresh')!;
      expect(formatKeybinding(binding)).toBe('R');
    });

    it('should format single combo search binding', () => {
      const binding = DEFAULT_KEYBINDINGS.find(b => b.action === 'search')!;
      const result = formatKeybinding(binding);
      expect(result).toBe('Ctrl+K');
    });
  });

  describe('eventToKeyCombo', () => {
    it('should convert keyboard event to KeyCombo', () => {
      const event = new KeyboardEvent('keydown', {
        key: 'k',
        ctrlKey: true,
        shiftKey: false,
        altKey: false,
        metaKey: false,
      });

      const combo = eventToKeyCombo(event);
      expect(combo.key).toBe('k');
      expect(combo.ctrl).toBe(true);
      expect(combo.shift).toBe(false);
    });
  });

  describe('matchesCombo', () => {
    it('should match simple key', () => {
      const event = new KeyboardEvent('keydown', { key: 'r' });
      expect(matchesCombo(event, { key: 'r' })).toBe(true);
      expect(matchesCombo(event, { key: 'R' })).toBe(true); // Case insensitive
    });

    it('should not match different key', () => {
      const event = new KeyboardEvent('keydown', { key: 'r' });
      expect(matchesCombo(event, { key: 'f' })).toBe(false);
    });

    it('should match key with modifiers', () => {
      const event = new KeyboardEvent('keydown', { key: 'k', ctrlKey: true });
      expect(matchesCombo(event, { key: 'k', ctrl: true })).toBe(true);
      expect(matchesCombo(event, { key: 'k' })).toBe(false); // Missing modifier
    });

    it('should not match when extra modifiers present', () => {
      const event = new KeyboardEvent('keydown', { key: 'k', ctrlKey: true, shiftKey: true });
      expect(matchesCombo(event, { key: 'k', ctrl: true })).toBe(false);
    });
  });

  describe('matchesKeybinding', () => {
    it('should match combo in binding', () => {
      const binding = DEFAULT_KEYBINDINGS.find(b => b.action === 'search')!;

      const ctrlKEvent = new KeyboardEvent('keydown', { key: 'k', ctrlKey: true });
      expect(matchesKeybinding(ctrlKEvent, binding)).toBe(true);

      const plainKEvent = new KeyboardEvent('keydown', { key: 'k' });
      expect(matchesKeybinding(plainKEvent, binding)).toBe(false);
    });
  });

  describe('findAction', () => {
    it('should find action for matching key event', () => {
      const event = new KeyboardEvent('keydown', { key: 'r' });
      expect(findAction(event)).toBe('refresh');
    });

    it('should return null for non-matching event', () => {
      const event = new KeyboardEvent('keydown', { key: 'x' });
      expect(findAction(event)).toBeNull();
    });

    it('should find action for key combo', () => {
      const event = new KeyboardEvent('keydown', { key: 'k', ctrlKey: true });
      expect(findAction(event)).toBe('search');
    });
  });

  describe('setKeybinding', () => {
    it('should update keybinding for action', () => {
      setKeybinding('refresh', [{ key: 'r', ctrl: true }]);

      const bindings = get(keybindings);
      const refreshBinding = bindings.find(b => b.action === 'refresh')!;
      expect(refreshBinding.combos).toEqual([{ key: 'r', ctrl: true }]);
    });

    it('should be retrievable via getKeybindingsForConfig', () => {
      setKeybinding('refresh', [{ key: 'r', ctrl: true }]);

      const config = getKeybindingsForConfig();
      expect(config.bindings).toBeDefined();
      expect(config.bindings!.refresh).toEqual([{ key: 'r', ctrl: true }]);
    });
  });

  describe('addKeyCombo', () => {
    it('should add combo to existing binding', () => {
      addKeyCombo('refresh', { key: 'F5' });

      const bindings = get(keybindings);
      const refreshBinding = bindings.find(b => b.action === 'refresh')!;
      expect(refreshBinding.combos.length).toBe(2);
      expect(refreshBinding.combos).toContainEqual({ key: 'F5' });
    });
  });

  describe('removeKeyCombo', () => {
    it('should remove combo from binding with multiple combos', () => {
      // Add a second combo first
      addKeyCombo('refresh', { key: 'F5' });

      // Remove the first combo
      removeKeyCombo('refresh', 0);

      const bindings = get(keybindings);
      const refreshBinding = bindings.find(b => b.action === 'refresh')!;
      expect(refreshBinding.combos.length).toBe(1);
      expect(refreshBinding.combos[0].key).toBe('F5');
    });

    it('should not remove last combo', () => {
      // Try to remove the only combo
      removeKeyCombo('refresh', 0);

      const bindings = get(keybindings);
      const refreshBinding = bindings.find(b => b.action === 'refresh')!;
      expect(refreshBinding.combos.length).toBe(1); // Still has one
    });
  });

  describe('resetKeybinding', () => {
    it('should reset single binding to default', () => {
      setKeybinding('refresh', [{ key: 'F5' }]);
      expect(isCustomized('refresh')).toBe(true);

      resetKeybinding('refresh');

      expect(isCustomized('refresh')).toBe(false);
      const bindings = get(keybindings);
      const refreshBinding = bindings.find(b => b.action === 'refresh')!;
      expect(refreshBinding.combos).toEqual([{ key: 'r' }]);
    });
  });

  describe('resetAllKeybindings', () => {
    it('should reset all bindings to defaults', () => {
      setKeybinding('refresh', [{ key: 'F5' }]);
      setKeybinding('fullscreen', [{ key: 'F11' }]);

      resetAllKeybindings();

      expect(isCustomized('refresh')).toBe(false);
      expect(isCustomized('fullscreen')).toBe(false);
    });
  });

  describe('isCustomized', () => {
    it('should return false for default bindings', () => {
      expect(isCustomized('refresh')).toBe(false);
    });

    it('should return true after customization', () => {
      setKeybinding('refresh', [{ key: 'F5' }]);
      expect(isCustomized('refresh')).toBe(true);
    });
  });

  describe('findConflicts', () => {
    it('should find conflicting bindings', () => {
      const conflicts = findConflicts({ key: 'r' });
      expect(conflicts.length).toBe(1);
      expect(conflicts[0].action).toBe('refresh');
    });

    it('should exclude specified action', () => {
      const conflicts = findConflicts({ key: 'r' }, 'refresh');
      expect(conflicts.length).toBe(0);
    });

    it('should return empty for non-conflicting combo', () => {
      const conflicts = findConflicts({ key: 'x', ctrl: true, shift: true });
      expect(conflicts.length).toBe(0);
    });
  });

  describe('comboEquals', () => {
    it('should match equal combos', () => {
      expect(comboEquals({ key: 'r' }, { key: 'r' })).toBe(true);
      expect(comboEquals({ key: 'r' }, { key: 'R' })).toBe(true); // Case insensitive
    });

    it('should match combos with same modifiers', () => {
      expect(comboEquals(
        { key: 'k', ctrl: true },
        { key: 'k', ctrl: true }
      )).toBe(true);
    });

    it('should not match different combos', () => {
      expect(comboEquals({ key: 'r' }, { key: 'f' })).toBe(false);
      expect(comboEquals(
        { key: 'k' },
        { key: 'k', ctrl: true }
      )).toBe(false);
    });
  });

  describe('exportKeybindings', () => {
    it('should export custom bindings as JSON', () => {
      setKeybinding('refresh', [{ key: 'F5' }]);

      const json = exportKeybindings();
      const parsed = JSON.parse(json);

      expect(parsed.refresh).toEqual([{ key: 'F5' }]);
    });
  });

  describe('importKeybindings', () => {
    it('should import valid JSON', () => {
      const json = JSON.stringify({ refresh: [{ key: 'F5' }] });
      const success = importKeybindings(json);

      expect(success).toBe(true);
      expect(isCustomized('refresh')).toBe(true);
    });

    it('should reject invalid JSON', () => {
      const success = importKeybindings('not json');
      expect(success).toBe(false);
    });

    it('should reject non-object JSON', () => {
      const success = importKeybindings('"string"');
      expect(success).toBe(false);
    });

    it('should reject null JSON', () => {
      const success = importKeybindings('null');
      expect(success).toBe(false);
    });

    it('should reject number JSON', () => {
      const success = importKeybindings('42');
      expect(success).toBe(false);
    });

    it('should accept array JSON (typeof array is object)', () => {
      // Arrays are objects in JS, so importKeybindings accepts them
      const success = importKeybindings('[]');
      expect(success).toBe(true);
    });
  });

  describe('getKeybindingsByCategory', () => {
    it('should group bindings by category', () => {
      const grouped = getKeybindingsByCategory();
      expect(grouped).toHaveProperty('navigation');
      expect(grouped).toHaveProperty('actions');
      expect(grouped).toHaveProperty('apps');
    });

    it('should include all bindings across categories', () => {
      const grouped = getKeybindingsByCategory();
      const total = Object.values(grouped).reduce(
        (sum: number, bindings) => sum + bindings.length,
        0
      );
      expect(total).toBe(DEFAULT_KEYBINDINGS.length);
    });
  });

  describe('initKeybindings', () => {
    it('should initialize from config', () => {
      initKeybindings({
        bindings: {
          refresh: [{ key: 'F5' }]
        }
      });

      expect(isCustomized('refresh')).toBe(true);
      const bindings = get(keybindings);
      const refreshBinding = bindings.find(b => b.action === 'refresh')!;
      expect(refreshBinding.combos).toEqual([{ key: 'F5' }]);
    });

    it('should handle undefined config', () => {
      initKeybindings(undefined);
      expect(isCustomized('refresh')).toBe(false);
    });

    it('should handle config without bindings', () => {
      initKeybindings({});
      expect(isCustomized('refresh')).toBe(false);
    });
  });

  describe('getKeybindingsForConfig', () => {
    it('should return empty bindings when no customizations', () => {
      const config = getKeybindingsForConfig();
      expect(config.bindings).toBeUndefined();
    });

    it('should return custom bindings', () => {
      setKeybinding('refresh', [{ key: 'F5' }]);
      setKeybinding('fullscreen', [{ key: 'F11' }]);

      const config = getKeybindingsForConfig();
      expect(config.bindings).toBeDefined();
      expect(config.bindings!.refresh).toEqual([{ key: 'F5' }]);
      expect(config.bindings!.fullscreen).toEqual([{ key: 'F11' }]);
    });
  });
});

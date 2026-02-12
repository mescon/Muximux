import { writable, derived, get } from 'svelte/store';
import type { KeyCombo, KeybindingsConfig } from './types';

// Re-export KeyCombo for convenience
export { type KeyCombo } from './types';

/**
 * Keybinding action identifiers
 */
export type KeyAction =
  | 'search'
  | 'settings'
  | 'shortcuts'
  | 'refresh'
  | 'fullscreen'
  | 'home'
  | 'nextApp'
  | 'prevApp'
  | 'app1' | 'app2' | 'app3' | 'app4' | 'app5'
  | 'app6' | 'app7' | 'app8' | 'app9';

/**
 * A keybinding definition
 */
export interface Keybinding {
  action: KeyAction;
  label: string;
  description: string;
  combos: KeyCombo[];    // Multiple combos can trigger the same action
  category: 'navigation' | 'actions' | 'apps';
  editable: boolean;     // Some bindings like Escape shouldn't be editable
}

/**
 * Storage format for custom bindings (matches server-side format)
 */
export interface StoredKeybindings {
  [action: string]: KeyCombo[];
}

/**
 * Default keybindings configuration
 */
export const DEFAULT_KEYBINDINGS: Keybinding[] = [
  // Navigation
  {
    action: 'search',
    label: 'Command Palette',
    description: 'Open the command palette',
    combos: [
      { key: '/' },
      { key: 'k', ctrl: true },
      { key: 'p', ctrl: true, shift: true }
    ],
    category: 'navigation',
    editable: true
  },
  {
    action: 'settings',
    label: 'Open Settings',
    description: 'Open the settings panel',
    combos: [
      { key: ',', ctrl: true }
    ],
    category: 'navigation',
    editable: true
  },
  {
    action: 'shortcuts',
    label: 'Show Shortcuts',
    description: 'Show keyboard shortcuts help',
    combos: [
      { key: '?' }
    ],
    category: 'navigation',
    editable: true
  },
  {
    action: 'home',
    label: 'Go Home',
    description: 'Return to the splash screen',
    combos: [
      { key: 'h', alt: true }
    ],
    category: 'navigation',
    editable: true
  },

  // Actions
  {
    action: 'refresh',
    label: 'Refresh App',
    description: 'Refresh the current app iframe',
    combos: [
      { key: 'r' }
    ],
    category: 'actions',
    editable: true
  },
  {
    action: 'fullscreen',
    label: 'Toggle Fullscreen',
    description: 'Hide/show navigation for fullscreen mode',
    combos: [
      { key: 'f' }
    ],
    category: 'actions',
    editable: true
  },
  {
    action: 'nextApp',
    label: 'Next App',
    description: 'Switch to the next app in the list',
    combos: [
      { key: 'Tab' }
    ],
    category: 'actions',
    editable: true
  },
  {
    action: 'prevApp',
    label: 'Previous App',
    description: 'Switch to the previous app in the list',
    combos: [
      { key: 'Tab', shift: true }
    ],
    category: 'actions',
    editable: true
  },

  // App quick access (1-9)
  {
    action: 'app1',
    label: 'App 1',
    description: 'Switch to app #1',
    combos: [{ key: '1' }],
    category: 'apps',
    editable: true
  },
  {
    action: 'app2',
    label: 'App 2',
    description: 'Switch to app #2',
    combos: [{ key: '2' }],
    category: 'apps',
    editable: true
  },
  {
    action: 'app3',
    label: 'App 3',
    description: 'Switch to app #3',
    combos: [{ key: '3' }],
    category: 'apps',
    editable: true
  },
  {
    action: 'app4',
    label: 'App 4',
    description: 'Switch to app #4',
    combos: [{ key: '4' }],
    category: 'apps',
    editable: true
  },
  {
    action: 'app5',
    label: 'App 5',
    description: 'Switch to app #5',
    combos: [{ key: '5' }],
    category: 'apps',
    editable: true
  },
  {
    action: 'app6',
    label: 'App 6',
    description: 'Switch to app #6',
    combos: [{ key: '6' }],
    category: 'apps',
    editable: true
  },
  {
    action: 'app7',
    label: 'App 7',
    description: 'Switch to app #7',
    combos: [{ key: '7' }],
    category: 'apps',
    editable: true
  },
  {
    action: 'app8',
    label: 'App 8',
    description: 'Switch to app #8',
    combos: [{ key: '8' }],
    category: 'apps',
    editable: true
  },
  {
    action: 'app9',
    label: 'App 9',
    description: 'Switch to app #9',
    combos: [{ key: '9' }],
    category: 'apps',
    editable: true
  }
];

/**
 * Store for custom keybinding overrides
 * This is populated from the server config and updated when user makes changes
 */
const customBindings = writable<StoredKeybindings>({});

/**
 * Initialize keybindings from server config
 * Should be called when config is loaded
 */
export function initKeybindings(config?: KeybindingsConfig): void {
  if (config?.bindings) {
    customBindings.set(config.bindings);
  } else {
    customBindings.set({});
  }
}

/**
 * Get current custom bindings for saving to config
 * Returns the format expected by the server
 */
export function getKeybindingsForConfig(): KeybindingsConfig {
  const bindings = get(customBindings);
  // Only include non-empty bindings
  const filtered: StoredKeybindings = {};
  for (const [action, combos] of Object.entries(bindings)) {
    if (combos && combos.length > 0) {
      filtered[action] = combos;
    }
  }
  return {
    bindings: Object.keys(filtered).length > 0 ? filtered : undefined
  };
}

/**
 * Derived store that merges defaults with custom bindings
 */
export const keybindings = derived(customBindings, ($custom) => {
  return DEFAULT_KEYBINDINGS.map(binding => {
    if ($custom[binding.action]) {
      return { ...binding, combos: $custom[binding.action] };
    }
    return binding;
  });
});

/**
 * Format a key combo for display
 */
export function formatKeyCombo(combo: KeyCombo): string {
  const parts: string[] = [];

  if (combo.ctrl) parts.push('Ctrl');
  if (combo.alt) parts.push('Alt');
  if (combo.shift) parts.push('Shift');
  if (combo.meta) parts.push('⌘');

  // Format the key nicely
  let key = combo.key;
  if (key === ' ') key = 'Space';
  else if (key === 'ArrowUp') key = '↑';
  else if (key === 'ArrowDown') key = '↓';
  else if (key === 'ArrowLeft') key = '←';
  else if (key === 'ArrowRight') key = '→';
  else if (key.length === 1) key = key.toUpperCase();

  parts.push(key);

  return parts.join('+');
}

/**
 * Format all combos for a keybinding
 */
export function formatKeybinding(binding: Keybinding): string {
  return binding.combos.map(formatKeyCombo).join(' or ');
}

/**
 * Parse a keyboard event into a KeyCombo
 */
export function eventToKeyCombo(event: KeyboardEvent): KeyCombo {
  return {
    key: event.key,
    ctrl: event.ctrlKey,
    alt: event.altKey,
    shift: event.shiftKey,
    meta: event.metaKey
  };
}

/**
 * Check if a keyboard event matches a key combo
 */
export function matchesCombo(event: KeyboardEvent, combo: KeyCombo): boolean {
  // Normalize key comparison (case-insensitive for letters)
  const eventKey = event.key.length === 1 ? event.key.toLowerCase() : event.key;
  const comboKey = combo.key.length === 1 ? combo.key.toLowerCase() : combo.key;

  if (eventKey !== comboKey) return false;
  if (!!combo.ctrl !== event.ctrlKey) return false;
  if (!!combo.alt !== event.altKey) return false;
  if (!!combo.shift !== event.shiftKey) return false;
  if (!!combo.meta !== event.metaKey) return false;

  return true;
}

/**
 * Check if a keyboard event matches any combo in a keybinding
 */
export function matchesKeybinding(event: KeyboardEvent, binding: Keybinding): boolean {
  return binding.combos.some(combo => matchesCombo(event, combo));
}

/**
 * Find the action for a keyboard event
 */
export function findAction(event: KeyboardEvent): KeyAction | null {
  const bindings = get(keybindings);

  for (const binding of bindings) {
    if (matchesKeybinding(event, binding)) {
      return binding.action;
    }
  }

  return null;
}

/**
 * Update keybinding for an action
 * Note: Changes are stored in memory; caller must save config to persist
 */
export function setKeybinding(action: KeyAction, combos: KeyCombo[]): void {
  customBindings.update(current => ({
    ...current,
    [action]: combos
  }));
}

/**
 * Add a combo to an action's keybinding
 */
export function addKeyCombo(action: KeyAction, combo: KeyCombo): void {
  const bindings = get(keybindings);
  const binding = bindings.find(b => b.action === action);

  if (binding) {
    const newCombos = [...binding.combos, combo];
    setKeybinding(action, newCombos);
  }
}

/**
 * Remove a combo from an action's keybinding
 */
export function removeKeyCombo(action: KeyAction, index: number): void {
  const bindings = get(keybindings);
  const binding = bindings.find(b => b.action === action);

  if (binding && binding.combos.length > 1) {
    const newCombos = binding.combos.filter((_, i) => i !== index);
    setKeybinding(action, newCombos);
  }
}

/**
 * Reset a single keybinding to default
 */
export function resetKeybinding(action: KeyAction): void {
  customBindings.update(current => {
    const { [action]: _, ...rest } = current;
    return rest;
  });
}

/**
 * Reset all keybindings to defaults
 */
export function resetAllKeybindings(): void {
  customBindings.set({});
}

/**
 * Check if a keybinding has been customized
 */
export function isCustomized(action: KeyAction): boolean {
  const custom = get(customBindings);
  return action in custom;
}

/**
 * Find conflicts with a proposed key combo
 */
export function findConflicts(combo: KeyCombo, excludeAction?: KeyAction): Keybinding[] {
  const bindings = get(keybindings);

  return bindings.filter(binding => {
    if (excludeAction && binding.action === excludeAction) return false;
    return binding.combos.some(c => comboEquals(c, combo));
  });
}

/**
 * Check if two key combos are equal
 */
export function comboEquals(a: KeyCombo, b: KeyCombo): boolean {
  const aKey = a.key.length === 1 ? a.key.toLowerCase() : a.key;
  const bKey = b.key.length === 1 ? b.key.toLowerCase() : b.key;

  return aKey === bKey &&
    !!a.ctrl === !!b.ctrl &&
    !!a.alt === !!b.alt &&
    !!a.shift === !!b.shift &&
    !!a.meta === !!b.meta;
}

/**
 * Get keybindings grouped by category
 */
export function getKeybindingsByCategory(): Record<string, Keybinding[]> {
  const bindings = get(keybindings);

  return bindings.reduce((acc, binding) => {
    if (!acc[binding.category]) {
      acc[binding.category] = [];
    }
    acc[binding.category].push(binding);
    return acc;
  }, {} as Record<string, Keybinding[]>);
}

/**
 * Export keybindings as JSON (for manual backup)
 */
export function exportKeybindings(): string {
  const custom = get(customBindings);
  return JSON.stringify(custom, null, 2);
}

/**
 * Import keybindings from JSON (for manual restore)
 */
export function importKeybindings(json: string): boolean {
  try {
    const data = JSON.parse(json);
    if (typeof data !== 'object' || data === null) {
      return false;
    }
    customBindings.set(data);
    return true;
  } catch {
    return false;
  }
}

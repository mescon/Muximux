import type { App } from './types';

export const openModes: readonly { value: App['open_mode']; label: string; description: string }[] = [
  { value: 'iframe', label: 'Embedded', description: 'Show inside Muximux' },
  { value: 'new_tab', label: 'New Tab', description: 'Open in a new browser tab' },
  { value: 'new_window', label: 'New Window', description: 'Open in a popup window' },
  { value: 'redirect', label: 'Redirect', description: 'Navigate away to the app URL' },
] as const;

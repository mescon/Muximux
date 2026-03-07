import type { App } from './types';
import * as m from '$lib/paraglide/messages.js';

export const openModes: readonly { value: App['open_mode']; get label(): string; get description(): string }[] = [
  { value: 'iframe', get label() { return m.appForm_embedded(); }, get description() { return m.appForm_embeddedDesc(); } },
  { value: 'new_tab', get label() { return m.appForm_newTab(); }, get description() { return m.appForm_newTabDesc(); } },
  { value: 'new_window', get label() { return m.appForm_newWindow(); }, get description() { return m.appForm_newWindowDesc(); } },
  { value: 'redirect', get label() { return m.appForm_redirect(); }, get description() { return m.appForm_redirectDesc(); } },
] as const;

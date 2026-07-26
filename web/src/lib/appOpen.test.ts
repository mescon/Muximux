import { describe, it, expect } from 'vitest';
import { wantsNewTab } from './appOpen';
import type { App } from './types';

function makeApp(overrides: Partial<App> = {}): App {
  return { name: 'A', url: 'http://a', open_mode: 'iframe', ...overrides } as App;
}

describe('wantsNewTab', () => {
  it('is false without an event (keyboard/programmatic selection)', () => {
    expect(wantsNewTab(undefined, makeApp())).toBe(false);
  });

  it('is true for Ctrl+Click', () => {
    expect(wantsNewTab(new MouseEvent('click', { ctrlKey: true }), makeApp())).toBe(true);
  });

  it('is true for Cmd+Click (macOS)', () => {
    expect(wantsNewTab(new MouseEvent('click', { metaKey: true }), makeApp())).toBe(true);
  });

  it('is true for middle-click', () => {
    expect(wantsNewTab(new MouseEvent('auxclick', { button: 1 }), makeApp())).toBe(true);
  });

  it('is false for a plain left click', () => {
    expect(wantsNewTab(new MouseEvent('click', { button: 0 }), makeApp())).toBe(false);
  });

  it('is false for Shift+Click alone', () => {
    expect(wantsNewTab(new MouseEvent('click', { shiftKey: true }), makeApp())).toBe(false);
  });

  it('ignores the modifier for http_action apps (no page to open)', () => {
    const app = makeApp({ open_mode: 'http_action' });
    expect(wantsNewTab(new MouseEvent('click', { ctrlKey: true }), app)).toBe(false);
    expect(wantsNewTab(new MouseEvent('auxclick', { button: 1 }), app)).toBe(false);
  });
});

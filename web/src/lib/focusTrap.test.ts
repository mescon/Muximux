import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { focusTrap } from './focusTrap';

describe('focusTrap', () => {
  let outside: HTMLButtonElement;
  let dialog: HTMLDivElement;
  let first: HTMLButtonElement;
  let last: HTMLButtonElement;

  beforeEach(() => {
    outside = document.createElement('button');
    outside.textContent = 'outside';
    document.body.appendChild(outside);
    outside.focus(); // pretend this had focus before the dialog opened

    dialog = document.createElement('div');
    dialog.tabIndex = -1;
    first = document.createElement('button');
    first.textContent = 'first';
    last = document.createElement('button');
    last.textContent = 'last';
    dialog.append(first, last);
    document.body.appendChild(dialog);
  });

  afterEach(() => {
    document.body.replaceChildren();
  });

  it('moves focus to the first focusable element on mount', () => {
    focusTrap(dialog);
    expect(document.activeElement).toBe(first);
  });

  it('wraps Tab from the last element back to the first', () => {
    focusTrap(dialog);
    last.focus();
    const ev = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
    dialog.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(first);
  });

  it('wraps Shift+Tab from the first element back to the last', () => {
    focusTrap(dialog);
    first.focus();
    const ev = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true, cancelable: true });
    dialog.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(last);
  });

  it('calls onEscape when Escape is pressed inside the dialog', () => {
    let escaped = 0;
    focusTrap(dialog, { onEscape: () => { escaped += 1; } });
    const ev = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true });
    dialog.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
    expect(escaped).toBe(1);
  });

  it('restores focus to the previously-focused element on destroy', () => {
    const handle = focusTrap(dialog);
    expect(document.activeElement).toBe(first);
    handle.destroy();
    expect(document.activeElement).toBe(outside);
  });
});

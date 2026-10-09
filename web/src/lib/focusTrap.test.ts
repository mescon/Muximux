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

  describe('inert background', () => {
    it('marks siblings along the path to body inert and removes it on destroy', () => {
      const wrap = document.createElement('div');
      const host = document.createElement('div');
      const aside = document.createElement('aside');
      wrap.append(host, aside);
      document.body.appendChild(wrap);
      host.appendChild(dialog);
      const handle = focusTrap(dialog);
      expect(aside.hasAttribute('inert')).toBe(true);
      expect(outside.hasAttribute('inert')).toBe(true);
      expect(wrap.hasAttribute('inert')).toBe(false);
      expect(host.hasAttribute('inert')).toBe(false);
      handle.destroy();
      expect(aside.hasAttribute('inert')).toBe(false);
      expect(outside.hasAttribute('inert')).toBe(false);
    });

    it('leaves live regions and toasts operable', () => {
      const live = document.createElement('div');
      live.setAttribute('aria-live', 'polite');
      const toast = document.createElement('section');
      toast.setAttribute('aria-label', 'Notifications alt+T');
      document.body.append(live, toast);
      const handle = focusTrap(dialog);
      expect(live.hasAttribute('inert')).toBe(false);
      expect(toast.hasAttribute('inert')).toBe(false);
      handle.destroy();
    });

    it('stacks for nested dialogs and only releases a sibling when the last trap closes', () => {
      const wrap = document.createElement('div');
      const second = document.createElement('div');
      second.tabIndex = -1;
      const btn = document.createElement('button');
      second.appendChild(btn);
      wrap.append(dialog, second);
      document.body.appendChild(wrap);
      const a = focusTrap(dialog);
      expect(second.hasAttribute('inert')).toBe(true);
      second.removeAttribute('inert');
      // dialog is inert while the second one is open
      const b = focusTrap(second);
      expect(dialog.hasAttribute('inert')).toBe(true);
      b.destroy();
      expect(dialog.hasAttribute('inert')).toBe(false);
      a.destroy();
      expect(outside.hasAttribute('inert')).toBe(false);
    });

    it('does not touch elements that were already inert', () => {
      outside.setAttribute('inert', '');
      const handle = focusTrap(dialog);
      handle.destroy();
      expect(outside.hasAttribute('inert')).toBe(true);
    });

    it('returns focus to the opener even though it was inert while open', () => {
      const handle = focusTrap(dialog);
      expect(outside.hasAttribute('inert')).toBe(true);
      handle.destroy();
      expect(outside.hasAttribute('inert')).toBe(false);
      expect(document.activeElement).toBe(outside);
    });

    it('skips restoring focus when the opener left the document', () => {
      const handle = focusTrap(dialog);
      outside.remove();
      expect(() => handle.destroy()).not.toThrow();
    });
  });

  describe('initial focus and edge cases', () => {
    it('prefers an autofocus element over the first focusable', () => {
      last.setAttribute('autofocus', '');
      focusTrap(dialog);
      expect(document.activeElement).toBe(last);
    });

    it('falls back to the dialog node when it has nothing focusable and keeps Tab on it', () => {
      first.remove();
      last.remove();
      focusTrap(dialog);
      expect(document.activeElement).toBe(dialog);
      const ev = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
      dialog.dispatchEvent(ev);
      expect(ev.defaultPrevented).toBe(true);
      expect(document.activeElement).toBe(dialog);
    });

    it('pulls focus back in when Tab is pressed while focus is outside the dialog', () => {
      focusTrap(dialog);
      outside.removeAttribute('inert');
      outside.focus();
      dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }));
      expect(document.activeElement).toBe(first);
    });

    it('update() swaps the params used by Escape', () => {
      let a = 0, b = 0;
      const handle = focusTrap(dialog, { onEscape: () => { a++; } });
      handle.update({ onEscape: () => { b++; } });
      dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
      expect([a, b]).toEqual([0, 1]);
    });
  });
  describe('picker nested over a trapped dialog', () => {
    it('keeps Tab in the picker, restores focus on close and leaves the outer trap intact', () => {
      const opener = document.createElement('button');
      opener.textContent = 'open picker';
      dialog.prepend(opener);
      const outer = focusTrap(dialog);
      opener.focus();

      // The picker is a sibling of the dialog, so Tab never reaches the dialog's listener.
      const picker = document.createElement('div');
      picker.tabIndex = -1;
      const p1 = document.createElement('button');
      const p2 = document.createElement('button');
      picker.append(p1, p2);
      document.body.appendChild(picker);
      const inner = focusTrap(picker);
      expect(document.activeElement).toBe(p1);
      expect(dialog.hasAttribute('inert')).toBe(true);

      p2.focus();
      p2.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }));
      expect(document.activeElement).toBe(p1);
      p1.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true, cancelable: true }));
      expect(document.activeElement).toBe(p2);

      inner.destroy();
      picker.remove();
      expect(document.activeElement).toBe(opener);
      expect(dialog.hasAttribute('inert')).toBe(false);
      // The outer trap still holds the background.
      expect(outside.hasAttribute('inert')).toBe(true);
      last.focus();
      last.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }));
      expect(document.activeElement).toBe(opener);

      outer.destroy();
      expect(outside.hasAttribute('inert')).toBe(false);
    });
  });
});

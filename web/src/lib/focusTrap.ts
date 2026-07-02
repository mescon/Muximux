// focusTrap is a Svelte action for modal dialogs. It keeps keyboard focus
// inside the node while it is mounted, moves focus into the dialog on open,
// and restores focus to the previously-focused element on close. Pair it
// with role="dialog" + aria-modal="true" and an Escape-to-close handler for
// an accessible modal.
//
// Visibility is judged by the `hidden` attribute / `disabled` rather than
// layout (offsetParent), so the behaviour is identical under jsdom (which
// has no layout) and in a real browser.

const FOCUSABLE = [
  'a[href]',
  'button:not([disabled])',
  'textarea:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',');

function focusableWithin(node: HTMLElement): HTMLElement[] {
  return Array.from(node.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    (el) => !el.hasAttribute('hidden') && el.closest('[hidden]') === null,
  );
}

export interface FocusTrapParams {
  // Called when Escape is pressed while focus is inside the dialog. Wire it
  // to the modal's close handler so the dialog is dismissable by keyboard.
  onEscape?: () => void;
}

export function focusTrap(node: HTMLElement, params: FocusTrapParams = {}) {
  const previouslyFocused = document.activeElement as HTMLElement | null;

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && params.onEscape) {
      e.preventDefault();
      params.onEscape();
      return;
    }
    if (e.key !== 'Tab') return;
    const focusable = focusableWithin(node);
    if (focusable.length === 0) {
      // Nothing to focus inside: keep focus on the dialog itself.
      e.preventDefault();
      node.focus();
      return;
    }
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    const active = document.activeElement;
    if (e.shiftKey) {
      if (active === first || !node.contains(active)) {
        e.preventDefault();
        last.focus();
      }
    } else if (active === last || !node.contains(active)) {
      e.preventDefault();
      first.focus();
    }
  }

  // Move focus into the dialog. Prefer the first focusable control; fall
  // back to the dialog node (which needs tabindex="-1" to be focusable).
  const initial = focusableWithin(node);
  (initial[0] ?? node).focus();

  node.addEventListener('keydown', handleKeydown);

  return {
    update(next: FocusTrapParams = {}) {
      params = next;
    },
    destroy() {
      node.removeEventListener('keydown', handleKeydown);
      // Restore focus to where it was before the dialog opened, so keyboard
      // users are not dumped at the top of the document.
      if (previouslyFocused && typeof previouslyFocused.focus === 'function') {
        previouslyFocused.focus();
      }
    },
  };
}

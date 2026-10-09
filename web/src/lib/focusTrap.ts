// focusTrap is a Svelte action for modal dialogs. It keeps keyboard focus
// inside the node while it is mounted, moves focus into the dialog on open,
// and restores focus to the previously-focused element on close. Pair it
// with role="dialog" + aria-modal="true" and an Escape-to-close handler for
// an accessible modal.
//
// While the dialog is mounted everything outside it is made inert (the
// `inert` attribute on every sibling along the path from the dialog up to
// <body>), so the background cannot be reached by Tab, pointer or assistive
// technology. Live regions (toasts, announcers) are left alone so status
// messages are still read. Nested dialogs stack: each element is reference
// counted and only un-inerted when the last trap that inerted it closes.
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

// Elements that must keep working while a dialog is open.
const KEEP_LIVE = '[aria-live], [role="status"], [role="alert"], [data-sonner-toaster], section[aria-label^="Notifications"], script, style, [data-focus-trap-ignore]';

// Inert reference counts, shared by all active traps.
const inertCounts = new Map<Element, number>();

function makeBackgroundInert(node: HTMLElement): () => void {
  const mine: Element[] = [];
  let el: HTMLElement | null = node;
  while (el && el !== document.body && el.parentElement) {
    const parent: HTMLElement = el.parentElement;
    for (const sib of Array.from(parent.children)) {
      if (sib === el || sib.matches(KEEP_LIVE)) continue;
      const count = inertCounts.get(sib);
      if (count === undefined) {
        // Already inert for some other reason: leave it as it is.
        if (sib.hasAttribute('inert')) continue;
        sib.setAttribute('inert', '');
        inertCounts.set(sib, 1);
      } else {
        inertCounts.set(sib, count + 1);
      }
      mine.push(sib);
    }
    el = parent;
  }
  return () => {
    for (const sib of mine) {
      const count = (inertCounts.get(sib) ?? 1) - 1;
      if (count <= 0) {
        inertCounts.delete(sib);
        sib.removeAttribute('inert');
      } else {
        inertCounts.set(sib, count);
      }
    }
  };
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
  // An element marked autofocus wins over the first one in DOM order.
  const releaseInert = makeBackgroundInert(node);
  const initial = focusableWithin(node);
  (node.querySelector<HTMLElement>('[autofocus]') ?? initial[0] ?? node).focus();

  node.addEventListener('keydown', handleKeydown);

  return {
    update(next: FocusTrapParams = {}) {
      params = next;
    },
    destroy() {
      node.removeEventListener('keydown', handleKeydown);
      // Un-inert first: an inert element cannot take focus back.
      releaseInert();
      // Restore focus to where it was before the dialog opened, so keyboard
      // users are not dumped at the top of the document.
      if (previouslyFocused && previouslyFocused.isConnected && typeof previouslyFocused.focus === 'function') {
        previouslyFocused.focus();
      }
    },
  };
}

import { writable } from 'svelte/store';

// A single app-wide polite live region. `announce()` publishes a short
// status message (e.g. the result of a keyboard reorder) that the mounted
// <Announcer> renders into an aria-live="polite" region for screen readers.
export const announcement = writable('');

let parity = false;

// announce publishes `message` to the live region. It toggles a trailing
// no-break space so that announcing the SAME text twice in a row still
// changes the region's text node -- otherwise an unchanged node is not
// re-read by assistive tech, and repeated identical feedback would be
// silently dropped.
export function announce(message: string): void {
  parity = !parity;
  announcement.set(parity ? message + ' ' : message);
}

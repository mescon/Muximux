// moveItem returns a new array with the item at `index` swapped one step in
// `direction` (-1 = toward the start, +1 = toward the end). It is the single
// source of truth for keyboard "move up / move down" reordering, shared by
// the Settings apps tab and the onboarding wizard so both surfaces reorder
// identically. When the move would run off either end (or `index` is out of
// range) it returns the ORIGINAL array reference unchanged, so callers can
// cheaply detect a bounds no-op with `result === items` and skip re-sync /
// announcements. The input array is never mutated on a successful move.
export function moveItem<T>(items: T[], index: number, direction: -1 | 1): T[] {
  const target = index + direction;
  if (index < 0 || index >= items.length || target < 0 || target >= items.length) {
    return items;
  }
  const next = items.slice();
  [next[index], next[target]] = [next[target], next[index]];
  return next;
}

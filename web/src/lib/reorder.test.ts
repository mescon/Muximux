import { describe, it, expect } from 'vitest';
import { moveItem } from './reorder';

describe('moveItem', () => {
  it('moves an item up (toward index 0)', () => {
    expect(moveItem(['a', 'b', 'c'], 1, -1)).toEqual(['b', 'a', 'c']);
  });

  it('moves an item down (toward the end)', () => {
    expect(moveItem(['a', 'b', 'c'], 1, 1)).toEqual(['a', 'c', 'b']);
  });

  it('returns the SAME array reference (no-op) when moving the first item up', () => {
    const items = ['a', 'b', 'c'];
    expect(moveItem(items, 0, -1)).toBe(items);
  });

  it('returns the SAME array reference (no-op) when moving the last item down', () => {
    const items = ['a', 'b', 'c'];
    expect(moveItem(items, 2, 1)).toBe(items);
  });

  it('is a no-op for an out-of-range index', () => {
    const items = ['a', 'b'];
    expect(moveItem(items, 5, -1)).toBe(items);
    expect(moveItem(items, -1, 1)).toBe(items);
  });

  it('is a no-op on a single-item list', () => {
    const items = ['only'];
    expect(moveItem(items, 0, 1)).toBe(items);
    expect(moveItem(items, 0, -1)).toBe(items);
  });

  it('does not mutate the input array on a successful move', () => {
    const items = ['a', 'b', 'c'];
    moveItem(items, 0, 1);
    expect(items).toEqual(['a', 'b', 'c']);
  });

  it('works with object items by reference', () => {
    const a = { id: 1 };
    const b = { id: 2 };
    const out = moveItem([a, b], 0, 1);
    expect(out).toEqual([b, a]);
    expect(out[0]).toBe(b);
    expect(out[1]).toBe(a);
  });
});

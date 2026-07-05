import { describe, it, expect } from 'vitest';
import { get } from 'svelte/store';
import { announcement, announce } from './announce';

describe('announce', () => {
  it('publishes the message to the announcement store', () => {
    announce('Moved Sonarr to position 2 of 5');
    expect(get(announcement)).toContain('Moved Sonarr to position 2 of 5');
  });

  it('changes the store value even when the same message is announced twice, so screen readers re-read it', () => {
    announce('Already first');
    const first = get(announcement);
    announce('Already first');
    const second = get(announcement);
    expect(second).not.toBe(first);
    // Both still carry the human-readable text (only a trailing
    // whitespace toggle differs).
    expect(first.trim()).toBe('Already first');
    expect(second.trim()).toBe('Already first');
  });
});

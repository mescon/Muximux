import { describe, it, expect, vi } from 'vitest';
const mq = vi.hoisted(() => ({ current: false }));
vi.mock('svelte/motion', () => ({ prefersReducedMotion: mq }));
import { motionMs } from './motion';

describe('motionMs', () => {
  it('returns the duration when motion is allowed', () => { mq.current = false; expect(motionMs(150)).toBe(150); });
  it('returns 0 when the OS asks for reduced motion', () => { mq.current = true; expect(motionMs(150)).toBe(0); });
});

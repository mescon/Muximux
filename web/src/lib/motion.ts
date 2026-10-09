import { prefersReducedMotion } from 'svelte/motion';

// Duration for a Svelte transition: 0 when the operating system asks for reduced
// motion, otherwise the given milliseconds. CSS animations are handled by the
// prefers-reduced-motion block in app.css; Svelte transitions use the Web
// Animations API and need this helper.
export function motionMs(ms: number): number {
  return prefersReducedMotion.current ? 0 : ms;
}

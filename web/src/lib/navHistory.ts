// history.state value Muximux puts on every history entry it creates, so a
// later hashchange can tell Back/Forward (tagged) from a link click (untagged).
export const NAV_STATE = { muximux: true } as const;

export function isMuximuxEntry(state: unknown): boolean {
  return typeof state === 'object' && state !== null && (state as { muximux?: unknown }).muximux === true;
}

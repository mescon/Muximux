import { describe, it, expect, vi } from 'vitest';
import { FRAME_NAME_PREFIX, frameRescueTarget, rescueMisroutedFrame } from './frameRescue';

const at = (pathname: string, extra: Partial<Parameters<typeof frameRescueTarget>[0]> = {}) =>
  frameRescueTarget({
    name: FRAME_NAME_PREFIX + '/proxy/dispatcharr',
    pathname,
    search: '',
    hash: '',
    base: '',
    ...extra,
  });

describe('frameRescueTarget', () => {
  it('sends a shell path back under the proxy prefix, keeping query and hash', () => {
    expect(at('/login', { search: '?next=%2Fchannels', hash: '#top' })).toBe(
      '/proxy/dispatcharr/login?next=%2Fchannels#top',
    );
  });

  it('maps the shell root to the proxied app root', () => {
    expect(at('/')).toBe('/proxy/dispatcharr/');
    expect(at('')).toBe('/proxy/dispatcharr/');
  });

  it('ignores windows that are not Muximux app frames', () => {
    expect(at('/login', { name: '' })).toBeNull();
    expect(at('/login', { name: 'someOtherName' })).toBeNull();
  });

  it('does nothing when the path is already proxied', () => {
    expect(at('/proxy/dispatcharr')).toBeNull();
    expect(at('/proxy/dispatcharr/login')).toBeNull();
  });

  it('strips the Muximux base path before re-prefixing', () => {
    const name = FRAME_NAME_PREFIX + '/muximux/proxy/dispatcharr';
    expect(at('/muximux/login', { name, base: '/muximux' })).toBe('/muximux/proxy/dispatcharr/login');
    expect(at('/muximux', { name, base: '/muximux' })).toBe('/muximux/proxy/dispatcharr/');
    expect(at('/muximux/proxy/dispatcharr/x', { name, base: '/muximux' })).toBeNull();
  });

  it('refuses names that are not a root-relative proxy path', () => {
    for (const bad of [
      'https://evil.example/proxy/x',
      '//evil.example/proxy/x',
      '/proxy/../login',
      '/proxy/Dispatcharr',
      '/proxy/',
      'proxy/dispatcharr',
      '/proxy/dispatcharr/',
      '/other/dispatcharr',
    ]) {
      expect(at('/login', { name: FRAME_NAME_PREFIX + bad })).toBeNull();
    }
  });
});

describe('rescueMisroutedFrame', () => {
  function fakeWindow(opts: { framed: boolean; name: string; pathname: string; base?: string; throwOnParent?: boolean }) {
    const replace = vi.fn();
    const win = {
      name: opts.name,
      location: { pathname: opts.pathname, search: '?a=1', hash: '#h', replace },
      __MUXIMUX_BASE__: opts.base,
    } as unknown as Window & { parent: Window };
    if (opts.throwOnParent) {
      Object.defineProperty(win, 'parent', {
        get() {
          throw new Error('blocked');
        },
      });
    } else {
      (win as { parent: unknown }).parent = opts.framed ? ({} as Window) : win;
    }
    return { win, replace };
  }

  it('redirects a framed shell load to the proxied path', () => {
    const { win, replace } = fakeWindow({ framed: true, name: FRAME_NAME_PREFIX + '/proxy/dispatcharr', pathname: '/login' });
    expect(rescueMisroutedFrame(win)).toBe(true);
    expect(replace).toHaveBeenCalledWith('/proxy/dispatcharr/login?a=1#h');
  });

  it('applies the configured base path', () => {
    const { win, replace } = fakeWindow({
      framed: true,
      name: FRAME_NAME_PREFIX + '/muximux/proxy/dispatcharr',
      pathname: '/muximux/login',
      base: '/muximux',
    });
    expect(rescueMisroutedFrame(win)).toBe(true);
    expect(replace).toHaveBeenCalledWith('/muximux/proxy/dispatcharr/login?a=1#h');
  });

  it('leaves a top-level window alone', () => {
    const { win, replace } = fakeWindow({ framed: false, name: FRAME_NAME_PREFIX + '/proxy/dispatcharr', pathname: '/login' });
    expect(rescueMisroutedFrame(win)).toBe(false);
    expect(replace).not.toHaveBeenCalled();
  });

  it('leaves a frame without a Muximux name alone', () => {
    const { win, replace } = fakeWindow({ framed: true, name: '', pathname: '/login' });
    expect(rescueMisroutedFrame(win)).toBe(false);
    expect(replace).not.toHaveBeenCalled();
  });

  it('treats an inaccessible parent as not framed by Muximux', () => {
    const { win, replace } = fakeWindow({ framed: true, name: FRAME_NAME_PREFIX + '/proxy/x', pathname: '/login', throwOnParent: true });
    expect(rescueMisroutedFrame(win)).toBe(false);
    expect(replace).not.toHaveBeenCalled();
  });
});

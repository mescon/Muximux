import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { installNotificationBridge } from './notificationBridge';
import { makeApp, type App } from './types';

class MockNotification {
  static permission: NotificationPermission = 'default';
  static requestPermission = vi.fn(async () => MockNotification.permission);

  title: string;
  options: NotificationOptions;
  onclick: (() => void) | null = null;
  close = vi.fn();
  static instances: MockNotification[] = [];

  constructor(title: string, options: NotificationOptions = {}) {
    this.title = title;
    this.options = options;
    MockNotification.instances.push(this);
  }
}

describe('notificationBridge', () => {
  let cleanup: (() => void) | null = null;
  let apps: App[];
  let onActivate: ReturnType<typeof vi.fn>;
  const appendedIframes: HTMLIFrameElement[] = [];

  beforeEach(() => {
    MockNotification.permission = 'granted';
    MockNotification.instances = [];
    MockNotification.requestPermission.mockClear();
    // @ts-expect-error test global
    globalThis.Notification = MockNotification;

    apps = [];
    onActivate = vi.fn();
  });

  afterEach(() => {
    cleanup?.();
    cleanup = null;
    for (const frame of appendedIframes) frame.remove();
    appendedIframes.length = 0;
  });

  function addIframeForApp(app: App): HTMLIFrameElement {
    const iframe = document.createElement('iframe');
    iframe.dataset.app = app.name;
    document.body.appendChild(iframe);
    appendedIframes.push(iframe);
    return iframe;
  }

  function installBridge() {
    cleanup = installNotificationBridge({
      getApps: () => apps,
      onActivate,
    });
  }

  it('ignores messages that are not muximux:notify', () => {
    installBridge();
    window.dispatchEvent(new MessageEvent('message', { data: { type: 'other' }, origin: 'https://app.local' }));
    expect(MockNotification.instances).toHaveLength(0);
  });

  it('ignores messages from unknown iframes', async () => {
    installBridge();
    window.dispatchEvent(new MessageEvent('message', {
      data: { type: 'muximux:notify', title: 'Hi' },
      origin: 'https://attacker.example',
    }));
    await new Promise(r => setTimeout(r, 0));
    expect(MockNotification.instances).toHaveLength(0);
  });

  it('ignores apps that have not opted into notifications', async () => {
    const app = makeApp({ name: 'NoOptIn', url: 'https://a.local', allow_notifications: false });
    apps = [app];
    const iframe = addIframeForApp(app);
    installBridge();

    window.dispatchEvent(new MessageEvent('message', {
      data: { type: 'muximux:notify', title: 'Hi' },
      origin: 'https://a.local',
      source: iframe.contentWindow!,
    }));
    await new Promise(r => setTimeout(r, 0));
    expect(MockNotification.instances).toHaveLength(0);
  });

  it('shows a notification for an opted-in app', async () => {
    const app = makeApp({ name: 'NotifyApp', url: 'https://n.local', allow_notifications: true });
    apps = [app];
    const iframe = addIframeForApp(app);
    installBridge();

    window.dispatchEvent(new MessageEvent('message', {
      data: { type: 'muximux:notify', title: 'Hello', body: 'World' },
      origin: 'https://n.local',
      source: iframe.contentWindow!,
    }));
    await new Promise(r => setTimeout(r, 0));

    expect(MockNotification.instances).toHaveLength(1);
    expect(MockNotification.instances[0].title).toBe('Hello');
    expect(MockNotification.instances[0].options.body).toBe('World');
  });

  it('ignores app-supplied icon URLs and uses the configured icon instead', async () => {
    const app = makeApp({
      name: 'IconApp',
      url: 'https://i.local',
      allow_notifications: true,
      icon: { type: 'lucide', name: 'bell' },
    });
    apps = [app];
    const iframe = addIframeForApp(app);
    installBridge();

    window.dispatchEvent(new MessageEvent('message', {
      data: {
        type: 'muximux:notify',
        title: 'Test',
        icon: 'https://malicious.example/bank-logo.png',
      },
      origin: 'https://i.local',
      source: iframe.contentWindow!,
    }));
    await new Promise(r => setTimeout(r, 0));

    expect(MockNotification.instances).toHaveLength(1);
    const used = MockNotification.instances[0].options.icon ?? '';
    expect(used).not.toContain('malicious.example');
  });

  it('rate-limits repeated notifications from the same app', async () => {
    const app = makeApp({ name: 'Spammer', url: 'https://s.local', allow_notifications: true });
    apps = [app];
    const iframe = addIframeForApp(app);
    installBridge();

    for (let i = 0; i < 5; i++) {
      window.dispatchEvent(new MessageEvent('message', {
        data: { type: 'muximux:notify', title: `msg ${i}` },
        origin: 'https://s.local',
        source: iframe.contentWindow!,
      }));
    }
    await new Promise(r => setTimeout(r, 0));

    expect(MockNotification.instances).toHaveLength(1);
  });

  it('truncates long titles and bodies', async () => {
    const app = makeApp({ name: 'LongText', url: 'https://l.local', allow_notifications: true });
    apps = [app];
    const iframe = addIframeForApp(app);
    installBridge();

    const longTitle = 'A'.repeat(500);
    const longBody = 'B'.repeat(1000);

    window.dispatchEvent(new MessageEvent('message', {
      data: { type: 'muximux:notify', title: longTitle, body: longBody },
      origin: 'https://l.local',
      source: iframe.contentWindow!,
    }));
    await new Promise(r => setTimeout(r, 0));

    const created = MockNotification.instances[0];
    expect(created.title.length).toBeLessThanOrEqual(120);
    expect(created.options.body!.length).toBeLessThanOrEqual(400);
  });

  it('calls onActivate with the correct app when the notification is clicked', async () => {
    const app = makeApp({ name: 'ClickMe', url: 'https://c.local', allow_notifications: true });
    apps = [app];
    const iframe = addIframeForApp(app);
    installBridge();

    window.dispatchEvent(new MessageEvent('message', {
      data: { type: 'muximux:notify', title: 'Click!' },
      origin: 'https://c.local',
      source: iframe.contentWindow!,
    }));
    await new Promise(r => setTimeout(r, 0));

    const n = MockNotification.instances[0];
    n.onclick?.();
    expect(onActivate).toHaveBeenCalledWith(app);
  });

  it('does not show a notification when permission is denied', async () => {
    MockNotification.permission = 'denied';
    const app = makeApp({ name: 'Denied', url: 'https://d.local', allow_notifications: true });
    apps = [app];
    const iframe = addIframeForApp(app);
    installBridge();

    window.dispatchEvent(new MessageEvent('message', {
      data: { type: 'muximux:notify', title: 'No-op' },
      origin: 'https://d.local',
      source: iframe.contentWindow!,
    }));
    await new Promise(r => setTimeout(r, 0));

    expect(MockNotification.instances).toHaveLength(0);
  });

  it('replies to a permission query with the current Notification.permission', async () => {
    MockNotification.permission = 'granted';
    const app = makeApp({ name: 'Q', url: 'https://q.local', allow_notifications: true });
    apps = [app];
    const iframe = addIframeForApp(app);
    installBridge();

    // The iframe's contentWindow is cross-realm in jsdom; stub postMessage on
    // the iframe's window so we can observe the reply.
    const postSpy = vi.fn();
    Object.defineProperty(iframe.contentWindow!, 'postMessage', { value: postSpy, writable: true });

    window.dispatchEvent(new MessageEvent('message', {
      data: { type: 'muximux:notify-query-permission' },
      origin: 'https://q.local',
      source: iframe.contentWindow!,
    }));
    await new Promise(r => setTimeout(r, 0));

    expect(postSpy).toHaveBeenCalledWith(
      { type: 'muximux:notify-permission', permission: 'granted' },
      'https://q.local',
    );
  });

  it('replies to a permission request after calling requestPermission', async () => {
    MockNotification.permission = 'default';
    MockNotification.requestPermission.mockImplementation(async () => {
      MockNotification.permission = 'granted';
      return 'granted';
    });
    const app = makeApp({ name: 'R', url: 'https://r.local', allow_notifications: true });
    apps = [app];
    const iframe = addIframeForApp(app);
    installBridge();

    const postSpy = vi.fn();
    Object.defineProperty(iframe.contentWindow!, 'postMessage', { value: postSpy, writable: true });

    window.dispatchEvent(new MessageEvent('message', {
      data: { type: 'muximux:notify-request-permission' },
      origin: 'https://r.local',
      source: iframe.contentWindow!,
    }));
    await new Promise(r => setTimeout(r, 0));

    expect(MockNotification.requestPermission).toHaveBeenCalled();
    expect(postSpy).toHaveBeenCalledWith(
      { type: 'muximux:notify-permission', permission: 'granted' },
      'https://r.local',
    );
  });

  it('ignores permission queries from unregistered iframes', async () => {
    installBridge();
    const postSpy = vi.fn();
    const fakeWindow = { postMessage: postSpy } as unknown as MessageEventSource;

    window.dispatchEvent(new MessageEvent('message', {
      data: { type: 'muximux:notify-query-permission' },
      origin: 'https://attacker.example',
      source: fakeWindow,
    }));
    await new Promise(r => setTimeout(r, 0));

    expect(postSpy).not.toHaveBeenCalled();
  });

  it('prefers ServiceWorkerRegistration.showNotification when available', async () => {
    const app = makeApp({ name: 'SWApp', url: 'https://sw.local', allow_notifications: true });
    apps = [app];
    const iframe = addIframeForApp(app);

    const swShow = vi.fn(async () => undefined);
    const fakeRegistration = { showNotification: swShow };
    const fakeSW = {
      ready: Promise.resolve(fakeRegistration),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    };
    Object.defineProperty(globalThis.navigator, 'serviceWorker', {
      value: fakeSW,
      configurable: true,
    });

    installBridge();

    window.dispatchEvent(new MessageEvent('message', {
      data: { type: 'muximux:notify', title: 'Hi mobile', body: 'body' },
      origin: 'https://sw.local',
      source: iframe.contentWindow!,
    }));
    // Allow navigator.serviceWorker.ready to resolve.
    await new Promise(r => setTimeout(r, 0));
    await new Promise(r => setTimeout(r, 0));

    expect(swShow).toHaveBeenCalledTimes(1);
    expect(swShow.mock.calls[0][0]).toBe('Hi mobile');
    expect(swShow.mock.calls[0][1]).toMatchObject({ body: 'body', data: { muximuxApp: 'SWApp' } });
    // Constructor path should be skipped when the SW path succeeded.
    expect(MockNotification.instances).toHaveLength(0);

    // Cleanup - remove the fake SW so later tests get the default jsdom value.
    Object.defineProperty(globalThis.navigator, 'serviceWorker', {
      value: undefined,
      configurable: true,
    });
  });

  it('routes a service-worker notification click to onActivate', async () => {
    const app = makeApp({ name: 'SWClick', url: 'https://swc.local', allow_notifications: true });
    apps = [app];
    addIframeForApp(app);

    let swMessageHandler: ((e: MessageEvent) => void) | null = null;
    const fakeSW = {
      ready: Promise.resolve({ showNotification: vi.fn() }),
      addEventListener: vi.fn((type: string, handler: (e: MessageEvent) => void) => {
        if (type === 'message') swMessageHandler = handler;
      }),
      removeEventListener: vi.fn(),
    };
    Object.defineProperty(globalThis.navigator, 'serviceWorker', {
      value: fakeSW,
      configurable: true,
    });

    installBridge();

    expect(swMessageHandler).not.toBeNull();
    swMessageHandler!(new MessageEvent('message', {
      data: { type: 'muximux:notification-click', appName: 'SWClick' },
    }));

    expect(onActivate).toHaveBeenCalledWith(app);

    Object.defineProperty(globalThis.navigator, 'serviceWorker', {
      value: undefined,
      configurable: true,
    });
  });
});

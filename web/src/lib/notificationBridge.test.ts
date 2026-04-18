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
});

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import OidcSettings from './OidcSettings.svelte';
import type { OIDCSettings, OIDCTestResult } from '$lib/types';

const mockTest = vi.fn();
vi.mock('$lib/api', () => ({
  testOIDCProvider: (...args: unknown[]) => mockTest(...args),
}));

function base(over: Partial<OIDCSettings> = {}): OIDCSettings {
  return {
    enabled: true,
    issuer_url: 'https://auth.example.com',
    client_id: 'muximux',
    client_secret_set: true,
    redirect_url: '',
    scopes: ['openid', 'profile'],
    username_claim: 'preferred_username',
    email_claim: 'email',
    groups_claim: 'groups',
    display_name_claim: 'name',
    admin_groups: ['admins'],
    provider_logout: false,
    post_logout_redirect_url: '',
    logout_url: '',
    auto_redirect: false,
    disable_local_login: false,
    env_fields: {},
    default_callback_url: 'https://mux.example.com/api/auth/oidc/callback',
    backchannel_url: 'https://mux.example.com/api/auth/oidc/backchannel-logout',
    current_session_is_oidc: true,
    ...over,
  };
}

const el = (id: string) => document.getElementById(id) as HTMLInputElement;
const last = (fn: ReturnType<typeof vi.fn>) => fn.mock.calls[fn.mock.calls.length - 1];

describe('OidcSettings', () => {
  let onchange: ReturnType<typeof vi.fn>;
  beforeEach(() => { onchange = vi.fn(); mockTest.mockReset(); });
  afterEach(() => { vi.useRealTimers(); });

  it('renders issuer, client id and the copyable URLs', () => {
    render(OidcSettings, { settings: base(), onchange });
    expect(el('oidc-issuer').value).toBe('https://auth.example.com');
    expect(el('oidc-client-id').value).toBe('muximux');
    expect(el('oidc-callback').value).toBe('https://mux.example.com/api/auth/oidc/callback');
    expect(el('oidc-backchannel').value).toContain('backchannel-logout');
    expect(el('oidc-redirect').placeholder).toBe('https://mux.example.com/api/auth/oidc/callback');
    expect(el('oidc-scopes').value).toBe('openid profile');
    expect(el('oidc-admin-groups').value).toBe('admins');
    const [payload, valid] = last(onchange);
    expect(valid).toBe(true);
    expect(payload.client_secret).toBeUndefined();
  });

  it('shows the custom redirect URL in the callback row and handles null lists', () => {
    render(OidcSettings, { settings: base({ redirect_url: 'https://x.example.com/cb', scopes: null, admin_groups: null }), onchange });
    expect(el('oidc-callback').value).toBe('https://x.example.com/cb');
    expect(el('oidc-scopes').value).toBe('');
  });

  it('secret placeholder reflects state and typing emits the secret', async () => {
    render(OidcSettings, { settings: base(), onchange });
    const input = el('oidc-client-secret');
    expect(input.value).toBe('');
    expect(input.placeholder).toBe('Set (leave empty to keep)');
    await fireEvent.input(input, { target: { value: 'abc' } });
    expect(last(onchange)[0].client_secret).toBe('abc');
  });

  it('shows not set placeholder when no secret', () => {
    render(OidcSettings, { settings: base({ client_secret_set: false }), onchange });
    expect(el('oidc-client-secret').placeholder).toBe('Not set');
  });

  it('env-provided fields are read-only, labelled and omitted from the payload', () => {
    render(OidcSettings, {
      settings: base({ env_fields: { client_secret: 'OIDC_CLIENT_SECRET', issuer_url: 'OIDC_ISSUER' } }),
      onchange,
    });
    expect(screen.getByText('From OIDC_CLIENT_SECRET')).toBeTruthy();
    expect(screen.getByText('From OIDC_ISSUER')).toBeTruthy();
    expect(el('oidc-client-secret').readOnly).toBe(true);
    expect(el('oidc-client-secret').value).toBe('');
    expect(el('oidc-issuer').readOnly).toBe(true);
    const [payload, valid] = last(onchange);
    expect('client_secret' in payload).toBe(false);
    expect('issuer_url' in payload).toBe(false);
    expect(payload.client_id).toBe('muximux');
    expect(valid).toBe(true);
  });

  it('clearing the issuer is invalid; non-http issuer is invalid', async () => {
    render(OidcSettings, { settings: base(), onchange });
    await fireEvent.input(el('oidc-issuer'), { target: { value: '' } });
    expect(last(onchange)[1]).toBe(false);
    await fireEvent.input(el('oidc-issuer'), { target: { value: 'ftp://x.example.com' } });
    expect(last(onchange)[1]).toBe(false);
    await fireEvent.input(el('oidc-issuer'), { target: { value: 'https://ok.example.com' } });
    expect(last(onchange)[1]).toBe(true);
  });

  it('empty client id is invalid', async () => {
    render(OidcSettings, { settings: base(), onchange });
    await fireEvent.input(el('oidc-client-id'), { target: { value: ' ' } });
    expect(last(onchange)[1]).toBe(false);
  });

  it.each([['oidc-logout-url'], ['oidc-post-logout'], ['oidc-redirect']])('relative URL in %s is invalid', async (id) => {
    render(OidcSettings, { settings: base(), onchange });
    await fireEvent.input(el(id), { target: { value: '/logout' } });
    expect(last(onchange)[1]).toBe(false);
    await fireEvent.input(el(id), { target: { value: 'https://ok.example.com/x' } });
    expect(last(onchange)[1]).toBe(true);
  });

  it('parses scopes and admin groups', async () => {
    render(OidcSettings, { settings: base(), onchange });
    await fireEvent.input(el('oidc-scopes'), { target: { value: ' openid   email ' } });
    await fireEvent.input(el('oidc-admin-groups'), { target: { value: 'a, ,b ,' } });
    const [payload] = last(onchange);
    expect(payload.scopes).toEqual(['openid', 'email']);
    expect(payload.admin_groups).toEqual(['a', 'b']);
  });

  it('toggles are reflected in the payload', async () => {
    render(OidcSettings, { settings: base(), onchange });
    await fireEvent.click(screen.getByLabelText('Also sign out at the identity provider'));
    await fireEvent.click(screen.getByLabelText('Skip the login page and go straight to SSO'));
    const [payload] = last(onchange);
    expect(payload.provider_logout).toBe(true);
    expect(payload.auto_redirect).toBe(true);
  });

  it('disables SSO-only when the session is not OIDC', () => {
    render(OidcSettings, { settings: base({ current_session_is_oidc: false }), onchange });
    expect(el('oidc-disable-local').disabled).toBe(true);
    expect(screen.getByText('Sign in with SSO once to turn this on.')).toBeTruthy();
  });

  it('shows the warning when SSO-only is checked and still lets it be unchecked', async () => {
    render(OidcSettings, { settings: base({ disable_local_login: true, current_session_is_oidc: false }), onchange });
    expect(screen.getByTestId('oidc-disable-local-warning')).toBeTruthy();
    expect(el('oidc-disable-local').disabled).toBe(false);
    await fireEvent.click(el('oidc-disable-local'));
    expect(last(onchange)[0].disable_local_login).toBe(false);
  });

  it('locks identity fields while SSO-only is in effect', () => {
    render(OidcSettings, { settings: base({ disable_local_login: true }), onchange });
    expect(el('oidc-issuer').readOnly).toBe(true);
    expect(el('oidc-client-id').readOnly).toBe(true);
    expect(el('oidc-client-secret').readOnly).toBe(true);
    expect(screen.getByTestId('oidc-identity-locked').textContent).toContain('Turn off "Allow only SSO sign-in"');
  });

  it('does not lock identity fields otherwise', () => {
    render(OidcSettings, { settings: base(), onchange });
    expect(el('oidc-issuer').readOnly).toBe(false);
    expect(screen.queryByTestId('oidc-identity-locked')).toBeNull();
  });

  it('disables SSO-only after the provider identity is edited', async () => {
    render(OidcSettings, { settings: base(), onchange });
    expect(el('oidc-disable-local').disabled).toBe(false);
    await fireEvent.input(el('oidc-client-id'), { target: { value: 'other' } });
    expect(el('oidc-disable-local').disabled).toBe(true);
    expect(screen.getByText('Apply the new provider and sign in with it first.')).toBeTruthy();
    await fireEvent.input(el('oidc-client-id'), { target: { value: 'muximux' } });
    expect(el('oidc-disable-local').disabled).toBe(false);
    await fireEvent.input(el('oidc-client-secret'), { target: { value: 'new' } });
    expect(el('oidc-disable-local').disabled).toBe(true);
    await fireEvent.input(el('oidc-client-secret'), { target: { value: '' } });
    await fireEvent.input(el('oidc-issuer'), { target: { value: 'https://other.example.com' } });
    expect(el('oidc-disable-local').disabled).toBe(true);
  });

  it('shows only the needs-SSO hint when both rules apply', async () => {
    render(OidcSettings, { settings: base({ current_session_is_oidc: false }), onchange });
    await fireEvent.input(el('oidc-client-id'), { target: { value: 'other' } });
    expect(screen.getByText('Sign in with SSO once to turn this on.')).toBeTruthy();
    expect(screen.queryByText('Apply the new provider and sign in with it first.')).toBeNull();
  });

  describe('test connection', () => {
    const ok: OIDCTestResult = {
      reachable: true, authorization: true, token: true, userinfo: false, jwks: true, end_session: true, backchannel_supported: false,
    };

    it('lists capability results', async () => {
      mockTest.mockResolvedValue(ok);
      render(OidcSettings, { settings: base(), onchange });
      await fireEvent.click(screen.getByText('Test connection'));
      await waitFor(() => expect(screen.getByText('Provider reachable')).toBeTruthy());
      expect(mockTest).toHaveBeenCalledWith('https://auth.example.com');
      expect(screen.getByTestId('oidc-cap-token').textContent).toContain('✓');
      expect(screen.getByTestId('oidc-cap-userinfo').textContent).toContain('✗');
      expect(screen.getByTestId('oidc-cap-backchannel').textContent).toContain('✗');
      expect(screen.getByTestId('oidc-cap-end_session').textContent).toContain('✓');
    });

    it('shows the error for an unreachable provider', async () => {
      mockTest.mockResolvedValue({ ...ok, reachable: false, error: 'boom' });
      render(OidcSettings, { settings: base(), onchange });
      await fireEvent.click(screen.getByText('Test connection'));
      await waitFor(() => expect(screen.getByText('Provider not reachable: boom')).toBeTruthy());
    });

    it('shows the error when the request throws', async () => {
      mockTest.mockRejectedValue(new Error('network down'));
      render(OidcSettings, { settings: base(), onchange });
      await fireEvent.click(screen.getByText('Test connection'));
      await waitFor(() => expect(screen.getByText('Provider not reachable: network down')).toBeTruthy());
    });

    it('stringifies non-Error rejections', async () => {
      mockTest.mockRejectedValue('nope');
      render(OidcSettings, { settings: base(), onchange });
      await fireEvent.click(screen.getByText('Test connection'));
      await waitFor(() => expect(screen.getByText('Provider not reachable: nope')).toBeTruthy());
    });

    it('is disabled with an empty issuer and while testing', async () => {
      let resolve!: (v: OIDCTestResult) => void;
      mockTest.mockReturnValue(new Promise<OIDCTestResult>((r) => { resolve = r; }));
      render(OidcSettings, { settings: base(), onchange });
      const btn = screen.getByText('Test connection') as HTMLButtonElement;
      await fireEvent.click(btn);
      expect((screen.getByText('Testing...') as HTMLButtonElement).disabled).toBe(true);
      resolve(ok);
      await waitFor(() => expect(screen.getByText('Test connection')).toBeTruthy());
      await fireEvent.input(el('oidc-issuer'), { target: { value: '' } });
      expect((screen.getByText('Test connection') as HTMLButtonElement).disabled).toBe(true);
    });
  });

  describe('copy buttons', () => {
    it('copies and flips to Copied for 2 seconds', async () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
      render(OidcSettings, { settings: base(), onchange });
      vi.useFakeTimers({ shouldAdvanceTime: true });
      await fireEvent.click(screen.getAllByText('Copy')[0]);
      await waitFor(() => expect(screen.getByText('Copied')).toBeTruthy());
      expect(writeText).toHaveBeenCalledWith('https://mux.example.com/api/auth/oidc/callback');
      vi.advanceTimersByTime(2100);
      await waitFor(() => expect(screen.queryByText('Copied')).toBeNull());
    });

    it('does nothing when the clipboard is unavailable', async () => {
      Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true });
      render(OidcSettings, { settings: base(), onchange });
      await fireEvent.click(screen.getAllByText('Copy')[1]);
      expect(screen.queryByText('Copied')).toBeNull();
    });
  });
});

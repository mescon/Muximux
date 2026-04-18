import { describe, it, expect } from 'vitest';
import {
  forwardAuthPresets,
  applyPreset,
  detectPreset,
  buildForwardAuthRequest,
} from './forwardAuthPresets';

describe('forwardAuthPresets', () => {
  describe('preset data', () => {
    it('has authelia preset with default headers', () => {
      const p = forwardAuthPresets.authelia;
      expect(p.user).toBe('Remote-User');
      expect(p.email).toBe('Remote-Email');
      expect(p.groups).toBe('Remote-Groups');
      expect(p.name).toBe('Remote-Name');
      expect(p.logoutUrl).toBe('https://auth.example.com/logout');
    });

    it('has authentik preset with X-authentik headers', () => {
      const p = forwardAuthPresets.authentik;
      expect(p.user).toBe('X-authentik-username');
      expect(p.email).toBe('X-authentik-email');
      expect(p.groups).toBe('X-authentik-groups');
      expect(p.name).toBe('X-authentik-name');
      expect(p.logoutUrl).toContain('outpost.goauthentik.io');
    });

    it('has custom preset with empty logout URL', () => {
      expect(forwardAuthPresets.custom.logoutUrl).toBe('');
    });
  });

  describe('applyPreset', () => {
    it('returns preset headers', () => {
      const result = applyPreset('authentik', '');
      expect(result.headers.user).toBe('X-authentik-username');
      expect(result.headers.email).toBe('X-authentik-email');
    });

    it('sets logoutUrl from preset when current is empty', () => {
      const result = applyPreset('authelia', '');
      expect(result.logoutUrl).toBe('https://auth.example.com/logout');
    });

    it('sets logoutUrl from preset when current is empty for authentik', () => {
      const result = applyPreset('authentik', '');
      expect(result.logoutUrl).toBe('https://auth.example.com/outpost.goauthentik.io/sign_out');
    });

    it('preserves existing logoutUrl when non-empty', () => {
      const result = applyPreset('authentik', 'https://my-custom.com/logout');
      expect(result.logoutUrl).toBe('https://my-custom.com/logout');
    });

    it('uses empty string for custom preset with no current URL', () => {
      const result = applyPreset('custom', '');
      expect(result.logoutUrl).toBe('');
    });
  });

  describe('detectPreset', () => {
    it('detects authelia from default Remote-User headers', () => {
      expect(detectPreset('Remote-User', 'Remote-Email')).toBe('authelia');
    });

    it('detects authentik from X-authentik headers', () => {
      expect(detectPreset('X-authentik-username', 'X-authentik-email')).toBe('authentik');
    });

    it('returns custom for unknown headers', () => {
      expect(detectPreset('X-Custom-User', 'X-Custom-Email')).toBe('custom');
    });

    it('returns custom when headers partially match', () => {
      expect(detectPreset('Remote-User', 'X-authentik-email')).toBe('custom');
    });
  });

  describe('buildForwardAuthRequest', () => {
    it('builds request with all fields', () => {
      const result = buildForwardAuthRequest(
        '10.0.0.0/8\n172.16.0.0/12',
        'Remote-User',
        'Remote-Email',
        'Remote-Groups',
        'Remote-Name',
        'https://auth.example.com/logout',
      );

      expect(result.trusted_proxies).toEqual(['10.0.0.0/8', '172.16.0.0/12']);
      expect(result.headers).toEqual({
        user: 'Remote-User',
        email: 'Remote-Email',
        groups: 'Remote-Groups',
        name: 'Remote-Name',
      });
      expect(result.logout_url).toBe('https://auth.example.com/logout');
    });

    it('filters empty proxy lines', () => {
      const result = buildForwardAuthRequest(
        '10.0.0.0/8\n\n  \n172.16.0.0/12',
        'u', 'e', 'g', 'n', '',
      );
      expect(result.trusted_proxies).toEqual(['10.0.0.0/8', '172.16.0.0/12']);
    });

    it('handles comma-separated proxies', () => {
      const result = buildForwardAuthRequest(
        '10.0.0.0/8, 172.16.0.0/12',
        'u', 'e', 'g', 'n', '',
      );
      expect(result.trusted_proxies).toEqual(['10.0.0.0/8', '172.16.0.0/12']);
    });

    it('handles empty proxy string', () => {
      const result = buildForwardAuthRequest('', 'u', 'e', 'g', 'n', '');
      expect(result.trusted_proxies).toEqual([]);
    });

    it('includes logout_url even when empty', () => {
      const result = buildForwardAuthRequest('10.0.0.0/8', 'u', 'e', 'g', 'n', '');
      expect(result.logout_url).toBe('');
    });
  });
});

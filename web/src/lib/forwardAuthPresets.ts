import type { ChangeAuthMethodRequest } from './types';

export interface ForwardAuthPreset {
  user: string;
  email: string;
  groups: string;
  name: string;
  logoutUrl: string;
}

export const forwardAuthPresets: Record<string, ForwardAuthPreset> = {
  authelia: {
    user: 'Remote-User',
    email: 'Remote-Email',
    groups: 'Remote-Groups',
    name: 'Remote-Name',
    logoutUrl: 'https://auth.example.com/logout',
  },
  authentik: {
    user: 'X-authentik-username',
    email: 'X-authentik-email',
    groups: 'X-authentik-groups',
    name: 'X-authentik-name',
    logoutUrl: 'https://auth.example.com/outpost.goauthentik.io/sign_out',
  },
  custom: {
    user: 'Remote-User',
    email: 'Remote-Email',
    groups: 'Remote-Groups',
    name: 'Remote-Name',
    logoutUrl: '',
  },
};

export type PresetName = keyof typeof forwardAuthPresets;

/**
 * Applies a preset's values to current field state.
 * Only sets logoutUrl if the current value is empty (preserves user-entered values).
 */
export function applyPreset(
  preset: PresetName,
  currentLogoutUrl: string,
): { headers: ForwardAuthPreset; logoutUrl: string } {
  const headers = forwardAuthPresets[preset];
  return {
    headers,
    logoutUrl: currentLogoutUrl || headers.logoutUrl,
  };
}

/**
 * Detects which preset matches the given header values.
 */
export function detectPreset(headerUser: string, headerEmail: string): PresetName {
  if (headerUser === forwardAuthPresets.authentik.user && headerEmail === forwardAuthPresets.authentik.email) {
    return 'authentik';
  }
  if (headerUser === forwardAuthPresets.authelia.user && headerEmail === forwardAuthPresets.authelia.email) {
    return 'authelia';
  }
  return 'custom';
}

/**
 * Builds the forward_auth-specific fields for a ChangeAuthMethodRequest.
 */
export function buildForwardAuthRequest(
  trustedProxies: string,
  headerUser: string,
  headerEmail: string,
  headerGroups: string,
  headerName: string,
  logoutUrl: string,
): Partial<ChangeAuthMethodRequest> {
  return {
    trusted_proxies: trustedProxies
      .split(/[,\n]/)
      .map(s => s.trim())
      .filter(s => s.length > 0),
    headers: {
      user: headerUser,
      email: headerEmail,
      groups: headerGroups,
      name: headerName,
    },
    logout_url: logoutUrl,
  };
}

<script lang="ts">
  import { untrack } from 'svelte';
  import type { OIDCSettings, OIDCSettingsUpdate, OIDCTestResult } from '$lib/types';
  import { testOIDCProvider, errorText } from '$lib/api';
  import * as m from '$lib/paraglide/messages.js';

  let { settings, onchange }: {
    settings: OIDCSettings;
    onchange: (update: OIDCSettingsUpdate, valid: boolean) => void;
  } = $props();

  const inputClass = 'w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 read-only:opacity-70';
  const checkboxRowClass = 'flex items-center gap-3 cursor-pointer';
  const checkboxClass = 'w-4 h-4 rounded border-border-subtle text-accent-text focus:ring-brand-500';

  // The form is initialised once from the loaded settings; later edits stay local.
  const initial = untrack(() => settings);
  let issuerUrl = $state(initial.issuer_url);
  let clientId = $state(initial.client_id);
  let clientSecret = $state('');
  let redirectUrl = $state(initial.redirect_url);
  let scopes = $state((initial.scopes ?? []).join(' '));
  let usernameClaim = $state(initial.username_claim);
  let emailClaim = $state(initial.email_claim);
  let displayNameClaim = $state(initial.display_name_claim);
  let groupsClaim = $state(initial.groups_claim);
  let adminGroups = $state((initial.admin_groups ?? []).join(', '));
  let providerLogout = $state(initial.provider_logout);
  let postLogoutUrl = $state(initial.post_logout_redirect_url);
  let logoutUrl = $state(initial.logout_url);
  let autoRedirect = $state(initial.auto_redirect);
  let disableLocal = $state(initial.disable_local_login);

  let testing = $state(false);
  let testResult = $state<OIDCTestResult | null>(null);
  let copied = $state<string | null>(null);

  const envFields = $derived(settings.env_fields ?? {});
  const fromEnv = (field: string) => field in envFields;
  const identityLocked = $derived(settings.enabled && settings.disable_local_login);
  const identityChanged = $derived(
    issuerUrl.trim() !== settings.issuer_url || clientId.trim() !== settings.client_id || clientSecret !== ''
  );
  const needsSso = $derived(!settings.current_session_is_oidc);
  const disableLocalDisabled = $derived(!disableLocal && (needsSso || identityChanged));
  // SSO-only newly ticked together with a provider change would be refused by the server.
  const newSsoOnlyWithChange = $derived(disableLocal && !identityLocked && identityChanged);
  const disableLocalHint = $derived(
    disableLocalDisabled || newSsoOnlyWithChange
      ? (needsSso ? m.oidc_disable_local_needs_sso() : m.oidc_disable_local_after_change())
      : ''
  );
  const callbackUrl = $derived(redirectUrl.trim() || settings.default_callback_url);

  function splitList(value: string, sep: RegExp): string[] {
    return value.split(sep).map((s) => s.trim()).filter((s) => s !== '');
  }

  function isAbsoluteHttpUrl(value: string): boolean {
    try {
      const u = new URL(value);
      return (u.protocol === 'http:' || u.protocol === 'https:') && u.host !== '';
    } catch {
      return false;
    }
  }

  const optionalUrlOk = (value: string) => value.trim() === '' || isAbsoluteHttpUrl(value.trim());

  const payload = $derived.by(() => {
    const p: Record<string, unknown> = {
      issuer_url: issuerUrl.trim(),
      client_id: clientId.trim(),
      redirect_url: redirectUrl.trim(),
      scopes: splitList(scopes, /\s+/),
      username_claim: usernameClaim.trim(),
      email_claim: emailClaim.trim(),
      display_name_claim: displayNameClaim.trim(),
      groups_claim: groupsClaim.trim(),
      admin_groups: splitList(adminGroups, /,/),
      provider_logout: providerLogout,
      post_logout_redirect_url: postLogoutUrl.trim(),
      logout_url: logoutUrl.trim(),
      auto_redirect: autoRedirect,
      disable_local_login: disableLocal,
    };
    if (clientSecret !== '') p.client_secret = clientSecret;
    for (const f of Object.keys(envFields)) delete p[f];
    return p as OIDCSettingsUpdate;
  });

  const valid = $derived(
    (fromEnv('issuer_url') || isAbsoluteHttpUrl(issuerUrl.trim())) &&
    (fromEnv('client_id') || clientId.trim() !== '') &&
    (fromEnv('redirect_url') || optionalUrlOk(redirectUrl)) &&
    (fromEnv('post_logout_redirect_url') || optionalUrlOk(postLogoutUrl)) &&
    (fromEnv('logout_url') || optionalUrlOk(logoutUrl)) &&
    !newSsoOnlyWithChange
  );

  $effect(() => {
    const p = payload;
    const v = valid;
    untrack(() => onchange(p, v));
  });

  async function runTest() {
    testing = true;
    testResult = null;
    try {
      testResult = await testOIDCProvider(issuerUrl.trim());
    } catch (e) {
      testResult = {
        reachable: false, error: errorText(e, String(e)),
        authorization: false, token: false, userinfo: false, jwks: false, end_session: false, backchannel_supported: false,
      };
    } finally {
      testing = false;
    }
  }

  async function copy(id: string, text: string) {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      return;
    }
    copied = id;
    setTimeout(() => { if (copied === id) copied = null; }, 2000);
  }

  const caps = $derived(testResult ? [
    { key: 'authorization', label: m.oidc_cap_authorization(), ok: testResult.authorization },
    { key: 'token', label: m.oidc_cap_token(), ok: testResult.token },
    { key: 'userinfo', label: m.oidc_cap_userinfo(), ok: testResult.userinfo },
    { key: 'jwks', label: m.oidc_cap_jwks(), ok: testResult.jwks },
    { key: 'end_session', label: m.oidc_cap_end_session(), ok: testResult.end_session },
    { key: 'backchannel', label: m.oidc_cap_backchannel(), ok: testResult.backchannel_supported },
  ] : []);
</script>

{#snippet envNote(field: string)}
  {#if fromEnv(field)}
    <p class="text-xs text-text-disabled mt-1">{m.oidc_from_env({ name: envFields[field] })}</p>
  {/if}
{/snippet}

{#snippet copyRow(id: string, label: string, value: string)}
  <div>
    <label for="oidc-{id}" class="block text-sm text-text-muted mb-1">{label}</label>
    <div class="flex gap-2">
      <input id="oidc-{id}" type="text" readonly value={value} class={inputClass} />
      <button type="button" class="px-3 py-2 text-sm rounded-md border border-border-subtle bg-bg-elevated text-text-muted hover:text-text-primary transition-colors"
        onclick={() => copy(id, value)}>
        {copied === id ? m.oidc_copied() : m.oidc_copy()}
      </button>
    </div>
  </div>
{/snippet}

<div class="space-y-4" data-testid="oidc-settings">
  <div>
    <label for="oidc-issuer" class="block text-sm text-text-muted mb-1">{m.oidc_issuer()}</label>
    <input id="oidc-issuer" type="url" bind:value={issuerUrl} readonly={fromEnv('issuer_url') || identityLocked}
      class={inputClass} placeholder="https://auth.example.com/realms/main" />
    {@render envNote('issuer_url')}
  </div>

  <div>
    <label for="oidc-client-id" class="block text-sm text-text-muted mb-1">{m.oidc_client_id()}</label>
    <input id="oidc-client-id" type="text" bind:value={clientId} readonly={fromEnv('client_id') || identityLocked}
      class={inputClass} />
    {@render envNote('client_id')}
  </div>

  <div>
    <label for="oidc-client-secret" class="block text-sm text-text-muted mb-1">{m.oidc_client_secret()}</label>
    {#if fromEnv('client_secret')}
      <input id="oidc-client-secret" type="password" readonly value="" class={inputClass} />
      {@render envNote('client_secret')}
    {:else}
      <input id="oidc-client-secret" type="password" bind:value={clientSecret} readonly={identityLocked}
        autocomplete="new-password" class={inputClass}
        placeholder={settings.client_secret_set ? m.oidc_secret_set() : m.oidc_secret_not_set()} />
    {/if}
  </div>
  {#if identityLocked}
    <p class="text-xs text-text-disabled" data-testid="oidc-identity-locked">{m.oidc_identity_locked_sso()}</p>
  {/if}

  <div>
    <label for="oidc-redirect" class="block text-sm text-text-muted mb-1">{m.oidc_redirect_url()}</label>
    <input id="oidc-redirect" type="url" bind:value={redirectUrl} readonly={fromEnv('redirect_url')}
      class={inputClass} placeholder={settings.default_callback_url} />
    {@render envNote('redirect_url')}
  </div>

  <div>
    <label for="oidc-scopes" class="block text-sm text-text-muted mb-1">{m.oidc_scopes()}</label>
    <input id="oidc-scopes" type="text" bind:value={scopes} readonly={fromEnv('scopes')} class={inputClass}
      placeholder="openid profile email" />
    {@render envNote('scopes')}
  </div>

  <div class="space-y-3">
    <h4 class="text-sm font-medium text-text-primary">{m.oidc_claims()}</h4>
    <!-- Placeholders are the defaults NewOIDCProvider (internal/auth/oidc.go) uses for an empty value. -->
    <div class="grid grid-cols-2 gap-3">
      <div>
        <label for="oidc-username-claim" class="block text-xs text-text-muted mb-1">{m.oidc_username_claim()}</label>
        <input id="oidc-username-claim" type="text" bind:value={usernameClaim} placeholder="preferred_username" readonly={fromEnv('username_claim')} class={inputClass} />
        {@render envNote('username_claim')}
      </div>
      <div>
        <label for="oidc-email-claim" class="block text-xs text-text-muted mb-1">{m.oidc_email_claim()}</label>
        <input id="oidc-email-claim" type="text" bind:value={emailClaim} placeholder="email" readonly={fromEnv('email_claim')} class={inputClass} />
        {@render envNote('email_claim')}
      </div>
      <div>
        <label for="oidc-display-name-claim" class="block text-xs text-text-muted mb-1">{m.oidc_display_name_claim()}</label>
        <input id="oidc-display-name-claim" type="text" bind:value={displayNameClaim} placeholder="name" readonly={fromEnv('display_name_claim')} class={inputClass} />
        {@render envNote('display_name_claim')}
      </div>
      <div>
        <label for="oidc-groups-claim" class="block text-xs text-text-muted mb-1">{m.oidc_groups_claim()}</label>
        <input id="oidc-groups-claim" type="text" bind:value={groupsClaim} placeholder="groups" readonly={fromEnv('groups_claim')} class={inputClass} />
        {@render envNote('groups_claim')}
      </div>
    </div>
    <div>
      <label for="oidc-admin-groups" class="block text-sm text-text-muted mb-1">{m.oidc_admin_groups()}</label>
      <input id="oidc-admin-groups" type="text" bind:value={adminGroups} readonly={fromEnv('admin_groups')} class={inputClass}
        placeholder="muximux-admins, homelab" />
      {@render envNote('admin_groups')}
    </div>
  </div>

  <div class="space-y-3">
    <h4 class="text-sm font-medium text-text-primary">{m.oidc_signout_heading()}</h4>

    <label class={checkboxRowClass}>
      <input type="checkbox" bind:checked={providerLogout} class={checkboxClass} />
      <span class="text-sm text-text-primary">{m.oidc_provider_logout()}</span>
    </label>

    <div>
      <label for="oidc-post-logout" class="block text-sm text-text-muted mb-1">{m.oidc_post_logout()}</label>
      <input id="oidc-post-logout" type="url" bind:value={postLogoutUrl} readonly={fromEnv('post_logout_redirect_url')} class={inputClass} />
      {@render envNote('post_logout_redirect_url')}
    </div>

    <div>
      <label for="oidc-logout-url" class="block text-sm text-text-muted mb-1">{m.oidc_logout_url()}</label>
      <input id="oidc-logout-url" type="url" bind:value={logoutUrl} readonly={fromEnv('logout_url')} class={inputClass} />
      {@render envNote('logout_url')}
    </div>

    <label class={checkboxRowClass}>
      <input type="checkbox" bind:checked={autoRedirect} class={checkboxClass} />
      <span class="text-sm text-text-primary">{m.oidc_auto_redirect()}</span>
    </label>

    <div>
      <label class={checkboxRowClass}>
        <input type="checkbox" id="oidc-disable-local" bind:checked={disableLocal} disabled={disableLocalDisabled} class={checkboxClass} />
        <span class="text-sm text-text-primary">{m.oidc_disable_local()}</span>
      </label>
      {#if disableLocalHint}
        <p class="text-xs text-text-disabled mt-1" data-testid="oidc-disable-local-hint">{disableLocalHint}</p>
      {/if}
      {#if disableLocal}
        <p class="text-xs text-warning-text mt-1" data-testid="oidc-disable-local-warning">{m.oidc_disable_local_warning()}</p>
      {/if}
    </div>
  </div>

  {@render copyRow('callback', m.oidc_callback_url(), callbackUrl)}
  {@render copyRow('backchannel', m.oidc_backchannel_url(), settings.backchannel_url)}

  <div class="space-y-2">
    <button type="button" class="px-3 py-2 text-sm rounded-md border border-border-subtle bg-bg-elevated text-text-primary hover:bg-bg-hover transition-colors disabled:opacity-50"
      disabled={testing || issuerUrl.trim() === ''} onclick={runTest}>
      {testing ? m.oidc_testing() : m.oidc_test()}
    </button>
    {#if testResult}
      <div data-testid="oidc-test-result" aria-live="polite">
        {#if testResult.reachable}
          <p class="text-sm text-success-text">{m.oidc_test_ok()}</p>
          <ul class="mt-1 space-y-0.5">
            {#each caps as c (c.key)}
              <li class="text-xs {c.ok ? 'text-success-text' : 'text-text-disabled'}" data-testid="oidc-cap-{c.key}">
                <span aria-hidden="true">{c.ok ? '✓' : '✗'}</span> {c.label}
              </li>
            {/each}
          </ul>
        {:else}
          <p class="text-sm text-danger-text">{m.oidc_test_failed({ error: testResult.error ?? '' })}</p>
        {/if}
      </div>
    {/if}
  </div>
</div>

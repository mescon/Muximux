<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { fly } from 'svelte/transition';
  import type { Config, UserInfo, ChangeAuthMethodRequest } from '$lib/types';
  import { listUsers, createUser, updateUser, deleteUserAccount, changeAuthMethod, getAPIKeyStatus, generateAPIKey, deleteAPIKey } from '$lib/api';
  import { changePassword, isAdmin, currentUser } from '$lib/authStore';
  import { forwardAuthPresets, applyPreset, detectPreset, buildForwardAuthRequest, type PresetName } from '$lib/forwardAuthPresets';
  import * as m from '$lib/paraglide/messages.js';

  let { localConfig }: { localConfig: Config } = $props();

  // Security tab state
  let securityUsers = $state<UserInfo[]>([]);
  let securityLoading = $state(false);
  let securityError = $state<string | null>(null);
  let securitySuccess = $state<string | null>(null);

  // Change password
  let cpCurrent = $state('');
  let cpNew = $state('');
  let cpConfirm = $state('');
  let cpLoading = $state(false);
  let cpMessage = $state<{ type: 'success' | 'error'; text: string } | null>(null);

  // Add user
  let showAddUser = $state(false);
  let newUserName = $state('');
  let newUserPassword = $state('');
  let newUserRole = $state('user');
  let setupUsername = $state('');
  let setupPassword = $state('');
  let addUserLoading = $state(false);
  let addUserError = $state<string | null>(null);

  // Delete user confirmation
  let confirmDeleteUser = $state<string | null>(null);

  // Auth method switching
  let selectedAuthMethod = $state<'builtin' | 'forward_auth' | 'none'>('none');
  let methodTrustedProxies = $state('');
  let methodLoading = $state(false);
  let methodError = $state<string | null>(null);

  // API key management
  let apiKeyConfigured = $state<boolean | null>(null); // null until first status fetch
  let apiKeyLoading = $state(false);
  let apiKeyError = $state<string | null>(null);
  let apiKeyPlaintext = $state<string | null>(null); // only set immediately after generate
  let apiKeyCopied = $state(false);
  let confirmRotateApiKey = $state(false);
  let confirmDeleteApiKey = $state(false);

  async function loadAPIKeyStatus() {
    try {
      const status = await getAPIKeyStatus();
      apiKeyConfigured = status.configured;
    } catch (e) {
      apiKeyError = e instanceof Error ? e.message : 'Failed to load API key status';
    }
  }

  async function handleGenerateAPIKey() {
    apiKeyLoading = true;
    apiKeyError = null;
    apiKeyCopied = false;
    try {
      const result = await generateAPIKey();
      if (result.success) {
        apiKeyPlaintext = result.key;
        apiKeyConfigured = true;
        confirmRotateApiKey = false;
      } else {
        apiKeyError = result.message || 'Failed to generate API key';
      }
    } catch (e) {
      apiKeyError = e instanceof Error ? e.message : 'Failed to generate API key';
    } finally {
      apiKeyLoading = false;
    }
  }

  async function handleDeleteAPIKey() {
    apiKeyLoading = true;
    apiKeyError = null;
    try {
      await deleteAPIKey();
      apiKeyConfigured = false;
      apiKeyPlaintext = null;
      confirmDeleteApiKey = false;
    } catch (e) {
      apiKeyError = e instanceof Error ? e.message : 'Failed to delete API key';
    } finally {
      apiKeyLoading = false;
    }
  }

  async function copyAPIKeyToClipboard() {
    if (!apiKeyPlaintext) return;
    try {
      await navigator.clipboard.writeText(apiKeyPlaintext);
      apiKeyCopied = true;
      setTimeout(() => { apiKeyCopied = false; }, 2000);
    } catch {
      apiKeyError = 'Clipboard write failed; copy the key manually.';
    }
  }

  function dismissAPIKeyPlaintext() {
    apiKeyPlaintext = null;
    apiKeyCopied = false;
  }

  // Forward auth preset & header fields
  let faPreset = $state<PresetName>('authelia');
  let faShowAdvanced = $state(false);
  let faHeaderUser = $state('Remote-User');
  let faHeaderEmail = $state('Remote-Email');
  let faHeaderGroups = $state('Remote-Groups');
  let faHeaderName = $state('Remote-Name');
  let faLogoutUrl = $state('');

  function selectFaPreset(p: PresetName) {
    faPreset = p;
    const result = applyPreset(p, faLogoutUrl);
    faHeaderUser = result.headers.user;
    faHeaderEmail = result.headers.email;
    faHeaderGroups = result.headers.groups;
    faHeaderName = result.headers.name;
    faLogoutUrl = result.logoutUrl;
  }

  // Security tab functions
  async function loadSecurityUsers() {
    securityLoading = true;
    securityError = null;
    try {
      securityUsers = (await listUsers()) ?? [];
    } catch (e) {
      securityError = e instanceof Error ? e.message : m.error_failedLoadUsers();
    } finally {
      securityLoading = false;
    }
  }

  async function handleChangePassword() {
    if (cpNew.length < 8 || cpNew !== cpConfirm) return;
    cpLoading = true;
    cpMessage = null;
    const result = await changePassword(cpCurrent, cpNew);
    cpLoading = false;
    if (result.success) {
      cpMessage = { type: 'success', text: m.toast_passwordChanged() };
      cpCurrent = '';
      cpNew = '';
      cpConfirm = '';
    } else {
      cpMessage = { type: 'error', text: result.message || m.error_failedChangePassword() };
    }
  }

  async function handleAddUser() {
    if (!newUserName.trim() || newUserPassword.length < 8) return;
    addUserLoading = true;
    addUserError = null;
    try {
      const result = await createUser({
        username: newUserName.trim(),
        password: newUserPassword,
        role: newUserRole,
      });
      if (result.success) {
        newUserName = '';
        newUserPassword = '';
        newUserRole = 'user';
        showAddUser = false;
        await loadSecurityUsers();
      } else {
        addUserError = result.message || m.error_failedCreateUser();
      }
    } catch (e) {
      addUserError = e instanceof Error ? e.message : m.error_failedCreateUser();
    } finally {
      addUserLoading = false;
    }
  }

  async function handleUpdateUserRole(username: string, role: string) {
    try {
      await updateUser(username, { role });
      await loadSecurityUsers();
    } catch (e) {
      securityError = e instanceof Error ? e.message : m.error_failedUpdateUser();
    }
  }

  async function handleUpdateUserGroups(username: string, raw: string) {
    // Comma-separated input, trimmed and de-empty'd; pass [] to clear so
    // the backend distinguishes "explicitly cleared" from "omitted".
    const groups = raw.split(',').map(g => g.trim()).filter(g => g.length > 0);
    try {
      await updateUser(username, { groups });
      await loadSecurityUsers();
    } catch (e) {
      securityError = e instanceof Error ? e.message : m.error_failedUpdateUser();
    }
  }

  async function handleDeleteUser(username: string) {
    try {
      await deleteUserAccount(username);
      confirmDeleteUser = null;
      await loadSecurityUsers();
    } catch (e) {
      securityError = e instanceof Error ? e.message : m.error_failedDeleteUser();
    }
  }

  async function handleChangeAuthMethod() {
    methodLoading = true;
    methodError = null;
    const previousMethod = localConfig.auth?.method || 'none';
    const req: ChangeAuthMethodRequest = { method: selectedAuthMethod };
    if (selectedAuthMethod === 'forward_auth') {
      Object.assign(req, buildForwardAuthRequest(
        methodTrustedProxies, faHeaderUser, faHeaderEmail, faHeaderGroups, faHeaderName, faLogoutUrl,
      ));
    }
    try {
      const result = await changeAuthMethod(req);
      if (result.success) {
        // Sync localConfig so derived dirty-checks reset
        if (!localConfig.auth) localConfig.auth = { method: selectedAuthMethod };
        localConfig.auth.method = selectedAuthMethod;
        if (selectedAuthMethod === 'forward_auth') {
          localConfig.auth.trusted_proxies = req.trusted_proxies;
          localConfig.auth.headers = req.headers;
          localConfig.auth.logout_url = req.logout_url;
        }

        // Switching FROM "none" to an auth method — the virtual admin session is now invalid.
        if (previousMethod === 'none' && selectedAuthMethod !== 'none') {
          if (selectedAuthMethod === 'forward_auth') {
            // Don't reload — the user is accessing directly (not through their proxy),
            // so a reload would lock them out. Show a persistent message instead.
            securitySuccess = m.common_forwardAuthEnabled();
          } else {
            // For builtin auth, reload so the user can log in with credentials.
            sessionStorage.setItem('muximux_return_to', 'security');
            window.location.reload();
            return;
          }
        } else {
          securitySuccess = m.toast_authMethodChanged({ method: selectedAuthMethod });
          setTimeout(() => securitySuccess = null, 3000);
        }
      } else {
        methodError = result.message || m.error_failedChangeMethod();
      }
    } catch (e) {
      methodError = e instanceof Error ? e.message : m.error_failedChangeMethod();
    } finally {
      methodLoading = false;
    }
  }

  // Derived values for template
  let currentMethod = $derived(localConfig.auth?.method || 'none');
  let methodChanged = $derived(selectedAuthMethod !== currentMethod);
  let faFieldsChanged = $derived(selectedAuthMethod === 'forward_auth' && currentMethod === 'forward_auth' && (
    methodTrustedProxies !== (localConfig.auth?.trusted_proxies?.join('\n') || '') ||
    faHeaderUser !== (localConfig.auth?.headers?.user || 'Remote-User') ||
    faHeaderEmail !== (localConfig.auth?.headers?.email || 'Remote-Email') ||
    faHeaderGroups !== (localConfig.auth?.headers?.groups || 'Remote-Groups') ||
    faHeaderName !== (localConfig.auth?.headers?.name || 'Remote-Name') ||
    faLogoutUrl !== (localConfig.auth?.logout_url || '')
  ));
  let showUpdateBtn = $derived(methodChanged || faFieldsChanged);

  // Load security users and initialize auth fields on mount
  onMount(() => {
    if ($isAdmin) {
      loadSecurityUsers();
      loadAPIKeyStatus();
    }

    selectedAuthMethod = (localConfig.auth?.method || 'none') as typeof selectedAuthMethod;
    // Pre-fill forward auth fields from existing config
    const proxies = localConfig.auth?.trusted_proxies;
    methodTrustedProxies = proxies?.length ? proxies.join('\n') : '';
    faLogoutUrl = localConfig.auth?.logout_url || '';
    const h = localConfig.auth?.headers;
    if (h) {
      faHeaderUser = h.user || 'Remote-User';
      faHeaderEmail = h.email || 'Remote-Email';
      faHeaderGroups = h.groups || 'Remote-Groups';
      faHeaderName = h.name || 'Remote-Name';
      faPreset = detectPreset(faHeaderUser, faHeaderEmail);
    }
  });
</script>

<div class="space-y-8">
  {#if securitySuccess}
    <div class="p-3 rounded-lg bg-green-500/10 border border-green-500/20 text-green-400 text-sm">
      {securitySuccess}
    </div>
  {/if}

  <!-- Authentication Method -->
  <div>
    <h3 class="text-lg font-semibold text-text-primary mb-1">{m.security_authMethod()}</h3>
    <p class="text-sm text-text-muted mb-4">{m.security_authMethodDesc()}</p>

    <div class="space-y-3">
      <!-- Password card -->
      <div
        class="rounded-xl border text-start transition-all overflow-hidden
               {selectedAuthMethod === 'builtin' ? 'border-brand-500 bg-brand-500/10' : 'border-border bg-bg-surface hover:border-border'}"
      >
        <button class="w-full p-4 flex items-start gap-4" onclick={() => { selectedAuthMethod = 'builtin'; }}>
          <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center flex-shrink-0 mt-0.5">
            <svg class="w-5 h-5 text-brand-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="11" width="18" height="11" rx="2" />
              <path d="M7 11V7a5 5 0 0110 0v4" />
            </svg>
          </div>
          <div class="flex-1 text-start">
            <div class="flex items-center gap-2">
              <h3 class="font-semibold text-text-primary">{m.security_passwordAuth()}</h3>
              {#if currentMethod === 'builtin'}
                <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-green-500/20 text-green-400 uppercase tracking-wider">{m.common_current()}</span>
              {/if}
            </div>
            <p class="text-sm text-text-muted mt-1">{m.security_passwordAuthDesc()}</p>
          </div>
        </button>
        {#if selectedAuthMethod === 'builtin'}
          <div class="px-4 pb-4 pt-0 ms-14" in:fly={{ y: -8, duration: 200 }}>
            <div class="border-t border-border pt-4">
              {#if currentMethod === 'builtin'}
                <p class="text-sm text-text-muted mb-4">{m.security_passwordAuthActive()}</p>

                <!-- Change Password (inline) -->
                <h4 class="text-sm font-semibold text-text-primary mb-2">{m.security_changePassword()}</h4>
                <div class="max-w-sm space-y-3">
                  <div>
                    <label for="cp-current" class="block text-xs text-text-muted mb-1">{m.security_currentPassword()}</label>
                    <input
                      id="cp-current"
                      type="password"
                      bind:value={cpCurrent}
                      class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                             focus:outline-none focus:ring-2 focus:ring-brand-500"
                      autocomplete="current-password"
                    />
                  </div>
                  <div>
                    <label for="cp-new" class="block text-xs text-text-muted mb-1">{m.security_newPassword()}</label>
                    <input
                      id="cp-new"
                      type="password"
                      bind:value={cpNew}
                      class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                             focus:outline-none focus:ring-2 focus:ring-brand-500"
                      placeholder={m.security_minEightChars()}
                      autocomplete="new-password"
                    />
                    {#if cpNew.length > 0 && cpNew.length < 8}
                      <p class="text-red-400 text-xs mt-1">{m.error_passwordTooShort()}</p>
                    {/if}
                  </div>
                  <div>
                    <label for="cp-confirm" class="block text-xs text-text-muted mb-1">{m.security_confirmNewPassword()}</label>
                    <input
                      id="cp-confirm"
                      type="password"
                      bind:value={cpConfirm}
                      class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                             focus:outline-none focus:ring-2 focus:ring-brand-500"
                      autocomplete="new-password"
                    />
                    {#if cpConfirm.length > 0 && cpNew !== cpConfirm}
                      <p class="text-red-400 text-xs mt-1">{m.error_passwordsMismatch()}</p>
                    {/if}
                  </div>

                  {#if cpMessage}
                    <div class="p-3 rounded-lg text-sm {cpMessage.type === 'success' ? 'bg-green-500/10 border border-green-500/20 text-green-400' : 'bg-red-500/10 border border-red-500/20 text-red-400'}">
                      {cpMessage.text}
                    </div>
                  {/if}

                  <button
                    class="btn btn-primary btn-sm disabled:opacity-50 flex items-center gap-2"
                    disabled={cpLoading || cpNew.length < 8 || cpNew !== cpConfirm || !cpCurrent}
                    onclick={handleChangePassword}
                  >
                    {#if cpLoading}
                      <span class="inline-block w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></span>
                    {/if}
                    {m.security_changePassword()}
                  </button>
                </div>
              {:else if securityUsers.length > 0}
                <p class="text-sm text-text-muted">{m.security_switchToPasswordAuth()}</p>
              {:else}
                <p class="text-sm text-text-muted mb-3">{m.security_createFirstUser()}</p>
                <div class="space-y-3 max-w-sm">
                  <div>
                    <label for="setup-username" class="block text-xs text-text-muted mb-1">{m.common_username()}</label>
                    <input
                      id="setup-username"
                      type="text"
                      bind:value={setupUsername}
                      class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                             focus:outline-none focus:ring-2 focus:ring-brand-500"
                      placeholder="admin"
                    />
                  </div>
                  <div>
                    <label for="setup-password" class="block text-xs text-text-muted mb-1">{m.security_passwordMinChars()}</label>
                    <input
                      id="setup-password"
                      type="password"
                      bind:value={setupPassword}
                      class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                             focus:outline-none focus:ring-2 focus:ring-brand-500"
                      placeholder="••••••••"
                    />
                  </div>
                  {#if addUserError}
                    <p class="text-red-400 text-xs">{addUserError}</p>
                  {/if}
                  {#if !setupUsername.trim() && setupPassword.length > 0}
                    <p class="text-amber-400 text-xs">{m.error_usernameRequired()}</p>
                  {:else if setupUsername.trim() && setupPassword.length > 0 && setupPassword.length < 8}
                    <p class="text-amber-400 text-xs">{m.error_passwordTooShortCount({ count: `${setupPassword.length}` })}</p>
                  {/if}
                  <button
                    class="btn btn-primary btn-sm disabled:opacity-50 flex items-center gap-2"
                    disabled={addUserLoading || !setupUsername.trim() || setupPassword.length < 8}
                    onclick={async () => {
                      const savedUser = setupUsername.trim();
                      const savedPass = setupPassword;
                      // Bridge to handleAddUser via shared state
                      newUserName = savedUser;
                      newUserPassword = savedPass;
                      newUserRole = 'admin';
                      await handleAddUser();
                      if (securityUsers.length > 0) {
                        // Call API directly (not handleChangeAuthMethod which reloads)
                        methodLoading = true;
                        try {
                          const result = await changeAuthMethod({ method: 'builtin' });
                          if (!result.success) {
                            methodError = result.message || m.error_failedEnableAuth();
                            return;
                          }
                        } catch (e) {
                          methodError = e instanceof Error ? e.message : m.error_failedEnableAuth();
                          return;
                        } finally {
                          methodLoading = false;
                        }
                        // Auth middleware is now "builtin" — store credentials for auto-login after reload
                        sessionStorage.setItem('muximux_return_to', 'security');
                        sessionStorage.setItem('muximux_auto_login', JSON.stringify({ u: savedUser, p: savedPass }));
                        window.location.reload();
                      }
                    }}
                  >
                    {#if addUserLoading || methodLoading}
                      <span class="inline-block w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></span>
                    {/if}
                    {m.security_createUserEnable()}
                  </button>
                </div>
              {/if}
            </div>
          </div>
        {/if}
      </div>

      <!-- Auth Proxy card -->
      <div
        class="rounded-xl border text-start transition-all overflow-hidden
               {selectedAuthMethod === 'forward_auth' ? 'border-brand-500 bg-brand-500/10' : 'border-border bg-bg-surface hover:border-border'}"
      >
        <button class="w-full p-4 flex items-start gap-4" onclick={async () => { selectedAuthMethod = 'forward_auth'; await tick(); document.getElementById('settings-proxies')?.focus(); }}>
          <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center flex-shrink-0 mt-0.5">
            <svg class="w-5 h-5 text-brand-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
            </svg>
          </div>
          <div class="flex-1 text-start">
            <div class="flex items-center gap-2">
              <h3 class="font-semibold text-text-primary">{m.security_authProxy()}</h3>
              {#if currentMethod === 'forward_auth'}
                <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-green-500/20 text-green-400 uppercase tracking-wider">{m.common_current()}</span>
              {/if}
            </div>
            <p class="text-sm text-text-muted mt-1">{m.security_authProxyDesc()}</p>
          </div>
        </button>
        {#if selectedAuthMethod === 'forward_auth'}
          <div class="px-4 pb-4 pt-0 space-y-4 ms-14" in:fly={{ y: -8, duration: 200 }}>
            <div class="border-t border-border pt-4">
              <span class="block text-sm text-text-muted mb-2">{m.security_proxyType()}</span>
              <div class="flex gap-2">
                {#each ['authelia', 'authentik', 'custom'] as p (p)}
                  <button
                    class="flex-1 px-3 py-2 text-sm rounded-md border transition-all
                           {faPreset === p ? 'border-brand-500 bg-brand-500/15 text-text-primary' : 'border-border-subtle bg-bg-elevated text-text-muted hover:text-text-primary'}"
                    onclick={() => selectFaPreset(p as 'authelia' | 'authentik' | 'custom')}
                  >
                    {p.charAt(0).toUpperCase() + p.slice(1)}
                  </button>
                {/each}
              </div>
            </div>

            <div>
              <label for="settings-proxies" class="block text-sm text-text-muted mb-1">{m.security_trustedProxies()}</label>
              <textarea
                id="settings-proxies"
                bind:value={methodTrustedProxies}
                class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                       focus:outline-none focus:ring-2 focus:ring-brand-500"
                placeholder="10.0.0.1/32&#10;172.16.0.0/12"
                rows="3"
              ></textarea>
              <p class="text-xs text-text-disabled mt-1">{m.security_proxyRangesHelp()}</p>
            </div>

            <div>
              <label for="settings-logout-url" class="block text-sm text-text-muted mb-1">{m.security_logoutUrl()}</label>
              <input
                id="settings-logout-url"
                type="url"
                bind:value={faLogoutUrl}
                class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                       focus:outline-none focus:ring-2 focus:ring-brand-500"
                placeholder={forwardAuthPresets[faPreset]?.logoutUrl || 'https://auth.example.com/logout'}
              />
              <p class="text-xs text-text-disabled mt-1">{m.security_logoutUrlHelp()}</p>
            </div>

            <button
              class="flex items-center gap-1.5 text-sm text-text-muted hover:text-text-secondary transition-colors"
              onclick={() => faShowAdvanced = !faShowAdvanced}
            >
              <svg class="w-4 h-4 transition-transform {faShowAdvanced ? 'rotate-90' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
              </svg>
              {m.security_advancedHeaders()}
            </button>

            {#if faShowAdvanced}
              <div class="grid grid-cols-2 gap-3 p-3 rounded-lg bg-bg-surface border border-border" in:fly={{ y: -10, duration: 150 }}>
                <div>
                  <label for="settings-header-user" class="block text-xs text-text-muted mb-1">{m.security_userHeader()}</label>
                  <input id="settings-header-user" type="text" bind:value={faHeaderUser}
                    class="w-full px-2 py-1.5 bg-bg-elevated border border-border-subtle rounded text-text-primary text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                </div>
                <div>
                  <label for="settings-header-email" class="block text-xs text-text-muted mb-1">{m.security_emailHeader()}</label>
                  <input id="settings-header-email" type="text" bind:value={faHeaderEmail}
                    class="w-full px-2 py-1.5 bg-bg-elevated border border-border-subtle rounded text-text-primary text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                </div>
                <div>
                  <label for="settings-header-groups" class="block text-xs text-text-muted mb-1">{m.security_groupsHeader()}</label>
                  <input id="settings-header-groups" type="text" bind:value={faHeaderGroups}
                    class="w-full px-2 py-1.5 bg-bg-elevated border border-border-subtle rounded text-text-primary text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                </div>
                <div>
                  <label for="settings-header-name" class="block text-xs text-text-muted mb-1">{m.security_nameHeader()}</label>
                  <input id="settings-header-name" type="text" bind:value={faHeaderName}
                    class="w-full px-2 py-1.5 bg-bg-elevated border border-border-subtle rounded text-text-primary text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                </div>
              </div>
            {/if}
          </div>
        {/if}
      </div>

      <!-- No authentication card -->
      <div
        class="rounded-xl border text-start transition-all overflow-hidden
               {selectedAuthMethod === 'none' ? 'border-amber-500 bg-amber-500/10' : 'border-border bg-bg-surface hover:border-border'}"
      >
        <button class="w-full p-4 flex items-start gap-4" onclick={() => { selectedAuthMethod = 'none'; }}>
          <div class="w-10 h-10 rounded-lg bg-amber-500/20 flex items-center justify-center flex-shrink-0 mt-0.5">
            <svg class="w-5 h-5 text-amber-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10" />
              <line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
            </svg>
          </div>
          <div class="flex-1 text-start">
            <div class="flex items-center gap-2">
              <h3 class="font-semibold text-text-primary">{m.security_noAuth()}</h3>
              {#if currentMethod === 'none'}
                <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-green-500/20 text-green-400 uppercase tracking-wider">{m.common_current()}</span>
              {/if}
            </div>
            <p class="text-sm text-text-muted mt-1">{m.security_noAuthDesc()}</p>
          </div>
        </button>
        {#if selectedAuthMethod === 'none'}
          <div class="px-4 pb-4 pt-0 ms-14" in:fly={{ y: -8, duration: 200 }}>
            <div class="border-t border-border pt-4">
              <div class="p-4 rounded-lg bg-amber-500/10 border border-amber-500/20">
                <div class="flex gap-3">
                  <svg class="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
                    <line x1="12" y1="9" x2="12" y2="13" />
                    <line x1="12" y1="17" x2="12.01" y2="17" />
                  </svg>
                  <div>
                    <h4 class="font-semibold text-amber-400 text-sm mb-1">{m.security_warningLabel()}</h4>
                    <p class="text-sm text-text-muted">{m.security_noAuthWarning()}</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        {/if}
      </div>
    </div>

    {#if methodError}
      <div class="p-3 mt-4 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm">
        {methodError}
      </div>
    {/if}

    {#if showUpdateBtn}
      <button
        class="btn btn-primary btn-sm mt-4 disabled:opacity-50 flex items-center gap-2"
        disabled={methodLoading || (selectedAuthMethod === 'forward_auth' && !methodTrustedProxies.trim())}
        onclick={handleChangeAuthMethod}
      >
        {#if methodLoading}
          <span class="inline-block w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></span>
        {/if}
        {m.security_updateMethod()}
      </button>
    {/if}
  </div>


  <!-- API Key (admin-only) -->
  {#if $isAdmin}
    <div>
      <div class="mb-4">
        <h3 class="text-lg font-semibold text-text-primary mb-1">API Key</h3>
        <p class="text-sm text-text-muted">
          A bearer token for non-browser integrations (scripts, cron jobs, webhook senders) that need to reach Muximux without a login session.
        </p>
      </div>

      <div class="mb-4 p-3 rounded-lg bg-bg-elevated/60 border border-border-subtle text-xs text-text-secondary space-y-2">
        <p class="font-semibold text-text-primary">What this key actually unlocks</p>
        <ul class="space-y-1 list-disc ms-5">
          <li><code>GET /api/appearance</code> for embedded or external apps reading Muximux's active language and theme.</li>
          <li>Any per-app proxy paths an admin has allowlisted with <code>auth_bypass</code> + <code>require_api_key: true</code> in <code>config.yaml</code>. Common case: webhook URLs reaching a proxied app's API.</li>
        </ul>
        <p class="text-text-muted">Everything else under <code>/api/*</code> still requires a session cookie. A leaked key cannot rotate users, change config, or write themes. See the <a href="https://github.com/mescon/Muximux/wiki/authentication#api-key-authentication" target="_blank" rel="noopener noreferrer" class="text-brand-400 hover:text-brand-300 underline">authentication wiki</a> for the full list and webhook example.</p>
      </div>

      {#if apiKeyError}
        <div class="mb-3 p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm">
          {apiKeyError}
        </div>
      {/if}

      {#if apiKeyPlaintext}
        <div class="mb-4 p-4 rounded-lg bg-amber-500/10 border border-amber-500/30">
          <div class="flex items-start gap-2 mb-3">
            <svg class="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01M5 19h14a2 2 0 001.84-2.75L13.74 4a2 2 0 00-3.5 0L3.16 16.25A2 2 0 005 19z" />
            </svg>
            <div class="flex-1">
              <p class="text-sm font-semibold text-amber-200 mb-1">This is the only time the key will be shown.</p>
              <p class="text-xs text-amber-300/80">Copy it somewhere safe now. Muximux stores only a hash and cannot show it again. If you lose it, generate a new one.</p>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <code class="flex-1 px-3 py-2 bg-bg-elevated border border-border-subtle rounded font-mono text-xs text-text-primary break-all select-all">{apiKeyPlaintext}</code>
            <button
              type="button"
              class="btn btn-secondary btn-sm flex items-center gap-1.5 flex-shrink-0"
              onclick={copyAPIKeyToClipboard}
            >
              {#if apiKeyCopied}
                <svg class="w-4 h-4 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                </svg>
                Copied
              {:else}
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3" />
                </svg>
                Copy
              {/if}
            </button>
          </div>
          <button
            type="button"
            class="mt-3 text-xs text-amber-300/80 hover:text-amber-200 underline"
            onclick={dismissAPIKeyPlaintext}
          >
            I've saved it, hide the key
          </button>
        </div>
      {:else if apiKeyConfigured === null}
        <div class="text-sm text-text-muted">Loading...</div>
      {:else if apiKeyConfigured}
        <div class="flex items-center gap-3 mb-3">
          <div class="flex items-center gap-2 text-sm text-text-secondary">
            <svg class="w-4 h-4 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
            </svg>
            An API key is configured.
          </div>
        </div>
        <div class="flex items-center gap-2">
          {#if confirmRotateApiKey}
            <span class="text-sm text-text-muted">Generating a new key invalidates the existing one.</span>
            <button
              type="button"
              class="btn btn-danger btn-sm disabled:opacity-50 flex items-center gap-2"
              onclick={handleGenerateAPIKey}
              disabled={apiKeyLoading}
            >
              {#if apiKeyLoading}
                <span class="inline-block w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></span>
              {/if}
              Rotate key
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              onclick={() => { confirmRotateApiKey = false; }}
            >Cancel</button>
          {:else if confirmDeleteApiKey}
            <span class="text-sm text-text-muted">Clients using this key will stop authenticating.</span>
            <button
              type="button"
              class="btn btn-danger btn-sm disabled:opacity-50 flex items-center gap-2"
              onclick={handleDeleteAPIKey}
              disabled={apiKeyLoading}
            >
              {#if apiKeyLoading}
                <span class="inline-block w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></span>
              {/if}
              Delete key
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              onclick={() => { confirmDeleteApiKey = false; }}
            >Cancel</button>
          {:else}
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              onclick={() => { confirmRotateApiKey = true; }}
            >Rotate</button>
            <button
              type="button"
              class="btn btn-ghost btn-sm text-red-400 hover:bg-red-500/10"
              onclick={() => { confirmDeleteApiKey = true; }}
            >Delete</button>
          {/if}
        </div>
      {:else}
        <div class="flex items-center gap-3">
          <span class="text-sm text-text-muted">No API key is configured.</span>
          <button
            type="button"
            class="btn btn-primary btn-sm disabled:opacity-50 flex items-center gap-2"
            onclick={handleGenerateAPIKey}
            disabled={apiKeyLoading}
          >
            {#if apiKeyLoading}
              <span class="inline-block w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></span>
            {/if}
            Generate API key
          </button>
        </div>
      {/if}
    </div>
  {/if}

  <!-- User Management (visible when builtin + admin) -->
  {#if currentMethod === 'builtin' && $isAdmin}
    <div>
      <div class="flex items-center justify-between mb-4">
        <div>
          <h3 class="text-lg font-semibold text-text-primary mb-1">{m.security_userManagement()}</h3>
          <p class="text-sm text-text-muted">{m.security_userManagementDesc()}</p>
        </div>
        <button
          class="btn btn-primary btn-sm flex items-center gap-1.5"
          onclick={() => showAddUser = !showAddUser}
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          {m.security_addUser()}
        </button>
      </div>

      {#if securityError}
        <div class="p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm mb-4">
          {securityError}
        </div>
      {/if}

      <!-- Add user form -->
      {#if showAddUser}
        <div class="p-4 rounded-lg bg-bg-surface border border-border mb-4 space-y-3" in:fly={{ y: -10, duration: 150 }}>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label for="new-user-name" class="block text-sm text-text-muted mb-1">{m.common_username()}</label>
              <input
                id="new-user-name"
                type="text"
                bind:value={newUserName}
                class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                       focus:outline-none focus:ring-2 focus:ring-brand-500"
                placeholder="username"
              />
            </div>
            <div>
              <label for="new-user-password" class="block text-sm text-text-muted mb-1">{m.common_password()}</label>
              <input
                id="new-user-password"
                type="password"
                bind:value={newUserPassword}
                class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                       focus:outline-none focus:ring-2 focus:ring-brand-500"
                placeholder={m.security_minEightCharsShort()}
              />
            </div>
          </div>
          <div>
            <label for="new-user-role" class="block text-sm text-text-muted mb-1">{m.common_role()}</label>
            <select
              id="new-user-role"
              bind:value={newUserRole}
              class="px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                     focus:outline-none focus:ring-2 focus:ring-brand-500"
            >
              <option value="admin">{m.common_roleAdmin()}</option>
              <option value="power-user">{m.common_rolePowerUser()}</option>
              <option value="user">{m.common_roleUser()}</option>
            </select>
          </div>

          {#if addUserError}
            <p class="text-red-400 text-sm">{addUserError}</p>
          {/if}

          <div class="flex gap-2">
            <button
              class="btn btn-primary btn-sm disabled:opacity-50 flex items-center gap-1.5"
              disabled={addUserLoading || !newUserName.trim() || newUserPassword.length < 8}
              onclick={handleAddUser}
            >
              {#if addUserLoading}
                <span class="inline-block w-3 h-3 border-2 border-white/30 border-t-white rounded-full animate-spin"></span>
              {/if}
              {m.common_add()}
            </button>
            <button
              class="px-3 py-1.5 text-sm text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover transition-colors"
              onclick={() => showAddUser = false}
            >
              {m.common_cancel()}
            </button>
          </div>
        </div>
      {/if}

      <!-- User list -->
      {#if securityLoading}
        <div class="text-center py-4 text-text-muted">{m.security_loadingUsers()}</div>
      {:else}
        <div class="space-y-2">
          {#each securityUsers as user (user.username)}
            <div class="p-3 rounded-lg bg-bg-surface border border-border">
              <div class="flex items-center gap-3">
                <div class="w-8 h-8 rounded-full bg-bg-elevated flex items-center justify-center text-sm font-medium text-text-secondary">
                  {user.username.charAt(0).toUpperCase()}
                </div>
                <div class="flex-1 min-w-0">
                  <div class="text-sm font-medium text-text-primary">{user.username}</div>
                  {#if user.email}
                    <div class="text-xs text-text-disabled">{user.email}</div>
                  {/if}
                </div>
                <select
                  value={user.role}
                  onchange={(e) => handleUpdateUserRole(user.username, e.currentTarget.value)}
                  class="px-2 py-1 text-xs bg-bg-elevated border border-border-subtle rounded text-text-primary
                         focus:outline-none focus:ring-1 focus:ring-brand-500"
                >
                  <option value="admin">{m.common_roleAdmin()}</option>
                  <option value="power-user">{m.common_rolePowerUser()}</option>
                  <option value="user">{m.common_roleUser()}</option>
                </select>
                {#if confirmDeleteUser === user.username}
                  <div class="flex items-center gap-1.5">
                    <button
                      class="btn btn-danger btn-sm"
                      onclick={() => handleDeleteUser(user.username)}
                    >{m.common_delete()}</button>
                    <button
                      class="btn btn-secondary btn-sm"
                      onclick={() => confirmDeleteUser = null}
                    >{m.common_cancel()}</button>
                  </div>
                {:else}
                  <button
                    class="p-1.5 text-text-disabled hover:text-red-400 rounded transition-colors"
                    onclick={() => confirmDeleteUser = user.username}
                    disabled={user.username === $currentUser?.username}
                    title={user.username === $currentUser?.username ? m.security_cantDeleteSelf() : m.security_deleteUser()}
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                  </button>
                {/if}
              </div>
              <div class="mt-2 flex items-center gap-2 ms-11">
                <label for="user-groups-{user.username}" class="text-xs text-text-muted whitespace-nowrap">
                  Groups
                </label>
                <input
                  id="user-groups-{user.username}"
                  type="text"
                  value={(user.groups ?? []).join(', ')}
                  onblur={(e) => {
                    const next = (user.groups ?? []).join(', ');
                    if (e.currentTarget.value !== next) {
                      handleUpdateUserGroups(user.username, e.currentTarget.value);
                    }
                  }}
                  placeholder="e.g. developers, on-call"
                  class="flex-1 px-2 py-1 text-xs bg-bg-elevated border border-border-subtle rounded text-text-primary
                         focus:outline-none focus:ring-1 focus:ring-brand-500"
                />
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>

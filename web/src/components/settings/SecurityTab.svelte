<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { fly } from 'svelte/transition';
  import type { Config, UserInfo, ChangeAuthMethodRequest } from '$lib/types';
  import { listUsers, createUser, updateUser, deleteUserAccount, changeAuthMethod } from '$lib/api';
  import { changePassword, isAdmin, currentUser } from '$lib/authStore';

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

  // Forward auth preset & header fields
  let faPreset = $state<'authelia' | 'authentik' | 'custom'>('authelia');
  let faShowAdvanced = $state(false);
  let faHeaderUser = $state('Remote-User');
  let faHeaderEmail = $state('Remote-Email');
  let faHeaderGroups = $state('Remote-Groups');
  let faHeaderName = $state('Remote-Name');
  let faLogoutUrl = $state('');

  const faPresets: Record<string, { user: string; email: string; groups: string; name: string; logoutUrl: string }> = {
    authelia: { user: 'Remote-User', email: 'Remote-Email', groups: 'Remote-Groups', name: 'Remote-Name', logoutUrl: 'https://auth.example.com/logout' },
    authentik: { user: 'X-authentik-username', email: 'X-authentik-email', groups: 'X-authentik-groups', name: 'X-authentik-name', logoutUrl: 'https://auth.example.com/outpost.goauthentik.io/sign_out' },
    custom: { user: 'Remote-User', email: 'Remote-Email', groups: 'Remote-Groups', name: 'Remote-Name', logoutUrl: '' },
  };

  function selectFaPreset(p: 'authelia' | 'authentik' | 'custom') {
    faPreset = p;
    const headers = faPresets[p];
    faHeaderUser = headers.user;
    faHeaderEmail = headers.email;
    faHeaderGroups = headers.groups;
    faHeaderName = headers.name;
    // Only set logout URL placeholder if the field is empty
    if (!faLogoutUrl) faLogoutUrl = headers.logoutUrl;
  }

  // Security tab functions
  async function loadSecurityUsers() {
    securityLoading = true;
    securityError = null;
    try {
      securityUsers = (await listUsers()) ?? [];
    } catch (e) {
      securityError = e instanceof Error ? e.message : 'Failed to load users';
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
      cpMessage = { type: 'success', text: 'Password changed successfully' };
      cpCurrent = '';
      cpNew = '';
      cpConfirm = '';
    } else {
      cpMessage = { type: 'error', text: result.message || 'Failed to change password' };
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
        addUserError = result.message || 'Failed to create user';
      }
    } catch (e) {
      addUserError = e instanceof Error ? e.message : 'Failed to create user';
    } finally {
      addUserLoading = false;
    }
  }

  async function handleUpdateUserRole(username: string, role: string) {
    try {
      await updateUser(username, { role });
      await loadSecurityUsers();
    } catch (e) {
      securityError = e instanceof Error ? e.message : 'Failed to update user';
    }
  }

  async function handleDeleteUser(username: string) {
    try {
      await deleteUserAccount(username);
      confirmDeleteUser = null;
      await loadSecurityUsers();
    } catch (e) {
      securityError = e instanceof Error ? e.message : 'Failed to delete user';
    }
  }

  async function handleChangeAuthMethod() {
    methodLoading = true;
    methodError = null;
    const previousMethod = localConfig.auth?.method || 'none';
    const req: ChangeAuthMethodRequest = { method: selectedAuthMethod };
    if (selectedAuthMethod === 'forward_auth') {
      req.trusted_proxies = methodTrustedProxies
        .split(/[,\n]/)
        .map(s => s.trim())
        .filter(s => s.length > 0);
      req.headers = {
        user: faHeaderUser,
        email: faHeaderEmail,
        groups: faHeaderGroups,
        name: faHeaderName,
      };
      req.logout_url = faLogoutUrl;
    }
    try {
      const result = await changeAuthMethod(req);
      if (result.success) {
        // If switching FROM "none" to an auth method, the current session is now invalid
        // (the virtual admin had no real session cookie). Force a page reload so the user
        // can authenticate properly.
        if (previousMethod === 'none' && selectedAuthMethod !== 'none') {
          sessionStorage.setItem('muximux_return_to', 'security');
          window.location.reload();
          return;
        }
        securitySuccess = `Authentication method changed to ${selectedAuthMethod}`;
        setTimeout(() => securitySuccess = null, 3000);
      } else {
        methodError = result.message || 'Failed to change method';
      }
    } catch (e) {
      methodError = e instanceof Error ? e.message : 'Failed to change method';
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
      // Detect preset from header values
      const matchesAuthelia = faHeaderUser === faPresets.authelia.user && faHeaderEmail === faPresets.authelia.email;
      const matchesAuthentik = faHeaderUser === faPresets.authentik.user && faHeaderEmail === faPresets.authentik.email;
      faPreset = matchesAuthentik ? 'authentik' : matchesAuthelia ? 'authelia' : 'custom';
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
    <h3 class="text-lg font-semibold text-text-primary mb-1">Authentication Method</h3>
    <p class="text-sm text-text-muted mb-4">Choose how users authenticate with Muximux</p>

    <div class="space-y-3">
      <!-- Password card -->
      <div
        class="rounded-xl border text-left transition-all overflow-hidden
               {selectedAuthMethod === 'builtin' ? 'border-brand-500 bg-brand-500/10' : 'border-border bg-bg-surface hover:border-border'}"
      >
        <button class="w-full p-4 flex items-start gap-4" onclick={() => { selectedAuthMethod = 'builtin'; }}>
          <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center flex-shrink-0 mt-0.5">
            <svg class="w-5 h-5 text-brand-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="11" width="18" height="11" rx="2" />
              <path d="M7 11V7a5 5 0 0110 0v4" />
            </svg>
          </div>
          <div class="flex-1 text-left">
            <div class="flex items-center gap-2">
              <h3 class="font-semibold text-text-primary">Password authentication</h3>
              {#if currentMethod === 'builtin'}
                <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-green-500/20 text-green-400 uppercase tracking-wider">Current</span>
              {/if}
            </div>
            <p class="text-sm text-text-muted mt-1">Set up a username and password to protect your dashboard</p>
          </div>
        </button>
        {#if selectedAuthMethod === 'builtin'}
          <div class="px-4 pb-4 pt-0 ml-14" in:fly={{ y: -8, duration: 200 }}>
            <div class="border-t border-border pt-4">
              {#if currentMethod === 'builtin'}
                <p class="text-sm text-text-muted mb-4">Password authentication is active.</p>

                <!-- Change Password (inline) -->
                <h4 class="text-sm font-semibold text-text-primary mb-2">Change Password</h4>
                <div class="max-w-sm space-y-3">
                  <div>
                    <label for="cp-current" class="block text-xs text-text-muted mb-1">Current password</label>
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
                    <label for="cp-new" class="block text-xs text-text-muted mb-1">New password</label>
                    <input
                      id="cp-new"
                      type="password"
                      bind:value={cpNew}
                      class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                             focus:outline-none focus:ring-2 focus:ring-brand-500"
                      placeholder="Minimum 8 characters"
                      autocomplete="new-password"
                    />
                    {#if cpNew.length > 0 && cpNew.length < 8}
                      <p class="text-red-400 text-xs mt-1">Password must be at least 8 characters</p>
                    {/if}
                  </div>
                  <div>
                    <label for="cp-confirm" class="block text-xs text-text-muted mb-1">Confirm new password</label>
                    <input
                      id="cp-confirm"
                      type="password"
                      bind:value={cpConfirm}
                      class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                             focus:outline-none focus:ring-2 focus:ring-brand-500"
                      autocomplete="new-password"
                    />
                    {#if cpConfirm.length > 0 && cpNew !== cpConfirm}
                      <p class="text-red-400 text-xs mt-1">Passwords do not match</p>
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
                    Change Password
                  </button>
                </div>
              {:else if securityUsers.length > 0}
                <p class="text-sm text-text-muted">Switch to password authentication using existing users.</p>
              {:else}
                <p class="text-sm text-text-muted mb-3">Create your first user to enable password authentication.</p>
                <div class="space-y-3 max-w-sm">
                  <div>
                    <label for="setup-username" class="block text-xs text-text-muted mb-1">Username</label>
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
                    <label for="setup-password" class="block text-xs text-text-muted mb-1">Password <span class="text-text-disabled">(min 8 characters)</span></label>
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
                    <p class="text-amber-400 text-xs">Username is required</p>
                  {:else if setupUsername.trim() && setupPassword.length > 0 && setupPassword.length < 8}
                    <p class="text-amber-400 text-xs">Password must be at least 8 characters ({setupPassword.length}/8)</p>
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
                            methodError = result.message || 'Failed to enable auth';
                            return;
                          }
                        } catch (e) {
                          methodError = e instanceof Error ? e.message : 'Failed to enable auth';
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
                    Create User & Enable
                  </button>
                </div>
              {/if}
            </div>
          </div>
        {/if}
      </div>

      <!-- Auth Proxy card -->
      <div
        class="rounded-xl border text-left transition-all overflow-hidden
               {selectedAuthMethod === 'forward_auth' ? 'border-brand-500 bg-brand-500/10' : 'border-border bg-bg-surface hover:border-border'}"
      >
        <button class="w-full p-4 flex items-start gap-4" onclick={async () => { selectedAuthMethod = 'forward_auth'; await tick(); document.getElementById('settings-proxies')?.focus(); }}>
          <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center flex-shrink-0 mt-0.5">
            <svg class="w-5 h-5 text-brand-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
            </svg>
          </div>
          <div class="flex-1 text-left">
            <div class="flex items-center gap-2">
              <h3 class="font-semibold text-text-primary">Auth proxy</h3>
              {#if currentMethod === 'forward_auth'}
                <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-green-500/20 text-green-400 uppercase tracking-wider">Current</span>
              {/if}
            </div>
            <p class="text-sm text-text-muted mt-1">Authelia, Authentik, or another reverse proxy handles authentication</p>
          </div>
        </button>
        {#if selectedAuthMethod === 'forward_auth'}
          <div class="px-4 pb-4 pt-0 space-y-4 ml-14" in:fly={{ y: -8, duration: 200 }}>
            <div class="border-t border-border pt-4">
              <span class="block text-sm text-text-muted mb-2">Proxy type</span>
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
              <label for="settings-proxies" class="block text-sm text-text-muted mb-1">Trusted proxy IPs</label>
              <textarea
                id="settings-proxies"
                bind:value={methodTrustedProxies}
                class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                       focus:outline-none focus:ring-2 focus:ring-brand-500"
                placeholder="10.0.0.1/32&#10;172.16.0.0/12"
                rows="3"
              ></textarea>
              <p class="text-xs text-text-disabled mt-1">IP addresses or CIDR ranges, one per line</p>
            </div>

            <div>
              <label for="settings-logout-url" class="block text-sm text-text-muted mb-1">Logout URL</label>
              <input
                id="settings-logout-url"
                type="url"
                bind:value={faLogoutUrl}
                class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                       focus:outline-none focus:ring-2 focus:ring-brand-500"
                placeholder={faPresets[faPreset]?.logoutUrl || 'https://auth.example.com/logout'}
              />
              <p class="text-xs text-text-disabled mt-1">Your auth provider's logout endpoint — clears the external session on sign-out</p>
            </div>

            <button
              class="flex items-center gap-1.5 text-sm text-text-muted hover:text-text-secondary transition-colors"
              onclick={() => faShowAdvanced = !faShowAdvanced}
            >
              <svg class="w-4 h-4 transition-transform {faShowAdvanced ? 'rotate-90' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
              </svg>
              Advanced: Header names
            </button>

            {#if faShowAdvanced}
              <div class="grid grid-cols-2 gap-3 p-3 rounded-lg bg-bg-surface border border-border" in:fly={{ y: -10, duration: 150 }}>
                <div>
                  <label for="settings-header-user" class="block text-xs text-text-muted mb-1">User header</label>
                  <input id="settings-header-user" type="text" bind:value={faHeaderUser}
                    class="w-full px-2 py-1.5 bg-bg-elevated border border-border-subtle rounded text-text-primary text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                </div>
                <div>
                  <label for="settings-header-email" class="block text-xs text-text-muted mb-1">Email header</label>
                  <input id="settings-header-email" type="text" bind:value={faHeaderEmail}
                    class="w-full px-2 py-1.5 bg-bg-elevated border border-border-subtle rounded text-text-primary text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                </div>
                <div>
                  <label for="settings-header-groups" class="block text-xs text-text-muted mb-1">Groups header</label>
                  <input id="settings-header-groups" type="text" bind:value={faHeaderGroups}
                    class="w-full px-2 py-1.5 bg-bg-elevated border border-border-subtle rounded text-text-primary text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                </div>
                <div>
                  <label for="settings-header-name" class="block text-xs text-text-muted mb-1">Name header</label>
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
        class="rounded-xl border text-left transition-all overflow-hidden
               {selectedAuthMethod === 'none' ? 'border-amber-500 bg-amber-500/10' : 'border-border bg-bg-surface hover:border-border'}"
      >
        <button class="w-full p-4 flex items-start gap-4" onclick={() => { selectedAuthMethod = 'none'; }}>
          <div class="w-10 h-10 rounded-lg bg-amber-500/20 flex items-center justify-center flex-shrink-0 mt-0.5">
            <svg class="w-5 h-5 text-amber-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10" />
              <line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
            </svg>
          </div>
          <div class="flex-1 text-left">
            <div class="flex items-center gap-2">
              <h3 class="font-semibold text-text-primary">No authentication</h3>
              {#if currentMethod === 'none'}
                <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-green-500/20 text-green-400 uppercase tracking-wider">Current</span>
              {/if}
            </div>
            <p class="text-sm text-text-muted mt-1">Anyone with network access gets full control</p>
          </div>
        </button>
        {#if selectedAuthMethod === 'none'}
          <div class="px-4 pb-4 pt-0 ml-14" in:fly={{ y: -8, duration: 200 }}>
            <div class="border-t border-border pt-4">
              <div class="p-4 rounded-lg bg-amber-500/10 border border-amber-500/20">
                <div class="flex gap-3">
                  <svg class="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
                    <line x1="12" y1="9" x2="12" y2="13" />
                    <line x1="12" y1="17" x2="12.01" y2="17" />
                  </svg>
                  <div>
                    <h4 class="font-semibold text-amber-400 text-sm mb-1">Security warning</h4>
                    <p class="text-sm text-text-muted">Without authentication, anyone who can reach this port has full access to your dashboard and all configured services.</p>
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
        Update Method
      </button>
    {/if}
  </div>


  <!-- User Management (visible when builtin + admin) -->
  {#if currentMethod === 'builtin' && $isAdmin}
    <div>
      <div class="flex items-center justify-between mb-4">
        <div>
          <h3 class="text-lg font-semibold text-text-primary mb-1">User Management</h3>
          <p class="text-sm text-text-muted">Manage dashboard users and roles</p>
        </div>
        <button
          class="btn btn-primary btn-sm flex items-center gap-1.5"
          onclick={() => showAddUser = !showAddUser}
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          Add User
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
              <label for="new-user-name" class="block text-sm text-text-muted mb-1">Username</label>
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
              <label for="new-user-password" class="block text-sm text-text-muted mb-1">Password</label>
              <input
                id="new-user-password"
                type="password"
                bind:value={newUserPassword}
                class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                       focus:outline-none focus:ring-2 focus:ring-brand-500"
                placeholder="Min 8 characters"
              />
            </div>
          </div>
          <div>
            <label for="new-user-role" class="block text-sm text-text-muted mb-1">Role</label>
            <select
              id="new-user-role"
              bind:value={newUserRole}
              class="px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm
                     focus:outline-none focus:ring-2 focus:ring-brand-500"
            >
              <option value="admin">Admin</option>
              <option value="power-user">Power User</option>
              <option value="user">User</option>
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
              Add
            </button>
            <button
              class="px-3 py-1.5 text-sm text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover transition-colors"
              onclick={() => showAddUser = false}
            >
              Cancel
            </button>
          </div>
        </div>
      {/if}

      <!-- User list -->
      {#if securityLoading}
        <div class="text-center py-4 text-text-muted">Loading users...</div>
      {:else}
        <div class="space-y-2">
          {#each securityUsers as user (user.username)}
            <div class="flex items-center gap-3 p-3 rounded-lg bg-bg-surface border border-border">
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
                <option value="admin">Admin</option>
                <option value="power-user">Power User</option>
                <option value="user">User</option>
              </select>
              {#if confirmDeleteUser === user.username}
                <div class="flex items-center gap-1.5">
                  <button
                    class="btn btn-danger btn-sm"
                    onclick={() => handleDeleteUser(user.username)}
                  >Delete</button>
                  <button
                    class="btn btn-secondary btn-sm"
                    onclick={() => confirmDeleteUser = null}
                  >Cancel</button>
                </div>
              {:else}
                <button
                  class="p-1.5 text-text-disabled hover:text-red-400 rounded transition-colors"
                  onclick={() => confirmDeleteUser = user.username}
                  disabled={user.username === $currentUser?.username}
                  title={user.username === $currentUser?.username ? "Can't delete yourself" : 'Delete user'}
                >
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </button>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>

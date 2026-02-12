<script lang="ts">
  import { fly, fade } from 'svelte/transition';
  import type { SetupRequest } from '$lib/types';
  import { submitSetup } from '$lib/api';

  let {
    oncomplete
  }: {
    oncomplete?: () => void;
  } = $props();

  // Wizard state
  let step = $state(1);
  let method = $state<'builtin' | 'forward_auth' | 'none' | null>(null);
  let loading = $state(false);
  let errorMsg = $state<string | null>(null);

  // Builtin fields
  let username = $state('admin');
  let password = $state('');
  let confirmPassword = $state('');

  // Forward auth fields
  let preset = $state<'authelia' | 'authentik' | 'custom'>('authelia');
  let trustedProxies = $state('');
  let showAdvanced = $state(false);
  let headerUser = $state('Remote-User');
  let headerEmail = $state('Remote-Email');
  let headerGroups = $state('Remote-Groups');
  let headerName = $state('Remote-Name');

  // None fields
  let acknowledgeRisk = $state(false);

  // Completion
  let completionMessage = $state('');

  // Preset header configs
  const presets = {
    authelia: { user: 'Remote-User', email: 'Remote-Email', groups: 'Remote-Groups', name: 'Remote-Name' },
    authentik: { user: 'X-authentik-username', email: 'X-authentik-email', groups: 'X-authentik-groups', name: 'X-authentik-name' },
    custom: { user: 'Remote-User', email: 'Remote-Email', groups: 'Remote-Groups', name: 'Remote-Name' },
  };

  function selectPreset(p: 'authelia' | 'authentik' | 'custom') {
    preset = p;
    const headers = presets[p];
    headerUser = headers.user;
    headerEmail = headers.email;
    headerGroups = headers.groups;
    headerName = headers.name;
  }

  // Validation
  let builtinValid = $derived(
    username.trim().length > 0 &&
    password.length >= 8 &&
    password === confirmPassword
  );

  let forwardAuthValid = $derived(
    trustedProxies.trim().length > 0
  );

  let noneValid = $derived(acknowledgeRisk);

  let canProceed = $derived(
    method === 'builtin' ? builtinValid :
    method === 'forward_auth' ? forwardAuthValid :
    method === 'none' ? noneValid :
    false
  );

  function selectMethod(m: 'builtin' | 'forward_auth' | 'none') {
    method = m;
    step = 2;
    errorMsg = null;
  }

  async function handleSubmit() {
    if (!method || !canProceed) return;
    loading = true;
    errorMsg = null;

    const req: SetupRequest = { method };

    if (method === 'builtin') {
      req.username = username.trim();
      req.password = password;
    } else if (method === 'forward_auth') {
      req.trusted_proxies = trustedProxies
        .split(/[,\n]/)
        .map(s => s.trim())
        .filter(s => s.length > 0);
      req.headers = {
        user: headerUser,
        email: headerEmail,
        groups: headerGroups,
        name: headerName,
      };
    }

    try {
      const resp = await submitSetup(req);
      if (resp.success) {
        step = 3;
        if (method === 'builtin') {
          completionMessage = 'Account created. You\'re logged in.';
        } else if (method === 'forward_auth') {
          completionMessage = 'Configuration saved. Access Muximux through your reverse proxy to continue.';
        } else {
          completionMessage = 'Authentication disabled.';
        }
      } else {
        errorMsg = resp.error || 'Setup failed';
      }
    } catch (e) {
      errorMsg = e instanceof Error ? e.message : 'Setup failed';
    } finally {
      loading = false;
    }
  }

  function goBack() {
    step = 1;
    method = null;
    errorMsg = null;
  }
</script>

<div class="setup-wizard" transition:fade={{ duration: 200 }}>
  <div class="setup-container">
    <!-- Header -->
    <div class="setup-header">
      <div class="logo-area">
        <svg class="logo-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="3" y="3" width="7" height="7" rx="1" />
          <rect x="14" y="3" width="7" height="7" rx="1" />
          <rect x="3" y="14" width="7" height="7" rx="1" />
          <rect x="14" y="14" width="7" height="7" rx="1" />
        </svg>
        <h1>Muximux Setup</h1>
      </div>
      <p class="subtitle">Configure how you want to secure your dashboard</p>

      <!-- Step indicator -->
      <div class="steps">
        {#each [1, 2, 3] as s (s)}
          <div class="step-dot" class:active={step >= s} class:current={step === s}></div>
        {/each}
      </div>
    </div>

    <!-- Step 1: Choose method -->
    {#if step === 1}
      <div class="step-content" in:fly={{ x: 20, duration: 200 }}>
        <div class="method-cards">
          <button class="method-card" onclick={() => selectMethod('builtin')}>
            <div class="method-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="3" y="11" width="18" height="11" rx="2" />
                <path d="M7 11V7a5 5 0 0110 0v4" />
              </svg>
            </div>
            <div>
              <h3>Create a password</h3>
              <p>Set up a username and password to protect your dashboard</p>
            </div>
            <span class="badge recommended">Recommended</span>
          </button>

          <button class="method-card" onclick={() => selectMethod('forward_auth')}>
            <div class="method-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
              </svg>
            </div>
            <div>
              <h3>I use an auth proxy</h3>
              <p>Authelia, Authentik, or another reverse proxy handles authentication</p>
            </div>
          </button>

          <button class="method-card method-card-warn" onclick={() => selectMethod('none')}>
            <div class="method-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" />
                <line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
              </svg>
            </div>
            <div>
              <h3>No authentication</h3>
              <p>Anyone with network access gets full control</p>
            </div>
          </button>
        </div>
      </div>
    {/if}

    <!-- Step 2: Configure -->
    {#if step === 2}
      <div class="step-content" in:fly={{ x: 20, duration: 200 }}>
        {#if method === 'builtin'}
          <div class="config-section">
            <h2>Create admin account</h2>
            <div class="form-group">
              <label for="username">Username</label>
              <input
                id="username"
                type="text"
                bind:value={username}
                placeholder="admin"
                autocomplete="username"
              />
            </div>
            <div class="form-group">
              <label for="password">Password</label>
              <input
                id="password"
                type="password"
                bind:value={password}
                placeholder="Minimum 8 characters"
                autocomplete="new-password"
              />
              {#if password.length > 0 && password.length < 8}
                <span class="field-error">Password must be at least 8 characters</span>
              {/if}
            </div>
            <div class="form-group">
              <label for="confirm-password">Confirm password</label>
              <input
                id="confirm-password"
                type="password"
                bind:value={confirmPassword}
                placeholder="Re-enter password"
                autocomplete="new-password"
              />
              {#if confirmPassword.length > 0 && password !== confirmPassword}
                <span class="field-error">Passwords do not match</span>
              {/if}
            </div>
          </div>
        {:else if method === 'forward_auth'}
          <div class="config-section">
            <h2>Configure auth proxy</h2>

            <div class="preset-selector" role="group" aria-label="Proxy type">
              <span class="preset-label">Proxy type</span>
              <div class="preset-buttons">
                <button
                  class="preset-btn"
                  class:active={preset === 'authelia'}
                  onclick={() => selectPreset('authelia')}
                >Authelia</button>
                <button
                  class="preset-btn"
                  class:active={preset === 'authentik'}
                  onclick={() => selectPreset('authentik')}
                >Authentik</button>
                <button
                  class="preset-btn"
                  class:active={preset === 'custom'}
                  onclick={() => selectPreset('custom')}
                >Custom</button>
              </div>
            </div>

            <div class="form-group">
              <label for="trusted-proxies">Trusted proxy IPs</label>
              <textarea
                id="trusted-proxies"
                bind:value={trustedProxies}
                placeholder="10.0.0.1/32&#10;172.16.0.0/12"
                rows="3"
              ></textarea>
              <span class="field-hint">IP addresses or CIDR ranges, one per line or comma-separated</span>
            </div>

            <button class="advanced-toggle" onclick={() => showAdvanced = !showAdvanced}>
              <svg class="toggle-chevron" class:open={showAdvanced} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="6 9 12 15 18 9" />
              </svg>
              Advanced: Header names
            </button>

            {#if showAdvanced}
              <div class="advanced-section" transition:fly={{ y: -10, duration: 150 }}>
                <div class="form-row">
                  <div class="form-group">
                    <label for="header-user">User header</label>
                    <input id="header-user" type="text" bind:value={headerUser} />
                  </div>
                  <div class="form-group">
                    <label for="header-email">Email header</label>
                    <input id="header-email" type="text" bind:value={headerEmail} />
                  </div>
                </div>
                <div class="form-row">
                  <div class="form-group">
                    <label for="header-groups">Groups header</label>
                    <input id="header-groups" type="text" bind:value={headerGroups} />
                  </div>
                  <div class="form-group">
                    <label for="header-name">Name header</label>
                    <input id="header-name" type="text" bind:value={headerName} />
                  </div>
                </div>
              </div>
            {/if}
          </div>
        {:else if method === 'none'}
          <div class="config-section">
            <div class="warning-box">
              <svg class="warning-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
                <line x1="12" y1="9" x2="12" y2="13" />
                <line x1="12" y1="17" x2="12.01" y2="17" />
              </svg>
              <div>
                <h3>Security warning</h3>
                <p>Without authentication, anyone who can reach this port has full access to your dashboard, including the ability to add, modify, and delete apps, change settings, and access all configured services.</p>
                <p>Only use this option if Muximux is on a fully trusted network or behind a separate authentication layer.</p>
              </div>
            </div>
            <label class="checkbox-label">
              <input type="checkbox" bind:checked={acknowledgeRisk} />
              <span>I understand the risks and want to proceed without authentication</span>
            </label>
          </div>
        {/if}

        {#if errorMsg}
          <div class="error-banner">{errorMsg}</div>
        {/if}

        <div class="button-row">
          <button class="btn btn-secondary" onclick={goBack}>Back</button>
          <button
            class="btn btn-primary"
            onclick={handleSubmit}
            disabled={!canProceed || loading}
          >
            {#if loading}
              <span class="spinner"></span>
            {:else}
              Complete setup
            {/if}
          </button>
        </div>
      </div>
    {/if}

    <!-- Step 3: Complete -->
    {#if step === 3}
      <div class="step-content" in:fly={{ x: 20, duration: 200 }}>
        <div class="completion">
          <div class="success-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M22 11.08V12a10 10 0 11-5.93-9.14" />
              <polyline points="22 4 12 14.01 9 11.01" />
            </svg>
          </div>
          <h2>Setup complete</h2>
          <p class="completion-msg">{completionMessage}</p>
          <button class="btn btn-primary" onclick={() => oncomplete?.()}>
            Continue
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .setup-wizard {
    position: fixed;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-base, #0f1117);
    z-index: 100;
    padding: 1rem;
  }

  .setup-container {
    width: 100%;
    max-width: 640px;
    background: var(--bg-surface, #1a1d27);
    border: 1px solid var(--border-default, #2a2d3a);
    border-radius: 12px;
    padding: 2rem;
    overflow-y: auto;
    max-height: 90vh;
  }

  .setup-header {
    text-align: center;
    margin-bottom: 2rem;
  }

  .logo-area {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.75rem;
    margin-bottom: 0.5rem;
  }

  .logo-icon {
    width: 32px;
    height: 32px;
    color: var(--accent-primary, #6366f1);
  }

  .setup-header h1 {
    font-size: 1.5rem;
    font-weight: 600;
    color: var(--text-primary, #e2e4e9);
    margin: 0;
  }

  .subtitle {
    color: var(--text-muted, #6b7280);
    font-size: 0.875rem;
    margin: 0.5rem 0 1.25rem;
  }

  .steps {
    display: flex;
    gap: 0.5rem;
    justify-content: center;
  }

  .step-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--border-default, #2a2d3a);
    transition: all 0.2s;
  }

  .step-dot.active {
    background: var(--accent-primary, #6366f1);
  }

  .step-dot.current {
    width: 24px;
    border-radius: 4px;
  }

  .step-content {
    min-height: 200px;
  }

  /* Method cards */
  .method-cards {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .method-card {
    display: flex;
    align-items: flex-start;
    gap: 1rem;
    padding: 1.25rem;
    background: var(--bg-elevated, #1e2130);
    border: 1px solid var(--border-default, #2a2d3a);
    border-radius: 8px;
    text-align: left;
    cursor: pointer;
    transition: all 0.15s;
    color: var(--text-primary, #e2e4e9);
    position: relative;
  }

  .method-card:hover {
    border-color: var(--accent-primary, #6366f1);
    background: var(--bg-hover, #252838);
  }

  .method-card-warn:hover {
    border-color: var(--warning, #f59e0b);
  }

  .method-icon {
    flex-shrink: 0;
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-surface, #1a1d27);
    border-radius: 8px;
    padding: 8px;
  }

  .method-icon svg {
    width: 24px;
    height: 24px;
    color: var(--accent-primary, #6366f1);
  }

  .method-card-warn .method-icon svg {
    color: var(--warning, #f59e0b);
  }

  .method-card h3 {
    font-size: 0.9375rem;
    font-weight: 600;
    margin: 0 0 0.25rem;
  }

  .method-card p {
    font-size: 0.8125rem;
    color: var(--text-muted, #6b7280);
    margin: 0;
    line-height: 1.4;
  }

  .badge {
    position: absolute;
    top: 0.75rem;
    right: 0.75rem;
    font-size: 0.6875rem;
    font-weight: 600;
    padding: 0.125rem 0.5rem;
    border-radius: 9999px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .badge.recommended {
    background: var(--accent-primary, #6366f1);
    color: white;
  }

  /* Config section */
  .config-section {
    margin-bottom: 1.5rem;
  }

  .config-section h2 {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--text-primary, #e2e4e9);
    margin: 0 0 1.25rem;
  }

  .form-group {
    margin-bottom: 1rem;
  }

  .form-group label {
    display: block;
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--text-secondary, #9ca3af);
    margin-bottom: 0.375rem;
  }

  .form-group input[type="text"],
  .form-group input[type="password"],
  .form-group textarea {
    width: 100%;
    padding: 0.625rem 0.75rem;
    background: var(--bg-elevated, #1e2130);
    border: 1px solid var(--border-default, #2a2d3a);
    border-radius: 6px;
    color: var(--text-primary, #e2e4e9);
    font-size: 0.875rem;
    font-family: inherit;
    outline: none;
    transition: border-color 0.15s;
    box-sizing: border-box;
  }

  .form-group input:focus,
  .form-group textarea:focus {
    border-color: var(--accent-primary, #6366f1);
  }

  .form-group textarea {
    resize: vertical;
    min-height: 60px;
  }

  .field-error {
    display: block;
    font-size: 0.75rem;
    color: var(--error, #ef4444);
    margin-top: 0.25rem;
  }

  .field-hint {
    display: block;
    font-size: 0.75rem;
    color: var(--text-muted, #6b7280);
    margin-top: 0.25rem;
  }

  .form-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.75rem;
  }

  /* Preset selector */
  .preset-selector {
    margin-bottom: 1rem;
  }

  .preset-label {
    display: block;
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--text-secondary, #9ca3af);
    margin-bottom: 0.375rem;
  }

  .preset-buttons {
    display: flex;
    gap: 0.5rem;
  }

  .preset-btn {
    flex: 1;
    padding: 0.5rem 0.75rem;
    background: var(--bg-elevated, #1e2130);
    border: 1px solid var(--border-default, #2a2d3a);
    border-radius: 6px;
    color: var(--text-secondary, #9ca3af);
    font-size: 0.8125rem;
    cursor: pointer;
    transition: all 0.15s;
  }

  .preset-btn:hover {
    border-color: var(--text-muted, #6b7280);
  }

  .preset-btn.active {
    border-color: var(--accent-primary, #6366f1);
    color: var(--text-primary, #e2e4e9);
    background: var(--bg-hover, #252838);
  }

  /* Advanced toggle */
  .advanced-toggle {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    background: none;
    border: none;
    color: var(--text-muted, #6b7280);
    font-size: 0.8125rem;
    cursor: pointer;
    padding: 0.25rem 0;
    margin-bottom: 0.75rem;
  }

  .advanced-toggle:hover {
    color: var(--text-secondary, #9ca3af);
  }

  .toggle-chevron {
    width: 16px;
    height: 16px;
    transition: transform 0.15s;
  }

  .toggle-chevron.open {
    transform: rotate(180deg);
  }

  .advanced-section {
    padding: 0.75rem;
    background: var(--bg-elevated, #1e2130);
    border-radius: 6px;
    margin-bottom: 1rem;
  }

  /* Warning box */
  .warning-box {
    display: flex;
    gap: 1rem;
    padding: 1rem;
    background: rgba(245, 158, 11, 0.08);
    border: 1px solid rgba(245, 158, 11, 0.2);
    border-radius: 8px;
    margin-bottom: 1.5rem;
  }

  .warning-icon {
    flex-shrink: 0;
    width: 24px;
    height: 24px;
    color: var(--warning, #f59e0b);
    margin-top: 2px;
  }

  .warning-box h3 {
    font-size: 0.9375rem;
    font-weight: 600;
    color: var(--warning, #f59e0b);
    margin: 0 0 0.5rem;
  }

  .warning-box p {
    font-size: 0.8125rem;
    color: var(--text-secondary, #9ca3af);
    margin: 0 0 0.5rem;
    line-height: 1.5;
  }

  .warning-box p:last-child {
    margin-bottom: 0;
  }

  .checkbox-label {
    display: flex;
    align-items: flex-start;
    gap: 0.75rem;
    cursor: pointer;
    font-size: 0.875rem;
    color: var(--text-secondary, #9ca3af);
    line-height: 1.4;
  }

  .checkbox-label input[type="checkbox"] {
    margin-top: 2px;
    accent-color: var(--accent-primary, #6366f1);
  }

  /* Error banner */
  .error-banner {
    padding: 0.75rem 1rem;
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.2);
    border-radius: 6px;
    color: var(--error, #ef4444);
    font-size: 0.8125rem;
    margin-bottom: 1rem;
  }

  /* Button row */
  .button-row {
    display: flex;
    justify-content: space-between;
    gap: 0.75rem;
    margin-top: 1.5rem;
  }

  .btn {
    padding: 0.625rem 1.25rem;
    border-radius: 6px;
    font-size: 0.875rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s;
    border: none;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .btn-primary {
    background: var(--accent-primary, #6366f1);
    color: white;
  }

  .btn-primary:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .btn-secondary {
    background: var(--bg-elevated, #1e2130);
    color: var(--text-secondary, #9ca3af);
    border: 1px solid var(--border-default, #2a2d3a);
  }

  .btn-secondary:hover:not(:disabled) {
    background: var(--bg-hover, #252838);
    color: var(--text-primary, #e2e4e9);
  }

  /* Completion */
  .completion {
    text-align: center;
    padding: 2rem 0;
  }

  .success-icon {
    width: 64px;
    height: 64px;
    margin: 0 auto 1.5rem;
    color: var(--success, #22c55e);
  }

  .success-icon svg {
    width: 100%;
    height: 100%;
  }

  .completion h2 {
    font-size: 1.25rem;
    font-weight: 600;
    color: var(--text-primary, #e2e4e9);
    margin: 0 0 0.75rem;
  }

  .completion-msg {
    color: var(--text-muted, #6b7280);
    font-size: 0.875rem;
    margin: 0 0 2rem;
    line-height: 1.5;
  }

  /* Spinner */
  .spinner {
    display: inline-block;
    width: 16px;
    height: 16px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-top-color: white;
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* Responsive */
  @media (max-width: 480px) {
    .setup-container {
      padding: 1.5rem;
    }

    .form-row {
      grid-template-columns: 1fr;
    }

    .preset-buttons {
      flex-direction: column;
    }
  }
</style>

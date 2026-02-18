<script lang="ts">
  import { onMount } from 'svelte';
  import { login } from '$lib/authStore';
  import { getBase } from '$lib/api';

  let { onsuccess }: { onsuccess?: () => void } = $props();

  let username = $state('');
  let password = $state('');
  let rememberMe = $state(false);
  let error = $state<string | null>(null);
  let loading = $state(false);
  let oidcEnabled = $state(false);

  onMount(async () => {
    // Check if OIDC is enabled
    try {
      const res = await fetch(`${getBase()}/api/auth/status`);
      if (res.ok) {
        const data = await res.json();
        oidcEnabled = data.oidc_enabled || false;
      }
    } catch {
      // Ignore errors
    }
  });

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    if (!username || !password) {
      error = 'Username and password are required';
      return;
    }

    loading = true;
    error = null;

    const result = await login(username, password, rememberMe);

    loading = false;

    if (result.success) {
      onsuccess?.();
    } else {
      error = result.message || 'Login failed';
    }
  }

  function handleOIDCLogin() {
    // Get current URL as redirect target
    const redirect = encodeURIComponent(window.location.pathname);
    window.location.href = `${getBase()}/api/auth/oidc/login?redirect=${redirect}`;
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') {
      handleSubmit(new SubmitEvent('submit'));
    }
  }
</script>

<div class="login-page min-h-screen flex items-center justify-center p-4">
  <div class="w-full max-w-md">
    <!-- Logo -->
    <div class="text-center mb-8">
      <svg class="login-logo w-40 h-auto mx-auto mb-3" viewBox="0 0 341 207" fill="currentColor">
        <path d="M 64.45 48.00 C 68.63 48.00 72.82 47.99 77.01 48.01 C 80.83 59.09 84.77 70.14 88.54 81.24 C 92.32 70.17 96.13 59.10 99.85 48.00 C 104.04 47.99 108.24 48.00 112.43 48.00 C 113.39 65.67 114.50 83.33 115.49 101.00 C 111.45 101.00 107.40 101.01 103.36 100.99 C 102.89 93.74 102.47 86.48 102.07 79.23 C 99.66 86.49 97.15 93.73 94.71 100.99 C 90.61 100.95 86.50 101.15 82.40 100.85 C 79.93 93.36 77.36 85.90 74.69 78.48 C 74.44 86.00 73.62 93.48 73.36 101.00 C 69.28 101.00 65.19 101.00 61.10 101.00 C 62.17 83.33 63.36 65.67 64.45 48.00 Z" />
        <path d="M 119.60 48.00 C 123.65 48.00 127.69 48.00 131.74 48.01 C 131.74 59.01 131.72 70.01 131.74 81.01 C 131.51 85.47 135.71 89.35 140.10 89.02 C 144.20 88.91 147.64 85.08 147.53 81.02 C 147.55 70.02 147.52 59.01 147.53 48.00 C 151.60 48.00 155.67 48.00 159.74 48.01 C 159.67 59.49 159.85 70.98 159.65 82.46 C 159.14 93.61 147.92 102.57 136.94 100.86 C 127.64 99.76 119.94 91.34 119.62 82.00 C 119.57 70.66 119.61 59.33 119.60 48.00 Z" />
        <path d="M 165.50 48.03 C 170.29 47.97 175.08 48.01 179.87 48.00 C 182.80 52.67 185.72 57.35 188.64 62.03 C 191.39 57.32 194.27 52.69 197.04 47.99 C 201.82 48.01 206.61 47.99 211.39 48.01 C 206.05 56.48 200.92 65.10 195.78 73.69 C 201.49 82.77 206.93 92.03 212.79 101.01 C 207.97 100.97 203.15 101.05 198.33 100.96 C 195.09 95.79 191.93 90.58 188.70 85.42 C 185.48 90.60 182.35 95.83 179.13 101.02 C 174.41 100.98 169.68 101.01 164.96 101.00 C 170.55 91.91 176.00 82.74 181.53 73.62 C 176.00 65.21 171.10 56.40 165.50 48.03 Z" />
        <path d="M 216.60 48.00 C 220.64 48.00 224.69 48.00 228.74 48.01 C 228.73 77.68 228.73 107.36 228.74 137.04 C 228.83 141.39 228.77 145.96 226.59 149.87 C 222.49 158.47 211.73 163.16 202.67 160.11 C 194.49 157.70 188.47 149.51 188.59 140.98 C 188.61 129.99 188.59 119.00 188.60 108.00 C 192.64 108.00 196.69 107.99 200.74 108.01 C 200.74 118.99 200.72 129.97 200.74 140.96 C 200.48 145.46 204.75 149.40 209.18 149.01 C 213.25 148.85 216.63 145.06 216.53 141.03 C 216.51 110.02 216.65 79.01 216.60 48.00 Z" />
        <path d="M 133.45 108.00 C 137.63 108.00 141.82 107.99 146.01 108.01 C 149.84 119.09 153.76 130.15 157.56 141.24 C 161.30 130.16 165.14 119.10 168.85 108.01 C 173.04 107.99 177.24 108.00 181.43 108.00 C 182.39 125.67 183.50 143.33 184.49 161.00 C 180.44 161.00 176.40 161.01 172.36 160.99 C 171.89 153.75 171.48 146.51 171.07 139.27 C 168.64 146.51 166.15 153.74 163.71 160.99 C 159.62 160.97 155.52 161.11 151.44 160.88 C 148.91 153.40 146.38 145.91 143.69 138.48 C 143.44 146.00 142.61 153.48 142.37 161.00 C 138.28 161.00 134.19 161.00 130.10 161.00 C 131.17 143.33 132.36 125.67 133.45 108.00 Z" />
        <path d="M 234.50 108.03 C 239.29 107.97 244.08 108.01 248.87 108.00 C 251.78 112.67 254.73 117.32 257.60 122.02 C 260.41 117.35 263.25 112.69 266.03 107.99 C 270.82 108.01 275.61 107.99 280.39 108.01 C 275.04 116.48 269.93 125.09 264.78 133.68 C 270.48 142.77 275.93 152.02 281.79 161.01 C 276.97 160.97 272.15 161.05 267.33 160.96 C 264.09 155.80 260.93 150.58 257.70 145.42 C 254.45 150.60 251.37 155.88 248.08 161.04 C 243.37 160.96 238.67 161.02 233.96 161.00 C 239.55 151.91 245.00 142.74 250.53 133.62 C 245.00 125.21 240.10 116.40 234.50 108.03 Z" />
      </svg>
      <p class="login-subtitle">Sign in to your dashboard</p>
    </div>

    <!-- Login form -->
    <div class="login-card rounded-lg shadow-xl p-8">
      {#if oidcEnabled}
        <!-- OIDC Login Button -->
        <button
          type="button"
          onclick={handleOIDCLogin}
          class="login-oidc-btn w-full py-2.5 px-4 font-medium
                 rounded-md transition-colors flex items-center justify-center gap-2
                 mb-6"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
          </svg>
          Sign in with SSO
        </button>

        <!-- Divider -->
        <div class="relative mb-6">
          <div class="absolute inset-0 flex items-center">
            <div class="login-divider w-full border-t"></div>
          </div>
          <div class="relative flex justify-center text-sm">
            <span class="login-divider-text px-2">or continue with username</span>
          </div>
        </div>
      {/if}

      <form onsubmit={handleSubmit}>
        {#if error}
          <div class="mb-4 p-3 bg-red-500/10 border border-red-500/50 rounded-md text-red-400 text-sm">
            {error}
          </div>
        {/if}

        <div class="mb-4">
          <label for="username" class="login-label block text-sm font-medium mb-1">
            Username
          </label>
          <input
            id="username"
            type="text"
            bind:value={username}
            onkeydown={handleKeydown}
            class="login-input w-full px-4 py-2 rounded-md
                   focus:outline-none focus:ring-2 focus:ring-[var(--accent-primary)]"
            placeholder="Enter your username"
            autocomplete="username"
            disabled={loading}
          />
        </div>

        <div class="mb-4">
          <label for="password" class="login-label block text-sm font-medium mb-1">
            Password
          </label>
          <input
            id="password"
            type="password"
            bind:value={password}
            onkeydown={handleKeydown}
            class="login-input w-full px-4 py-2 rounded-md
                   focus:outline-none focus:ring-2 focus:ring-[var(--accent-primary)]"
            placeholder="Enter your password"
            autocomplete="current-password"
            disabled={loading}
          />
        </div>

        <div class="flex items-center justify-between mb-6">
          <label class="login-label flex items-center text-sm">
            <input
              type="checkbox"
              bind:checked={rememberMe}
              class="w-4 h-4 rounded border-[var(--border-default)] text-[var(--accent-primary)] focus:ring-[var(--accent-primary)]"
              disabled={loading}
            />
            <span class="ml-2">Remember me</span>
          </label>
        </div>

        <button
          type="submit"
          disabled={loading}
          class="login-submit w-full py-2 px-4 font-medium rounded-md
                 transition-colors focus:outline-none focus:ring-2 focus:ring-[var(--accent-primary)]
                 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {#if loading}
            <span class="inline-flex items-center">
              <svg class="animate-spin -ml-1 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              Signing in...
            </span>
          {:else}
            Sign in
          {/if}
        </button>
      </form>
    </div>
  </div>
</div>

<style>
  .login-page {
    background: var(--bg-base);
  }
  .login-logo {
    color: var(--accent-primary);
  }
  .login-subtitle {
    color: var(--text-muted);
  }
  .login-card {
    background: var(--bg-surface);
    border: 1px solid var(--border-default);
  }
  .login-label {
    color: var(--text-secondary);
  }
  .login-input {
    background: var(--bg-elevated);
    border: 1px solid var(--border-default);
    color: var(--text-primary);
  }
  .login-input::placeholder {
    color: var(--text-muted);
  }
  .login-input:focus {
    border-color: var(--accent-primary);
  }
  .login-submit {
    background: var(--accent-primary);
    color: var(--accent-on-primary);
  }
  .login-submit:hover:not(:disabled) {
    filter: brightness(1.1);
  }
  .login-oidc-btn {
    background: var(--bg-elevated);
    border: 1px solid var(--border-default);
    color: var(--text-primary);
  }
  .login-oidc-btn:hover {
    background: var(--bg-hover);
  }
  .login-divider {
    border-color: var(--border-default);
  }
  .login-divider-text {
    background: var(--bg-surface);
    color: var(--text-muted);
  }
</style>

<script lang="ts">
  import { onMount } from 'svelte';
  import { login, authState, checkAuthStatus } from '$lib/authStore';

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
      const res = await fetch('/api/auth/status');
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
    window.location.href = `/api/auth/oidc/login?redirect=${redirect}`;
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') {
      handleSubmit(new SubmitEvent('submit'));
    }
  }
</script>

<div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900 p-4">
  <div class="w-full max-w-md">
    <!-- Logo/Title -->
    <div class="text-center mb-8">
      <h1 class="text-3xl font-bold text-white mb-2">
        <span class="text-brand-500">Muxi</span>mux
      </h1>
      <p class="text-gray-400">Sign in to your dashboard</p>
    </div>

    <!-- Login form -->
    <div class="bg-gray-800 rounded-lg shadow-xl p-8 border border-gray-700">
      {#if oidcEnabled}
        <!-- OIDC Login Button -->
        <button
          type="button"
          onclick={handleOIDCLogin}
          class="w-full py-2.5 px-4 bg-gray-700 hover:bg-gray-600 text-white font-medium
                 rounded-md transition-colors flex items-center justify-center gap-2
                 border border-gray-600 mb-6"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
          </svg>
          Sign in with SSO
        </button>

        <!-- Divider -->
        <div class="relative mb-6">
          <div class="absolute inset-0 flex items-center">
            <div class="w-full border-t border-gray-700"></div>
          </div>
          <div class="relative flex justify-center text-sm">
            <span class="px-2 bg-gray-800 text-gray-500">or continue with username</span>
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
          <label for="username" class="block text-sm font-medium text-gray-300 mb-1">
            Username
          </label>
          <input
            id="username"
            type="text"
            bind:value={username}
            onkeydown={handleKeydown}
            class="w-full px-4 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                   placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-brand-500
                   focus:border-transparent"
            placeholder="Enter your username"
            autocomplete="username"
            disabled={loading}
          />
        </div>

        <div class="mb-4">
          <label for="password" class="block text-sm font-medium text-gray-300 mb-1">
            Password
          </label>
          <input
            id="password"
            type="password"
            bind:value={password}
            onkeydown={handleKeydown}
            class="w-full px-4 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                   placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-brand-500
                   focus:border-transparent"
            placeholder="Enter your password"
            autocomplete="current-password"
            disabled={loading}
          />
        </div>

        <div class="flex items-center justify-between mb-6">
          <label class="flex items-center text-sm text-gray-400">
            <input
              type="checkbox"
              bind:checked={rememberMe}
              class="w-4 h-4 bg-gray-700 border-gray-600 rounded focus:ring-brand-500 text-brand-600"
              disabled={loading}
            />
            <span class="ml-2">Remember me</span>
          </label>
        </div>

        <button
          type="submit"
          disabled={loading}
          class="w-full py-2 px-4 bg-brand-600 hover:bg-brand-700 disabled:bg-brand-800
                 disabled:cursor-not-allowed text-white font-medium rounded-md
                 transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500
                 focus:ring-offset-2"
        >
          {#if loading}
            <span class="inline-flex items-center">
              <svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
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

    <!-- Footer -->
    <p class="mt-6 text-center text-sm text-gray-500">
      Muximux Dashboard
    </p>
  </div>
</div>

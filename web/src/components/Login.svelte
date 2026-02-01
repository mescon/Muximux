<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { login, authState } from '$lib/authStore';

  const dispatch = createEventDispatcher<{
    success: void;
  }>();

  let username = '';
  let password = '';
  let rememberMe = false;
  let error: string | null = null;
  let loading = false;

  async function handleSubmit() {
    if (!username || !password) {
      error = 'Username and password are required';
      return;
    }

    loading = true;
    error = null;

    const result = await login(username, password, rememberMe);

    loading = false;

    if (result.success) {
      dispatch('success');
    } else {
      error = result.message || 'Login failed';
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') {
      handleSubmit();
    }
  }
</script>

<div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900 p-4">
  <div class="w-full max-w-md">
    <!-- Logo/Title -->
    <div class="text-center mb-8">
      <h1 class="text-3xl font-bold text-white mb-2">Muximux</h1>
      <p class="text-gray-400">Sign in to your dashboard</p>
    </div>

    <!-- Login form -->
    <div class="bg-gray-800 rounded-lg shadow-xl p-8 border border-gray-700">
      <form on:submit|preventDefault={handleSubmit}>
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
            on:keydown={handleKeydown}
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
            on:keydown={handleKeydown}
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
                 focus:ring-offset-2 focus:ring-offset-gray-800"
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

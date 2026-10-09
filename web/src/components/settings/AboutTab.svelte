<script lang="ts">
  import { onMount } from 'svelte';
  import type { SystemInfo, UpdateInfo } from '$lib/types';
  import { fetchSystemInfo, checkForUpdates, errorText } from '$lib/api';
  import { renderChangelog } from '$lib/changelog';
  import * as m from '$lib/paraglide/messages.js';

  let systemInfo = $state<SystemInfo | null>(null);
  let updateInfo = $state<UpdateInfo | null>(null);
  let aboutLoading = $state(false);
  let aboutError = $state<string | null>(null);
  let updateInstructionsExpanded = $state(false);
  let changelogExpanded = $state(false);
  // Pre-rendered changelog HTML. We dynamic-import `marked` only
  // when a changelog actually arrives so the ~50KB markdown parser
  // stays out of the Settings chunk for the (common) case where no
  // update is available or the operator never opens the About tab.
  let parsedChangelog = $state<string>('');

  async function loadAboutData() {
    aboutLoading = true;
    aboutError = null;
    try {
      const [sysInfo, updInfo] = await Promise.all([
        fetchSystemInfo(),
        checkForUpdates().catch(() => null)
      ]);
      systemInfo = sysInfo;
      updateInfo = updInfo;
      if (updInfo?.changelog) {
        changelogExpanded = true;
        // Parse + sanitize off the main bundle. The changelog is remote
        // (update-check) content rendered via {@html}, so it must be
        // sanitized before it hits the DOM (see renderChangelog). Failure
        // here is silent on purpose: an empty parsedChangelog is handled
        // by the truthy-check around the {@html ...} block.
        try {
          parsedChangelog = await renderChangelog(updInfo.changelog);
        } catch {
          parsedChangelog = '';
        }
      }
      if (updInfo?.update_available) updateInstructionsExpanded = true;
    } catch (e) {
      aboutError = errorText(e, m.error_failedLoad());
    } finally {
      aboutLoading = false;
    }
  }

  onMount(() => { loadAboutData(); });
</script>

<div class="space-y-6">
  {#if aboutLoading}
    <div class="flex items-center justify-center py-16">
      <svg class="w-6 h-6 text-accent-text animate-spin" viewBox="0 0 24 24" fill="none">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
      </svg>
      <span class="ms-3 text-text-muted">{m.about_loadingSystemInfo()}</span>
    </div>
  {:else if aboutError}
    <div class="p-4 rounded-lg notice notice-danger" role="alert">
      <div class="flex items-center justify-between">
        <span class="text-sm">{aboutError}</span>
        <button
          class="px-3 py-1 text-xs bg-danger-bg hover:brightness-110 rounded text-danger-text transition-colors"
          onclick={() => { systemInfo = null; loadAboutData(); }}
        >{m.common_retry()}</button>
      </div>
    </div>
  {:else if systemInfo}
    <!-- Version Status -->
    <div class="rounded-xl border p-5 {updateInfo?.update_available ? 'border-warning-border bg-warning-bg' : 'border-success-border bg-success-bg'}">
      <div class="flex items-start gap-4">
        {#if updateInfo?.update_available}
          <div class="w-10 h-10 rounded-lg bg-warning-bg flex items-center justify-center flex-shrink-0">
            <svg class="w-5 h-5 text-warning-text" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 10l7-7m0 0l7 7m-7-7v18" />
            </svg>
          </div>
        {:else}
          <div class="w-10 h-10 rounded-lg bg-success-bg flex items-center justify-center flex-shrink-0">
            <svg class="w-5 h-5 text-success-text" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
            </svg>
          </div>
        {/if}
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 flex-wrap">
            <h3 class="text-lg font-semibold text-text-primary">
              {updateInfo?.update_available ? m.about_updateAvailable() : m.about_upToDate()}
            </h3>
            {#if updateInfo?.update_available}
              <span class="px-2 py-0.5 text-xs font-medium bg-warning-bg text-warning-text rounded-full">
                v{updateInfo.latest_version}
              </span>
            {/if}
          </div>
          <div class="flex flex-wrap gap-x-4 gap-y-1 mt-1.5 text-sm text-text-muted">
            <span>{m.about_currentVersion()} <span class="text-text-primary font-mono">{systemInfo.version}</span></span>
            {#if updateInfo}
              <span>{m.about_latestVersion()} <span class="text-text-primary font-mono">{updateInfo.latest_version}</span></span>
              {#if updateInfo.published_at}
                <span>{m.about_released()} {new Date(updateInfo.published_at).toLocaleDateString()}</span>
              {/if}
            {/if}
          </div>
        </div>
        {#if updateInfo?.release_url}
          <a
            href={updateInfo.release_url}
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-secondary btn-sm flex items-center gap-1.5 flex-shrink-0"
          >
            {m.about_viewOnGitHub()}
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
            </svg>
          </a>
        {/if}
      </div>
    </div>

    <!-- Release Notes (collapsible) -->
    {#if updateInfo?.changelog}
      <div class="focus-inset rounded-xl border border-border overflow-hidden">
        <button
          class="w-full flex items-center justify-between p-4 text-start hover:bg-bg-surface/50 transition-colors"
          onclick={() => changelogExpanded = !changelogExpanded}
        >
          <h3 class="text-sm font-semibold text-text-primary">{m.about_releaseNotes()}</h3>
          <svg
            class="w-4 h-4 text-text-muted transition-transform {changelogExpanded ? 'rotate-180' : ''}"
            fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        {#if changelogExpanded && parsedChangelog}
          <div class="px-4 pb-4 border-t border-border">
            <div class="mt-3 text-sm text-text-secondary leading-relaxed max-h-64 overflow-y-auto changelog-content">
              <!-- eslint-disable-next-line svelte/no-at-html-tags -- changelog from GitHub release notes, sanitized by marked -->
              {@html parsedChangelog}
            </div>
          </div>
        {/if}
      </div>
    {/if}

    <!-- How to Update (collapsible) -->
    {#if updateInfo}
      <div class="focus-inset rounded-xl border border-border overflow-hidden">
        <button
          class="w-full flex items-center justify-between p-4 text-start hover:bg-bg-surface/50 transition-colors"
          onclick={() => updateInstructionsExpanded = !updateInstructionsExpanded}
        >
          <h3 class="text-sm font-semibold text-text-primary">{m.about_howToUpdate()}</h3>
          <svg
            class="w-4 h-4 text-text-muted transition-transform {updateInstructionsExpanded ? 'rotate-180' : ''}"
            fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        {#if updateInstructionsExpanded}
          <div class="border-t border-border divide-y divide-border-subtle">
            <!-- Docker -->
            <div class="p-4">
              <div class="flex items-center gap-2 mb-2">
                <svg class="w-5 h-5 text-blue-400" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M13.983 11.078h2.119a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.119a.186.186 0 00-.185.186v1.887c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 00.186-.186V3.574a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.888c0 .102.082.185.185.186m0 2.716h2.118a.187.187 0 00.186-.186V6.29a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.887c0 .102.082.186.185.186m-2.93 0h2.12a.186.186 0 00.184-.186V6.29a.185.185 0 00-.185-.185H8.1a.186.186 0 00-.185.185v1.887c0 .102.083.186.185.186m-2.964 0h2.119a.186.186 0 00.185-.186V6.29a.186.186 0 00-.185-.185H5.136a.186.186 0 00-.186.185v1.887c0 .102.084.186.186.186m5.893 2.715h2.118a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.118a.185.185 0 00-.185.186v1.887c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.186v1.887c0 .102.083.185.185.185m-2.964 0h2.119a.186.186 0 00.185-.185V9.006a.186.186 0 00-.185-.186H5.136a.186.186 0 00-.186.186v1.887c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.186v1.887c0 .102.082.185.185.185M23.763 9.89c-.065-.051-.672-.51-1.954-.51-.338.001-.676.03-1.01.087-.248-1.7-1.653-2.53-1.716-2.566l-.344-.199-.226.327c-.284.438-.49.922-.612 1.43-.23.97-.09 1.882.403 2.661-.595.332-1.55.413-1.744.42H.751a.751.751 0 00-.75.748 11.687 11.687 0 00.692 4.062c.545 1.428 1.355 2.48 2.41 3.124 1.18.723 3.1 1.137 5.275 1.137.983.003 1.963-.086 2.93-.266a12.248 12.248 0 003.823-1.389c.98-.567 1.86-1.288 2.61-2.136 1.252-1.418 1.998-2.997 2.553-4.4h.221c1.372 0 2.215-.549 2.68-1.009.309-.293.55-.65.707-1.046l.098-.288z"/>
                </svg>
                <span class="text-sm font-medium text-text-primary">{m.about_docker()}</span>
                {#if systemInfo.environment === 'docker'}
                  <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-accent-muted text-accent-text rounded uppercase tracking-wider">{m.about_yourPlatform()}</span>
                {/if}
              </div>
              <pre class="text-xs text-text-secondary bg-bg-base/50 rounded-lg p-3 overflow-x-auto font-mono">cd /path/to/muximux
docker compose pull
docker compose up -d</pre>
            </div>

            <!-- Linux -->
            <div class="p-4">
              <div class="flex items-center gap-2 mb-2">
                <svg class="w-5 h-5" viewBox="0 0 24 24" aria-hidden="true">
                  <path fill="#fff" d="M 13.03 3.4 L 13.05 3.4 C 13.26 3.4 13.44 3.47 13.63 3.6 C 13.82 3.74 13.96 3.93 14.07 4.14 C 14.17 4.4 14.23 4.59 14.23 4.86 C 14.23 4.84 14.24 4.82 14.24 4.8 L 14.24 4.91 A 0.09 0.09 0 0 1 14.24 4.88 L 14.23 4.86 A 1.81 1.81 0 0 1 14.08 5.57 A 0.95 0.95 0 0 1 13.87 5.9 A 0.71 0.71 0 0 0 13.78 5.86 C 13.68 5.81 13.58 5.8 13.5 5.73 A 1.31 1.31 0 0 0 13.28 5.66 C 13.33 5.6 13.42 5.53 13.46 5.46 C 13.51 5.33 13.54 5.2 13.55 5.06 L 13.55 5.04 A 1.21 1.21 0 0 0 13.49 4.64 C 13.44 4.51 13.39 4.44 13.3 4.31 C 13.22 4.24 13.14 4.18 13.04 4.18 L 13.02 4.18 C 12.93 4.18 12.85 4.21 12.76 4.31 A 0.8 0.8 0 0 0 12.55 4.64 A 1.18 1.18 0 0 0 12.46 5.04 L 12.46 5.06 C 12.47 5.15 12.47 5.24 12.48 5.33 C 12.29 5.26 12.05 5.19 11.88 5.13 A 1.64 1.64 0 0 1 11.86 4.93 L 11.86 4.91 A 1.77 1.77 0 0 1 12.01 4.14 C 12.09 3.92 12.24 3.73 12.44 3.6 A 0.98 0.98 0 0 1 13.03 3.4 L 13.03 3.4 M 10.07 3.46 L 10.11 3.46 C 10.25 3.46 10.38 3.51 10.51 3.6 C 10.65 3.73 10.77 3.89 10.85 4.06 C 10.94 4.26 10.99 4.46 11 4.73 L 11 4.73 C 11.01 4.87 11.01 4.93 11 5 L 11 5.08 C 10.97 5.09 10.95 5.1 10.92 5.1 C 10.77 5.16 10.64 5.24 10.52 5.3 C 10.54 5.21 10.54 5.12 10.53 5.04 L 10.53 5.02 C 10.52 4.89 10.49 4.82 10.45 4.69 A 0.61 0.61 0 0 0 10.28 4.42 A 0.25 0.25 0 0 0 10.1 4.36 L 10.08 4.36 C 10 4.36 9.95 4.4 9.89 4.49 A 0.55 0.55 0 0 0 9.77 4.76 A 0.94 0.94 0 0 0 9.75 5.09 L 9.75 5.11 C 9.76 5.24 9.78 5.31 9.83 5.44 C 9.87 5.57 9.92 5.64 9.99 5.71 C 10 5.72 10.01 5.73 10.03 5.73 C 9.96 5.79 9.91 5.8 9.85 5.87 A 0.3 0.3 0 0 1 9.72 5.94 A 2.62 2.62 0 0 1 9.44 5.53 A 1.77 1.77 0 0 1 9.29 4.87 A 1.76 1.76 0 0 1 9.37 4.2 A 1.43 1.43 0 0 1 9.65 3.66 C 9.78 3.53 9.91 3.46 10.07 3.46 L 10.07 3.46 M 14.24 7.31 C 14.6 8.73 15.44 10.79 15.98 11.79 C 16.26 12.32 16.83 13.44 17.08 14.81 C 17.23 14.8 17.41 14.83 17.59 14.87 C 18.24 13.2 17.05 11.41 16.5 10.91 C 16.28 10.71 16.27 10.57 16.38 10.57 C 16.97 11.11 17.74 12.14 18.03 13.33 C 18.16 13.86 18.19 14.43 18.05 15 C 18.11 15.03 18.18 15.06 18.25 15.07 C 19.28 15.6 19.66 16 19.48 16.6 L 19.48 16.56 C 19.42 16.56 19.36 16.56 19.3 16.56 L 19.29 16.56 C 19.44 16.09 19.1 15.73 18.22 15.34 C 17.31 14.94 16.57 15 16.45 15.8 C 16.44 15.84 16.44 15.87 16.43 15.94 C 16.36 15.96 16.29 15.99 16.22 16 C 15.79 16.27 15.56 16.67 15.43 17.19 C 15.3 17.72 15.26 18.34 15.23 19.06 L 15.23 19.06 C 15.21 19.39 15.06 19.9 14.91 20.41 C 13.41 21.48 11.33 21.95 9.56 20.74 A 2.65 2.65 0 0 0 9.16 20.21 A 1.45 1.45 0 0 0 8.88 19.88 C 9.06 19.88 9.22 19.85 9.35 19.81 A 0.61 0.61 0 0 0 9.66 19.48 C 9.77 19.21 9.66 18.78 9.32 18.31 C 8.97 17.85 8.38 17.32 7.53 16.79 C 6.9 16.39 6.54 15.92 6.38 15.4 C 6.21 14.86 6.23 14.31 6.36 13.75 C 6.61 12.68 7.24 11.64 7.64 10.99 C 7.74 10.92 7.67 11.12 7.23 11.96 C 6.83 12.71 6.09 14.46 7.11 15.82 A 8.12 8.12 0 0 1 7.75 12.94 C 8.32 11.66 9.5 9.44 9.59 7.67 C 9.64 7.71 9.81 7.81 9.88 7.87 C 10.1 8.01 10.26 8.21 10.47 8.34 C 10.68 8.54 10.95 8.67 11.34 8.67 C 11.38 8.68 11.42 8.68 11.45 8.68 C 11.87 8.68 12.18 8.55 12.45 8.41 C 12.74 8.28 12.97 8.08 13.19 8.01 L 13.2 8.01 C 13.66 7.88 14.03 7.61 14.24 7.31 L 14.24 7.31"/>
                  <path fill="#fcc624" d="M 11.44 5.17 C 11.77 5.17 12.17 5.24 12.66 5.57 C 12.95 5.77 13.18 5.84 13.71 6.04 L 13.71 6.04 C 13.97 6.17 14.12 6.3 14.19 6.44 L 14.19 6.3 A 0.57 0.57 0 0 1 14.21 6.77 C 14.08 7.08 13.69 7.42 13.14 7.62 L 13.14 7.62 C 12.87 7.75 12.64 7.95 12.37 8.08 C 12.09 8.22 11.78 8.38 11.36 8.35 A 1.14 1.14 0 0 1 10.91 8.28 A 3.57 3.57 0 0 1 10.59 8.09 C 10.39 7.95 10.22 7.75 9.97 7.62 L 9.97 7.62 L 9.97 7.62 C 9.57 7.37 9.35 7.1 9.28 6.91 C 9.21 6.64 9.28 6.44 9.48 6.31 C 9.7 6.17 9.86 6.03 9.96 5.97 C 10.06 5.9 10.1 5.87 10.13 5.84 L 10.14 5.84 L 10.14 5.84 C 10.31 5.63 10.57 5.37 10.98 5.23 C 11.11 5.2 11.27 5.17 11.44 5.17 L 11.44 5.17 M 16.43 16.27 C 16.46 16.87 16.77 17.52 17.31 17.65 C 17.9 17.78 18.74 17.31 19.1 16.88 L 19.31 16.87 C 19.62 16.86 19.89 16.88 20.16 17.14 L 20.16 17.14 C 20.37 17.34 20.46 17.67 20.55 18.02 C 20.64 18.42 20.71 18.8 20.96 19.08 C 21.45 19.61 21.61 19.99 21.6 20.22 L 21.6 20.22 L 21.6 20.24 L 21.6 20.22 C 21.58 20.49 21.41 20.62 21.1 20.82 C 20.47 21.22 19.35 21.53 18.64 22.39 C 18.02 23.13 17.27 23.53 16.6 23.58 C 15.94 23.63 15.37 23.38 15.03 22.68 L 15.03 22.68 C 14.82 22.28 14.91 21.65 15.08 20.99 C 15.26 20.32 15.51 19.64 15.54 19.09 C 15.58 18.38 15.62 17.76 15.74 17.28 C 15.86 16.81 16.05 16.48 16.38 16.29 L 16.43 16.27 L 16.43 16.27 M 5.61 16.32 L 5.62 16.32 C 5.67 16.32 5.73 16.32 5.78 16.33 C 6.15 16.39 6.48 16.67 6.8 17.08 L 7.71 18.75 L 7.71 18.75 C 7.96 19.29 8.47 19.82 8.9 20.39 C 9.34 20.99 9.67 21.52 9.63 21.96 L 9.63 21.96 C 9.58 22.71 9.15 23.11 8.51 23.26 C 7.86 23.39 6.99 23.26 6.11 22.8 C 5.14 22.26 3.99 22.33 3.26 22.19 C 2.89 22.13 2.65 21.99 2.53 21.79 C 2.42 21.59 2.42 21.19 2.66 20.56 L 2.66 20.56 L 2.66 20.56 C 2.77 20.22 2.69 19.8 2.63 19.44 C 2.58 19.04 2.55 18.73 2.67 18.5 C 2.83 18.16 3.07 18.1 3.36 17.96 C 3.66 17.83 4 17.76 4.28 17.5 L 4.28 17.5 L 4.28 17.49 C 4.54 17.23 4.73 16.89 4.95 16.66 C 5.14 16.45 5.33 16.32 5.61 16.32 L 5.61 16.32"/>
                  <path fill="#000" d="M12.504 0c-.155 0-.315.008-.48.021-4.226.333-3.105 4.807-3.17 6.298-.076 1.092-.3 1.953-1.05 3.02-.885 1.051-2.127 2.75-2.716 4.521-.278.832-.41 1.684-.287 2.489a.424.424 0 00-.11.135c-.26.268-.45.6-.663.839-.199.199-.485.267-.797.4-.313.136-.658.269-.864.68-.09.189-.136.394-.132.602 0 .199.027.4.055.536.058.399.116.728.04.97-.249.68-.28 1.145-.106 1.484.174.334.535.47.94.601.81.2 1.91.135 2.774.6.926.466 1.866.67 2.616.47.526-.116.97-.464 1.208-.946.587-.003 1.23-.269 2.26-.334.699-.058 1.574.267 2.577.2.025.134.063.198.114.333l.003.003c.391.778 1.113 1.132 1.884 1.071.771-.06 1.592-.536 2.257-1.306.631-.765 1.683-1.084 2.378-1.503.348-.199.629-.469.649-.853.023-.4-.2-.811-.714-1.376v-.097l-.003-.003c-.17-.2-.25-.535-.338-.926-.085-.401-.182-.786-.492-1.046h-.003c-.059-.054-.123-.067-.188-.135a.357.357 0 00-.19-.064c.431-1.278.264-2.55-.173-3.694-.533-1.41-1.465-2.638-2.175-3.483-.796-1.005-1.576-1.957-1.56-3.368.026-2.152.236-6.133-3.544-6.139zm.529 3.405h.013c.213 0 .396.062.584.198.19.135.33.332.438.533.105.259.158.459.166.724 0-.02.006-.04.006-.06v.105a.086.086 0 01-.004-.021l-.004-.024a1.807 1.807 0 01-.15.706.953.953 0 01-.213.335.71.71 0 00-.088-.042c-.104-.045-.198-.064-.284-.133a1.312 1.312 0 00-.22-.066c.05-.06.146-.133.183-.198.053-.128.082-.264.088-.402v-.02a1.21 1.21 0 00-.061-.4c-.045-.134-.101-.2-.183-.333-.084-.066-.167-.132-.267-.132h-.016c-.093 0-.176.03-.262.132a.8.8 0 00-.205.334 1.18 1.18 0 00-.09.4v.019c.002.089.008.179.02.267-.193-.067-.438-.135-.607-.202a1.635 1.635 0 01-.018-.2v-.02a1.772 1.772 0 01.15-.768c.082-.22.232-.406.43-.533a.985.985 0 01.594-.2zm-2.962.059h.036c.142 0 .27.048.399.135.146.129.264.288.344.465.09.199.14.4.153.667v.004c.007.134.006.2-.002.266v.08c-.03.007-.056.018-.083.024-.152.055-.274.135-.393.2.012-.09.013-.18.003-.267v-.015c-.012-.133-.04-.2-.082-.333a.613.613 0 00-.166-.267.248.248 0 00-.183-.064h-.021c-.071.006-.13.04-.186.132a.552.552 0 00-.12.27.944.944 0 00-.023.33v.015c.012.135.037.2.08.334.046.134.098.2.166.268.01.009.02.018.034.024-.07.057-.117.07-.176.136a.304.304 0 01-.131.068 2.62 2.62 0 01-.275-.402 1.772 1.772 0 01-.155-.667 1.759 1.759 0 01.08-.668 1.43 1.43 0 01.283-.535c.128-.133.26-.2.418-.2zm1.37 1.706c.332 0 .733.065 1.216.399.293.2.523.269 1.052.468h.003c.255.136.405.266.478.399v-.131a.571.571 0 01.016.47c-.123.31-.516.643-1.063.842v.002c-.268.135-.501.333-.775.465-.276.135-.588.292-1.012.267a1.139 1.139 0 01-.448-.067 3.566 3.566 0 01-.322-.198c-.195-.135-.363-.332-.612-.465v-.005h-.005c-.4-.246-.616-.512-.686-.71-.07-.268-.005-.47.193-.6.224-.135.38-.271.483-.336.104-.074.143-.102.176-.131h.002v-.003c.169-.202.436-.47.839-.601.139-.036.294-.065.466-.065zm2.8 2.142c.358 1.417 1.196 3.475 1.735 4.473.286.534.855 1.659 1.102 3.024.156-.005.33.018.513.064.646-1.671-.546-3.467-1.089-3.966-.22-.2-.232-.335-.123-.335.59.534 1.365 1.572 1.646 2.757.13.535.16 1.104.021 1.67.067.028.135.06.205.067 1.032.534 1.413.938 1.23 1.537v-.043c-.06-.003-.12 0-.18 0h-.016c.151-.467-.182-.825-1.065-1.224-.915-.4-1.646-.336-1.77.465-.008.043-.013.066-.018.135-.068.023-.139.053-.209.064-.43.268-.662.669-.793 1.187-.13.533-.17 1.156-.205 1.869v.003c-.02.334-.17.838-.319 1.35-1.5 1.072-3.58 1.538-5.348.334a2.645 2.645 0 00-.402-.533 1.45 1.45 0 00-.275-.333c.182 0 .338-.03.465-.067a.615.615 0 00.314-.334c.108-.267 0-.697-.345-1.163-.345-.467-.931-.995-1.788-1.521-.63-.4-.986-.87-1.15-1.396-.165-.534-.143-1.085-.015-1.645.245-1.07.873-2.11 1.274-2.763.107-.065.037.135-.408.974-.396.751-1.14 2.497-.122 3.854a8.123 8.123 0 01.647-2.876c.564-1.278 1.743-3.504 1.836-5.268.048.036.217.135.289.202.218.133.38.333.59.465.21.201.477.335.876.335.039.003.075.006.11.006.412 0 .73-.134.997-.268.29-.134.52-.334.74-.4h.005c.467-.135.835-.402 1.044-.7zm2.185 8.958c.037.6.343 1.245.882 1.377.588.134 1.434-.333 1.791-.765l.211-.01c.315-.007.577.01.847.268l.003.003c.208.199.305.53.391.876.085.4.154.78.409 1.066.486.527.645.906.636 1.14l.003-.007v.018l-.003-.012c-.015.262-.185.396-.498.595-.63.401-1.746.712-2.457 1.57-.618.737-1.37 1.14-2.036 1.191-.664.053-1.237-.2-1.574-.898l-.005-.003c-.21-.4-.12-1.025.056-1.69.176-.668.428-1.344.463-1.897.037-.714.076-1.335.195-1.814.12-.465.308-.797.641-.984l.045-.022zm-10.814.049h.01c.053 0 .105.005.157.014.376.055.706.333 1.023.752l.91 1.664.003.003c.243.533.754 1.064 1.189 1.637.434.598.77 1.131.729 1.57v.006c-.057.744-.48 1.148-1.125 1.294-.645.135-1.52.002-2.395-.464-.968-.536-2.118-.469-2.857-.602-.369-.066-.61-.2-.723-.4-.11-.2-.113-.602.123-1.23v-.004l.002-.003c.117-.334.03-.752-.027-1.118-.055-.401-.083-.71.043-.94.16-.334.396-.4.69-.533.294-.135.64-.202.915-.47h.002v-.002c.256-.268.445-.601.668-.838.19-.201.38-.336.663-.336zm7.159-9.074c-.435.201-.945.535-1.488.535-.542 0-.97-.267-1.28-.466-.154-.134-.28-.268-.373-.335-.164-.134-.144-.333-.074-.333.109.016.129.134.199.2.096.066.215.2.36.333.292.2.68.467 1.167.467.485 0 1.053-.267 1.398-.466.195-.135.445-.334.648-.467.156-.136.149-.267.279-.267.128.016.034.134-.147.332a8.097 8.097 0 01-.69.468zm-1.082-1.583V5.64c-.006-.02.013-.042.029-.05.074-.043.18-.027.26.004.063 0 .16.067.15.135-.006.049-.085.066-.135.066-.055 0-.092-.043-.141-.068-.052-.018-.146-.008-.163-.065zm-.551 0c-.02.058-.113.049-.166.066-.047.025-.086.068-.14.068-.05 0-.13-.02-.136-.068-.01-.066.088-.133.15-.133.08-.031.184-.047.259-.005.019.009.036.03.03.05v.02h.003z"/>
                </svg>
                <span class="text-sm font-medium text-text-primary">{m.about_linux()}</span>
                {#if systemInfo.environment === 'native' && systemInfo.os === 'linux'}
                  <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-accent-muted text-accent-text rounded uppercase tracking-wider">{m.about_yourPlatform()}</span>
                {/if}
              </div>
              <pre class="text-xs text-text-secondary bg-bg-base/50 rounded-lg p-3 overflow-x-auto font-mono"># Stop the running instance, then:
curl -LO https://github.com/mescon/Muximux/releases/latest/download/muximux-linux-amd64
chmod +x muximux-linux-amd64
./muximux-linux-amd64</pre>
              {#if updateInfo.download_urls?.linux_amd64}
                <div class="flex gap-2 mt-2">
                  <a href={updateInfo.download_urls.linux_amd64} class="text-xs text-accent-text hover:underline transition-colors">{m.about_downloadLinuxAmd64()}</a>
                  {#if updateInfo.download_urls?.linux_arm64}
                    <span class="text-text-muted">|</span>
                    <a href={updateInfo.download_urls.linux_arm64} class="text-xs text-accent-text hover:underline transition-colors">{m.about_downloadLinuxArm64()}</a>
                  {/if}
                </div>
              {/if}
            </div>

            <!-- macOS -->
            <div class="p-4">
              <div class="flex items-center gap-2 mb-2">
                <svg class="w-5 h-5 text-text-secondary" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M18.71 19.5c-.83 1.24-1.71 2.45-3.05 2.47-1.34.03-1.77-.79-3.29-.79-1.53 0-2 .77-3.27.82-1.31.05-2.3-1.32-3.14-2.53C4.25 17 2.94 12.45 4.7 9.39c.87-1.52 2.43-2.48 4.12-2.51 1.28-.02 2.5.87 3.29.87.78 0 2.26-1.07 3.8-.91.65.03 2.47.26 3.64 1.98-.09.06-2.17 1.28-2.15 3.81.03 3.02 2.65 4.03 2.68 4.04-.03.07-.42 1.44-1.38 2.83M13 3.5c.73-.83 1.94-1.46 2.94-1.5.13 1.17-.34 2.35-1.04 3.19-.69.85-1.83 1.51-2.95 1.42-.15-1.15.41-2.35 1.05-3.11z"/>
                </svg>
                <span class="text-sm font-medium text-text-primary">{m.about_macos()}</span>
                {#if systemInfo.environment === 'native' && systemInfo.os === 'darwin'}
                  <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-accent-muted text-accent-text rounded uppercase tracking-wider">{m.about_yourPlatform()}</span>
                {/if}
              </div>
              <pre class="text-xs text-text-secondary bg-bg-base/50 rounded-lg p-3 overflow-x-auto font-mono">curl -LO https://github.com/mescon/Muximux/releases/latest/download/muximux-darwin-arm64
chmod +x muximux-darwin-arm64
./muximux-darwin-arm64</pre>
              {#if updateInfo.download_urls?.darwin_arm64}
                <div class="flex gap-2 mt-2">
                  <a href={updateInfo.download_urls.darwin_arm64} class="text-xs text-accent-text hover:underline transition-colors">{m.about_downloadDarwinArm64()}</a>
                  {#if updateInfo.download_urls?.darwin_amd64}
                    <span class="text-text-muted">|</span>
                    <a href={updateInfo.download_urls.darwin_amd64} class="text-xs text-accent-text hover:underline transition-colors">{m.about_downloadDarwinAmd64()}</a>
                  {/if}
                </div>
              {/if}
            </div>

            <!-- Windows -->
            <div class="p-4">
              <div class="flex items-center gap-2 mb-2">
                <svg class="w-5 h-5 text-cyan-400" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M0 3.449L9.75 2.1v9.451H0m10.949-9.602L24 0v11.4H10.949M0 12.6h9.75v9.451L0 20.699M10.949 12.6H24V24l-12.9-1.801"/>
                </svg>
                <span class="text-sm font-medium text-text-primary">{m.about_windows()}</span>
                {#if systemInfo.environment === 'native' && systemInfo.os === 'windows'}
                  <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-accent-muted text-accent-text rounded uppercase tracking-wider">{m.about_yourPlatform()}</span>
                {/if}
              </div>
              <pre class="text-xs text-text-secondary bg-bg-base/50 rounded-lg p-3 overflow-x-auto font-mono"># Download muximux-windows-amd64.exe from the release page
# Replace the existing executable
# Restart</pre>
              {#if updateInfo.download_urls?.windows_amd64}
                <div class="mt-2">
                  <a href={updateInfo.download_urls.windows_amd64} class="text-xs text-accent-text hover:underline transition-colors">{m.about_downloadWindowsAmd64()}</a>
                </div>
              {/if}
            </div>
          </div>
        {/if}
      </div>
    {/if}

    <!-- System Information -->
    <div>
      <h3 class="text-sm font-semibold text-text-primary mb-3">{m.about_systemInfo()}</h3>
      <div class="grid grid-cols-3 gap-3 mb-3">
        <div class="rounded-lg bg-bg-surface border border-border p-3 text-center">
          <div class="flex items-center justify-center gap-1.5 mb-1">
            {#if systemInfo.environment === 'docker'}
              <svg class="w-4 h-4 text-blue-400" viewBox="0 0 24 24" fill="currentColor">
                <path d="M13.983 11.078h2.119a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.119a.186.186 0 00-.185.186v1.887c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 00.186-.186V3.574a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.888c0 .102.082.185.185.186m0 2.716h2.118a.187.187 0 00.186-.186V6.29a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.887c0 .102.082.186.185.186m-2.93 0h2.12a.186.186 0 00.184-.186V6.29a.185.185 0 00-.185-.185H8.1a.186.186 0 00-.185.185v1.887c0 .102.083.186.185.186m-2.964 0h2.119a.186.186 0 00.185-.186V6.29a.186.186 0 00-.185-.185H5.136a.186.186 0 00-.186.185v1.887c0 .102.084.186.186.186m5.893 2.715h2.118a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.118a.185.185 0 00-.185.186v1.887c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.186v1.887c0 .102.083.185.185.185m-2.964 0h2.119a.186.186 0 00.185-.185V9.006a.186.186 0 00-.185-.186H5.136a.186.186 0 00-.186.186v1.887c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.186v1.887c0 .102.082.185.185.185M23.763 9.89c-.065-.051-.672-.51-1.954-.51-.338.001-.676.03-1.01.087-.248-1.7-1.653-2.53-1.716-2.566l-.344-.199-.226.327c-.284.438-.49.922-.612 1.43-.23.97-.09 1.882.403 2.661-.595.332-1.55.413-1.744.42H.751a.751.751 0 00-.75.748 11.687 11.687 0 00.692 4.062c.545 1.428 1.355 2.48 2.41 3.124 1.18.723 3.1 1.137 5.275 1.137.983.003 1.963-.086 2.93-.266a12.248 12.248 0 003.823-1.389c.98-.567 1.86-1.288 2.61-2.136 1.252-1.418 1.998-2.997 2.553-4.4h.221c1.372 0 2.215-.549 2.68-1.009.309-.293.55-.65.707-1.046l.098-.288z"/>
              </svg>
            {:else}
              <svg class="w-4 h-4 text-text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <rect x="2" y="3" width="20" height="14" rx="2" /><path d="M8 21h8m-4-4v4" />
              </svg>
            {/if}
          </div>
          <div class="text-xs text-text-muted mb-0.5">{m.about_environment()}</div>
          <div class="text-sm text-text-primary capitalize">{systemInfo.environment}</div>
        </div>
        <div class="rounded-lg bg-bg-surface border border-border p-3 text-center">
          <div class="flex items-center justify-center gap-1.5 mb-1">
            <svg class="w-4 h-4 text-text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
            </svg>
          </div>
          <div class="text-xs text-text-muted mb-0.5">{m.about_platform()}</div>
          <div class="text-sm text-text-primary">{systemInfo.os}/{systemInfo.arch}</div>
        </div>
        <div class="rounded-lg bg-bg-surface border border-border p-3 text-center">
          <div class="flex items-center justify-center gap-1.5 mb-1">
            <svg class="w-4 h-4 text-text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10" /><path d="M12 6v6l4 2" />
            </svg>
          </div>
          <div class="text-xs text-text-muted mb-0.5">{m.about_uptime()}</div>
          <div class="text-sm text-text-primary">{systemInfo.uptime}</div>
        </div>
      </div>

      <div class="rounded-lg bg-bg-surface border border-border divide-y divide-border-subtle">
        <div class="flex items-center justify-between px-4 py-2.5">
          <span class="text-xs text-text-muted">{m.about_dataDirectory()}</span>
          <span class="text-xs text-text-secondary font-mono">{systemInfo.data_dir}</span>
        </div>
        <div class="flex items-center justify-between px-4 py-2.5">
          <span class="text-xs text-text-muted">{m.about_goVersion()}</span>
          <span class="text-xs text-text-secondary font-mono">{systemInfo.go_version}</span>
        </div>
        <div class="flex items-center justify-between px-4 py-2.5">
          <span class="text-xs text-text-muted">{m.about_buildDate()}</span>
          <span class="text-xs text-text-secondary font-mono">{systemInfo.build_date}</span>
        </div>
        <div class="flex items-center justify-between px-4 py-2.5">
          <span class="text-xs text-text-muted">{m.about_commit()}</span>
          <span class="text-xs text-text-secondary font-mono">{systemInfo.commit.length > 8 ? systemInfo.commit.slice(0, 8) : systemInfo.commit}</span>
        </div>
      </div>
    </div>

    <!-- Links -->
    <div>
      <h3 class="text-sm font-semibold text-text-primary mb-3">{m.about_links()}</h3>
      <div class="flex flex-wrap gap-2">
        <a
          href={systemInfo.links.github}
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-bg-surface hover:bg-bg-hover border border-border text-text-secondary hover:text-text-primary rounded-lg transition-colors"
        >
          <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
          </svg>
          {m.about_github()}
        </a>
        <a
          href={systemInfo.links.issues}
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-bg-surface hover:bg-bg-hover border border-border text-text-secondary hover:text-text-primary rounded-lg transition-colors"
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10" /><path d="M12 8v4m0 4h.01" />
          </svg>
          {m.about_issues()}
        </a>
        <a
          href={systemInfo.links.releases}
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-bg-surface hover:bg-bg-hover border border-border text-text-secondary hover:text-text-primary rounded-lg transition-colors"
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A2 2 0 013 12V7a4 4 0 014-4z" />
          </svg>
          {m.about_releases()}
        </a>
        <a
          href={systemInfo.links.wiki}
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-bg-surface hover:bg-bg-hover border border-border text-text-secondary hover:text-text-primary rounded-lg transition-colors"
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
          </svg>
          {m.about_wiki()}
        </a>
      </div>
    </div>
  {/if}
</div>

<style>
  /* Markdown changelog styling */
  .changelog-content :global(h1),
  .changelog-content :global(h2),
  .changelog-content :global(h3) {
    font-weight: 600;
    color: var(--text-primary, #fff);
    margin-top: 1em;
    margin-bottom: 0.5em;
  }
  .changelog-content :global(h1) { font-size: 1.25rem; }
  .changelog-content :global(h2) { font-size: 1.1rem; }
  .changelog-content :global(h3) { font-size: 1rem; }

  .changelog-content :global(ul),
  .changelog-content :global(ol) {
    padding-inline-start: 1.5em;
    margin: 0.5em 0;
  }
  .changelog-content :global(ul) { list-style: disc; }
  .changelog-content :global(ol) { list-style: decimal; }

  .changelog-content :global(li) {
    margin: 0.25em 0;
  }

  .changelog-content :global(a) {
    color: var(--accent-primary, #3b82f6);
    text-decoration: underline;
    text-underline-offset: 2px;
  }
  .changelog-content :global(a:hover) {
    opacity: 0.8;
  }

  .changelog-content :global(code) {
    background: var(--bg-elevated);
    padding: 0.15em 0.4em;
    border-radius: 4px;
    font-size: 0.9em;
  }

  .changelog-content :global(pre) {
    background: var(--bg-base);
    padding: 0.75em 1em;
    border-radius: 6px;
    overflow-x: auto;
    margin: 0.5em 0;
  }
  .changelog-content :global(pre code) {
    background: none;
    padding: 0;
  }

  .changelog-content :global(p) {
    margin: 0.5em 0;
  }

  .changelog-content :global(strong) {
    color: var(--text-primary, #fff);
    font-weight: 600;
  }

  .changelog-content :global(blockquote) {
    border-inline-start: 3px solid var(--border-subtle, #374151);
    padding-inline-start: 1em;
    margin: 0.5em 0;
    color: var(--text-secondary, #9ca3af);
  }
</style>

<script lang="ts">
  import { onMount } from 'svelte';
  import { marked } from 'marked';
  import type { SystemInfo, UpdateInfo } from '$lib/types';
  import { fetchSystemInfo, checkForUpdates } from '$lib/api';

  let systemInfo = $state<SystemInfo | null>(null);
  let updateInfo = $state<UpdateInfo | null>(null);
  let aboutLoading = $state(false);
  let aboutError = $state<string | null>(null);
  let updateInstructionsExpanded = $state(false);
  let changelogExpanded = $state(false);

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
      if (updInfo?.changelog) changelogExpanded = true;
      if (updInfo?.update_available) updateInstructionsExpanded = true;
    } catch (e) {
      aboutError = e instanceof Error ? e.message : 'Failed to load';
    } finally {
      aboutLoading = false;
    }
  }

  onMount(() => { loadAboutData(); });
</script>

<div class="space-y-6">
  {#if aboutLoading}
    <div class="flex items-center justify-center py-16">
      <svg class="w-6 h-6 text-brand-400 animate-spin" viewBox="0 0 24 24" fill="none">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
      </svg>
      <span class="ml-3 text-text-muted">Loading system info...</span>
    </div>
  {:else if aboutError}
    <div class="p-4 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400">
      <div class="flex items-center justify-between">
        <span class="text-sm">{aboutError}</span>
        <button
          class="px-3 py-1 text-xs bg-red-500/20 hover:bg-red-500/30 rounded text-red-300 transition-colors"
          onclick={() => { systemInfo = null; loadAboutData(); }}
        >Retry</button>
      </div>
    </div>
  {:else if systemInfo}
    <!-- Version Status -->
    <div class="rounded-xl border p-5 {updateInfo?.update_available ? 'border-amber-500/30 bg-amber-500/5' : 'border-green-500/30 bg-green-500/5'}">
      <div class="flex items-start gap-4">
        {#if updateInfo?.update_available}
          <div class="w-10 h-10 rounded-lg bg-amber-500/20 flex items-center justify-center flex-shrink-0">
            <svg class="w-5 h-5 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 10l7-7m0 0l7 7m-7-7v18" />
            </svg>
          </div>
        {:else}
          <div class="w-10 h-10 rounded-lg bg-green-500/20 flex items-center justify-center flex-shrink-0">
            <svg class="w-5 h-5 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
            </svg>
          </div>
        {/if}
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 flex-wrap">
            <h3 class="text-lg font-semibold text-text-primary">
              {updateInfo?.update_available ? 'Update Available' : "You're up to date"}
            </h3>
            {#if updateInfo?.update_available}
              <span class="px-2 py-0.5 text-xs font-medium bg-amber-500/20 text-amber-300 rounded-full">
                v{updateInfo.latest_version}
              </span>
            {/if}
          </div>
          <div class="flex flex-wrap gap-x-4 gap-y-1 mt-1.5 text-sm text-text-muted">
            <span>Current: <span class="text-text-primary font-mono">{systemInfo.version}</span></span>
            {#if updateInfo}
              <span>Latest: <span class="text-text-primary font-mono">{updateInfo.latest_version}</span></span>
              {#if updateInfo.published_at}
                <span>Released: {new Date(updateInfo.published_at).toLocaleDateString()}</span>
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
            View on GitHub
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
            </svg>
          </a>
        {/if}
      </div>
    </div>

    <!-- Release Notes (collapsible) -->
    {#if updateInfo?.changelog}
      <div class="rounded-xl border border-border overflow-hidden">
        <button
          class="w-full flex items-center justify-between p-4 text-left hover:bg-bg-surface/50 transition-colors"
          onclick={() => changelogExpanded = !changelogExpanded}
        >
          <h3 class="text-sm font-semibold text-text-primary">Release Notes</h3>
          <svg
            class="w-4 h-4 text-text-muted transition-transform {changelogExpanded ? 'rotate-180' : ''}"
            fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        {#if changelogExpanded}
          <div class="px-4 pb-4 border-t border-border">
            <div class="mt-3 text-sm text-text-secondary leading-relaxed max-h-64 overflow-y-auto changelog-content">
              <!-- eslint-disable-next-line svelte/no-at-html-tags -- changelog from GitHub release notes, sanitized by marked -->
              {@html marked.parse(updateInfo.changelog)}
            </div>
          </div>
        {/if}
      </div>
    {/if}

    <!-- How to Update (collapsible) -->
    {#if updateInfo}
      <div class="rounded-xl border border-border overflow-hidden">
        <button
          class="w-full flex items-center justify-between p-4 text-left hover:bg-bg-surface/50 transition-colors"
          onclick={() => updateInstructionsExpanded = !updateInstructionsExpanded}
        >
          <h3 class="text-sm font-semibold text-text-primary">How to Update</h3>
          <svg
            class="w-4 h-4 text-text-muted transition-transform {updateInstructionsExpanded ? 'rotate-180' : ''}"
            fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        {#if updateInstructionsExpanded}
          <div class="border-t border-border divide-y divide-gray-700/50">
            <!-- Docker -->
            <div class="p-4">
              <div class="flex items-center gap-2 mb-2">
                <svg class="w-5 h-5 text-blue-400" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M13.983 11.078h2.119a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.119a.186.186 0 00-.185.186v1.887c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 00.186-.186V3.574a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.888c0 .102.082.185.185.186m0 2.716h2.118a.187.187 0 00.186-.186V6.29a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.887c0 .102.082.186.185.186m-2.93 0h2.12a.186.186 0 00.184-.186V6.29a.185.185 0 00-.185-.185H8.1a.186.186 0 00-.185.185v1.887c0 .102.083.186.185.186m-2.964 0h2.119a.186.186 0 00.185-.186V6.29a.186.186 0 00-.185-.185H5.136a.186.186 0 00-.186.185v1.887c0 .102.084.186.186.186m5.893 2.715h2.118a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.118a.185.185 0 00-.185.186v1.887c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.186v1.887c0 .102.083.185.185.185m-2.964 0h2.119a.186.186 0 00.185-.185V9.006a.186.186 0 00-.185-.186H5.136a.186.186 0 00-.186.186v1.887c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.186v1.887c0 .102.082.185.185.185M23.763 9.89c-.065-.051-.672-.51-1.954-.51-.338.001-.676.03-1.01.087-.248-1.7-1.653-2.53-1.716-2.566l-.344-.199-.226.327c-.284.438-.49.922-.612 1.43-.23.97-.09 1.882.403 2.661-.595.332-1.55.413-1.744.42H.751a.751.751 0 00-.75.748 11.687 11.687 0 00.692 4.062c.545 1.428 1.355 2.48 2.41 3.124 1.18.723 3.1 1.137 5.275 1.137.983.003 1.963-.086 2.93-.266a12.248 12.248 0 003.823-1.389c.98-.567 1.86-1.288 2.61-2.136 1.252-1.418 1.998-2.997 2.553-4.4h.221c1.372 0 2.215-.549 2.68-1.009.309-.293.55-.65.707-1.046l.098-.288z"/>
                </svg>
                <span class="text-sm font-medium text-text-primary">Docker</span>
                {#if systemInfo.environment === 'docker'}
                  <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-brand-500/20 text-brand-300 rounded uppercase tracking-wider">Your Platform</span>
                {/if}
              </div>
              <pre class="text-xs text-text-secondary bg-bg-base/50 rounded-lg p-3 overflow-x-auto font-mono">cd /path/to/muximux
docker compose pull
docker compose up -d</pre>
            </div>

            <!-- Linux -->
            <div class="p-4">
              <div class="flex items-center gap-2 mb-2">
                <svg class="w-5 h-5 text-yellow-400" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12.504 0c-.155 0-.315.008-.48.021-4.226.333-3.105 4.807-3.17 6.298-.076 1.092-.3 1.953-1.05 3.02-.885 1.051-2.127 2.75-2.716 4.521-.278.832-.41 1.684-.287 2.489a.424.424 0 00-.11.135c-.26.268-.45.6-.663.839-.199.199-.485.267-.797.4-.313.136-.658.269-.864.68-.09.189-.136.394-.132.602 0 .199.027.4.055.536.058.399.116.728.04.97-.249.68-.28 1.145-.106 1.484.174.334.535.47.94.601.81.2 1.91.135 2.774.6.926.466 1.866.67 2.616.47.526-.116.97-.464 1.208-.946.587-.003 1.23-.269 2.26-.334.699-.058 1.574.267 2.577.2.025.134.063.198.114.333l.003.003c.391.778 1.113 1.368 1.884 1.43.199.023.4-.002.64-.078.66-.27.735-.95.791-1.573.042-.468-.017-1.006.017-1.57.265-.112.49-.292.662-.545.272-.352.287-.803.163-1.202-.124-.398-.37-.724-.593-.975-.363-.4-.551-.486-.64-.608-.082-.125-.06-.312-.001-.524.104-.34.349-.608.606-.87.263-.268.545-.565.639-1.014.018-.013.033-.027.05-.04.28-.27.434-.556.469-.96.002-.395-.147-.742-.344-1.075-.2-.34-.432-.588-.595-.85-.115-.2-.131-.529.053-.779.223-.267.333-.485.3-.792-.03-.29-.201-.571-.424-.739-.322-.208-.583-.183-.757-.263-.168-.074-.277-.24-.432-.57-.097-.198-.237-.537-.427-.669-.19-.13-.45-.065-.585.002-.162.074-.27.068-.352.036-.05-.025-.088-.065-.074-.156.15-.4.24-.86.205-1.345-.046-.672-.202-1.349-.392-1.972-.19-.623-.428-1.206-.628-1.67-.36-.873-.663-1.432-.663-1.432z"/>
                </svg>
                <span class="text-sm font-medium text-text-primary">Linux</span>
                {#if systemInfo.environment === 'native' && systemInfo.os === 'linux'}
                  <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-brand-500/20 text-brand-300 rounded uppercase tracking-wider">Your Platform</span>
                {/if}
              </div>
              <pre class="text-xs text-text-secondary bg-bg-base/50 rounded-lg p-3 overflow-x-auto font-mono"># Stop the running instance, then:
curl -LO https://github.com/mescon/Muximux/releases/latest/download/muximux-linux-amd64
chmod +x muximux-linux-amd64
./muximux-linux-amd64</pre>
              {#if updateInfo.download_urls?.linux_amd64}
                <div class="flex gap-2 mt-2">
                  <a href={updateInfo.download_urls.linux_amd64} class="text-xs text-brand-400 hover:text-brand-300 transition-colors">Download linux-amd64</a>
                  {#if updateInfo.download_urls?.linux_arm64}
                    <span class="text-text-disabled">|</span>
                    <a href={updateInfo.download_urls.linux_arm64} class="text-xs text-brand-400 hover:text-brand-300 transition-colors">Download linux-arm64</a>
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
                <span class="text-sm font-medium text-text-primary">macOS</span>
                {#if systemInfo.environment === 'native' && systemInfo.os === 'darwin'}
                  <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-brand-500/20 text-brand-300 rounded uppercase tracking-wider">Your Platform</span>
                {/if}
              </div>
              <pre class="text-xs text-text-secondary bg-bg-base/50 rounded-lg p-3 overflow-x-auto font-mono">curl -LO https://github.com/mescon/Muximux/releases/latest/download/muximux-darwin-arm64
chmod +x muximux-darwin-arm64
./muximux-darwin-arm64</pre>
              {#if updateInfo.download_urls?.darwin_arm64}
                <div class="flex gap-2 mt-2">
                  <a href={updateInfo.download_urls.darwin_arm64} class="text-xs text-brand-400 hover:text-brand-300 transition-colors">Download darwin-arm64</a>
                  {#if updateInfo.download_urls?.darwin_amd64}
                    <span class="text-text-disabled">|</span>
                    <a href={updateInfo.download_urls.darwin_amd64} class="text-xs text-brand-400 hover:text-brand-300 transition-colors">Download darwin-amd64</a>
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
                <span class="text-sm font-medium text-text-primary">Windows</span>
                {#if systemInfo.environment === 'native' && systemInfo.os === 'windows'}
                  <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-brand-500/20 text-brand-300 rounded uppercase tracking-wider">Your Platform</span>
                {/if}
              </div>
              <pre class="text-xs text-text-secondary bg-bg-base/50 rounded-lg p-3 overflow-x-auto font-mono"># Download muximux-windows-amd64.exe from the release page
# Replace the existing executable
# Restart</pre>
              {#if updateInfo.download_urls?.windows_amd64}
                <div class="mt-2">
                  <a href={updateInfo.download_urls.windows_amd64} class="text-xs text-brand-400 hover:text-brand-300 transition-colors">Download windows-amd64</a>
                </div>
              {/if}
            </div>
          </div>
        {/if}
      </div>
    {/if}

    <!-- System Information -->
    <div>
      <h3 class="text-sm font-semibold text-text-primary mb-3">System Information</h3>
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
          <div class="text-xs text-text-disabled mb-0.5">Environment</div>
          <div class="text-sm text-text-primary capitalize">{systemInfo.environment}</div>
        </div>
        <div class="rounded-lg bg-bg-surface border border-border p-3 text-center">
          <div class="flex items-center justify-center gap-1.5 mb-1">
            <svg class="w-4 h-4 text-text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
            </svg>
          </div>
          <div class="text-xs text-text-disabled mb-0.5">Platform</div>
          <div class="text-sm text-text-primary">{systemInfo.os}/{systemInfo.arch}</div>
        </div>
        <div class="rounded-lg bg-bg-surface border border-border p-3 text-center">
          <div class="flex items-center justify-center gap-1.5 mb-1">
            <svg class="w-4 h-4 text-text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10" /><path d="M12 6v6l4 2" />
            </svg>
          </div>
          <div class="text-xs text-text-disabled mb-0.5">Uptime</div>
          <div class="text-sm text-text-primary">{systemInfo.uptime}</div>
        </div>
      </div>

      <div class="rounded-lg bg-bg-surface border border-border divide-y divide-gray-700/50">
        <div class="flex items-center justify-between px-4 py-2.5">
          <span class="text-xs text-text-disabled">Data Directory</span>
          <span class="text-xs text-text-secondary font-mono">{systemInfo.data_dir}</span>
        </div>
        <div class="flex items-center justify-between px-4 py-2.5">
          <span class="text-xs text-text-disabled">Go Version</span>
          <span class="text-xs text-text-secondary font-mono">{systemInfo.go_version}</span>
        </div>
        <div class="flex items-center justify-between px-4 py-2.5">
          <span class="text-xs text-text-disabled">Build Date</span>
          <span class="text-xs text-text-secondary font-mono">{systemInfo.build_date}</span>
        </div>
        <div class="flex items-center justify-between px-4 py-2.5">
          <span class="text-xs text-text-disabled">Commit</span>
          <span class="text-xs text-text-secondary font-mono">{systemInfo.commit.length > 8 ? systemInfo.commit.slice(0, 8) : systemInfo.commit}</span>
        </div>
      </div>
    </div>

    <!-- Links -->
    <div>
      <h3 class="text-sm font-semibold text-text-primary mb-3">Links</h3>
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
          GitHub
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
          Issues
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
          Releases
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
          Wiki
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
    padding-left: 1.5em;
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
    background: rgba(255,255,255,0.1);
    padding: 0.15em 0.4em;
    border-radius: 4px;
    font-size: 0.9em;
  }

  .changelog-content :global(pre) {
    background: rgba(0,0,0,0.3);
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
    border-left: 3px solid var(--border-subtle, #374151);
    padding-left: 1em;
    margin: 0.5em 0;
    color: var(--text-secondary, #9ca3af);
  }
</style>

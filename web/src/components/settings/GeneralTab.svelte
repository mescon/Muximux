<script lang="ts">
  import type { App, Config } from '$lib/types';
  import AppIcon from '../AppIcon.svelte';
  import MuximuxLogo from '../MuximuxLogo.svelte';
  import LocaleSelect from '../LocaleSelect.svelte';
  import * as m from '$lib/paraglide/messages.js';

  let {
    localConfig = $bindable(),
    localApps = $bindable(),
    onexport,
    onimportselect,
    onopenhomeicon,
  }: {
    localConfig: Config;
    localApps: App[];
    onexport: () => void;
    onimportselect: (e: Event) => void;
    onopenhomeicon?: () => void;
  } = $props();

  let importFileInput = $state<HTMLInputElement | undefined>(undefined);

  const navPositions = [
    { value: 'top' as const, get label() { return m.general_navPositionTop(); }, get description() { return m.general_navPositionTopDesc(); } },
    { value: 'left' as const, get label() { return m.general_navPositionLeft(); }, get description() { return m.general_navPositionLeftDesc(); } },
    { value: 'right' as const, get label() { return m.general_navPositionRight(); }, get description() { return m.general_navPositionRightDesc(); } },
    { value: 'bottom' as const, get label() { return m.general_navPositionBottom(); }, get description() { return m.general_navPositionBottomDesc(); } },
    { value: 'floating' as const, get label() { return m.general_navPositionFloating(); }, get description() { return m.general_navPositionFloatingDesc(); } }
  ];
</script>

<div class="space-y-6">
  <!-- Dashboard Title -->
  <div>
    <label for="title" class="block text-sm font-medium text-text-secondary mb-2">
      {m.general_dashboardTitle()}
    </label>
    <input
      id="title"
      type="text"
      bind:value={localConfig.title}
      class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary
             focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
      placeholder="Muximux"
    />
    <p class="text-xs text-text-disabled mt-1.5">
      {m.general_titleHint()}
    </p>
  </div>

  <!-- Language -->
  <div>
    <label for="language" class="block text-sm font-medium text-text-secondary mb-2">
      {m.general_language()}
    </label>
    <LocaleSelect id="language" bind:value={localConfig.language} />
    <p class="text-xs text-text-disabled mt-1.5">
      {m.general_languageAfterSave()}
    </p>
  </div>

  <!-- Navigation Position -->
  <div>
    <span class="block text-sm font-medium text-text-secondary mb-2">
      {m.general_navPosition()}
    </span>
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
      {#each navPositions as pos (pos.value)}
        <button
          class="p-3 rounded-lg border text-start transition-colors
                 {localConfig.navigation.position === pos.value
                   ? 'border-brand-500 bg-brand-500/10 text-text-primary'
                   : 'border-border-subtle hover:border-border-strong text-text-secondary'}"
          onclick={() => localConfig.navigation.position = pos.value}
        >
          <div class="font-medium">{pos.label}</div>
          <div class="text-xs text-text-muted mt-1">{pos.description}</div>
        </button>
      {/each}
    </div>
  </div>

  <!-- Bar Style (only shown when top or bottom is selected) -->
  {#if localConfig.navigation.position === 'top' || localConfig.navigation.position === 'bottom'}
    <div>
      <span class="block text-sm font-medium text-text-secondary mb-2">
        {m.general_barStyle()}
      </span>
      <div class="grid grid-cols-2 gap-3">
        {#each [
          { value: 'grouped', get label() { return m.general_barStyleGrouped(); }, get description() { return m.general_barStyleGroupedDesc(); } },
          { value: 'flat', get label() { return m.general_barStyleFlat(); }, get description() { return m.general_barStyleFlatDesc(); } }
        ] as style (style.value)}
          <button
            class="p-3 rounded-lg border text-start transition-colors
                   {(localConfig.navigation.bar_style || 'grouped') === style.value
                     ? 'border-brand-500 bg-brand-500/10 text-text-primary'
                     : 'border-border-subtle hover:border-border-strong text-text-secondary'}"
            onclick={() => localConfig.navigation.bar_style = style.value as 'grouped' | 'flat'}
          >
            <div class="font-medium text-sm">{style.label}</div>
            <div class="text-xs text-text-muted mt-1">{style.description}</div>
          </button>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Floating Position (only shown when floating is selected) -->
  {#if localConfig.navigation.position === 'floating'}
    <div>
      <span class="block text-sm font-medium text-text-secondary mb-2">
        {m.general_floatingPosition()}
      </span>
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {#each [
          { value: 'bottom-right', get label() { return m.general_floatingBottomRight(); } },
          { value: 'bottom-left', get label() { return m.general_floatingBottomLeft(); } },
          { value: 'top-right', get label() { return m.general_floatingTopRight(); } },
          { value: 'top-left', get label() { return m.general_floatingTopLeft(); } }
        ] as fp (fp.value)}
          <button
            class="p-2 rounded-lg border text-center text-sm transition-colors
                   {(localConfig.navigation.floating_position || 'bottom-right') === fp.value
                     ? 'border-brand-500 bg-brand-500/10 text-text-primary'
                     : 'border-border-subtle hover:border-border-strong text-text-secondary'}"
            onclick={() => localConfig.navigation.floating_position = fp.value as 'bottom-right' | 'bottom-left' | 'top-right' | 'top-left'}
          >
            {fp.label}
          </button>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Navigation Options -->
  <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
    <label class="flex items-center gap-3 p-3 bg-bg-hover rounded-lg cursor-pointer">
      <input
        type="checkbox"
        bind:checked={localConfig.navigation.show_labels}
        class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
      />
      <div>
        <div class="text-sm text-text-primary">{m.general_showLabels()}</div>
        <div class="text-xs text-text-muted">{m.general_showLabelsDesc()}</div>
      </div>
    </label>

    <div class="p-3 bg-bg-hover rounded-lg sm:col-span-2">
      <div class="text-sm text-text-primary mb-2">{m.general_overviewButton()}</div>
      <div class="text-xs text-text-muted mb-3">{m.general_overviewButtonDesc()}</div>
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
        <!-- Hidden -->
        <button
          class="flex flex-col items-center gap-1.5 p-3 rounded-lg border-2 transition-colors {!localConfig.navigation.show_home_button ? 'border-brand-500 bg-brand-500/10' : 'border-transparent bg-bg-surface hover:bg-bg-hover'}"
          onclick={() => { localConfig.navigation.show_home_button = false; localConfig.navigation.show_splash_on_startup = false; }}
        >
          <svg class="w-5 h-5 text-text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" />
          </svg>
          <span class="text-xs text-text-muted">{m.general_overviewHidden()}</span>
        </button>
        <!-- Muximux Logo -->
        <button
          class="flex flex-col items-center gap-1.5 p-3 rounded-lg border-2 transition-colors {localConfig.navigation.show_home_button !== false && localConfig.navigation.show_logo && !localConfig.navigation.home_icon?.name ? 'border-brand-500 bg-brand-500/10' : 'border-transparent bg-bg-surface hover:bg-bg-hover'}"
          onclick={() => { localConfig.navigation.show_home_button = true; localConfig.navigation.show_logo = true; localConfig.navigation.home_icon = undefined; }}
        >
          <MuximuxLogo height="20" />
          <span class="text-xs text-text-muted">{m.general_overviewLogo()}</span>
        </button>
        <!-- House Icon -->
        <button
          class="flex flex-col items-center gap-1.5 p-3 rounded-lg border-2 transition-colors {localConfig.navigation.show_home_button !== false && !localConfig.navigation.show_logo && !localConfig.navigation.home_icon?.name ? 'border-brand-500 bg-brand-500/10' : 'border-transparent bg-bg-surface hover:bg-bg-hover'}"
          onclick={() => { localConfig.navigation.show_home_button = true; localConfig.navigation.show_logo = false; localConfig.navigation.home_icon = undefined; }}
        >
          <svg class="w-5 h-5" style="color: var(--accent-primary);" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1h-2z" />
          </svg>
          <span class="text-xs text-text-muted">{m.general_overviewHouse()}</span>
        </button>
        <!-- Custom Icon -->
        <button
          class="flex flex-col items-center gap-1.5 p-3 rounded-lg border-2 transition-colors {localConfig.navigation.show_home_button !== false && localConfig.navigation.home_icon?.name ? 'border-brand-500 bg-brand-500/10' : 'border-transparent bg-bg-surface hover:bg-bg-hover'}"
          onclick={() => { localConfig.navigation.show_home_button = true; onopenhomeicon?.(); }}
        >
          {#if localConfig.navigation.home_icon?.name}
            <AppIcon icon={localConfig.navigation.home_icon} name={localConfig.title} color="" size="sm" showBackground={false} />
          {:else}
            <svg class="w-5 h-5 text-text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
            </svg>
          {/if}
          <span class="text-xs text-text-muted">{m.general_overviewCustom()}</span>
        </button>
      </div>
    </div>

    <label class="flex items-center gap-3 p-3 bg-bg-hover rounded-lg cursor-pointer">
      <input
        type="checkbox"
        bind:checked={localConfig.navigation.show_app_colors}
        class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
      />
      <div>
        <div class="text-sm text-text-primary">{m.general_appColorAccents()}</div>
        <div class="text-xs text-text-muted">{m.general_appColorAccentsDesc()}</div>
      </div>
    </label>

    <label class="flex items-center gap-3 p-3 bg-bg-hover rounded-lg cursor-pointer">
      <input
        type="checkbox"
        bind:checked={localConfig.navigation.show_icon_background}
        class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
      />
      <div>
        <div class="text-sm text-text-primary">{m.general_iconBackground()}</div>
        <div class="text-xs text-text-muted">{m.general_iconBackgroundDesc()}</div>
      </div>
    </label>

    <div class="p-3 bg-bg-hover rounded-lg sm:col-span-2">
      <div class="flex items-center justify-between mb-2">
        <div>
          <div class="text-sm text-text-primary">{m.general_iconSize()}</div>
          <div class="text-xs text-text-muted">{m.general_iconSizeDesc()}</div>
        </div>
        <span class="text-sm text-text-secondary tabular-nums">{localConfig.navigation.icon_scale}×</span>
      </div>
      <input type="range" min="0.5" max="2" step="0.25"
        bind:value={localConfig.navigation.icon_scale}
        class="w-full" />
    </div>

    <div class="p-3 bg-bg-hover rounded-lg" class:opacity-50={localConfig.navigation.show_home_button === false}>
      <label class="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          checked={localConfig.navigation.show_splash_on_startup}
          disabled={localConfig.navigation.show_home_button === false}
          onchange={(e) => {
            localConfig.navigation.show_splash_on_startup = (e.currentTarget as HTMLInputElement).checked;
            if (localConfig.navigation.show_splash_on_startup) {
              localApps.forEach(a => a.default = false);
            }
          }}
          class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
        />
        <div>
          <div class="text-sm text-text-primary">{m.general_startOnOverview()}</div>
          <div class="text-xs text-text-muted">{m.general_startOnOverviewDesc()}</div>
        </div>
      </label>
      {#if localApps.find(a => a.default) && !localConfig.navigation.show_splash_on_startup}
        <p class="text-xs text-brand-400 mt-1 ps-7">
          {m.general_defaultAppDisabled({ appName: localApps.find(a => a.default)?.name || '' })}
        </p>
      {/if}
    </div>

    <div class="p-3 bg-bg-hover rounded-lg sm:col-span-2">
      <label class="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          bind:checked={localConfig.navigation.auto_hide}
          class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
        />
        <div class="flex-1">
          <div class="text-sm text-text-primary">{m.general_autoHideMenu()}</div>
          <div class="text-xs text-text-muted">{m.general_autoHideMenuDesc()}</div>
        </div>
      </label>
      {#if localConfig.navigation.auto_hide}
        <div class="flex items-center gap-3 mt-3 pt-3 border-t border-border-subtle">
          <div class="flex-1 text-xs text-text-muted ps-7">{m.general_hideAfter()}</div>
          <select
            bind:value={localConfig.navigation.auto_hide_delay}
            class="px-2 py-1 text-xs bg-bg-overlay border border-border-strong rounded text-text-primary focus:ring-brand-500 focus:border-brand-500"
          >
            <option value="0.25s">0.25s</option>
            <option value="0.5s">0.5s</option>
            <option value="1s">1s</option>
            <option value="2s">2s</option>
            <option value="3s">3s</option>
          </select>
        </div>
        <label class="flex items-center gap-3 mt-2 ps-7 cursor-pointer">
          <input
            type="checkbox"
            bind:checked={localConfig.navigation.show_shadow}
            class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
          />
          <div class="text-xs text-text-muted">{m.general_shadow()}</div>
        </label>
      {/if}
    </div>

    {#if localConfig.navigation.position === 'left' || localConfig.navigation.position === 'right' || localConfig.navigation.position === 'top' || localConfig.navigation.position === 'bottom'}
      <div class="p-3 bg-bg-hover rounded-lg sm:col-span-2">
        <label class="flex items-center gap-3 cursor-pointer">
          <input
            type="checkbox"
            bind:checked={localConfig.navigation.hide_sidebar_footer}
            class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
          />
          <div class="flex-1">
            {#if localConfig.navigation.position === 'top' || localConfig.navigation.position === 'bottom'}
              <div class="text-sm text-text-primary">{m.general_collapsibleToolbar()}</div>
              <div class="text-xs text-text-muted">{m.general_collapsibleToolbarDesc()}</div>
            {:else}
              <div class="text-sm text-text-primary">{m.general_collapsibleFooter()}</div>
              <div class="text-xs text-text-muted">{m.general_collapsibleFooterDesc()}</div>
            {/if}
          </div>
        </label>
      </div>
    {/if}

    <div class="mt-4 pt-4" style="border-top: 1px solid var(--border-subtle);">
      <label class="flex items-center gap-3">
        <div class="flex-1">
          <div class="text-sm font-medium" style="color: var(--text-primary);">{m.general_maxOpenTabs()}</div>
          <div class="text-xs" style="color: var(--text-muted);">{m.general_maxOpenTabsDesc()}</div>
        </div>
        <input
          type="number"
          min="0"
          max="50"
          value={localConfig.navigation.max_open_tabs}
          onchange={(e) => {
            localConfig.navigation.max_open_tabs = parseInt((e.currentTarget as HTMLInputElement).value) || 0;
          }}
          class="w-20 px-2 py-1 text-sm rounded-lg text-center"
          style="background: var(--bg-surface); color: var(--text-primary); border: 1px solid var(--border-default);"
        />
      </label>
    </div>
  </div>

  <!-- Health Monitoring bulk actions -->
  <div class="pt-4 border-t border-border">
    <div class="flex items-center justify-between mb-1">
      <h3 class="text-sm font-medium text-text-secondary">{m.general_healthChecks()}</h3>
      <div class="flex gap-2">
        <button
          class="text-xs px-2 py-1 rounded text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors"
          onclick={() => localApps.forEach(a => a.health_check = true)}
        >{m.general_enableAll()}</button>
        <button
          class="text-xs px-2 py-1 rounded text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors"
          onclick={() => localApps.forEach(a => a.health_check = undefined)}
        >{m.general_disableAll()}</button>
      </div>
    </div>
    <p class="text-xs text-text-disabled">{m.general_healthCheckHint()}</p>
  </div>

  <!-- Advanced -->
  <div class="pt-4 border-t border-border">
    <h3 class="text-sm font-medium text-text-secondary mb-3">{m.general_advanced()}</h3>

    <div class="flex items-center gap-3 mb-4">
      <label for="log-level" class="text-sm text-text-muted whitespace-nowrap">{m.general_logLevel()}</label>
      <select
        id="log-level"
        bind:value={localConfig.log_level}
        class="px-2 py-1 text-sm bg-bg-elevated border border-border-subtle rounded-md text-text-primary
               focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
      >
        <option value="debug">{m.general_logDebug()}</option>
        <option value="info">{m.general_logInfo()}</option>
        <option value="warn">{m.general_logWarning()}</option>
        <option value="error">{m.general_logError()}</option>
      </select>
      <span class="text-xs text-text-disabled">{m.general_logLevelHint()}</span>
    </div>

    <div class="flex items-center gap-3 mb-4">
      <label for="proxy-timeout" class="text-sm text-text-muted whitespace-nowrap">{m.general_proxyTimeout()}</label>
      <input
        id="proxy-timeout"
        type="text"
        bind:value={localConfig.proxy_timeout}
        placeholder="30s"
        class="w-20 px-2 py-1 text-sm bg-bg-elevated border border-border-subtle rounded-md text-text-primary
               focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
      />
      <span class="text-xs text-text-disabled">{m.general_proxyTimeoutHint()}</span>
    </div>

    <div class="flex flex-wrap gap-3">
      <button
        class="btn btn-secondary btn-sm flex items-center gap-2"
        onclick={onexport}
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
        </svg>
        {m.general_exportConfig()}
      </button>
      <button
        class="btn btn-secondary btn-sm flex items-center gap-2"
        onclick={() => importFileInput?.click()}
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
        </svg>
        {m.general_importConfig()}
      </button>
      <input
        bind:this={importFileInput}
        type="file"
        accept=".yaml,.yml"
        class="hidden"
        onchange={onimportselect}
      />
    </div>
    <p class="text-xs text-text-disabled mt-2">
      {m.general_configHint()}
    </p>
  </div>
</div>

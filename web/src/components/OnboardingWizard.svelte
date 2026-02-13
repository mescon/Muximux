<script lang="ts">
  import { onMount } from 'svelte';
  import { SvelteMap, SvelteSet } from 'svelte/reactivity';
  import { get } from 'svelte/store';
  import { fly, fade } from 'svelte/transition';
  import { flip } from 'svelte/animate';
  import { dndzone, type DndEvent } from 'svelte-dnd-action';
  import type { App, AppIcon as AppIconConfig, Group, NavigationConfig, ThemeConfig, SetupRequest } from '$lib/types';
  import {
    currentStep,
    selectedApps,
    selectedNavigation,
    showLabels,
    nextStep,
    prevStep,
    stepProgress,
    configureSteps,
    activeStepOrder,
    type OnboardingStep
  } from '$lib/onboardingStore';
  import { submitSetup } from '$lib/api';
  import { popularApps, getAllGroups, templateToApp } from '$lib/popularApps';
  import type { PopularAppTemplate } from '$lib/popularApps';
  import AppIcon from './AppIcon.svelte';
  import IconBrowser from './IconBrowser.svelte';
  import {
    themeFamilies,
    selectedFamily,
    variantMode,
    setThemeFamily,
    setVariantMode,
    detectCustomThemes,
    type VariantMode
  } from '$lib/themeStore';

  // Props
  let {
    oncomplete,
    needsSetup = false,
    onsetupcomplete
  }: {
    oncomplete?: (detail: { apps: App[]; navigation: NavigationConfig; groups: Group[]; theme: ThemeConfig }) => void;
    needsSetup?: boolean;
    onsetupcomplete?: () => void;
  } = $props();

  // Track which apps are selected with their URLs
  let appSelections = new SvelteMap<string, { selected: boolean; url: string }>();

  // Custom app form
  let showCustomApp = $state(false);
  let showAdvanced = $state(false);
  let customApp = $state({
    name: '',
    url: '',
    icon: { type: 'dashboard', name: '', file: '', url: '', variant: '' } as AppIconConfig,
    color: '#22c55e',
    group: '',
    open_mode: 'iframe' as App['open_mode'],
    proxy: false,
    health_url: '',
    scale: 1
  });

  // Per-app customization overrides (color, icon)
  let appOverrides = new SvelteMap<string, { color: string; icon: AppIconConfig }>();

  // Groups editing state
  let wizardGroups = $state<Group[]>([]);
  let iconBrowserContext = $state<'custom-app' | 'app-override' | number | null>(null);
  let iconBrowserAppName = $state<string>('');
  let groupsInitialized = $state(false);

  const flipDurationMs = 200;

  let dndUnsorted = $state<{id: string; name: string}[]>([]);
  let dndGroupApps = $state<Record<string, {id: string; name: string}[]>>({});

  // Security step state
  let authMethod = $state<'builtin' | 'forward_auth' | 'none' | null>(null);
  let setupUsername = $state('admin');
  let setupPassword = $state('');
  let setupConfirmPassword = $state('');
  let setupLoading = $state(false);
  let setupError = $state<string | null>(null);
  let setupDone = $state(false);

  // Forward auth fields
  let faPreset = $state<'authelia' | 'authentik' | 'custom'>('authelia');
  let faTrustedProxies = $state('');
  let faShowAdvanced = $state(false);
  let faHeaderUser = $state('Remote-User');
  let faHeaderEmail = $state('Remote-Email');
  let faHeaderGroups = $state('Remote-Groups');
  let faHeaderName = $state('Remote-Name');

  // None
  let acknowledgeRisk = $state(false);

  // Preset configs
  const faPresets = {
    authelia: { user: 'Remote-User', email: 'Remote-Email', groups: 'Remote-Groups', name: 'Remote-Name' },
    authentik: { user: 'X-authentik-username', email: 'X-authentik-email', groups: 'X-authentik-groups', name: 'X-authentik-name' },
    custom: { user: 'Remote-User', email: 'Remote-Email', groups: 'Remote-Groups', name: 'Remote-Name' },
  };

  function selectFaPreset(p: 'authelia' | 'authentik' | 'custom') {
    faPreset = p;
    const headers = faPresets[p];
    faHeaderUser = headers.user;
    faHeaderEmail = headers.email;
    faHeaderGroups = headers.groups;
    faHeaderName = headers.name;
  }

  // Validation
  let builtinValid = $derived(
    setupUsername.trim().length > 0 &&
    setupPassword.length >= 8 &&
    setupPassword === setupConfirmPassword
  );
  let forwardAuthValid = $derived(faTrustedProxies.trim().length > 0);
  let noneValid = $derived(acknowledgeRisk);
  let securityStepValid = $derived(
    authMethod === 'builtin' ? builtinValid :
    authMethod === 'forward_auth' ? forwardAuthValid :
    authMethod === 'none' ? noneValid :
    false
  );

  async function handleSecuritySubmit() {
    if (!authMethod || !securityStepValid) return;
    setupLoading = true;
    setupError = null;

    const req: SetupRequest = { method: authMethod };

    if (authMethod === 'builtin') {
      req.username = setupUsername.trim();
      req.password = setupPassword;
    } else if (authMethod === 'forward_auth') {
      req.trusted_proxies = faTrustedProxies
        .split(/[,\n]/)
        .map(s => s.trim())
        .filter(s => s.length > 0);
      req.headers = {
        user: faHeaderUser,
        email: faHeaderEmail,
        groups: faHeaderGroups,
        name: faHeaderName,
      };
    }

    try {
      const resp = await submitSetup(req);
      if (resp.success) {
        setupDone = true;
        onsetupcomplete?.();
        nextStep();
      } else {
        setupError = resp.error || 'Setup failed';
      }
    } catch (e) {
      setupError = e instanceof Error ? e.message : 'Setup failed';
    } finally {
      setupLoading = false;
    }
  }

  // Initialize app selections and load custom themes
  onMount(() => {
    configureSteps(needsSetup);

    Object.values(popularApps).flat().forEach(app => {
      appSelections.set(app.name, { selected: false, url: app.defaultUrl });
    });

    // Load custom themes for the theme picker
    detectCustomThemes();
  });

  // Toggle app selection
  function toggleApp(app: PopularAppTemplate) {
    const current = appSelections.get(app.name);
    if (current) {
      const nowSelected = !current.selected;
      appSelections.set(app.name, { ...current, selected: nowSelected });
      if (!nowSelected) {
        dndUnsorted = dndUnsorted.filter(item => item.name !== app.name);
        for (const groupName of Object.keys(dndGroupApps)) {
          dndGroupApps[groupName] = dndGroupApps[groupName].filter(item => item.name !== app.name);
        }
      }
      rebuildDndFromSelections();
    }
  }

  // Update app URL
  function updateAppUrl(appName: string, url: string) {
    const current = appSelections.get(appName);
    if (current) {
      appSelections.set(appName, { ...current, url });
    }
  }

  // Get selected apps count
  const selectedCount = $derived([...appSelections.values()].filter(a => a.selected).length);

  // Get suggested groups based on selected apps
  const suggestedGroups = $derived.by(() => {
    const groupsWithApps = new SvelteSet<string>();
    appSelections.forEach((value, key) => {
      if (value.selected) {
        const template = Object.values(popularApps).flat().find(a => a.name === key);
        if (template) {
          groupsWithApps.add(template.group);
        }
      }
    });
    return getAllGroups().filter(g => groupsWithApps.has(g));
  });

  // Auto-populate wizardGroups from suggestedGroups on first entry
  $effect(() => {
    if (!groupsInitialized && suggestedGroups.length > 0) {
      wizardGroups = suggestedGroups.map((name, i) => ({
        name,
        icon: { type: 'lucide' as const, name: '', file: '', url: '', variant: '' },
        color: getGroupColor(name),
        order: i,
        expanded: true
      }));
      groupsInitialized = true;
      rebuildDndFromSelections();
    }
  });

  // Navigation position options
  const navPositions: { value: NavigationConfig['position']; label: string; description: string; icon: string }[] = [
    { value: 'top', label: 'Top Bar', description: 'Horizontal navigation at the top', icon: 'top' },
    { value: 'left', label: 'Left Sidebar', description: 'Vertical sidebar on the left', icon: 'left' },
    { value: 'right', label: 'Right Sidebar', description: 'Vertical sidebar on the right', icon: 'right' },
    { value: 'bottom', label: 'Bottom Dock', description: 'macOS-style dock at the bottom', icon: 'bottom' },
    { value: 'floating', label: 'Floating', description: 'Minimal floating buttons', icon: 'floating' }
  ];

  // Mock nav items for the live preview (uses first few selected apps or defaults)
  const mockNavItems = $derived.by(() => {
    const items: { name: string; color: string }[] = [];
    appSelections.forEach((value, key) => {
      if (value.selected && items.length < 5) {
        const template = Object.values(popularApps).flat().find(a => a.name === key);
        if (template) items.push({ name: template.name, color: template.color });
      }
    });
    // Fallback if no apps selected
    if (items.length === 0) {
      items.push(
        { name: 'Plex', color: '#E5A00D' },
        { name: 'Sonarr', color: '#00CCFF' },
        { name: 'Portainer', color: '#13BEF9' },
        { name: 'Grafana', color: '#F46800' }
      );
    }
    return items;
  });

  // Variant options
  const variantOptions: { value: VariantMode; label: string }[] = [
    { value: 'dark', label: 'Dark' },
    { value: 'system', label: 'System' },
    { value: 'light', label: 'Light' }
  ];

  // Add custom app
  function addCustomApp() {
    if (!customApp.name || !customApp.url) return;

    const newApp: App = {
      name: customApp.name,
      url: customApp.url,
      icon: { ...customApp.icon },
      color: customApp.color,
      group: customApp.group,
      order: selectedCount,
      enabled: true,
      default: false,
      open_mode: customApp.open_mode,
      proxy: customApp.proxy,
      scale: customApp.scale,
      disable_keyboard_shortcuts: false,
      ...(customApp.health_url ? { health_url: customApp.health_url } : {})
    };

    selectedApps.update(apps => [...apps, newApp]);

    // Auto-create group if it doesn't exist
    if (customApp.group && !wizardGroups.some(g => g.name === customApp.group)) {
      wizardGroups = [...wizardGroups, {
        name: customApp.group,
        icon: { type: 'lucide' as const, name: '', file: '', url: '', variant: '' },
        color: getGroupColor(customApp.group) || '#22c55e',
        order: wizardGroups.length,
        expanded: true
      }];
    }

    customApp = { name: '', url: '', icon: { type: 'dashboard', name: '', file: '', url: '', variant: '' } as AppIconConfig, color: '#22c55e', group: '', open_mode: 'iframe', proxy: false, health_url: '', scale: 1 };
    showAdvanced = false;
    showCustomApp = false;
    rebuildDndFromSelections();
  }

  // Group editing functions
  function updateGroupName(index: number, name: string) {
    const oldName = wizardGroups[index].name;
    if (oldName !== name && dndGroupApps[oldName]) {
      dndGroupApps[name] = dndGroupApps[oldName];
      delete dndGroupApps[oldName];
    }
    wizardGroups = wizardGroups.map((g, i) => i === index ? { ...g, name } : g);
  }

  function updateGroupColor(index: number, color: string) {
    wizardGroups = wizardGroups.map((g, i) => i === index ? { ...g, color } : g);
  }

  function deleteGroup(index: number) {
    const groupName = wizardGroups[index].name;
    if (dndGroupApps[groupName]) {
      dndUnsorted = [...dndUnsorted, ...dndGroupApps[groupName]];
      delete dndGroupApps[groupName];
    }
    wizardGroups = wizardGroups.filter((_, i) => i !== index);
  }

  function addGroup() {
    wizardGroups = [...wizardGroups, {
      name: 'New Group',
      icon: { type: 'lucide' as const, name: '', file: '', url: '', variant: '' },
      color: '#22c55e',
      order: wizardGroups.length,
      expanded: true
    }];
    dndGroupApps['New Group'] = [];
  }

  function handleIconSelect(detail: { name: string; variant: string; type: string }) {
    const newIcon: AppIconConfig = { type: detail.type as AppIconConfig['type'], name: detail.name, file: '', url: '', variant: detail.variant };
    if (iconBrowserContext === 'custom-app') {
      customApp.icon = newIcon;
    } else if (iconBrowserContext === 'app-override') {
      const existing = appOverrides.get(iconBrowserAppName);
      appOverrides.set(iconBrowserAppName, {
        color: existing?.color || getAppDisplayColor(iconBrowserAppName),
        icon: newIcon
      });
    } else if (typeof iconBrowserContext === 'number') {
      wizardGroups = wizardGroups.map((g, i) =>
        i === iconBrowserContext
          ? { ...g, icon: newIcon }
          : g
      );
    }
    iconBrowserContext = null;
  }

  // Complete onboarding
  function handleComplete() {
    // Build final apps list from selections
    const apps: App[] = [];
    let order = 0;

    // Add selected popular apps
    appSelections.forEach((value, key) => {
      if (value.selected) {
        const template = Object.values(popularApps).flat().find(a => a.name === key);
        if (template) {
          apps.push(templateToApp(template, value.url, order++));
        }
      }
    });

    // Add custom apps
    get(selectedApps).forEach(app => {
      apps.push({ ...app, order: order++ });
    });

    // Apply per-app customizations (color, icon)
    for (const app of apps) {
      const override = appOverrides.get(app.name);
      if (override) {
        app.color = override.color;
        if (override.icon.name) app.icon = override.icon;
      }
    }

    // Assign groups based on DnD state
    for (const [groupName, items] of Object.entries(dndGroupApps)) {
      for (const item of items) {
        const app = apps.find(a => a.name === item.name);
        if (app) app.group = groupName;
      }
    }
    for (const item of dndUnsorted) {
      const app = apps.find(a => a.name === item.name);
      if (app) app.group = '';
    }

    // Set first app as default if any
    if (apps.length > 0) {
      apps[0].default = true;
    }

    // Build groups from wizard state
    const groups: Group[] = wizardGroups.map((g, i) => ({ ...g, order: i }));

    // Build navigation config
    const navigation: NavigationConfig = {
      position: get(selectedNavigation),
      width: '220px',
      auto_hide: false,
      auto_hide_delay: '3s',
      show_on_hover: true,
      show_labels: get(showLabels),
      show_logo: true,
      show_app_colors: true,
      show_icon_background: false,
      show_splash_on_startup: true,
      show_shadow: true
    };

    // Capture current theme from stores
    const theme: ThemeConfig = {
      family: get(selectedFamily),
      variant: get(variantMode)
    };

    oncomplete?.({ apps, navigation, groups, theme });
  }

  function rebuildDndFromSelections() {
    const allSelected = new SvelteSet<string>();
    appSelections.forEach((value, key) => {
      if (value.selected) allSelected.add(key);
    });
    for (const app of get(selectedApps)) {
      allSelected.add(app.name);
    }

    const placed = new SvelteSet<string>();
    for (const items of Object.values(dndGroupApps)) {
      for (const item of items) placed.add(item.name);
    }

    const newUnsorted = dndUnsorted.filter(item => allSelected.has(item.name));
    for (const name of allSelected) {
      if (!placed.has(name) && !newUnsorted.some(item => item.name === name)) {
        newUnsorted.push({ id: name, name });
      }
    }
    dndUnsorted = newUnsorted;

    for (const groupName of Object.keys(dndGroupApps)) {
      dndGroupApps[groupName] = dndGroupApps[groupName].filter(item => allSelected.has(item.name));
    }

    for (const group of wizardGroups) {
      if (!dndGroupApps[group.name]) {
        dndGroupApps[group.name] = [];
      }
    }

    const validGroupNames = new SvelteSet(wizardGroups.map(g => g.name));
    for (const key of Object.keys(dndGroupApps)) {
      if (!validGroupNames.has(key)) {
        delete dndGroupApps[key];
      }
    }
  }

  function handleUnsortedConsider(e: CustomEvent<DndEvent<{id: string; name: string}>>) {
    dndUnsorted = e.detail.items;
  }
  function handleUnsortedFinalize(e: CustomEvent<DndEvent<{id: string; name: string}>>) {
    dndUnsorted = e.detail.items;
  }
  function handleGroupAppConsider(e: CustomEvent<DndEvent<{id: string; name: string}>>, groupName: string) {
    dndGroupApps[groupName] = e.detail.items;
  }
  function handleGroupAppFinalize(e: CustomEvent<DndEvent<{id: string; name: string}>>, groupName: string) {
    dndGroupApps[groupName] = e.detail.items;
  }

  function getGroupColor(group: string): string {
    const colors: Record<string, string> = {
      'Media': '#E5A00D',
      'Downloads': '#00CCFF',
      'System': '#F46800',
      'Utilities': '#0082C9'
    };
    return colors[group] || '#22c55e';
  }

  function getAppDisplayColor(appName: string): string {
    const override = appOverrides.get(appName);
    if (override) return override.color;
    const template = Object.values(popularApps).flat().find(a => a.name === appName);
    if (template) return template.color;
    const custom = get(selectedApps).find(a => a.name === appName);
    if (custom) return custom.color;
    return '#22c55e';
  }

  function getAppDisplayIcon(appName: string): AppIconConfig {
    const override = appOverrides.get(appName);
    if (override?.icon.name) return override.icon;
    const template = Object.values(popularApps).flat().find(a => a.name === appName);
    if (template) return { type: 'dashboard', name: template.icon, file: '', url: '', variant: 'svg' };
    const custom = get(selectedApps).find(a => a.name === appName);
    if (custom) return custom.icon;
    return { type: 'dashboard', name: '', file: '', url: '', variant: '' };
  }

  function updateAppColor(appName: string, color: string) {
    const existing = appOverrides.get(appName);
    appOverrides.set(appName, {
      color,
      icon: existing?.icon || getAppDisplayIcon(appName)
    });
  }

  function openAppIconBrowser(appName: string) {
    iconBrowserAppName = appName;
    iconBrowserContext = 'app-override';
  }

  // Step indicators — dynamic based on configured steps
  const stepLabelMap: Record<OnboardingStep, string> = {
    welcome: 'Welcome',
    security: 'Security',
    apps: 'Apps',
    navigation: 'Style',
    theme: 'Theme',
    complete: 'Done'
  };
  const steps = $derived($activeStepOrder.map(s => stepLabelMap[s]));

  function handleGlobalKeydown(e: KeyboardEvent) {
    if (e.key !== 'Enter') return;
    // Don't intercept Enter inside textareas, selects, or buttons
    const tag = (e.target as HTMLElement)?.tagName;
    if (tag === 'TEXTAREA' || tag === 'SELECT' || tag === 'BUTTON') return;
    // Don't intercept if icon browser is open
    if (iconBrowserContext !== null) return;

    const step = get(currentStep);
    if (step === 'welcome') {
      nextStep();
    } else if (step === 'security') {
      if (securityStepValid && !setupLoading && !setupDone) handleSecuritySubmit();
    } else if (step === 'apps') {
      if (selectedCount + $selectedApps.length > 0) nextStep();
    } else if (step === 'navigation' || step === 'theme') {
      nextStep();
    } else if (step === 'complete') {
      handleComplete();
    }
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<div class="fixed inset-0 z-50 bg-gray-900 overflow-hidden flex flex-col" onkeydown={handleGlobalKeydown} role="dialog" aria-label="Setup wizard" tabindex="0">
  <!-- Progress bar -->
  <div class="flex-shrink-0 px-8 pt-6">
    <div class="max-w-3xl mx-auto">
      <div class="flex items-center justify-between mb-2">
        {#each steps as _step, i (i)}
          <div class="flex items-center">
            <div
              class="w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium transition-colors duration-300
                     {i <= $stepProgress ? 'bg-brand-500 text-white' : 'bg-gray-700 text-gray-400'}"
            >
              {#if i < $stepProgress}
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
              {:else}
                {i + 1}
              {/if}
            </div>
            {#if i < steps.length - 1}
              <div class="w-12 sm:w-16 h-0.5 mx-1 {i < $stepProgress ? 'bg-brand-500' : 'bg-gray-700'}"></div>
            {/if}
          </div>
        {/each}
      </div>
      <div class="flex justify-between text-xs text-gray-500">
        {#each steps as step, i (i)}
          <span class="w-8 text-center {i <= $stepProgress ? 'text-gray-300' : ''}">{step}</span>
          {#if i < steps.length - 1}
            <span class="w-12 sm:w-16"></span>
          {/if}
        {/each}
      </div>
    </div>
  </div>

  <!-- Content area -->
  <div class="flex-1 overflow-y-auto px-8 py-6">
    <div class="max-w-4xl mx-auto">
      <!-- Step 1: Welcome -->
      {#if $currentStep === 'welcome'}
        <div class="text-center py-12" in:fly={{ x: 30, duration: 300 }} out:fade={{ duration: 150 }}>
          <!-- Logo -->
          <div class="mb-8">
            <svg class="w-48 h-auto mx-auto text-brand-500" viewBox="0 0 341 207" fill="currentColor">
              <path d="M 64.45 48.00 C 68.63 48.00 72.82 47.99 77.01 48.01 C 80.83 59.09 84.77 70.14 88.54 81.24 C 92.32 70.17 96.13 59.10 99.85 48.00 C 104.04 47.99 108.24 48.00 112.43 48.00 C 113.39 65.67 114.50 83.33 115.49 101.00 C 111.45 101.00 107.40 101.01 103.36 100.99 C 102.89 93.74 102.47 86.48 102.07 79.23 C 99.66 86.49 97.15 93.73 94.71 100.99 C 90.61 100.95 86.50 101.15 82.40 100.85 C 79.93 93.36 77.36 85.90 74.69 78.48 C 74.44 86.00 73.62 93.48 73.36 101.00 C 69.28 101.00 65.19 101.00 61.10 101.00 C 62.17 83.33 63.36 65.67 64.45 48.00 Z" />
              <path d="M 119.60 48.00 C 123.65 48.00 127.69 48.00 131.74 48.01 C 131.74 59.01 131.72 70.01 131.74 81.01 C 131.51 85.47 135.71 89.35 140.10 89.02 C 144.20 88.91 147.64 85.08 147.53 81.02 C 147.55 70.02 147.52 59.01 147.53 48.00 C 151.60 48.00 155.67 48.00 159.74 48.01 C 159.67 59.49 159.85 70.98 159.65 82.46 C 159.14 93.61 147.92 102.57 136.94 100.86 C 127.64 99.76 119.94 91.34 119.62 82.00 C 119.57 70.66 119.61 59.33 119.60 48.00 Z" />
              <path d="M 165.50 48.03 C 170.29 47.97 175.08 48.01 179.87 48.00 C 182.80 52.67 185.72 57.35 188.64 62.03 C 191.39 57.32 194.27 52.69 197.04 47.99 C 201.82 48.01 206.61 47.99 211.39 48.01 C 206.05 56.48 200.92 65.10 195.78 73.69 C 201.49 82.77 206.93 92.03 212.79 101.01 C 207.97 100.97 203.15 101.05 198.33 100.96 C 195.09 95.79 191.93 90.58 188.70 85.42 C 185.48 90.60 182.35 95.83 179.13 101.02 C 174.41 100.98 169.68 101.01 164.96 101.00 C 170.55 91.91 176.00 82.74 181.53 73.62 C 176.00 65.21 171.10 56.40 165.50 48.03 Z" />
              <path d="M 216.60 48.00 C 220.64 48.00 224.69 48.00 228.74 48.01 C 228.73 77.68 228.73 107.36 228.74 137.04 C 228.83 141.39 228.77 145.96 226.59 149.87 C 222.49 158.47 211.73 163.16 202.67 160.11 C 194.49 157.70 188.47 149.51 188.59 140.98 C 188.61 129.99 188.59 119.00 188.60 108.00 C 192.64 108.00 196.69 107.99 200.74 108.01 C 200.74 118.99 200.72 129.97 200.74 140.96 C 200.48 145.46 204.75 149.40 209.18 149.01 C 213.25 148.85 216.63 145.06 216.53 141.03 C 216.51 110.02 216.65 79.01 216.60 48.00 Z" />
              <path d="M 133.45 108.00 C 137.63 108.00 141.82 107.99 146.01 108.01 C 149.84 119.09 153.76 130.15 157.56 141.24 C 161.30 130.16 165.14 119.10 168.85 108.01 C 173.04 107.99 177.24 108.00 181.43 108.00 C 182.39 125.67 183.50 143.33 184.49 161.00 C 180.44 161.00 176.40 161.01 172.36 160.99 C 171.89 153.75 171.48 146.51 171.07 139.27 C 168.64 146.51 166.15 153.74 163.71 160.99 C 159.62 160.97 155.52 161.11 151.44 160.88 C 148.91 153.40 146.38 145.91 143.69 138.48 C 143.44 146.00 142.61 153.48 142.37 161.00 C 138.28 161.00 134.19 161.00 130.10 161.00 C 131.17 143.33 132.36 125.67 133.45 108.00 Z" />
              <path d="M 234.50 108.03 C 239.29 107.97 244.08 108.01 248.87 108.00 C 251.78 112.67 254.73 117.32 257.60 122.02 C 260.41 117.35 263.25 112.69 266.03 107.99 C 270.82 108.01 275.61 107.99 280.39 108.01 C 275.04 116.48 269.93 125.09 264.78 133.68 C 270.48 142.77 275.93 152.02 281.79 161.01 C 276.97 160.97 272.15 161.05 267.33 160.96 C 264.09 155.80 260.93 150.58 257.70 145.42 C 254.45 150.60 251.37 155.88 248.08 161.04 C 243.37 160.96 238.67 161.02 233.96 161.00 C 239.55 151.91 245.00 142.74 250.53 133.62 C 245.00 125.21 240.10 116.40 234.50 108.03 Z" />
            </svg>
          </div>

          <h1 class="text-4xl font-bold text-white mb-4">Welcome to Muximux</h1>
          <p class="text-xl text-gray-400 mb-8 max-w-2xl mx-auto">
            {needsSetup
              ? "Your unified homelab dashboard. Let's secure and set up your applications."
              : "Your unified homelab dashboard. Let's set up your applications in a few quick steps."}
          </p>

          <!-- Feature highlights -->
          <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-12 max-w-3xl mx-auto text-left">
            <div class="p-4 bg-gray-800/50 rounded-lg border border-gray-700">
              <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center mb-3">
                <svg class="w-5 h-5 text-brand-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
                </svg>
              </div>
              <h3 class="font-semibold text-white mb-1">Embedded Apps</h3>
              <p class="text-sm text-gray-400">View all your services in iframes without leaving the dashboard</p>
            </div>

            <div class="p-4 bg-gray-800/50 rounded-lg border border-gray-700">
              <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center mb-3">
                <svg class="w-5 h-5 text-brand-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </div>
              <h3 class="font-semibold text-white mb-1">Health Monitoring</h3>
              <p class="text-sm text-gray-400">See at a glance which services are online and healthy</p>
            </div>

            <div class="p-4 bg-gray-800/50 rounded-lg border border-gray-700">
              <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center mb-3">
                <svg class="w-5 h-5 text-brand-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
              </div>
              <h3 class="font-semibold text-white mb-1">Quick Access</h3>
              <p class="text-sm text-gray-400">Keyboard shortcuts and search for lightning-fast navigation</p>
            </div>
          </div>

          <button
            class="px-8 py-3 bg-brand-600 hover:bg-brand-700 text-white font-medium rounded-lg text-lg transition-colors"
            onclick={nextStep}
          >
            Let's Get Started
          </button>
        </div>

      <!-- Security Step -->
      {:else if $currentStep === 'security'}
        <div class="py-6" in:fly={{ x: 30, duration: 300 }} out:fade={{ duration: 150 }}>
          <div class="text-center mb-8">
            <h2 class="text-2xl font-bold text-white mb-2">Secure Your Dashboard</h2>
            <p class="text-gray-400">Choose how you want to protect access to Muximux</p>
          </div>

          {#if setupDone}
            <!-- Already completed -->
            <div class="max-w-md mx-auto text-center">
              <div class="w-16 h-16 mx-auto mb-4 rounded-full bg-green-500/20 flex items-center justify-center">
                <svg class="w-8 h-8 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
              </div>
              <p class="text-gray-300">Security has been configured.</p>
            </div>
          {:else}
            <!-- Method selection cards -->
            <div class="max-w-2xl mx-auto space-y-3 mb-6">
              <!-- Builtin password -->
              <button
                class="w-full p-4 rounded-xl border text-left transition-all flex items-start gap-4
                       {authMethod === 'builtin' ? 'border-brand-500 bg-brand-500/10' : 'border-gray-700 bg-gray-800/50 hover:border-gray-600'}"
                onclick={() => authMethod = 'builtin'}
              >
                <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center flex-shrink-0 mt-0.5">
                  <svg class="w-5 h-5 text-brand-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="3" y="11" width="18" height="11" rx="2" />
                    <path d="M7 11V7a5 5 0 0110 0v4" />
                  </svg>
                </div>
                <div class="flex-1">
                  <div class="flex items-center gap-2">
                    <h3 class="font-semibold text-white">Create a password</h3>
                    <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-brand-500 text-white uppercase tracking-wider">Recommended</span>
                  </div>
                  <p class="text-sm text-gray-400 mt-1">Set up a username and password to protect your dashboard</p>
                </div>
              </button>

              <!-- Forward auth -->
              <button
                class="w-full p-4 rounded-xl border text-left transition-all flex items-start gap-4
                       {authMethod === 'forward_auth' ? 'border-brand-500 bg-brand-500/10' : 'border-gray-700 bg-gray-800/50 hover:border-gray-600'}"
                onclick={() => authMethod = 'forward_auth'}
              >
                <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center flex-shrink-0 mt-0.5">
                  <svg class="w-5 h-5 text-brand-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                  </svg>
                </div>
                <div>
                  <h3 class="font-semibold text-white">I use an auth proxy</h3>
                  <p class="text-sm text-gray-400 mt-1">Authelia, Authentik, or another reverse proxy handles authentication</p>
                </div>
              </button>

              <!-- None -->
              <button
                class="w-full p-4 rounded-xl border text-left transition-all flex items-start gap-4
                       {authMethod === 'none' ? 'border-amber-500 bg-amber-500/10' : 'border-gray-700 bg-gray-800/50 hover:border-gray-600'}"
                onclick={() => authMethod = 'none'}
              >
                <div class="w-10 h-10 rounded-lg bg-amber-500/20 flex items-center justify-center flex-shrink-0 mt-0.5">
                  <svg class="w-5 h-5 text-amber-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10" />
                    <line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
                  </svg>
                </div>
                <div>
                  <h3 class="font-semibold text-white">No authentication</h3>
                  <p class="text-sm text-gray-400 mt-1">Anyone with network access gets full control</p>
                </div>
              </button>
            </div>

            <!-- Configuration form (slides in when method selected) -->
            {#if authMethod === 'builtin'}
              <div class="max-w-md mx-auto space-y-4" in:fly={{ y: 10, duration: 200 }}>
                <div>
                  <label for="setup-username" class="block text-sm text-gray-400 mb-1">Username</label>
                  <input
                    id="setup-username"
                    type="text"
                    bind:value={setupUsername}
                    class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                           focus:outline-none focus:ring-2 focus:ring-brand-500"
                    placeholder="admin"
                    autocomplete="username"
                  />
                </div>
                <div>
                  <label for="setup-password" class="block text-sm text-gray-400 mb-1">Password</label>
                  <input
                    id="setup-password"
                    type="password"
                    bind:value={setupPassword}
                    class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                           focus:outline-none focus:ring-2 focus:ring-brand-500"
                    placeholder="Minimum 8 characters"
                    autocomplete="new-password"
                  />
                  {#if setupPassword.length > 0 && setupPassword.length < 8}
                    <p class="text-red-400 text-xs mt-1">Password must be at least 8 characters</p>
                  {/if}
                </div>
                <div>
                  <label for="setup-confirm" class="block text-sm text-gray-400 mb-1">Confirm password</label>
                  <input
                    id="setup-confirm"
                    type="password"
                    bind:value={setupConfirmPassword}
                    class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                           focus:outline-none focus:ring-2 focus:ring-brand-500"
                    placeholder="Re-enter password"
                    autocomplete="new-password"
                  />
                  {#if setupConfirmPassword.length > 0 && setupPassword !== setupConfirmPassword}
                    <p class="text-red-400 text-xs mt-1">Passwords do not match</p>
                  {/if}
                </div>
              </div>
            {:else if authMethod === 'forward_auth'}
              <div class="max-w-md mx-auto space-y-4" in:fly={{ y: 10, duration: 200 }}>
                <!-- Preset selector -->
                <div>
                  <span class="block text-sm text-gray-400 mb-2">Proxy type</span>
                  <div class="flex gap-2">
                    {#each ['authelia', 'authentik', 'custom'] as p (p)}
                      <button
                        class="flex-1 px-3 py-2 text-sm rounded-md border transition-all
                               {faPreset === p ? 'border-brand-500 bg-brand-500/15 text-white' : 'border-gray-600 bg-gray-700 text-gray-400 hover:text-white'}"
                        onclick={() => selectFaPreset(p as 'authelia' | 'authentik' | 'custom')}
                      >
                        {p.charAt(0).toUpperCase() + p.slice(1)}
                      </button>
                    {/each}
                  </div>
                </div>

                <div>
                  <label for="setup-proxies" class="block text-sm text-gray-400 mb-1">Trusted proxy IPs</label>
                  <textarea
                    id="setup-proxies"
                    bind:value={faTrustedProxies}
                    class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                           focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
                    placeholder="10.0.0.1/32&#10;172.16.0.0/12"
                    rows="3"
                  ></textarea>
                  <p class="text-xs text-gray-500 mt-1">IP addresses or CIDR ranges, one per line</p>
                </div>

                <button
                  class="flex items-center gap-1.5 text-sm text-gray-400 hover:text-gray-300 transition-colors"
                  onclick={() => faShowAdvanced = !faShowAdvanced}
                >
                  <svg class="w-4 h-4 transition-transform {faShowAdvanced ? 'rotate-90' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                  </svg>
                  Advanced: Header names
                </button>

                {#if faShowAdvanced}
                  <div class="grid grid-cols-2 gap-3 p-3 rounded-lg bg-gray-800/50 border border-gray-700" in:fly={{ y: -10, duration: 150 }}>
                    <div>
                      <label for="fa-header-user" class="block text-xs text-gray-400 mb-1">User header</label>
                      <input id="fa-header-user" type="text" bind:value={faHeaderUser}
                        class="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                    </div>
                    <div>
                      <label for="fa-header-email" class="block text-xs text-gray-400 mb-1">Email header</label>
                      <input id="fa-header-email" type="text" bind:value={faHeaderEmail}
                        class="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                    </div>
                    <div>
                      <label for="fa-header-groups" class="block text-xs text-gray-400 mb-1">Groups header</label>
                      <input id="fa-header-groups" type="text" bind:value={faHeaderGroups}
                        class="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                    </div>
                    <div>
                      <label for="fa-header-name" class="block text-xs text-gray-400 mb-1">Name header</label>
                      <input id="fa-header-name" type="text" bind:value={faHeaderName}
                        class="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                    </div>
                  </div>
                {/if}
              </div>
            {:else if authMethod === 'none'}
              <div class="max-w-md mx-auto" in:fly={{ y: 10, duration: 200 }}>
                <div class="p-4 rounded-lg bg-amber-500/10 border border-amber-500/20 mb-4">
                  <div class="flex gap-3">
                    <svg class="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
                      <line x1="12" y1="9" x2="12" y2="13" />
                      <line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <div>
                      <h4 class="font-semibold text-amber-400 text-sm mb-1">Security warning</h4>
                      <p class="text-sm text-gray-400">Without authentication, anyone who can reach this port has full access to your dashboard and all configured services.</p>
                    </div>
                  </div>
                </div>
                <label class="flex items-start gap-3 cursor-pointer">
                  <input type="checkbox" bind:checked={acknowledgeRisk}
                    class="mt-1 w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500" />
                  <span class="text-sm text-gray-400">I understand the risks and want to proceed without authentication</span>
                </label>
              </div>
            {/if}

            {#if setupError}
              <div class="max-w-md mx-auto mt-4 p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm">
                {setupError}
              </div>
            {/if}
          {/if}
        </div>

      <!-- Step 2: Add Apps (two-column layout with groups) -->
      {:else if $currentStep === 'apps'}
        <div class="py-6" in:fly={{ x: 30, duration: 300 }} out:fade={{ duration: 150 }}>
          <div class="text-center mb-8">
            <h2 class="text-2xl font-bold text-white mb-2">Choose Your Apps</h2>
            <p class="text-gray-400">Select from popular apps or add your own</p>
          </div>

          <div class="apps-two-col gap-6">
            <!-- LEFT COLUMN: Custom app + template apps (scrollable) -->
            <div class="apps-left-col space-y-6">
              <!-- Custom App Card (prominent, at top) -->
              {#if showCustomApp}
                <div class="p-5 rounded-xl border-2 border-brand-500/50 bg-brand-500/5">
                  <h4 class="font-semibold text-white mb-4 flex items-center gap-2">
                    <svg class="w-5 h-5 text-brand-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                    </svg>
                    Add Custom Application
                  </h4>
                  <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div>
                      <label for="custom-name" class="block text-sm text-gray-400 mb-1">Name</label>
                      <input
                        id="custom-name"
                        type="text"
                        bind:value={customApp.name}
                        class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                               focus:outline-none focus:ring-2 focus:ring-brand-500"
                        placeholder="My App"
                      />
                    </div>
                    <div>
                      <label for="custom-url" class="block text-sm text-gray-400 mb-1">URL</label>
                      <input
                        id="custom-url"
                        type="url"
                        bind:value={customApp.url}
                        class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                               focus:outline-none focus:ring-2 focus:ring-brand-500"
                        placeholder="http://localhost:8080"
                      />
                    </div>
                    <div>
                      <span class="block text-sm text-gray-400 mb-1">Icon</span>
                      <button
                        class="flex items-center gap-2 w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                               hover:bg-gray-600 transition-colors"
                        onclick={() => iconBrowserContext = 'custom-app'}
                      >
                        {#if customApp.icon.name}
                          <AppIcon icon={customApp.icon} name={customApp.name} color={customApp.color} size="sm" />
                          <span class="text-sm truncate">{customApp.icon.name}</span>
                        {:else}
                          <div class="w-6 h-6 rounded bg-gray-600 flex items-center justify-center text-xs font-bold text-gray-400">
                            {customApp.name ? customApp.name.charAt(0).toUpperCase() : '?'}
                          </div>
                          <span class="text-sm text-gray-400">Browse Icons</span>
                        {/if}
                      </button>
                    </div>
                    <div>
                      <label for="custom-color" class="block text-sm text-gray-400 mb-1">Color</label>
                      <div class="flex gap-2">
                        <input
                          id="custom-color"
                          type="color"
                          bind:value={customApp.color}
                          class="w-10 h-10 rounded cursor-pointer"
                        />
                        <input
                          type="text"
                          bind:value={customApp.color}
                          class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                                 focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
                        />
                      </div>
                    </div>
                    <div class="sm:col-span-2">
                      <label for="custom-group" class="block text-sm text-gray-400 mb-1">Group</label>
                      <select
                        id="custom-group"
                        bind:value={customApp.group}
                        class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                               focus:outline-none focus:ring-2 focus:ring-brand-500"
                      >
                        <option value="">No Group</option>
                        {#each wizardGroups as g (g.name)}
                          <option value={g.name}>{g.name}</option>
                        {/each}
                        {#each suggestedGroups.filter(sg => !wizardGroups.some(wg => wg.name === sg)) as sg (sg)}
                          <option value={sg}>{sg}</option>
                        {/each}
                      </select>
                    </div>
                  </div>
                  <!-- Advanced Settings Toggle -->
                  <div class="mt-4">
                    <button
                      class="flex items-center gap-1.5 text-sm text-gray-400 hover:text-gray-300 transition-colors"
                      onclick={() => showAdvanced = !showAdvanced}
                    >
                      <svg class="w-4 h-4 transition-transform {showAdvanced ? 'rotate-90' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                      </svg>
                      Advanced Settings
                    </button>

                    {#if showAdvanced}
                      <div class="mt-3 grid grid-cols-1 sm:grid-cols-2 gap-4 p-3 rounded-lg bg-gray-800/50 border border-gray-700">
                        <div>
                          <label for="custom-open-mode" class="block text-sm text-gray-400 mb-1">Open Mode</label>
                          <select
                            id="custom-open-mode"
                            bind:value={customApp.open_mode}
                            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                                   focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
                          >
                            <option value="iframe">Embed in iframe</option>
                            <option value="new_tab">Open in new tab</option>
                            <option value="new_window">Open in new window</option>
                            <option value="redirect">Redirect (leave dashboard)</option>
                          </select>
                        </div>
                        <div>
                          <label for="custom-health-url" class="block text-sm text-gray-400 mb-1">Health Check URL</label>
                          <input
                            id="custom-health-url"
                            type="url"
                            bind:value={customApp.health_url}
                            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                                   focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
                            placeholder="Same as app URL"
                          />
                        </div>
                        <div>
                          <label class="flex items-center gap-3 cursor-pointer py-2">
                            <input
                              type="checkbox"
                              bind:checked={customApp.proxy}
                              class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
                            />
                            <div>
                              <div class="text-sm text-white">Reverse Proxy</div>
                              <div class="text-xs text-gray-500">Route through Muximux to avoid CORS/CSP issues</div>
                            </div>
                          </label>
                        </div>
                        <div>
                          <label for="custom-scale" class="block text-sm text-gray-400 mb-1">Iframe Scale</label>
                          <input
                            id="custom-scale"
                            type="number"
                            min="0.5"
                            max="2"
                            step="0.1"
                            bind:value={customApp.scale}
                            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                                   focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
                          />
                        </div>
                      </div>
                    {/if}
                  </div>

                  <div class="flex gap-2 mt-4">
                    <button
                      class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md disabled:opacity-50"
                      disabled={!customApp.name || !customApp.url}
                      onclick={addCustomApp}
                    >
                      Add App
                    </button>
                    <button
                      class="px-4 py-2 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
                      onclick={() => showCustomApp = false}
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              {:else}
                <button
                  class="w-full p-4 rounded-xl border-2 border-dashed border-gray-600 hover:border-brand-500
                         bg-gray-800/30 hover:bg-brand-500/5 transition-all flex items-center justify-center gap-2
                         text-gray-400 hover:text-brand-400 font-medium"
                  onclick={() => showCustomApp = true}
                >
                  <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                  </svg>
                  Add Custom App
                </button>
              {/if}

              <!-- App categories (template apps) -->
              {#each Object.entries(popularApps) as [category, apps] (category)}
                <div>
                  <h3 class="text-sm font-semibold text-gray-400 uppercase tracking-wider mb-3 flex items-center gap-2">
                    <span class="w-2 h-2 rounded-full" style="background-color: {getGroupColor(category)}"></span>
                    {category}
                  </h3>

                  <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                    {#each apps as app (app.name)}
                      {@const selection = appSelections.get(app.name)}
                      <div
                        class="relative p-3 rounded-lg border transition-all cursor-pointer
                               {selection?.selected
                                 ? 'bg-brand-500/10 border-brand-500'
                                 : 'bg-gray-800/50 border-gray-700 hover:border-gray-600'}"
                        onclick={() => toggleApp(app)}
                        onkeydown={(e) => e.key === 'Enter' && toggleApp(app)}
                        role="checkbox"
                        aria-checked={selection?.selected}
                        tabindex="0"
                      >
                        <!-- Checkbox -->
                        <div class="absolute top-2.5 right-2.5">
                          <div class="w-5 h-5 rounded border flex items-center justify-center
                                      {selection?.selected ? 'bg-brand-500 border-brand-500' : 'border-gray-600'}">
                            {#if selection?.selected}
                              <svg class="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
                              </svg>
                            {/if}
                          </div>
                        </div>

                        <div class="flex items-start gap-3">
                          <AppIcon
                            icon={{ type: 'dashboard', name: app.icon, file: '', url: '', variant: 'svg' }}
                            name={app.name}
                            color={app.color}
                            size="lg"
                          />
                          <div class="flex-1 min-w-0 pr-6">
                            <h4 class="font-medium text-white text-sm">{app.name}</h4>
                            <p class="text-xs text-gray-500">{app.description}</p>
                            {#if selection?.selected}
                              <input
                                type="url"
                                value={selection.url}
                                oninput={(e) => updateAppUrl(app.name, e.currentTarget.value)}
                                onclick={(e) => e.stopPropagation()}
                                class="w-full mt-1.5 px-2 py-1 text-xs bg-gray-700 border border-gray-600 rounded
                                       text-white placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
                                placeholder="http://localhost:8080"
                              />
                            {/if}
                          </div>
                        </div>
                      </div>
                    {/each}
                  </div>
                </div>
              {/each}
            </div>

            <!-- RIGHT COLUMN: Selected apps + Groups (sticky on desktop) -->
            <div class="apps-right-col">
              <div class="apps-right-sticky space-y-6">
                <div>
                  <h3 class="text-sm font-semibold text-gray-300 mb-3">
                    Unsorted Apps ({dndUnsorted.length})
                  </h3>
                  {#if selectedCount + $selectedApps.length === 0}
                    <p class="text-sm text-gray-500 italic">Select apps from the left to get started</p>
                  {:else if dndUnsorted.length === 0}
                    <p class="text-sm text-gray-500 italic">All apps have been sorted into groups</p>
                  {:else}
                    <div class="space-y-1 min-h-[36px] p-2 rounded-lg border border-dashed border-gray-700 bg-gray-800/30"
                         use:dndzone={{items: dndUnsorted, flipDurationMs, type: 'wizard-apps', dropTargetStyle: {}}}
                         onconsider={handleUnsortedConsider}
                         onfinalize={handleUnsortedFinalize}>
                      {#each dndUnsorted as item (item.id)}
                        {@const appColor = getAppDisplayColor(item.name)}
                        {@const appIcon = getAppDisplayIcon(item.name)}
                        <div class="flex items-center gap-1.5 p-2 rounded-md bg-gray-800/50 cursor-grab"
                             animate:flip={{duration: flipDurationMs}}>
                          <input
                            type="color"
                            value={appColor}
                            oninput={(e) => updateAppColor(item.name, e.currentTarget.value)}
                            onclick={(e) => e.stopPropagation()}
                            class="w-6 h-6 rounded cursor-pointer border-0 p-0 flex-shrink-0"
                          />
                          <button
                            class="flex-shrink-0 w-6 h-6 rounded bg-gray-700 flex items-center justify-center hover:bg-gray-600 transition-colors"
                            onclick={() => openAppIconBrowser(item.name)}
                            title="Change icon"
                          >
                            {#if appIcon.name}
                              <AppIcon icon={appIcon} name={item.name} color={appColor} size="sm" />
                            {:else}
                              <div class="w-5 h-5 rounded flex items-center justify-center text-[10px] font-bold text-gray-400">
                                {item.name.charAt(0).toUpperCase()}
                              </div>
                            {/if}
                          </button>
                          <span class="text-sm text-white truncate flex-1">{item.name}</span>
                          <button
                            class="p-1 text-gray-500 hover:text-red-400 transition-opacity flex-shrink-0"
                            onclick={() => {
                              const t = Object.values(popularApps).flat().find(a => a.name === item.name);
                              if (t) toggleApp(t);
                              else selectedApps.update(apps => apps.filter(a => a.name !== item.name));
                            }}
                            aria-label="Remove {item.name}"
                          >
                            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                          </button>
                        </div>
                      {/each}
                    </div>
                  {/if}
                </div>

                <div>
                  <h3 class="text-sm font-semibold text-gray-300 mb-3">
                    Groups ({wizardGroups.length})
                  </h3>
                  {#if wizardGroups.length > 0}
                    <div class="space-y-2">
                      {#each wizardGroups as group, i (group.name)}
                        {@const groupApps = dndGroupApps[group.name] || []}
                        <div class="rounded-lg border border-gray-700 bg-gray-800/30 overflow-hidden">
                          <div class="flex items-center gap-2 p-2.5">
                            <input
                              type="color"
                              value={group.color}
                              oninput={(e) => updateGroupColor(i, e.currentTarget.value)}
                              class="w-7 h-7 rounded cursor-pointer border-0 p-0 flex-shrink-0"
                              style="background-color: {group.color}"
                            />
                            <button
                              class="flex-shrink-0 w-7 h-7 rounded bg-gray-700 flex items-center justify-center hover:bg-gray-600 transition-colors"
                              onclick={() => iconBrowserContext = i}
                              title="Change icon"
                            >
                              {#if group.icon.name}
                                <AppIcon icon={group.icon} name={group.name} color={group.color} size="sm" />
                              {:else}
                                <svg class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                                        d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                                </svg>
                              {/if}
                            </button>
                            <div class="flex-1 min-w-0">
                              <input
                                type="text"
                                value={group.name}
                                oninput={(e) => updateGroupName(i, e.currentTarget.value)}
                                class="w-full px-1.5 py-0.5 bg-transparent border-b border-transparent hover:border-gray-600
                                       focus:border-brand-500 text-sm text-white font-medium
                                       focus:outline-none transition-colors"
                              />
                            </div>
                            <button
                              class="flex-shrink-0 p-1 text-gray-500 hover:text-red-400 rounded transition-colors"
                              onclick={() => deleteGroup(i)}
                              aria-label="Remove group"
                            >
                              <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                              </svg>
                            </button>
                          </div>
                          <div class="px-2.5 pb-2.5">
                            <div class="space-y-1 min-h-[36px] p-2 rounded-md border border-dashed border-gray-600 bg-gray-900/30"
                                 use:dndzone={{items: groupApps, flipDurationMs, type: 'wizard-apps', dropTargetStyle: {}}}
                                 onconsider={(e) => handleGroupAppConsider(e, group.name)}
                                 onfinalize={(e) => handleGroupAppFinalize(e, group.name)}>
                              {#each groupApps as item (item.id)}
                                {@const appColor = getAppDisplayColor(item.name)}
                                {@const appIcon = getAppDisplayIcon(item.name)}
                                <div class="flex items-center gap-1.5 p-1.5 rounded bg-gray-800/70 cursor-grab text-sm text-white"
                                     animate:flip={{duration: flipDurationMs}}>
                                  <input
                                    type="color"
                                    value={appColor}
                                    oninput={(e) => updateAppColor(item.name, e.currentTarget.value)}
                                    onclick={(e) => e.stopPropagation()}
                                    class="w-5 h-5 rounded cursor-pointer border-0 p-0 flex-shrink-0"
                                  />
                                  <button
                                    class="flex-shrink-0 w-5 h-5 rounded bg-gray-700 flex items-center justify-center hover:bg-gray-600 transition-colors"
                                    onclick={() => openAppIconBrowser(item.name)}
                                    title="Change icon"
                                  >
                                    {#if appIcon.name}
                                      <AppIcon icon={appIcon} name={item.name} color={appColor} size="sm" />
                                    {:else}
                                      <div class="text-[9px] font-bold text-gray-400">
                                        {item.name.charAt(0).toUpperCase()}
                                      </div>
                                    {/if}
                                  </button>
                                  <span class="truncate flex-1">{item.name}</span>
                                </div>
                              {/each}
                              {#if groupApps.length === 0}
                                <p class="text-xs text-gray-600 text-center py-1">Drop apps here</p>
                              {/if}
                            </div>
                          </div>
                        </div>
                      {/each}
                    </div>
                  {:else}
                    <p class="text-sm text-gray-500 italic">Groups auto-appear when you select apps</p>
                  {/if}

                  <button
                    class="mt-2 flex items-center gap-1.5 px-3 py-1.5 text-sm text-gray-400 hover:text-white rounded-md
                           hover:bg-gray-800 transition-colors"
                    onclick={addGroup}
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                    </svg>
                    Add Group
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

      <!-- Step 3: Navigation Style -->
      {:else if $currentStep === 'navigation'}
        <div class="py-6" in:fly={{ x: 30, duration: 300 }} out:fade={{ duration: 150 }}>
          <div class="text-center mb-6">
            <h2 class="text-2xl font-bold text-white mb-2">Choose Your Navigation Style</h2>
            <p class="text-gray-400">Select how you want to navigate between your apps</p>
          </div>

          <!-- Live preview area -->
          <div class="max-w-2xl mx-auto mb-8">
            <div class="nav-preview-container rounded-xl border border-gray-700 overflow-hidden bg-gray-950" style="height: 240px;">
              <!-- Nav bar mock -->
              {#if $selectedNavigation === 'top'}
                <div class="flex flex-col h-full transition-all duration-300">
                  <div class="flex items-center gap-3 px-4 py-2 bg-gray-800/80 border-b border-gray-700">
                    <div class="w-5 h-5 rounded bg-brand-500/60"></div>
                    {#each mockNavItems as item (item.name)}
                      <div class="flex items-center gap-1.5">
                        <div class="w-3.5 h-3.5 rounded" style="background-color: {item.color}"></div>
                        {#if $showLabels}<span class="text-[10px] text-gray-400">{item.name}</span>{/if}
                      </div>
                    {/each}
                  </div>
                  <div class="flex-1 p-3"><div class="w-full h-full rounded-lg bg-gray-800/40 border border-gray-800"></div></div>
                </div>
              {:else if $selectedNavigation === 'left'}
                <div class="flex h-full transition-all duration-300">
                  <div class="flex flex-col items-center gap-2 py-3 px-2 bg-gray-800/80 border-r border-gray-700" style="width: {$showLabels ? '100px' : '44px'}">
                    <div class="w-5 h-5 rounded bg-brand-500/60 mb-1"></div>
                    {#each mockNavItems as item (item.name)}
                      <div class="flex items-center gap-1.5 {$showLabels ? 'w-full px-1.5' : ''}">
                        <div class="w-3.5 h-3.5 rounded flex-shrink-0" style="background-color: {item.color}"></div>
                        {#if $showLabels}<span class="text-[9px] text-gray-400 truncate">{item.name}</span>{/if}
                      </div>
                    {/each}
                  </div>
                  <div class="flex-1 p-3"><div class="w-full h-full rounded-lg bg-gray-800/40 border border-gray-800"></div></div>
                </div>
              {:else if $selectedNavigation === 'right'}
                <div class="flex h-full transition-all duration-300">
                  <div class="flex-1 p-3"><div class="w-full h-full rounded-lg bg-gray-800/40 border border-gray-800"></div></div>
                  <div class="flex flex-col items-center gap-2 py-3 px-2 bg-gray-800/80 border-l border-gray-700" style="width: {$showLabels ? '100px' : '44px'}">
                    <div class="w-5 h-5 rounded bg-brand-500/60 mb-1"></div>
                    {#each mockNavItems as item (item.name)}
                      <div class="flex items-center gap-1.5 {$showLabels ? 'w-full px-1.5' : ''}">
                        <div class="w-3.5 h-3.5 rounded flex-shrink-0" style="background-color: {item.color}"></div>
                        {#if $showLabels}<span class="text-[9px] text-gray-400 truncate">{item.name}</span>{/if}
                      </div>
                    {/each}
                  </div>
                </div>
              {:else if $selectedNavigation === 'bottom'}
                <div class="flex flex-col h-full transition-all duration-300">
                  <div class="flex-1 p-3"><div class="w-full h-full rounded-lg bg-gray-800/40 border border-gray-800"></div></div>
                  <div class="flex items-center justify-center gap-3 px-4 py-2 bg-gray-800/80 border-t border-gray-700">
                    {#each mockNavItems as item (item.name)}
                      <div class="flex flex-col items-center gap-0.5">
                        <div class="w-3.5 h-3.5 rounded" style="background-color: {item.color}"></div>
                        {#if $showLabels}<span class="text-[8px] text-gray-400">{item.name}</span>{/if}
                      </div>
                    {/each}
                  </div>
                </div>
              {:else if $selectedNavigation === 'floating'}
                <div class="relative h-full transition-all duration-300">
                  <div class="absolute inset-3"><div class="w-full h-full rounded-lg bg-gray-800/40 border border-gray-800"></div></div>
                  <div class="absolute bottom-4 left-1/2 -translate-x-1/2 flex items-center gap-2 px-3 py-1.5 bg-gray-800/90 rounded-full border border-gray-700 backdrop-blur-sm">
                    {#each mockNavItems as item (item.name)}
                      <div class="w-3.5 h-3.5 rounded" style="background-color: {item.color}"></div>
                    {/each}
                  </div>
                </div>
              {/if}
            </div>
          </div>

          <!-- Position selector buttons -->
          <div class="flex flex-wrap justify-center gap-2 max-w-2xl mx-auto mb-6">
            {#each navPositions as pos (pos.value)}
              <button
                class="px-4 py-2 rounded-lg border text-sm font-medium transition-all
                       {$selectedNavigation === pos.value
                         ? 'border-brand-500 bg-brand-500/15 text-white'
                         : 'border-gray-700 hover:border-gray-500 bg-gray-800/50 text-gray-400 hover:text-white'}"
                onclick={() => selectedNavigation.set(pos.value)}
              >
                {pos.label}
              </button>
            {/each}
          </div>

          <!-- Show labels toggle -->
          <div class="max-w-md mx-auto">
            <label class="flex items-center gap-3 p-4 bg-gray-800/50 rounded-lg border border-gray-700 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={$showLabels}
                class="w-5 h-5 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <div class="text-white font-medium">Show App Labels</div>
                <div class="text-sm text-gray-400">Display app names next to icons in navigation</div>
              </div>
            </label>
          </div>
        </div>

      <!-- Step 4: Theme -->
      {:else if $currentStep === 'theme'}
        <div class="py-6" in:fly={{ x: 30, duration: 300 }} out:fade={{ duration: 150 }}>
          <div class="text-center mb-8">
            <h2 class="text-2xl font-bold text-white mb-2">Choose Your Theme</h2>
            <p class="text-gray-400">Pick a visual style for your dashboard</p>
          </div>

          <!-- Variant mode selector (segmented control) -->
          <div class="flex justify-center mb-8">
            <div class="inline-flex bg-gray-800 rounded-lg p-1 border border-gray-700">
              {#each variantOptions as opt (opt.value)}
                <button
                  class="px-5 py-2 text-sm font-medium rounded-md transition-all
                         {$variantMode === opt.value
                           ? 'bg-brand-600 text-white shadow-sm'
                           : 'text-gray-400 hover:text-white'}"
                  onclick={() => setVariantMode(opt.value)}
                >
                  {opt.label}
                </button>
              {/each}
            </div>
          </div>

          <!-- Theme family grid -->
          <div class="grid grid-cols-2 sm:grid-cols-3 gap-4 max-w-3xl mx-auto">
            {#each $themeFamilies as family (family.id)}
              {@const isSelected = $selectedFamily === family.id}
              {@const darkPreview = family.darkTheme?.preview}
              {@const lightPreview = family.lightTheme?.preview}
              {@const preview = darkPreview || lightPreview}
              <button
                class="relative p-4 rounded-xl border text-left transition-all
                       {isSelected
                         ? 'border-brand-500 bg-brand-500/10 ring-1 ring-brand-500/30'
                         : 'border-gray-700 hover:border-gray-500 bg-gray-800/50'}"
                onclick={() => setThemeFamily(family.id)}
              >
                <!-- Color swatches preview -->
                {#if preview}
                  <div class="flex gap-1.5 mb-3">
                    <div class="w-8 h-8 rounded-md border border-white/10" style="background-color: {preview.bg}"></div>
                    <div class="w-8 h-8 rounded-md border border-white/10" style="background-color: {preview.surface}"></div>
                    <div class="w-8 h-8 rounded-md border border-white/10" style="background-color: {preview.accent}"></div>
                  </div>
                {:else}
                  <div class="flex gap-1.5 mb-3">
                    <div class="w-8 h-8 rounded-md bg-gray-700 border border-white/10"></div>
                    <div class="w-8 h-8 rounded-md bg-gray-600 border border-white/10"></div>
                    <div class="w-8 h-8 rounded-md bg-gray-500 border border-white/10"></div>
                  </div>
                {/if}

                <div class="font-medium text-white text-sm">{family.name}</div>
                {#if family.description}
                  <div class="text-xs text-gray-400 mt-0.5">{family.description}</div>
                {/if}

                <!-- Selection checkmark -->
                {#if isSelected}
                  <div class="absolute top-2 right-2 w-5 h-5 rounded-full bg-brand-500 flex items-center justify-center">
                    <svg class="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
                    </svg>
                  </div>
                {/if}
              </button>
            {/each}
          </div>

          <p class="text-center text-gray-500 text-sm mt-6 max-w-md mx-auto">
            Changes apply live — you can create custom themes later in Settings
          </p>
        </div>

      <!-- Step 5: Complete -->
      {:else if $currentStep === 'complete'}
        <div class="text-center py-12" in:fly={{ x: 30, duration: 300 }} out:fade={{ duration: 150 }}>
          <div class="w-20 h-20 mx-auto mb-6 rounded-full bg-brand-500/20 flex items-center justify-center">
            <svg class="w-10 h-10 text-brand-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
            </svg>
          </div>

          <h2 class="text-3xl font-bold text-white mb-4">You're All Set!</h2>
          <p class="text-xl text-gray-400 mb-8 max-w-lg mx-auto">
            Your dashboard is ready with {selectedCount + $selectedApps.length} app{selectedCount + $selectedApps.length !== 1 ? 's' : ''}.
          </p>

          <!-- Summary -->
          <div class="max-w-md mx-auto mb-8 p-4 bg-gray-800/50 rounded-lg border border-gray-700 text-left">
            <h4 class="font-medium text-gray-300 mb-3">Setup Summary</h4>
            <dl class="space-y-2 text-sm">
              <div class="flex justify-between">
                <dt class="text-gray-400">Applications</dt>
                <dd class="text-white">{selectedCount + $selectedApps.length}</dd>
              </div>
              <div class="flex justify-between">
                <dt class="text-gray-400">Navigation</dt>
                <dd class="text-white capitalize">{$selectedNavigation}</dd>
              </div>
              <div class="flex justify-between">
                <dt class="text-gray-400">Theme</dt>
                <dd class="text-white capitalize">{$themeFamilies.find(f => f.id === $selectedFamily)?.name || $selectedFamily}</dd>
              </div>
              <div class="flex justify-between">
                <dt class="text-gray-400">Groups</dt>
                <dd class="text-white">{wizardGroups.length}</dd>
              </div>
              <div class="flex justify-between">
                <dt class="text-gray-400">Show Labels</dt>
                <dd class="text-white">{$showLabels ? 'Yes' : 'No'}</dd>
              </div>
            </dl>
          </div>

          <button
            class="px-8 py-3 bg-brand-600 hover:bg-brand-700 text-white font-medium rounded-lg text-lg transition-colors"
            onclick={handleComplete}
          >
            Launch Dashboard
          </button>
        </div>
      {/if}
    </div>
  </div>

  <!-- Navigation buttons -->
  <div class="flex-shrink-0 px-8 py-4 border-t border-gray-800">
    <div class="max-w-4xl mx-auto flex justify-between items-center">
      <div>
        {#if $currentStep !== 'welcome'}
          <button
            class="px-4 py-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-800 transition-colors"
            onclick={prevStep}
          >
            Back
          </button>
        {/if}
      </div>

      <div class="text-sm text-gray-500">
        {#if $currentStep === 'apps'}
          {selectedCount + $selectedApps.length} app{selectedCount + $selectedApps.length !== 1 ? 's' : ''} selected
        {:else if $currentStep === 'security' && authMethod}
          {authMethod === 'builtin' ? 'Password' : authMethod === 'forward_auth' ? 'Auth proxy' : 'No auth'}
        {/if}
      </div>

      <div>
        {#if $currentStep !== 'welcome' && $currentStep !== 'complete'}
          {#if $currentStep === 'security'}
            <button
              class="px-6 py-2 bg-brand-600 hover:bg-brand-700 text-white rounded-md transition-colors disabled:opacity-50 flex items-center gap-2"
              disabled={!securityStepValid || setupLoading || setupDone}
              onclick={handleSecuritySubmit}
            >
              {#if setupLoading}
                <span class="inline-block w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></span>
              {/if}
              {setupDone ? 'Configured' : 'Continue'}
            </button>
          {:else}
            <button
              class="px-6 py-2 bg-brand-600 hover:bg-brand-700 text-white rounded-md transition-colors disabled:opacity-50"
              disabled={$currentStep === 'apps' && selectedCount + $selectedApps.length === 0}
              onclick={nextStep}
            >
              {$currentStep === 'theme' ? 'Finish' : 'Continue'}
            </button>
          {/if}
        {/if}
      </div>
    </div>
  </div>
</div>

<!-- Icon Browser modal -->
{#if iconBrowserContext !== null}
  {@const browserIcon = iconBrowserContext === 'custom-app'
    ? customApp.icon
    : iconBrowserContext === 'app-override'
    ? appOverrides.get(iconBrowserAppName)?.icon || getAppDisplayIcon(iconBrowserAppName)
    : wizardGroups[iconBrowserContext]?.icon}
  <div class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 backdrop-blur-sm">
    <div class="w-full max-w-4xl max-h-[80vh] bg-gray-900 rounded-xl border border-gray-700 shadow-2xl overflow-hidden">
      <IconBrowser
        selectedIcon={browserIcon?.name || ''}
        selectedVariant={browserIcon?.variant || 'svg'}
        selectedType={browserIcon?.type as 'dashboard' | 'lucide' | 'custom' || 'dashboard'}
        onselect={handleIconSelect}
        onclose={() => iconBrowserContext = null}
      />
    </div>
  </div>
{/if}

<style>
  /* Smooth transitions for step changes */
  :global(.fade-in) {
    animation: fadeIn 0.3s ease-out;
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: translateY(10px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  /* Nav preview smooth layout transitions */
  .nav-preview-container > div {
    transition: all 0.3s ease;
  }

  /* Apps step two-column layout */
  .apps-two-col {
    display: flex;
    flex-direction: column;
  }

  .apps-left-col {
    min-width: 0;
  }

  .apps-right-col {
    min-width: 0;
  }

  .apps-right-sticky {
    position: static;
  }

  @media (min-width: 768px) {
    .apps-two-col {
      flex-direction: row;
    }

    .apps-left-col {
      flex: 3;
    }

    .apps-right-col {
      flex: 2;
    }

    .apps-right-sticky {
      position: sticky;
      top: 0;
    }
  }
</style>

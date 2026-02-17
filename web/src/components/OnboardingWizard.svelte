<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { SvelteMap, SvelteSet } from 'svelte/reactivity';
  import { get } from 'svelte/store';
  import { fly, fade } from 'svelte/transition';
  import { flip } from 'svelte/animate';
  import { dndzone, type DndEvent } from 'svelte-dnd-action';
  import type { App, AppIcon as AppIconConfig, Config, Group, NavigationConfig, ThemeConfig, SetupRequest } from '$lib/types';
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
  import { popularApps, getAllGroups, templateToApp } from '$lib/popularApps';
  import type { PopularAppTemplate } from '$lib/popularApps';
  import AppIcon from './AppIcon.svelte';
  // MuximuxLogo available but not currently used in wizard layout
  import Navigation from './Navigation.svelte';
  import IconBrowser from './IconBrowser.svelte';
  import {
    themeFamilies,
    selectedFamily,
    variantMode,
    setThemeFamily,
    setVariantMode,
    detectCustomThemes,
    systemTheme,
    type VariantMode
  } from '$lib/themeStore';

  // Props
  let {
    oncomplete,
    needsSetup = false
  }: {
    oncomplete?: (detail: { apps: App[]; navigation: NavigationConfig; groups: Group[]; theme: ThemeConfig; setup?: SetupRequest }) => void;
    needsSetup?: boolean;
  } = $props();

  // Track which apps are selected with their URLs
  let appSelections = new SvelteMap<string, { selected: boolean; url: string }>();

  // Custom app form (minimal: just name + URL, rest configured in right column)
  let customApp = $state({ name: '', url: '' });

  // Per-app customization overrides
  interface AppOverride {
    color: string;
    icon: AppIconConfig;
    open_mode: App['open_mode'];
    proxy: boolean;
  }
  let appOverrides = new SvelteMap<string, AppOverride>();

  const openModes: { value: App['open_mode']; label: string }[] = [
    { value: 'iframe', label: 'Embedded' },
    { value: 'new_tab', label: 'New Tab' },
    { value: 'new_window', label: 'New Window' }
  ];

  // Groups editing state — each group gets a stable id for keyed-each
  type WizardGroup = Group & { id: string };
  let groupIdCounter = 0;
  let wizardGroups = $state<WizardGroup[]>([]);
  let iconBrowserContext = $state<'app-override' | number | null>(null);
  let iconBrowserAppName = $state<string>('');

  // Navigation behavior options
  let navShowLogo = $state(true);
  let navShowAppColors = $state(true);
  let navShowIconBg = $state(false);
  let navIconScale = $state(1);
  let navShowSplash = $state(true);
  let navAutoHide = $state(false);
  let navAutoHideDelay = $state('0.5s');
  let navShowShadow = $state(true);
  let navShowOnHover = $state(true);
  let navHideSidebarFooter = $state(false);
  let navFloatingPosition = $state<'bottom-right' | 'bottom-left' | 'top-right' | 'top-left'>('bottom-right');
  let navBarStyle = $state<'grouped' | 'flat'>('grouped');

  const flipDurationMs = 200;

  // Unified DnD state: group name keys = grouped apps
  type DndItem = {id: string; name: string};
  let dndApps = $state<Record<string, DndItem[]>>({});

  // Security step state
  let authMethod = $state<'builtin' | 'forward_auth' | 'none' | null>(null);
  let setupUsername = $state('admin');
  let setupPassword = $state('');
  let setupConfirmPassword = $state('');


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

  function handleSecuritySubmit() {
    if (!authMethod || !securityStepValid) return;
    nextStep();
  }

  // Build the setup request from wizard state (called at completion)
  function buildSetupRequest(): SetupRequest {
    const req: SetupRequest = { method: authMethod! };

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

    return req;
  }

  // Initialize app selections and load custom themes
  onMount(() => {
    configureSteps(needsSetup);

    // Reset theme to default so the wizard starts with a clean slate.
    // The user will pick their theme during the Theme step, which persists it.
    setThemeFamily('default');
    setVariantMode('dark');

    Object.values(popularApps).flat().forEach(app => {
      appSelections.set(app.name, { selected: false, url: app.defaultUrl });
    });

    // Load custom themes for the theme picker
    detectCustomThemes();
  });

  // Toggle app selection on/off
  function toggleApp(app: PopularAppTemplate) {
    const current = appSelections.get(app.name);
    if (current) {
      appSelections.set(app.name, { ...current, selected: !current.selected });
      rebuildDndFromSelections();
    }
  }

  // Add a numbered duplicate of a popular app template
  function addInstanceOf(app: PopularAppTemplate) {
    const allNames = new SvelteSet<string>();
    appSelections.forEach((v, k) => { if (v.selected) allNames.add(k); });
    get(selectedApps).forEach(a => allNames.add(a.name));

    let num = 2;
    while (allNames.has(`${app.name} ${num}`)) num++;

    const targetGroup = app.group;
    const newApp: App = {
      name: `${app.name} ${num}`,
      url: app.defaultUrl,
      icon: { type: 'dashboard', name: app.icon, file: '', url: '', variant: 'svg' },
      color: app.color,
      group: targetGroup,
      order: selectedCount + get(selectedApps).length,
      enabled: true,
      default: false,
      open_mode: 'iframe',
      proxy: false,
      scale: 1
    };

    selectedApps.update(apps => [...apps, newApp]);
    rebuildDndFromSelections();
  }

  // Get app URL (popular or custom)
  function getAppUrl(appName: string): string {
    const sel = appSelections.get(appName);
    if (sel) return sel.url;
    const custom = get(selectedApps).find(a => a.name === appName);
    return custom?.url || '';
  }

  // Update app URL (popular or custom)
  function updateAppUrl(appName: string, url: string) {
    const current = appSelections.get(appName);
    if (current) {
      appSelections.set(appName, { ...current, url });
    } else {
      selectedApps.update(apps => apps.map(a =>
        a.name === appName ? { ...a, url } : a));
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

  // Default icons for well-known group categories
  const defaultGroupIcons: Record<string, string> = {
    'Media': 'play',
    'Downloads': 'download',
    'System': 'server',
    'Utilities': 'wrench',
    'AI': 'brain',
    'Other': 'folder'
  };

  // Auto-add groups when new categories appear from selected apps or custom apps need "Other"
  $effect(() => {
    const existingNames = new Set(wizardGroups.map(g => g.name));
    const toAdd = suggestedGroups.filter(name => !existingNames.has(name));

    // Check if custom apps exist that need an "Other" group
    const hasCustomApps = get(selectedApps).length > 0;
    if (hasCustomApps && !existingNames.has('Other') && !toAdd.includes('Other') && wizardGroups.length === 0) {
      toAdd.push('Other');
    }

    if (toAdd.length > 0) {
      wizardGroups = [
        ...wizardGroups,
        ...toAdd.map((name, i) => ({
          id: `grp-${++groupIdCounter}`,
          name,
          icon: { type: 'lucide' as const, name: defaultGroupIcons[name] || '', file: '', url: '', variant: 'svg' },
          color: getGroupColor(name),
          order: wizardGroups.length + i,
          expanded: true
        }))
      ];
      // Just ensure the new group buckets exist — don't rebuild, as a DnD
      // operation may be in progress and rebuilding would duplicate items.
      for (const name of toAdd) {
        if (!dndApps[name]) dndApps[name] = [];
      }
    }
  });

  // Navigation position options
  const navPositions: { value: NavigationConfig['position']; label: string; description: string; icon: string }[] = [
    { value: 'top', label: 'Top Bar', description: 'Horizontal navigation at the top', icon: 'top' },
    { value: 'left', label: 'Left Sidebar', description: 'Vertical sidebar on the left', icon: 'left' },
    { value: 'right', label: 'Right Sidebar', description: 'Vertical sidebar on the right', icon: 'right' },
    { value: 'bottom', label: 'Bottom Bar', description: 'Horizontal bar at the bottom', icon: 'bottom' },
    { value: 'floating', label: 'Floating', description: 'Minimal floating button', icon: 'floating' }
  ];

  // Build the full preview app list — mirrors completeOnboarding() so the preview
  // is a faithful representation of what the finished dashboard will look like.
  const previewApps = $derived.by(() => {
    const apps: App[] = [];
    let order = 0;

    // Popular apps from selections
    appSelections.forEach((value, key) => {
      if (value.selected) {
        const template = Object.values(popularApps).flat().find(a => a.name === key);
        if (template) {
          apps.push(templateToApp(template, value.url, order++));
        }
      }
    });

    // Custom apps
    get(selectedApps).forEach(app => {
      apps.push({ ...app, order: order++ });
    });

    // Apply per-app overrides (color, icon, open_mode, proxy)
    for (const app of apps) {
      const override = appOverrides.get(app.name);
      if (override) {
        app.color = override.color;
        if (override.icon.name) app.icon = override.icon;
        app.open_mode = override.open_mode;
        app.proxy = override.proxy;
      }
    }

    // Assign groups from DnD state
    for (const [groupName, items] of Object.entries(dndApps)) {
      for (const item of items) {
        const app = apps.find(a => a.name === item.name);
        if (app) app.group = groupName;
      }
    }

    if (apps.length > 0) apps[0].default = true;

    // Fallback when nothing is selected yet
    if (apps.length === 0) {
      return [
        { name: 'Plex', color: '#E5A00D', icon: { type: 'dashboard' as const, name: 'plex', file: '', url: '', variant: 'svg' }, group: '', order: 0, url: '#', enabled: true, default: true, open_mode: 'iframe' as const, proxy: false, scale: 1 },
        { name: 'Sonarr', color: '#00CCFF', icon: { type: 'dashboard' as const, name: 'sonarr', file: '', url: '', variant: 'svg' }, group: '', order: 1, url: '#', enabled: true, default: false, open_mode: 'iframe' as const, proxy: false, scale: 1 },
        { name: 'Portainer', color: '#13BEF9', icon: { type: 'dashboard' as const, name: 'portainer', file: '', url: '', variant: 'svg' }, group: '', order: 2, url: '#', enabled: true, default: false, open_mode: 'iframe' as const, proxy: false, scale: 1 },
        { name: 'Grafana', color: '#F46800', icon: { type: 'dashboard' as const, name: 'grafana', file: '', url: '', variant: 'svg' }, group: '', order: 3, url: '#', enabled: true, default: false, open_mode: 'iframe' as const, proxy: false, scale: 1 },
      ];
    }

    return apps;
  });

  const previewGroups = $derived(wizardGroups.map(g => ({
    name: g.name, color: g.color, icon: g.icon, order: g.order, expanded: true
  })));

  const previewConfig = $derived<Config>({
    title: 'Muximux',
    navigation: {
      position: $selectedNavigation, width: '220px',
      auto_hide: navAutoHide, auto_hide_delay: navAutoHideDelay,
      show_on_hover: navShowOnHover, show_labels: $showLabels,
      show_logo: navShowLogo, show_app_colors: navShowAppColors,
      show_icon_background: navShowIconBg,
      icon_scale: navIconScale,
      show_splash_on_startup: navShowSplash,
      show_shadow: navShowShadow,
      floating_position: navFloatingPosition,
      bar_style: navBarStyle,
      hide_sidebar_footer: navHideSidebarFooter
    },
    groups: previewGroups, apps: previewApps
  });

  // Default to first app so color accents are visible in the preview
  let previewCurrentApp = $derived.by(() => {
    if (previewCurrentAppOverride) return previewCurrentAppOverride;
    return previewApps.length > 0 ? previewApps[0] : null;
  });
  let previewCurrentAppOverride = $state<App | null>(null);

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
      icon: { type: 'dashboard', name: '', file: '', url: '', variant: '' },
      color: '#22c55e',
      group: '',
      order: selectedCount,
      enabled: true,
      default: false,
      open_mode: 'iframe',
      proxy: false,
      scale: 1
    };

    selectedApps.update(apps => [...apps, newApp]);
    customApp = { name: '', url: '' };
    rebuildDndFromSelections();
  }

  // Group editing functions
  function updateGroupName(index: number, name: string) {
    const oldName = wizardGroups[index].name;
    if (oldName !== name && dndApps[oldName]) {
      dndApps[name] = dndApps[oldName];
      delete dndApps[oldName];
    }
    wizardGroups = wizardGroups.map((g, i) => i === index ? { ...g, name } : g);
  }

  function updateGroupColor(index: number, color: string) {
    wizardGroups = wizardGroups.map((g, i) => i === index ? { ...g, color } : g);
  }

  function deleteGroup(index: number) {
    const groupName = wizardGroups[index].name;
    if (dndApps[groupName]) {
      // Deselect all apps in this group
      for (const item of dndApps[groupName]) {
        const sel = appSelections.get(item.name);
        if (sel) {
          appSelections.set(item.name, { ...sel, selected: false });
        }
        // Remove from custom apps store too
        selectedApps.update(apps => apps.filter(a => a.name !== item.name));
      }
      delete dndApps[groupName];
    }
    wizardGroups = wizardGroups.filter((_, i) => i !== index);
  }

  function addGroup() {
    const existingNames = new Set(wizardGroups.map(g => g.name));
    let name = 'New Group';
    let num = 2;
    while (existingNames.has(name)) {
      name = `New Group ${num++}`;
    }
    wizardGroups = [...wizardGroups, {
      id: `grp-${++groupIdCounter}`,
      name,
      icon: { type: 'lucide' as const, name: '', file: '', url: '', variant: '' },
      color: '#22c55e',
      order: wizardGroups.length,
      expanded: true
    }];
    dndApps[name] = [];
  }

  function handleIconSelect(detail: { name: string; variant: string; type: string }) {
    const newIcon: AppIconConfig = { type: detail.type as AppIconConfig['type'], name: detail.name, file: '', url: '', variant: detail.variant };
    if (iconBrowserContext === 'app-override') {
      const existing = appOverrides.get(iconBrowserAppName);
      appOverrides.set(iconBrowserAppName, {
        color: existing?.color || getAppDisplayColor(iconBrowserAppName),
        icon: newIcon,
        open_mode: existing?.open_mode || 'iframe',
        proxy: existing?.proxy || false
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

    // Apply per-app customizations
    for (const app of apps) {
      const override = appOverrides.get(app.name);
      if (override) {
        app.color = override.color;
        if (override.icon.name) app.icon = override.icon;
        app.open_mode = override.open_mode;
        app.proxy = override.proxy;
      }
    }

    // Assign groups based on DnD state
    for (const [groupName, items] of Object.entries(dndApps)) {
      for (const item of items) {
        const app = apps.find(a => a.name === item.name);
        if (app) app.group = groupName;
      }
    }

    // Set first app as default if any
    if (apps.length > 0) {
      apps[0].default = true;
    }

    // Build groups from wizard state (strip internal _id)
    const groups: Group[] = wizardGroups.map(({ id: _id, ...g }, i) => ({ ...g, order: i }));

    // Build navigation config
    const navigation: NavigationConfig = {
      position: get(selectedNavigation),
      width: '220px',
      auto_hide: navAutoHide,
      auto_hide_delay: navAutoHideDelay,
      show_on_hover: navShowOnHover,
      show_labels: get(showLabels),
      show_logo: navShowLogo,
      show_app_colors: navShowAppColors,
      show_icon_background: navShowIconBg,
      icon_scale: navIconScale,
      show_splash_on_startup: navShowSplash,
      show_shadow: navShowShadow,
      floating_position: navFloatingPosition,
      bar_style: navBarStyle,
      hide_sidebar_footer: navHideSidebarFooter
    };

    // Capture current theme from stores
    const theme: ThemeConfig = {
      family: get(selectedFamily),
      variant: get(variantMode)
    };

    oncomplete?.({ apps, navigation, groups, theme, ...(needsSetup && authMethod ? { setup: buildSetupRequest() } : {}) });
  }

  function rebuildDndFromSelections() {
    const allSelected = new SvelteSet<string>();
    appSelections.forEach((value, key) => {
      if (value.selected) allSelected.add(key);
    });
    for (const app of get(selectedApps)) {
      allSelected.add(app.name);
    }

    // Remove deselected apps from ALL zones and deduplicate
    for (const groupName of Object.keys(dndApps)) {
      dndApps[groupName] = dedup(dndApps[groupName].filter(item => allSelected.has(item.name)));
    }

    // Track which apps are already placed in a group
    const placed = new SvelteSet<string>();
    for (const items of Object.values(dndApps)) {
      for (const item of items) placed.add(item.name);
    }

    // Place unplaced apps into their matching category group
    for (const name of allSelected) {
      if (placed.has(name)) continue;

      // Find the template to determine group
      const template = Object.values(popularApps).flat().find(a => a.name === name);
      // Also check custom apps (e.g. "Radarr 2" created by addInstanceOf) which store their group
      const customApp = get(selectedApps).find(a => a.name === name);
      const targetGroup = template?.group || customApp?.group || null;

      if (targetGroup && (wizardGroups.some(g => g.name === targetGroup) || template)) {
        // Ensure the group bucket exists (the $effect will auto-create the wizardGroup entry)
        if (!dndApps[targetGroup]) dndApps[targetGroup] = [];
        dndApps[targetGroup].push({ id: name, name });
      } else {
        // Custom app: use first existing group, or create "Other"
        const firstGroup = wizardGroups.length > 0 ? wizardGroups[0].name : 'Other';
        if (!dndApps[firstGroup]) dndApps[firstGroup] = [];
        dndApps[firstGroup].push({ id: name, name });
      }
    }

    // Ensure group buckets exist for all wizard groups
    for (const group of wizardGroups) {
      if (!dndApps[group.name]) {
        dndApps[group.name] = [];
      }
    }

    // Remove stale group buckets (no matching wizardGroup and empty)
    const validGroupNames = new SvelteSet(wizardGroups.map(g => g.name));
    for (const key of Object.keys(dndApps)) {
      if (!validGroupNames.has(key) && (!dndApps[key] || dndApps[key].length === 0)) {
        delete dndApps[key];
      }
    }
  }

  // Deduplicate DnD items by id (svelte-dnd-action can produce transient duplicates
  // during cross-zone drags, which triggers Svelte's each_key_duplicate error)
  function dedup(items: DndItem[]): DndItem[] {
    const seen = new SvelteSet<string>();
    return items.filter(item => {
      if (seen.has(item.id)) return false;
      seen.add(item.id);
      return true;
    });
  }

  function handleDndConsider(e: CustomEvent<DndEvent<DndItem>>, groupName: string) {
    dndApps[groupName] = dedup(e.detail.items);
  }
  function handleDndFinalize(e: CustomEvent<DndEvent<DndItem>>, groupName: string) {
    dndApps[groupName] = dedup(e.detail.items);
  }

  // Group reordering via DnD
  function handleGroupDndConsider(e: CustomEvent<DndEvent<WizardGroup>>) {
    wizardGroups = e.detail.items;
  }
  function handleGroupDndFinalize(e: CustomEvent<DndEvent<WizardGroup>>) {
    wizardGroups = e.detail.items;
  }

  function getGroupColor(group: string): string {
    const colors: Record<string, string> = {
      'Media': '#E5A00D',
      'Downloads': '#00CCFF',
      'System': '#F46800',
      'Utilities': '#0082C9',
      'AI': '#A855F7',
      'Other': '#22c55e'
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
      icon: existing?.icon || getAppDisplayIcon(appName),
      open_mode: existing?.open_mode || 'iframe',
      proxy: existing?.proxy || false
    });
  }

  // Svelte action: position help tooltips using fixed positioning to escape
  // overflow clipping and stacking contexts created by animate:flip
  function positionTooltip(trigger: HTMLElement) {
    const tooltip = trigger.querySelector(':scope > .help-tooltip') as HTMLElement | null;
    if (!tooltip) return;
    function show() {
      const rect = trigger.getBoundingClientRect();
      // Vertical: prefer above, fall back to below if near top
      if (rect.top > 120) {
        tooltip!.style.top = `${rect.top - 6}px`;
        tooltip!.style.transform = 'translate(-50%, -100%)';
      } else {
        tooltip!.style.top = `${rect.bottom + 6}px`;
        tooltip!.style.transform = 'translateX(-50%)';
      }
      // Horizontal: center on trigger, clamped to viewport edges
      const centerX = rect.left + rect.width / 2;
      tooltip!.style.left = `${Math.max(130, Math.min(centerX, window.innerWidth - 130))}px`;
    }
    trigger.addEventListener('mouseenter', show);
    return { destroy() { trigger.removeEventListener('mouseenter', show); } };
  }

  function getAppOpenMode(appName: string): App['open_mode'] {
    return appOverrides.get(appName)?.open_mode || 'iframe';
  }

  function getAppProxy(appName: string): boolean {
    return appOverrides.get(appName)?.proxy || false;
  }

  function updateAppSetting(appName: string, key: 'open_mode' | 'proxy', value: App['open_mode'] | boolean) {
    const existing = appOverrides.get(appName);
    appOverrides.set(appName, {
      color: existing?.color || getAppDisplayColor(appName),
      icon: existing?.icon || getAppDisplayIcon(appName),
      open_mode: key === 'open_mode' ? value as App['open_mode'] : (existing?.open_mode || 'iframe'),
      proxy: key === 'proxy' ? value as boolean : (existing?.proxy || false)
    });
  }

  function removeApp(appName: string) {
    // Deselect from popular apps
    const current = appSelections.get(appName);
    if (current?.selected) {
      appSelections.set(appName, { ...current, selected: false });
    }
    // Remove from custom apps
    selectedApps.update(apps => apps.filter(a => a.name !== appName));
    // Remove from DnD state
    for (const key of Object.keys(dndApps)) {
      dndApps[key] = dndApps[key].filter(item => item.name !== appName);
    }
  }

  function renameApp(oldName: string, newName: string) {
    const trimmed = newName.trim();
    if (!trimmed || trimmed === oldName) return;
    // Update appSelections (popular apps)
    const sel = appSelections.get(oldName);
    if (sel) {
      appSelections.delete(oldName);
      appSelections.set(trimmed, sel);
    }
    // Update custom apps
    selectedApps.update(apps => apps.map(a =>
      a.name === oldName ? { ...a, name: trimmed } : a));
    // Update overrides
    const ovr = appOverrides.get(oldName);
    if (ovr) {
      appOverrides.delete(oldName);
      appOverrides.set(trimmed, ovr);
    }
    // Update DnD state
    for (const key of Object.keys(dndApps)) {
      dndApps[key] = dndApps[key].map(item =>
        item.name === oldName ? { ...item, name: trimmed } : item);
    }
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
      if (securityStepValid) handleSecuritySubmit();
    } else if (step === 'apps') {
      if (selectedCount + $selectedApps.length > 0) nextStep();
    } else if (step === 'navigation' || step === 'theme') {
      nextStep();
    } else if (step === 'complete') {
      handleComplete();
    }
  }
</script>

<div class="fixed inset-0 z-50 bg-gray-900 overflow-hidden flex flex-col" onkeydown={handleGlobalKeydown} role="dialog" aria-label="Setup wizard" tabindex="0">
  <!-- Top bar nav preview — rendered as wizard root flex child so it sits above the stepper -->
  {#if ($currentStep === 'navigation' || $currentStep === 'theme') && $selectedNavigation === 'top'}
    <div class="flex-shrink-0" style="z-index: 20;">
      <Navigation
        apps={previewApps}
        currentApp={previewCurrentApp}
        config={previewConfig}
        showHealth={false}
        onselect={(app) => { previewCurrentAppOverride = app; }}
      />
    </div>
  {/if}

  <!-- Progress stepper -->
  <div class="flex-shrink-0 px-8 pt-6 relative z-10" style="background: inherit;">
    <div class="max-w-2xl mx-auto">
      <div class="stepper-track">
        <!-- Background rail -->
        <div class="stepper-rail"></div>
        <!-- Filled portion of the rail -->
        <div class="stepper-rail-fill" style="width: {$stepProgress / (steps.length - 1) * 100}%"></div>

        {#each steps as step, i (i)}
          <div class="stepper-node">
            <div
              class="stepper-circle transition-all duration-300
                     {i < $stepProgress ? 'completed' : i === $stepProgress ? 'active' : 'pending'}"
            >
              {#if i < $stepProgress}
                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                </svg>
              {:else}
                <span class="text-xs font-semibold">{i + 1}</span>
              {/if}
            </div>
            <span class="stepper-label transition-colors duration-300
                         {i <= $stepProgress ? 'text-gray-200' : 'text-gray-400'}">{step}</span>
          </div>
        {/each}
      </div>
    </div>
  </div>

  <!-- Content area — grid stacking prevents layout jump during step transitions -->
  <div class="flex-1 overflow-hidden" style="display: grid; grid-template: 1fr / 1fr;">
    {#if $currentStep === 'navigation' || $currentStep === 'theme'}
      <!-- Style + Theme steps: live Navigation preview alongside content -->
      <div
        class="overflow-hidden h-full"
        style="grid-area: 1/1;"
        class:flex={$selectedNavigation === 'left' || $selectedNavigation === 'right'}
        class:flex-row={$selectedNavigation === 'left'}
        class:flex-row-reverse={$selectedNavigation === 'right'}
        in:fly={{ x: 30, duration: 300 }}
        out:fade={{ duration: 150 }}
      >
        <!-- Fixed-size wrapper — absolute positioning lets nav span full wizard height/width -->
        {#if $selectedNavigation === 'left' || $selectedNavigation === 'right'}
          <div style="flex-shrink: 0; min-width: 220px; position: absolute; top: 0; bottom: 0; {$selectedNavigation === 'right' ? 'right: 0; display: flex; justify-content: flex-end;' : 'left: 0;'} overflow: visible; z-index: 20;">
            <Navigation
              apps={previewApps}
              currentApp={previewCurrentApp}
              config={previewConfig}
              showHealth={false}
              onselect={(app) => { previewCurrentAppOverride = app; }}
            />
          </div>
        {:else if $selectedNavigation === 'top' || $selectedNavigation === 'bottom'}
          <!-- Top/bottom nav is rendered outside the content grid as a wizard root flex child -->
        {:else}
          <Navigation
            apps={previewApps}
            currentApp={previewCurrentApp}
            config={previewConfig}
            showHealth={false}
            onselect={(app) => { previewCurrentAppOverride = app; }}
          />
        {/if}

        <div class="flex-1 overflow-y-auto px-8 py-6 relative" style="background: var(--bg-base); {$selectedNavigation === 'left' ? 'margin-left: 220px;' : $selectedNavigation === 'right' ? 'margin-right: 220px;' : ''}">
          <div class="max-w-3xl mx-auto">
            {#if $currentStep === 'navigation'}
            <div class="text-center mb-6">
              <h2 class="text-2xl font-bold text-white mb-2">Choose Your Navigation Style</h2>
              <p class="text-gray-400">Select how you want to navigate between your apps</p>
            </div>

            <!-- Position selector buttons -->
            <div class="flex flex-wrap justify-center gap-2 mb-6">
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

            <!-- Bar Style (right below position selector, only for top/bottom) -->
            {#if $selectedNavigation === 'top' || $selectedNavigation === 'bottom'}
              <div class="flex justify-center gap-2 mb-4">
                {#each [
                  { value: 'grouped', label: 'Group Dropdowns' },
                  { value: 'flat', label: 'Flat List' }
                ] as style (style.value)}
                  <button
                    class="px-3 py-1.5 rounded-lg border text-xs font-medium transition-all
                           {navBarStyle === style.value
                             ? 'border-brand-500 bg-brand-500/15 text-white'
                             : 'border-gray-700 hover:border-gray-500 bg-gray-800/50 text-gray-400 hover:text-white'}"
                    onclick={() => navBarStyle = style.value as typeof navBarStyle}
                  >
                    {style.label}
                  </button>
                {/each}
              </div>
            {/if}

            <!-- Floating position selector -->
            {#if $selectedNavigation === 'floating'}
              <div class="flex flex-wrap justify-center gap-2 mb-4">
                {#each [
                  { value: 'bottom-right', label: 'Bottom Right' },
                  { value: 'bottom-left', label: 'Bottom Left' },
                  { value: 'top-right', label: 'Top Right' },
                  { value: 'top-left', label: 'Top Left' }
                ] as fp (fp.value)}
                  <button
                    class="px-3 py-1.5 rounded-lg border text-xs font-medium transition-all
                           {navFloatingPosition === fp.value
                             ? 'border-brand-500 bg-brand-500/15 text-white'
                             : 'border-gray-700 hover:border-gray-500 bg-gray-800/50 text-gray-400 hover:text-white'}"
                    onclick={() => navFloatingPosition = fp.value as typeof navFloatingPosition}
                  >
                    {fp.label}
                  </button>
                {/each}
              </div>
            {/if}

            <!-- Settings controls -->
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 max-w-lg mx-auto">
              <label class="flex items-center gap-3 p-3 bg-gray-800/50 rounded-lg border border-gray-700 cursor-pointer">
                <input type="checkbox" bind:checked={$showLabels}
                  class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500" />
                <div>
                  <div class="text-sm text-white">Show Labels</div>
                  <div class="text-xs text-gray-400">Display app names next to icons</div>
                </div>
              </label>

              <label class="flex items-center gap-3 p-3 bg-gray-800/50 rounded-lg border border-gray-700 cursor-pointer">
                <input type="checkbox" bind:checked={navShowLogo}
                  class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500" />
                <div>
                  <div class="text-sm text-white">Show Logo</div>
                  <div class="text-xs text-gray-400">Display the Muximux logo in the menu</div>
                </div>
              </label>

              <label class="flex items-center gap-3 p-3 bg-gray-800/50 rounded-lg border border-gray-700 cursor-pointer">
                <input type="checkbox" bind:checked={navShowAppColors}
                  class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500" />
                <div>
                  <div class="text-sm text-white">App Color Accents</div>
                  <div class="text-xs text-gray-400">Highlight the active app with its color</div>
                </div>
              </label>

              <label class="flex items-center gap-3 p-3 bg-gray-800/50 rounded-lg border border-gray-700 cursor-pointer">
                <input type="checkbox" bind:checked={navShowIconBg}
                  class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500" />
                <div>
                  <div class="text-sm text-white">Icon Background</div>
                  <div class="text-xs text-gray-400">Show colored circle behind app icons</div>
                </div>
              </label>

              <div class="p-3 bg-gray-800/50 rounded-lg border border-gray-700 sm:col-span-2">
                <div class="flex items-center justify-between mb-2">
                  <div>
                    <div class="text-sm text-white">Icon Size</div>
                    <div class="text-xs text-gray-400">Scale app icons in the navigation</div>
                  </div>
                  <span class="text-sm text-gray-300 tabular-nums">{navIconScale}×</span>
                </div>
                <input type="range" min="0.5" max="2" step="0.25"
                  bind:value={navIconScale}
                  class="w-full accent-brand-500" />
              </div>

              <label class="flex items-center gap-3 p-3 bg-gray-800/50 rounded-lg border border-gray-700 cursor-pointer sm:col-span-2 sm:max-w-[calc(50%-0.375rem)]">
                <input type="checkbox" bind:checked={navShowSplash}
                  class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500" />
                <div>
                  <div class="text-sm text-white">Start on Overview</div>
                  <div class="text-xs text-gray-400">Show the dashboard overview when Muximux opens</div>
                </div>
              </label>

              <div class="p-3 bg-gray-800/50 rounded-lg border border-gray-700 sm:col-span-2">
                <label class="flex items-center gap-3 cursor-pointer">
                  <input type="checkbox" bind:checked={navAutoHide}
                    class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500" />
                  <div class="flex-1">
                    <div class="text-sm text-white">Auto-hide Menu</div>
                    <div class="text-xs text-gray-400">Automatically collapse the menu after inactivity</div>
                  </div>
                </label>
                {#if navAutoHide}
                  <div class="flex items-center gap-3 mt-3 pt-3 border-t border-gray-700">
                    <div class="flex-1 text-xs text-gray-400 pl-7">Hide after</div>
                    <select bind:value={navAutoHideDelay}
                      class="px-2 py-1 text-xs bg-gray-700 border border-gray-600 rounded text-white focus:ring-brand-500 focus:border-brand-500">
                      <option value="0.25s">0.25s</option>
                      <option value="0.5s">0.5s</option>
                      <option value="1s">1s</option>
                      <option value="2s">2s</option>
                      <option value="3s">3s</option>
                    </select>
                  </div>
                  <label class="flex items-center gap-3 mt-2 pl-7 cursor-pointer">
                    <input type="checkbox" bind:checked={navShowShadow}
                      class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500" />
                    <div class="text-xs text-gray-400">Shadow — show a drop shadow on the expanded menu</div>
                  </label>
                {/if}
              </div>

              {#if $selectedNavigation === 'left' || $selectedNavigation === 'right'}
                <div class="p-3 bg-gray-800/50 rounded-lg border border-gray-700 sm:col-span-2">
                  <label class="flex items-center gap-3 cursor-pointer">
                    <input type="checkbox" bind:checked={navHideSidebarFooter}
                      class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500" />
                    <div class="flex-1">
                      <div class="text-sm text-white">Collapsible Footer</div>
                      <div class="text-xs text-gray-400">Hide utility buttons in a drawer that reveals on hover</div>
                    </div>
                  </label>
                </div>
              {/if}

            </div>
            {:else}
            <!-- Theme step content -->
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
                {@const wantDark = $variantMode === 'system' ? $systemTheme === 'dark' : $variantMode === 'dark'}
                {@const preferred = wantDark ? family.darkTheme?.preview : family.lightTheme?.preview}
                {@const preview = preferred || family.darkTheme?.preview || family.lightTheme?.preview}
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
            {/if}
          </div>
        </div>
      </div>
    {:else}
      <!-- All other steps: normal padded, scrollable layout -->
      <div class="overflow-y-auto px-8 py-6" style="grid-area: 1/1;">
        <div class="max-w-4xl mx-auto" style="display: grid; grid-template: 1fr / 1fr;">
      <!-- Step 1: Welcome -->
      {#if $currentStep === 'welcome'}
        <div class="text-center py-12" style="grid-area: 1/1;" in:fly={{ x: 30, duration: 300 }} out:fade={{ duration: 150 }}>
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
          <p class="text-xl text-gray-400 mb-8 max-w-3xl mx-auto">
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
              <h2 class="font-semibold text-white mb-1 text-base">Embedded Apps</h2>
              <p class="text-sm text-gray-400">View all your services in iframes without leaving the dashboard</p>
            </div>

            <div class="p-4 bg-gray-800/50 rounded-lg border border-gray-700">
              <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center mb-3">
                <svg class="w-5 h-5 text-brand-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </div>
              <h2 class="font-semibold text-white mb-1 text-base">Health Monitoring</h2>
              <p class="text-sm text-gray-400">See at a glance which services are online and healthy</p>
            </div>

            <div class="p-4 bg-gray-800/50 rounded-lg border border-gray-700">
              <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center mb-3">
                <svg class="w-5 h-5 text-brand-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
              </div>
              <h2 class="font-semibold text-white mb-1 text-base">Quick Access</h2>
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
        <div class="py-6" style="grid-area: 1/1;" in:fly={{ x: 30, duration: 300 }} out:fade={{ duration: 150 }}>
          <div class="text-center mb-8">
            <h2 class="text-2xl font-bold text-white mb-2">Secure Your Dashboard</h2>
            <p class="text-gray-400">Choose how you want to protect access to Muximux</p>
          </div>

          <!-- Method selection cards (accordion — form expands inline) -->
            <div class="max-w-2xl mx-auto space-y-3">
              <!-- Builtin password -->
              <div
                class="rounded-xl border text-left transition-all overflow-hidden
                       {authMethod === 'builtin' ? 'border-brand-500 bg-brand-500/10' : 'border-gray-700 bg-gray-800/50 hover:border-gray-600'}"
              >
                <button class="w-full p-4 flex items-start gap-4" onclick={async () => { authMethod = authMethod === 'builtin' ? null : 'builtin'; if (authMethod === 'builtin') { await tick(); document.getElementById('setup-username')?.focus(); } }}>
                  <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center flex-shrink-0 mt-0.5">
                    <svg class="w-5 h-5 text-brand-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <rect x="3" y="11" width="18" height="11" rx="2" />
                      <path d="M7 11V7a5 5 0 0110 0v4" />
                    </svg>
                  </div>
                  <div class="flex-1 text-left">
                    <div class="flex items-center gap-2">
                      <h3 class="font-semibold text-white">Create a password</h3>
                      <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-brand-500 text-white uppercase tracking-wider">Recommended</span>
                    </div>
                    <p class="text-sm text-gray-400 mt-1">Set up a username and password to protect your dashboard</p>
                  </div>
                </button>
                {#if authMethod === 'builtin'}
                  <div class="px-4 pb-4 pt-0 space-y-4 ml-14" in:fly={{ y: -8, duration: 200 }}>
                    <div class="border-t border-gray-700 pt-4">
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
                {/if}
              </div>

              <!-- Forward auth -->
              <div
                class="rounded-xl border text-left transition-all overflow-hidden
                       {authMethod === 'forward_auth' ? 'border-brand-500 bg-brand-500/10' : 'border-gray-700 bg-gray-800/50 hover:border-gray-600'}"
              >
                <button class="w-full p-4 flex items-start gap-4" onclick={async () => { authMethod = authMethod === 'forward_auth' ? null : 'forward_auth'; if (authMethod === 'forward_auth') { await tick(); document.getElementById('setup-proxies')?.focus(); } }}>
                  <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center flex-shrink-0 mt-0.5">
                    <svg class="w-5 h-5 text-brand-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                    </svg>
                  </div>
                  <div class="text-left">
                    <h3 class="font-semibold text-white">I use an auth proxy</h3>
                    <p class="text-sm text-gray-400 mt-1">Authelia, Authentik, or another reverse proxy handles authentication</p>
                  </div>
                </button>
                {#if authMethod === 'forward_auth'}
                  <div class="px-4 pb-4 pt-0 space-y-4 ml-14" in:fly={{ y: -8, duration: 200 }}>
                    <div class="border-t border-gray-700 pt-4">
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
                {/if}
              </div>

              <!-- None -->
              <div
                class="rounded-xl border text-left transition-all overflow-hidden
                       {authMethod === 'none' ? 'border-amber-500 bg-amber-500/10' : 'border-gray-700 bg-gray-800/50 hover:border-gray-600'}"
              >
                <button class="w-full p-4 flex items-start gap-4" onclick={async () => { authMethod = authMethod === 'none' ? null : 'none'; if (authMethod === 'none') { await tick(); (document.querySelector('#setup-none-ack') as HTMLElement)?.focus(); } }}>
                  <div class="w-10 h-10 rounded-lg bg-amber-500/20 flex items-center justify-center flex-shrink-0 mt-0.5">
                    <svg class="w-5 h-5 text-amber-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" />
                      <line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
                    </svg>
                  </div>
                  <div class="text-left">
                    <h3 class="font-semibold text-white">No authentication</h3>
                    <p class="text-sm text-gray-400 mt-1">Anyone with network access gets full control</p>
                  </div>
                </button>
                {#if authMethod === 'none'}
                  <div class="px-4 pb-4 pt-0 ml-14" in:fly={{ y: -8, duration: 200 }}>
                    <div class="border-t border-gray-700 pt-4">
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
                        <input id="setup-none-ack" type="checkbox" bind:checked={acknowledgeRisk}
                          class="mt-1 w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500" />
                        <span class="text-sm text-gray-400">I understand the risks and want to proceed without authentication</span>
                      </label>
                    </div>
                  </div>
                {/if}
              </div>
            </div>

        </div>

      <!-- Step 2: Add Apps (two-column layout with groups) -->
      {:else if $currentStep === 'apps'}
        <div class="py-6" style="grid-area: 1/1;" in:fly={{ x: 30, duration: 300 }} out:fade={{ duration: 150 }}>
          <div class="text-center mb-8">
            <h2 class="text-2xl font-bold text-white mb-2">What apps do you have?</h2>
            <p class="text-gray-400">Select the services you're already running</p>
          </div>

          <div class="apps-two-col gap-6">
            <!-- LEFT COLUMN: Custom app + template apps (scrollable) -->
            <div class="apps-left-col space-y-6">
              <div class="flex items-center gap-2 pb-2 border-b border-gray-700/50">
                <svg class="w-5 h-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                </svg>
                <div>
                  <h3 class="text-sm font-semibold text-gray-300">App Catalog</h3>
                  <p class="text-xs text-gray-500">Click to add apps to your menu</p>
                </div>
              </div>
              <!-- Custom App Quick Add -->
              <div class="flex gap-2 items-end">
                <div class="flex-1">
                  <label for="custom-name" class="block text-xs text-gray-400 mb-1">Custom app</label>
                  <input
                    id="custom-name"
                    type="text"
                    bind:value={customApp.name}
                    class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white text-sm
                           focus:outline-none focus:ring-2 focus:ring-brand-500"
                    placeholder="App name"
                  />
                </div>
                <div class="flex-1">
                  <input
                    id="custom-url"
                    type="url"
                    bind:value={customApp.url}
                    class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white text-sm
                           focus:outline-none focus:ring-2 focus:ring-brand-500"
                    placeholder="http://localhost:8080"
                  />
                </div>
                <button
                  class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md disabled:opacity-50 flex-shrink-0"
                  disabled={!customApp.name || !customApp.url}
                  onclick={addCustomApp}
                >
                  Add
                </button>
              </div>

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
                        onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), toggleApp(app))}
                        role="checkbox"
                        aria-checked={selection?.selected}
                        tabindex="0"
                      >
                        <!-- Checkbox + Add Instance -->
                        <div class="absolute top-2.5 right-2.5 flex items-center gap-1">
                          {#if selection?.selected}
                            <button
                              class="w-5 h-5 rounded border border-brand-500 bg-brand-500/20 flex items-center justify-center
                                     hover:bg-brand-500/40 transition-colors"
                              onclick={(e) => { e.stopPropagation(); addInstanceOf(app); }}
                              title="Add another {app.name}"
                            >
                              <svg class="w-3 h-3 text-brand-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M12 4v16m8-8H4" />
                              </svg>
                            </button>
                          {/if}
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
                          </div>
                        </div>
                      </div>
                    {/each}
                  </div>
                </div>
              {/each}
            </div>

            <!-- RIGHT COLUMN: Groups (sticky on desktop) -->
            <div class="apps-right-col">
              <div class="apps-right-sticky space-y-6">
                <div class="flex items-center gap-2 pb-2 border-b border-gray-700/50">
                  <svg class="w-5 h-5 text-brand-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
                  </svg>
                  <div>
                    <h3 class="text-sm font-semibold text-gray-300">Your Menu</h3>
                    <p class="text-xs text-gray-500">Drag apps between groups to organize</p>
                  </div>
                </div>

                {#if selectedCount + $selectedApps.length === 0}
                  <p class="text-sm text-gray-500 italic">Select apps from the left to get started</p>
                {:else}
                <div>
                  <h3 class="text-sm font-semibold text-gray-300 mb-3">
                    Groups ({wizardGroups.length})
                  </h3>
                  {#if wizardGroups.length > 0}
                    <div class="space-y-2"
                         use:dndzone={{items: wizardGroups, flipDurationMs, type: 'wizard-groups', dropTargetStyle: {}}}
                         onconsider={handleGroupDndConsider}
                         onfinalize={handleGroupDndFinalize}>
                      {#each wizardGroups as group, i (group.id)}
                        {@const groupApps = dndApps[group.name] || []}
                        <div class="rounded-lg border border-gray-700 bg-gray-800/30 overflow-hidden cursor-grab"
                             animate:flip={{duration: flipDurationMs}}>
                          <div class="flex items-center gap-2 p-2.5 group/grpdrag">
                            <svg class="w-4 h-4 text-gray-600 group-hover/grpdrag:text-gray-400 flex-shrink-0 transition-colors" viewBox="0 0 24 24" fill="currentColor">
                              <circle cx="9" cy="5" r="1.5"/><circle cx="15" cy="5" r="1.5"/><circle cx="9" cy="12" r="1.5"/><circle cx="15" cy="12" r="1.5"/><circle cx="9" cy="19" r="1.5"/><circle cx="15" cy="19" r="1.5"/>
                            </svg>
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
                                 onconsider={(e) => handleDndConsider(e, group.name)}
                                 onfinalize={(e) => handleDndFinalize(e, group.name)}>
                              {#each groupApps as item (item.id)}
                                {@const appColor = getAppDisplayColor(item.name)}
                                {@const appIcon = getAppDisplayIcon(item.name)}
                                <div class="p-2 rounded bg-gray-800/70 cursor-grab group/drag text-sm text-white"
                                     animate:flip={{duration: flipDurationMs}}>
                                  <div class="flex items-center gap-1.5 min-w-0">
                                    <svg class="w-3.5 h-3.5 text-gray-600 group-hover/drag:text-gray-400 flex-shrink-0 transition-colors" viewBox="0 0 24 24" fill="currentColor">
                                      <circle cx="9" cy="5" r="1.5"/><circle cx="15" cy="5" r="1.5"/><circle cx="9" cy="12" r="1.5"/><circle cx="15" cy="12" r="1.5"/><circle cx="9" cy="19" r="1.5"/><circle cx="15" cy="19" r="1.5"/>
                                    </svg>
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
                                    <input
                                      type="text"
                                      value={item.name}
                                      onchange={(e) => renameApp(item.name, e.currentTarget.value)}
                                      onclick={(e) => e.stopPropagation()}
                                      class="text-sm text-white truncate flex-1 min-w-0 bg-transparent border-0 border-b border-transparent hover:border-gray-600 focus:border-brand-500 focus:outline-none px-0 py-0"
                                    />
                                    <button
                                      class="p-1 text-gray-500 hover:text-red-400 transition-opacity flex-shrink-0"
                                      onclick={() => removeApp(item.name)}
                                      aria-label="Remove {item.name}"
                                    >
                                      <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                      </svg>
                                    </button>
                                  </div>
                                  <input
                                    type="url"
                                    value={getAppUrl(item.name)}
                                    oninput={(e) => updateAppUrl(item.name, e.currentTarget.value)}
                                    onclick={(e) => e.stopPropagation()}
                                    class="mt-1 ml-[66px] px-1.5 py-0.5 text-[11px] bg-gray-700 border border-gray-600 rounded
                                           text-gray-300 placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
                                    placeholder="http://localhost:8080"
                                    style="width: calc(100% - 66px)"
                                  />
                                  <div class="flex items-center gap-2 mt-1 ml-[66px]">
                                    <select
                                      value={getAppOpenMode(item.name)}
                                      onchange={(e) => updateAppSetting(item.name, 'open_mode', e.currentTarget.value as App['open_mode'])}
                                      onclick={(e) => e.stopPropagation()}
                                      class="text-[11px] px-1.5 py-0.5 bg-gray-700 border border-gray-600 rounded text-gray-300 focus:outline-none focus:ring-1 focus:ring-brand-500"
                                    >
                                      {#each openModes as mode (mode.value)}
                                        <option value={mode.value}>{mode.label}</option>
                                      {/each}
                                    </select>
                                    <span class="help-trigger relative ml-0.5" use:positionTooltip>
                                      <svg class="w-3 h-3 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                        <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                                      </svg>
                                      <span class="help-tooltip">
                                        <b>Embedded</b> — loads inside Muximux in an iframe. Best for most apps.<br/>
                                        <b>New Tab</b> — opens in a separate browser tab.<br/>
                                        <b>New Window</b> — opens in a popup window.
                                      </span>
                                    </span>
                                    <label class="flex items-center gap-1 cursor-pointer">
                                      <input
                                        type="checkbox"
                                        checked={getAppProxy(item.name)}
                                        onclick={(e) => e.stopPropagation()}
                                        onchange={(e) => updateAppSetting(item.name, 'proxy', e.currentTarget.checked)}
                                        class="w-3 h-3 rounded border-gray-600 text-brand-500"
                                      />
                                      <span class="text-[11px] text-gray-400">Proxy</span>
                                      <span class="help-trigger relative ml-0.5" use:positionTooltip>
                                        <svg class="w-3 h-3 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                          <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                                        </svg>
                                        <span class="help-tooltip">
                                          Routes traffic through Muximux so the app doesn't need to be directly reachable from your browser. Enable this if the app is on an internal network or a different host that your browser can't access directly.
                                        </span>
                                      </span>
                                    </label>
                                  </div>
                                </div>
                              {/each}
                            </div>
                            {#if groupApps.length === 0}
                              <p class="text-xs text-gray-600 text-center py-1 mt-1">Drop apps here</p>
                            {/if}
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
                {/if}
              </div>
            </div>
          </div>
        </div>

      <!-- Step 5: Complete -->
      {:else if $currentStep === 'complete'}
        <div class="text-center py-12" style="grid-area: 1/1;" in:fly={{ x: 30, duration: 300 }} out:fade={{ duration: 150 }}>
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
    {/if}
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
              class="px-6 py-2 bg-brand-600 hover:bg-brand-700 text-white rounded-md transition-colors disabled:opacity-50"
              disabled={!securityStepValid}
              onclick={handleSecuritySubmit}
            >
              Continue
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

  <!-- Bottom dock nav preview — rendered as wizard root flex child so it sits below buttons -->
  {#if ($currentStep === 'navigation' || $currentStep === 'theme') && $selectedNavigation === 'bottom'}
    <div class="flex-shrink-0" style="z-index: 20;">
      <Navigation
        apps={previewApps}
        currentApp={previewCurrentApp}
        config={previewConfig}
        showHealth={false}
        onselect={(app) => { previewCurrentAppOverride = app; }}
      />
    </div>
  {/if}
</div>

<!-- Icon Browser modal -->
{#if iconBrowserContext !== null}
  {@const browserIcon = iconBrowserContext === 'app-override'
    ? appOverrides.get(iconBrowserAppName)?.icon || getAppDisplayIcon(iconBrowserAppName)
    : wizardGroups[iconBrowserContext as number]?.icon}
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
  /* Progress stepper */
  .stepper-track {
    position: relative;
    display: flex;
    justify-content: space-between;
    padding-bottom: 4px;
  }

  .stepper-rail,
  .stepper-rail-fill {
    position: absolute;
    top: 14px; /* vertically center on the 28px circles */
    left: 14px;
    right: 14px;
    height: 2px;
    border-radius: 1px;
  }

  .stepper-rail {
    background: var(--border-subtle, #374151);
  }

  .stepper-rail-fill {
    background: var(--accent-primary, #6366f1);
    transition: width 0.4s ease;
  }

  .stepper-node {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    position: relative; /* sit above the rail */
    z-index: 1;
  }

  .stepper-circle {
    width: 28px;
    height: 28px;
    border-radius: 9999px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    border: 2px solid transparent;
  }

  .stepper-circle.completed {
    background: var(--accent-primary, #6366f1);
    border-color: var(--accent-primary, #6366f1);
    color: #fff;
  }

  .stepper-circle.active {
    background: var(--bg-surface, #1f2937);
    border-color: var(--accent-primary, #6366f1);
    color: #fff;
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent-primary, #6366f1) 25%, transparent);
  }

  .stepper-circle.pending {
    background: var(--bg-surface, #1f2937);
    border-color: var(--border-subtle, #374151);
    color: var(--text-muted, #6b7280);
  }

  .stepper-label {
    font-size: 11px;
    font-weight: 500;
    white-space: nowrap;
  }

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
  }

  /* Help tooltips — use position:fixed so they escape overflow clipping
     and stacking contexts from animate:flip transforms. Coordinates are
     set by the positionTooltip Svelte action on mouseenter. */
  .help-tooltip {
    display: none;
    position: fixed;
    width: 240px;
    padding: 8px 10px;
    border-radius: 8px;
    background: #1f2937;
    border: 1px solid #374151;
    color: #d1d5db;
    font-size: 11px;
    line-height: 1.4;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
    z-index: 9999;
    pointer-events: none;
  }

  .help-trigger:hover > .help-tooltip {
    display: block;
  }

</style>

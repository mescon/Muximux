<script lang="ts">
  import { onMount, tick, untrack } from 'svelte';
  import { fade, fly } from 'svelte/transition';
  import { flip } from 'svelte/animate';
  import type { App, Config, Group } from '$lib/types';
  import IconBrowser from './IconBrowser.svelte';
  import AppIcon from './AppIcon.svelte';
  import KeybindingsEditor from './KeybindingsEditor.svelte';
  import { get } from 'svelte/store';
  import { resolvedTheme, allThemes, isDarkTheme, saveCustomThemeToServer, deleteCustomThemeFromServer, getCurrentThemeVariables, themeVariableGroups, sanitizeThemeId, selectedFamily, variantMode, themeFamilies, setThemeFamily, setVariantMode } from '$lib/themeStore';
  import { isMobileViewport } from '$lib/useSwipe';
  import { exportConfig, parseImportedConfig, type ImportedConfig, listUsers, createUser, updateUser, deleteUserAccount, changeAuthMethod, fetchSystemInfo, checkForUpdates } from '$lib/api';
  import type { SystemInfo, UpdateInfo } from '$lib/types';
  import { changePassword, isAdmin, currentUser } from '$lib/authStore';
  import type { UserInfo, ChangeAuthMethodRequest } from '$lib/types';
  import { toasts } from '$lib/toastStore';
  import { getKeybindingsForConfig } from '$lib/keybindingsStore';
  import { dndzone, type DndEvent } from 'svelte-dnd-action';
  import { appSchema, groupSchema, extractErrors } from '$lib/schemas';
  import { marked } from 'marked';
  import { popularApps, templateToApp, type PopularAppTemplate } from '$lib/popularApps';

  let {
    config,
    apps,
    initialTab = 'general',
    onclose,
    onsave,
  }: {
    config: Config;
    apps: App[];
    initialTab?: 'general' | 'apps' | 'theme' | 'keybindings' | 'security' | 'about';
    onclose?: () => void;
    onsave?: (config: Config) => void;
  } = $props();

  // Exported: returns true if Escape was consumed by closing an inner sub-modal.
  export function handleEscape(): boolean {
    if (showIconBrowser) { showIconBrowser = false; iconBrowserTarget = null; return true; }
    if (editingApp) { editingApp = null; return true; }
    if (editingGroup) { editingGroup = null; return true; }
    if (showAddApp) { showAddApp = false; return true; }
    if (showAddGroup) { showAddGroup = false; return true; }
    if (pendingImport) { pendingImport = null; showImportConfirm = false; return true; }
    return false; // No sub-modal was open; caller should close Settings
  }

  let isMobile = $state(false);

  onMount(() => {
    isMobile = isMobileViewport();
    const handleResize = () => { isMobile = isMobileViewport(); };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  });

  // Active tab
  let activeTab = $state(untrack(() => initialTab ?? 'general'));

  // Local copy of config for editing
  let localConfig = $state(untrack(() => JSON.parse(JSON.stringify(config)) as Config));
  let localApps = $state(untrack(() => JSON.parse(JSON.stringify(apps)) as App[]));

  // Icon browser state
  let showIconBrowser = $state(false);
  let iconBrowserTarget = $state<'newApp' | 'editApp' | 'newGroup' | 'editGroup' | null>(null);

  // Drag and drop config
  const flipDurationMs = 200;

  // Track keybindings changes
  let keybindingsChanged = $state(false);

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
  let addUserLoading = $state(false);
  let addUserError = $state<string | null>(null);

  // Delete user confirmation
  let confirmDeleteUser = $state<string | null>(null);

  // Auth method switching
  let selectedAuthMethod = $state<'builtin' | 'forward_auth' | 'none'>('none');
  let methodTrustedProxies = $state('');
  let _methodHeaders = $state<Record<string, string>>({});
  let methodLoading = $state(false);
  let methodError = $state<string | null>(null);

  // Forward auth preset & header fields
  let faPreset = $state<'authelia' | 'authentik' | 'custom'>('authelia');
  let faShowAdvanced = $state(false);
  let faHeaderUser = $state('Remote-User');
  let faHeaderEmail = $state('Remote-Email');
  let faHeaderGroups = $state('Remote-Groups');
  let faHeaderName = $state('Remote-Name');

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

  // About tab state
  let systemInfo = $state<SystemInfo | null>(null);
  let updateInfo = $state<UpdateInfo | null>(null);
  let aboutLoading = $state(false);
  let aboutError = $state<string | null>(null);
  let updateInstructionsExpanded = $state(false);
  let changelogExpanded = $state(false);

  $effect(() => {
    if (activeTab === 'about' && !systemInfo) {
      loadAboutData();
    }
  });

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

  // Track if changes have been made (declared below after snapshot variables)

  // Editing state
  let editingApp = $state<App | null>(null);
  let editingGroup = $state<Group | null>(null);
  let showAddApp = $state(false);
  let addAppStep = $state<'choose' | 'configure'>('choose');
  let addAppSearch = $state('');
  let addAppSearchLower = $derived(addAppSearch.toLowerCase());
  let showAddGroup = $state(false);

  // Import/export state
  let importFileInput = $state<HTMLInputElement | undefined>(undefined);
  let showImportConfirm = $state(false);
  let pendingImport = $state<ImportedConfig | null>(null);

  // New app/group templates
  const newAppTemplate: App = {
    name: '',
    url: '',
    icon: { type: 'dashboard', name: '', file: '', url: '', variant: '' },
    color: '#22c55e',
    group: '',
    order: 0,
    enabled: true,
    default: false,
    open_mode: 'iframe',
    proxy: false,
    scale: 1,
  };

  const newGroupTemplate: Group = {
    name: '',
    icon: { type: 'dashboard', name: '', file: '', url: '', variant: '' },
    color: '#3498db',
    order: 0,
    expanded: true
  };

  let newApp = $state({ ...newAppTemplate });
  let newGroup = $state({ ...newGroupTemplate });

  // Validation error state
  let appErrors = $state<Record<string, string>>({});
  let groupErrors = $state<Record<string, string>>({});

  // Assign stable `id` fields for svelte-dnd-action (must be done once, before building dnd arrays)
  untrack(() => localApps).forEach(a => { (a as App & Record<string, unknown>).id = a.name; });
  untrack(() => localConfig).groups.forEach(g => { (g as Group & Record<string, unknown>).id = g.name; });

  // Snapshot taken AFTER id fields are added, so hasChanges starts as false
  const initialConfigSnapshot = untrack(() => JSON.stringify(localConfig));
  const initialAppsSnapshot = untrack(() => JSON.stringify(localApps));

  // Snapshot theme so we can revert on close without save
  const initialFamily = untrack(() => get(selectedFamily));
  const initialVariant = untrack(() => get(variantMode));

  // Track if changes have been made
  let hasChanges = $derived(JSON.stringify(localConfig) !== initialConfigSnapshot ||
                  JSON.stringify(localApps) !== initialAppsSnapshot ||
                  keybindingsChanged ||
                  $selectedFamily !== initialFamily ||
                  $variantMode !== initialVariant);

  // Mutable arrays for svelte-dnd-action (NOT reactive derivations — the library owns these)
  let dndGroups = $state<Group[]>([...untrack(() => localConfig).groups].sort((a, b) => a.order - b.order));
  let dndGroupedApps = $state<Record<string, App[]>>(buildGroupedApps());

  function buildGroupedApps(): Record<string, App[]> {
    const acc: Record<string, App[]> = {};
    for (const app of localApps) {
      const group = app.group || '';
      if (!acc[group]) acc[group] = [];
      acc[group].push(app);
    }
    Object.values(acc).forEach(arr => arr.sort((a, b) => a.order - b.order));
    return acc;
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

  $effect(() => {
    if (activeTab === 'security' && $isAdmin) {
      loadSecurityUsers();
    }
  });

  $effect(() => {
    if (activeTab === 'security') {
      selectedAuthMethod = (localConfig.auth?.method || 'none') as typeof selectedAuthMethod;
      // Pre-fill forward auth fields from existing config
      const proxies = localConfig.auth?.trusted_proxies;
      methodTrustedProxies = proxies?.length ? proxies.join('\n') : '';
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
    }
  });

  function rebuildDndArrays() {
    dndGroups = [...localConfig.groups].sort((a, b) => a.order - b.order);
    dndGroupedApps = buildGroupedApps();
  }

  // DnD handlers for groups
  function handleGroupDndConsider(e: CustomEvent<DndEvent<Group>>) {
    dndGroups = e.detail.items;
  }
  function handleGroupDndFinalize(e: CustomEvent<DndEvent<Group>>) {
    dndGroups = e.detail.items;
    dndGroups.forEach((g, i) => { g.order = i; });
    localConfig.groups = [...dndGroups];
  }

  // DnD handlers for apps within a group
  function handleAppDndConsider(e: CustomEvent<DndEvent<App>>, groupName: string) {
    dndGroupedApps[groupName] = e.detail.items;
  }
  function handleAppDndFinalize(e: CustomEvent<DndEvent<App>>, groupName: string) {
    const newItems = e.detail.items;
    newItems.forEach((a, i) => { a.group = groupName; a.order = i; (a as App & Record<string, unknown>).id = a.name; });
    dndGroupedApps[groupName] = newItems;
    // Sync back to localApps
    const otherApps = localApps.filter(a => (a.group || '') !== groupName && !newItems.find(n => n.name === a.name));
    localApps = [...otherApps, ...newItems];
  }

  function handleSave() {
    // Update config with local changes
    localConfig.apps = localApps;
    // Capture current theme from stores into config
    localConfig.theme = {
      family: get(selectedFamily),
      variant: get(variantMode)
    };
    // Include keybindings if changed
    if (keybindingsChanged) {
      localConfig.keybindings = getKeybindingsForConfig();
    }
    onsave?.(localConfig);
    onclose?.();
  }

  // Inline confirmation state
  let confirmClose = $state(false);
  let confirmDeleteApp = $state<App | null>(null);
  let confirmDeleteGroup = $state<Group | null>(null);
  let confirmDeleteTheme = $state<string | null>(null);

  function handleClose() {
    if (hasChanges) {
      confirmClose = true;
      return;
    }
    revertTheme();
    onclose?.();
  }

  function confirmCloseDiscard() {
    confirmClose = false;
    revertTheme();
    onclose?.();
  }

  function revertTheme() {
    setThemeFamily(initialFamily);
    setVariantMode(initialVariant);
  }

  function selectPopularApp(template: PopularAppTemplate) {
    const app = templateToApp(template, template.defaultUrl, localApps.length);
    newApp = { ...app };
    addAppStep = 'configure';
  }

  function startCustomApp() {
    newApp = { ...newAppTemplate };
    addAppStep = 'configure';
  }

  function addApp() {
    const result = appSchema.safeParse(newApp);
    if (!result.success) {
      appErrors = extractErrors(result);
      return;
    }
    appErrors = {};
    newApp.order = localApps.length;
    const app: App & Record<string, unknown> = { ...newApp };
    app.id = app.name;
    localApps = [...localApps, app];
    newApp = { ...newAppTemplate };
    showAddApp = false;
    rebuildDndArrays();
  }

  function deleteApp(app: App) {
    confirmDeleteApp = app;
  }

  function confirmDeleteAppAction() {
    if (confirmDeleteApp) {
      localApps = localApps.filter(a => a.name !== confirmDeleteApp!.name);
      confirmDeleteApp = null;
      rebuildDndArrays();
    }
  }

  function addGroup() {
    const result = groupSchema.safeParse(newGroup);
    if (!result.success) {
      groupErrors = extractErrors(result);
      return;
    }
    groupErrors = {};
    newGroup.order = localConfig.groups.length;
    const group: Group & Record<string, unknown> = { ...newGroup };
    group.id = group.name;
    localConfig.groups = [...localConfig.groups, group];
    newGroup = { ...newGroupTemplate };
    showAddGroup = false;
    rebuildDndArrays();
  }

  function deleteGroup(group: Group) {
    confirmDeleteGroup = group;
  }

  function confirmDeleteGroupAction() {
    if (confirmDeleteGroup) {
      localConfig.groups = localConfig.groups.filter(g => g.name !== confirmDeleteGroup!.name);
      localApps = localApps.map(app =>
        app.group === confirmDeleteGroup!.name ? { ...app, group: '' } : app
      );
      localApps.forEach(a => { (a as App & Record<string, unknown>).id = a.name; });
      confirmDeleteGroup = null;
      rebuildDndArrays();
    }
  }

  function closeEditApp() {
    if (editingApp) {
      (editingApp as App & Record<string, unknown>).id = editingApp.name;
      // Sync DnD app changes back to localApps before rebuilding
      const allApps: App[] = [];
      for (const apps of Object.values(dndGroupedApps)) {
        allApps.push(...apps);
      }
      localApps = allApps;
    }
    editingApp = null;
    rebuildDndArrays();
  }

  function closeEditGroup() {
    if (editingGroup) {
      (editingGroup as Group & Record<string, unknown>).id = editingGroup.name;
      // Sync DnD group changes back to localConfig before rebuilding
      localConfig.groups = [...dndGroups];
    }
    editingGroup = null;
    rebuildDndArrays();
  }

  // Export config as YAML file
  function handleExport() {
    exportConfig();
    toasts.success('Configuration exported');
  }

  // Handle import file selection
  async function handleImportSelect(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    try {
      const content = await file.text();
      pendingImport = await parseImportedConfig(content);
      showImportConfirm = true;
    } catch (err) {
      toasts.error(err instanceof Error ? err.message : 'Failed to parse config file');
    }

    // Reset input so same file can be selected again
    input.value = '';
  }

  // Apply imported config
  function applyImport() {
    if (!pendingImport) return;

    localConfig = {
      ...localConfig,
      title: pendingImport.title,
      navigation: pendingImport.navigation,
      groups: pendingImport.groups,
    };
    localApps = pendingImport.apps;

    // Assign stable ids for svelte-dnd-action
    localApps.forEach(a => { (a as App & Record<string, unknown>).id = a.name; });
    localConfig.groups.forEach(g => { (g as Group & Record<string, unknown>).id = g.name; });
    rebuildDndArrays();

    showImportConfirm = false;
    pendingImport = null;
    toasts.success('Configuration imported - save to apply changes');
  }

  function cancelImport() {
    showImportConfirm = false;
    pendingImport = null;
  }

  function handleIconSelect(detail: { name: string; variant: string; type: string }) {
    const { name, variant, type } = detail;
    const iconData = { type: type as 'dashboard' | 'lucide' | 'custom', name, variant, file: '', url: '', color: '', background: '' };

    if (iconBrowserTarget === 'newApp') {
      newApp = { ...newApp, icon: iconData };
    } else if (iconBrowserTarget === 'editApp' && editingApp) {
      // Replace in dndGroupedApps and editingApp with the same new object
      const updated = { ...editingApp, icon: iconData };
      for (const apps of Object.values(dndGroupedApps)) {
        const idx = apps.indexOf(editingApp);
        if (idx !== -1) { apps[idx] = updated; break; }
      }
      editingApp = updated;
    } else if (iconBrowserTarget === 'newGroup') {
      newGroup = { ...newGroup, icon: iconData };
    } else if (iconBrowserTarget === 'editGroup' && editingGroup) {
      // Replace in dndGroups and editingGroup with the same new object
      const updated = { ...editingGroup, icon: iconData };
      const idx = dndGroups.indexOf(editingGroup);
      if (idx !== -1) dndGroups[idx] = updated;
      editingGroup = updated;
    }
    showIconBrowser = false;
    iconBrowserTarget = null;
  }

  function openIconBrowser(target: 'newApp' | 'editApp' | 'newGroup' | 'editGroup') {
    iconBrowserTarget = target;
    showIconBrowser = true;
  }

  const navPositions = [
    { value: 'top', label: 'Top Bar', description: 'Horizontal bar at the top' },
    { value: 'left', label: 'Left Sidebar', description: 'Vertical sidebar on the left' },
    { value: 'right', label: 'Right Sidebar', description: 'Vertical sidebar on the right' },
    { value: 'bottom', label: 'Bottom Bar', description: 'Horizontal bar at the bottom' },
    { value: 'floating', label: 'Floating', description: 'Minimal floating button' }
  ] as const;

  const openModes = [
    { value: 'iframe', label: 'Embedded', description: 'Show inside Muximux' },
    { value: 'new_tab', label: 'New Tab', description: 'Open in a new browser tab' },
    { value: 'new_window', label: 'New Window', description: 'Open in a popup window' }
  ];

  // Theme editor state
  let showThemeEditor = $state(false);
  let themeEditorVars: Record<string, string> = $state({});
  let themeEditorDefaults: Record<string, string> = $state({});
  let saveThemeName = $state('');
  let saveThemeDescription = $state('');
  let saveThemeAuthor = $state('');
  let isSavingTheme = $state(false);

  function openThemeEditor() {
    themeEditorDefaults = getCurrentThemeVariables();
    themeEditorVars = { ...themeEditorDefaults };
    showThemeEditor = true;
  }

  // Refresh theme editor when the active theme changes while editor is open
  $effect(() => {
    $resolvedTheme; // track
    if (showThemeEditor) {
      // Clear any live preview overrides from the previous theme
      const varNames = untrack(() => Object.keys(themeEditorVars));
      for (const name of varNames) {
        document.documentElement.style.removeProperty(name);
      }
      // Re-read the new theme's variables
      // Use a microtask so the theme CSS has loaded
      queueMicrotask(() => {
        themeEditorDefaults = getCurrentThemeVariables();
        themeEditorVars = { ...themeEditorDefaults };
      });
    }
  });

  function closeThemeEditor() {
    // Revert live preview changes
    for (const name of Object.keys(themeEditorVars)) {
      document.documentElement.style.removeProperty(name);
    }
    showThemeEditor = false;
    saveThemeName = '';
  }

  function updateThemeVar(name: string, value: string) {
    themeEditorVars[name] = value;
    // Live preview
    document.documentElement.style.setProperty(name, value);
  }

  function resetThemeVar(name: string) {
    themeEditorVars[name] = themeEditorDefaults[name];
    document.documentElement.style.removeProperty(name);
  }

  function resetAllThemeVars() {
    for (const name of Object.keys(themeEditorVars)) {
      document.documentElement.style.removeProperty(name);
    }
    themeEditorVars = { ...themeEditorDefaults };
  }

  async function handleSaveTheme() {
    if (!saveThemeName.trim()) return;
    isSavingTheme = true;
    const success = await saveCustomThemeToServer(
      saveThemeName.trim(),
      $resolvedTheme,
      $isDarkTheme,
      themeEditorVars,
      saveThemeDescription.trim(),
      saveThemeAuthor.trim()
    );
    isSavingTheme = false;
    if (success) {
      // Clear inline overrides — the saved CSS file takes over
      for (const name of Object.keys(themeEditorVars)) {
        document.documentElement.style.removeProperty(name);
      }
      // Switch to the new theme (as a standalone family)
      const id = sanitizeThemeId(saveThemeName.trim());
      setThemeFamily(id);
      setVariantMode($isDarkTheme ? 'dark' : 'light');
      showThemeEditor = false;
      saveThemeName = '';
      saveThemeDescription = '';
      saveThemeAuthor = '';
      toasts.success('Theme saved');
    } else {
      toasts.error('Failed to save theme');
    }
  }

  function handleDeleteTheme(themeId: string) {
    confirmDeleteTheme = themeId;
  }

  async function confirmDeleteThemeAction() {
    if (!confirmDeleteTheme) return;
    const themeId = confirmDeleteTheme;
    confirmDeleteTheme = null;
    const success = await deleteCustomThemeFromServer(themeId);
    if (success) {
      toasts.success('Theme deleted');
    } else {
      toasts.error('Failed to delete theme');
    }
  }

  // Convert CSS color to hex (for color input compatibility)
  function cssColorToHex(color: string): string {
    if (!color) return '#000000';
    // Already hex
    if (color.startsWith('#')) return color.length === 4
      ? '#' + color[1] + color[1] + color[2] + color[2] + color[3] + color[3]
      : color;
    // Try rgb/rgba
    const match = color.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/);
    if (match) {
      const r = parseInt(match[1]).toString(16).padStart(2, '0');
      const g = parseInt(match[2]).toString(16).padStart(2, '0');
      const b = parseInt(match[3]).toString(16).padStart(2, '0');
      return `#${r}${g}${b}`;
    }
    return '#000000';
  }

  // Variable display names
  const varLabels: Record<string, string> = {
    '--bg-base': 'Base',
    '--bg-surface': 'Surface',
    '--bg-elevated': 'Elevated',
    '--text-primary': 'Primary',
    '--text-secondary': 'Secondary',
    '--text-muted': 'Muted',
    '--accent-primary': 'Primary',
    '--accent-secondary': 'Secondary',
    '--status-success': 'Success',
    '--status-warning': 'Warning',
    '--status-error': 'Error',
    '--status-info': 'Info',
  };
</script>

<div class="settings">
<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 {isMobile ? 'p-0' : 'p-4'}"
  transition:fade={{ duration: 150 }}
>
  <div
    class="bg-gray-800 shadow-2xl w-full overflow-hidden border border-gray-700 flex flex-col
           {isMobile
             ? 'h-full max-h-full rounded-none'
             : 'rounded-xl max-w-4xl max-h-[90vh]'}"
    in:fly={{ y: isMobile ? 50 : 20, duration: 200 }}
    out:fade={{ duration: 100 }}
  >
    <!-- Header -->
    <div class="flex items-center justify-between p-4 border-b border-gray-700 flex-shrink-0">
      <h2 class="text-lg font-semibold text-white">Settings</h2>
      <div class="flex items-center gap-2">
        {#if hasChanges}
          <span class="text-xs text-yellow-400">Unsaved changes</span>
        {/if}
        <button
          class="px-3 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md disabled:opacity-50"
          disabled={!hasChanges}
          onclick={handleSave}
        >
          Save Changes
        </button>
        <button
          class="p-1.5 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={handleClose}
          aria-label="Close settings"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Unsaved changes confirmation banner -->
    {#if confirmClose}
      <div class="flex items-center justify-between px-4 py-2 bg-yellow-600/20 border-b border-yellow-600/40">
        <span class="text-sm text-yellow-200">You have unsaved changes. Discard?</span>
        <div class="flex gap-2">
          <button
            class="px-3 py-1 text-xs rounded bg-gray-600 hover:bg-gray-500 text-white"
            onclick={() => confirmClose = false}
          >Keep Editing</button>
          <button
            class="px-3 py-1 text-xs rounded bg-red-600 hover:bg-red-500 text-white"
            onclick={confirmCloseDiscard}
          >Discard</button>
        </div>
      </div>
    {/if}

    <!-- Tabs - scrollable on mobile -->
    <div class="flex border-b border-gray-700 flex-shrink-0 overflow-x-auto scrollbar-hide">
      {#each [
        { id: 'general', label: 'General' },
        { id: 'apps', label: 'Apps & Groups' },
        { id: 'theme', label: 'Theme' },
        { id: 'keybindings', label: 'Keybindings' },
        { id: 'security', label: 'Security' },
        { id: 'about', label: 'About' }
      ] as tab (tab.id)}
        <button
          class="px-4 py-3 text-sm font-medium transition-colors border-b-2 whitespace-nowrap min-h-[48px]
                 {activeTab === tab.id
                   ? 'text-brand-400 border-brand-400'
                   : 'text-gray-400 border-transparent hover:text-gray-300 hover:border-gray-600'}"
          onclick={() => activeTab = tab.id as typeof activeTab}
        >
          {tab.label}
        </button>
      {/each}
    </div>

    <!-- Content -->
    <div class="flex-1 overflow-y-auto p-6">
      <!-- General Settings -->
      {#if activeTab === 'general'}
        <div class="space-y-6">
          <!-- Dashboard Title -->
          <div>
            <label for="title" class="block text-sm font-medium text-gray-300 mb-2">
              Dashboard Title
            </label>
            <input
              id="title"
              type="text"
              bind:value={localConfig.title}
              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                     focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
              placeholder="Muximux"
            />
            <p class="text-xs text-gray-500 mt-1.5">
              Variables: <code class="text-gray-400">%title%</code> (app name),
              <code class="text-gray-400">%group%</code>,
              <code class="text-gray-400">%version%</code>,
              <code class="text-gray-400">%url%</code>.
              Example: <code class="text-gray-400">Muximux - %title%</code>
            </p>
          </div>

          <!-- Navigation Position -->
          <div>
            <span class="block text-sm font-medium text-gray-300 mb-2">
              Navigation Position
            </span>
            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {#each navPositions as pos (pos.value)}
                <button
                  class="p-3 rounded-lg border text-left transition-colors
                         {localConfig.navigation.position === pos.value
                           ? 'border-brand-500 bg-brand-500/10 text-white'
                           : 'border-gray-600 hover:border-gray-500 text-gray-300'}"
                  onclick={() => localConfig.navigation.position = pos.value}
                >
                  <div class="font-medium">{pos.label}</div>
                  <div class="text-xs text-gray-400 mt-1">{pos.description}</div>
                </button>
              {/each}
            </div>
          </div>

          <!-- Bar Style (only shown when top or bottom is selected) -->
          {#if localConfig.navigation.position === 'top' || localConfig.navigation.position === 'bottom'}
            <div>
              <span class="block text-sm font-medium text-gray-300 mb-2">
                Bar Style
              </span>
              <div class="grid grid-cols-2 gap-3">
                {#each [
                  { value: 'grouped', label: 'Group Dropdowns', description: 'Apps organized in dropdown menus by group' },
                  { value: 'flat', label: 'Flat List', description: 'All apps in a single scrollable row' }
                ] as style (style.value)}
                  <button
                    class="p-3 rounded-lg border text-left transition-colors
                           {(localConfig.navigation.bar_style || 'grouped') === style.value
                             ? 'border-brand-500 bg-brand-500/10 text-white'
                             : 'border-gray-600 hover:border-gray-500 text-gray-300'}"
                    onclick={() => localConfig.navigation.bar_style = style.value as 'grouped' | 'flat'}
                  >
                    <div class="font-medium text-sm">{style.label}</div>
                    <div class="text-xs text-gray-400 mt-1">{style.description}</div>
                  </button>
                {/each}
              </div>
            </div>
          {/if}

          <!-- Floating Position (only shown when floating is selected) -->
          {#if localConfig.navigation.position === 'floating'}
            <div>
              <span class="block text-sm font-medium text-gray-300 mb-2">
                Floating Button Position
              </span>
              <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
                {#each [
                  { value: 'bottom-right', label: 'Bottom Right' },
                  { value: 'bottom-left', label: 'Bottom Left' },
                  { value: 'top-right', label: 'Top Right' },
                  { value: 'top-left', label: 'Top Left' }
                ] as fp (fp.value)}
                  <button
                    class="p-2 rounded-lg border text-center text-sm transition-colors
                           {(localConfig.navigation.floating_position || 'bottom-right') === fp.value
                             ? 'border-brand-500 bg-brand-500/10 text-white'
                             : 'border-gray-600 hover:border-gray-500 text-gray-300'}"
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
            <label class="flex items-center gap-3 p-3 bg-gray-700/50 rounded-lg cursor-pointer">
              <input
                type="checkbox"
                bind:checked={localConfig.navigation.show_labels}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <div class="text-sm text-white">Show Labels</div>
                <div class="text-xs text-gray-400">Display app names next to icons</div>
              </div>
            </label>

            <label class="flex items-center gap-3 p-3 bg-gray-700/50 rounded-lg cursor-pointer">
              <input
                type="checkbox"
                bind:checked={localConfig.navigation.show_logo}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <div class="text-sm text-white">Show Logo</div>
                <div class="text-xs text-gray-400">Display the Muximux logo in the menu</div>
              </div>
            </label>

            <label class="flex items-center gap-3 p-3 bg-gray-700/50 rounded-lg cursor-pointer">
              <input
                type="checkbox"
                bind:checked={localConfig.navigation.show_app_colors}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <div class="text-sm text-white">App Color Accents</div>
                <div class="text-xs text-gray-400">Highlight the active app with its color</div>
              </div>
            </label>

            <label class="flex items-center gap-3 p-3 bg-gray-700/50 rounded-lg cursor-pointer">
              <input
                type="checkbox"
                bind:checked={localConfig.navigation.show_icon_background}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <div class="text-sm text-white">Icon Background</div>
                <div class="text-xs text-gray-400">Show colored circle behind app icons</div>
              </div>
            </label>

            <div class="p-3 bg-gray-700/50 rounded-lg sm:col-span-2">
              <div class="flex items-center justify-between mb-2">
                <div>
                  <div class="text-sm text-white">Icon Size</div>
                  <div class="text-xs text-gray-400">Scale app icons in the navigation</div>
                </div>
                <span class="text-sm text-gray-300 tabular-nums">{localConfig.navigation.icon_scale}×</span>
              </div>
              <input type="range" min="0.5" max="2" step="0.25"
                bind:value={localConfig.navigation.icon_scale}
                class="w-full accent-brand-500" />
            </div>

            <label class="flex items-center gap-3 p-3 bg-gray-700/50 rounded-lg cursor-pointer">
              <input
                type="checkbox"
                bind:checked={localConfig.navigation.show_splash_on_startup}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <div class="text-sm text-white">Start on Overview</div>
                <div class="text-xs text-gray-400">Show the dashboard overview when Muximux opens</div>
              </div>
            </label>

            <div class="p-3 bg-gray-700/50 rounded-lg sm:col-span-2">
              <label class="flex items-center gap-3 cursor-pointer">
                <input
                  type="checkbox"
                  bind:checked={localConfig.navigation.auto_hide}
                  class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
                />
                <div class="flex-1">
                  <div class="text-sm text-white">Auto-hide Menu</div>
                  <div class="text-xs text-gray-400">Automatically collapse the menu after inactivity</div>
                </div>
              </label>
              {#if localConfig.navigation.auto_hide}
                <div class="flex items-center gap-3 mt-3 pt-3 border-t border-gray-600">
                  <div class="flex-1 text-xs text-gray-400 pl-7">Hide after</div>
                  <select
                    bind:value={localConfig.navigation.auto_hide_delay}
                    class="px-2 py-1 text-xs bg-gray-600 border border-gray-500 rounded text-white focus:ring-brand-500 focus:border-brand-500"
                  >
                    <option value="0.25s">0.25s</option>
                    <option value="0.5s">0.5s</option>
                    <option value="1s">1s</option>
                    <option value="2s">2s</option>
                    <option value="3s">3s</option>
                  </select>
                </div>
                <label class="flex items-center gap-3 mt-2 pl-7 cursor-pointer">
                  <input
                    type="checkbox"
                    bind:checked={localConfig.navigation.show_shadow}
                    class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
                  />
                  <div class="text-xs text-gray-400">Shadow — show a drop shadow on the expanded menu</div>
                </label>
              {/if}
            </div>

            {#if localConfig.navigation.position === 'left' || localConfig.navigation.position === 'right'}
              <div class="p-3 bg-gray-700/50 rounded-lg sm:col-span-2">
                <label class="flex items-center gap-3 cursor-pointer">
                  <input
                    type="checkbox"
                    bind:checked={localConfig.navigation.hide_sidebar_footer}
                    class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
                  />
                  <div class="flex-1">
                    <div class="text-sm text-white">Collapsible Footer</div>
                    <div class="text-xs text-gray-400">Hide utility buttons in a drawer that reveals on hover</div>
                  </div>
                </label>
              </div>
            {/if}
          </div>

          <!-- Health Monitoring bulk actions -->
          <div class="pt-4 border-t border-gray-700">
            <div class="flex items-center justify-between mb-1">
              <h3 class="text-sm font-medium text-gray-300">Health Checks</h3>
              <div class="flex gap-2">
                <button
                  class="text-xs px-2 py-1 rounded text-gray-400 hover:text-white hover:bg-gray-700 transition-colors"
                  onclick={() => localApps.forEach(a => a.health_check = undefined)}
                >Enable all</button>
                <button
                  class="text-xs px-2 py-1 rounded text-gray-400 hover:text-white hover:bg-gray-700 transition-colors"
                  onclick={() => localApps.forEach(a => a.health_check = false)}
                >Disable all</button>
              </div>
            </div>
            <p class="text-xs text-gray-500">Toggle per app in the app editor</p>
          </div>

          <!-- Advanced -->
          <div class="pt-4 border-t border-gray-700">
            <h3 class="text-sm font-medium text-gray-300 mb-3">Advanced</h3>

            <div class="flex items-center gap-3 mb-4">
              <label for="log-level" class="text-sm text-gray-400 whitespace-nowrap">Log Level</label>
              <select
                id="log-level"
                bind:value={localConfig.log_level}
                class="px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded-md text-white
                       focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
              >
                <option value="debug">Debug</option>
                <option value="info">Info</option>
                <option value="warn">Warning</option>
                <option value="error">Error</option>
              </select>
              <span class="text-xs text-gray-500">Takes effect on restart</span>
            </div>

            <div class="flex items-center gap-3 mb-4">
              <label for="proxy-timeout" class="text-sm text-gray-400 whitespace-nowrap">Proxy Timeout</label>
              <input
                id="proxy-timeout"
                type="text"
                bind:value={localConfig.proxy_timeout}
                placeholder="30s"
                class="w-20 px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded-md text-white
                       focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
              />
              <span class="text-xs text-gray-500">Max wait time for proxied backends (e.g. 30s, 1m)</span>
            </div>

            <div class="flex flex-wrap gap-3">
              <button
                class="px-4 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md flex items-center gap-2"
                onclick={handleExport}
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                </svg>
                Export Config
              </button>
              <button
                class="px-4 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md flex items-center gap-2"
                onclick={() => importFileInput?.click()}
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                </svg>
                Import Config
              </button>
              <input
                bind:this={importFileInput}
                type="file"
                accept=".yaml,.yml"
                class="hidden"
                onchange={handleImportSelect}
              />
            </div>
            <p class="text-xs text-gray-500 mt-2">
              Export your current configuration or import a previously saved one.
            </p>
          </div>

        </div>

      <!-- Apps & Groups Settings -->
      {:else if activeTab === 'apps'}
        <div class="space-y-4">
          <!-- Action buttons -->
          <div class="flex justify-between items-center">
            <h3 class="text-sm font-medium text-gray-300">Apps & Groups</h3>
            <div class="flex gap-2">
              <button
                class="px-3 py-1.5 text-sm bg-gray-600 hover:bg-gray-500 text-white rounded-md flex items-center gap-1"
                onclick={() => { groupErrors = {}; showAddGroup = true; }}
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 14v6m-3-3h6M6 10h2a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v2a2 2 0 002 2zm10 0h2a2 2 0 002-2V6a2 2 0 00-2-2h-2a2 2 0 00-2 2v2a2 2 0 002 2zM6 20h2a2 2 0 002-2v-2a2 2 0 00-2-2H6a2 2 0 00-2 2v2a2 2 0 002 2z" />
                </svg>
                Add Group
              </button>
              <button
                class="px-3 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md flex items-center gap-1"
                onclick={() => { appErrors = {}; addAppStep = 'choose'; addAppSearch = ''; showAddApp = true; }}
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                </svg>
                Add App
              </button>
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500">
            <span>Drag apps to reorder or move between groups. Drag group headers to reorder groups.</span>
            <span class="flex items-center gap-3 text-gray-500">
              <span class="flex items-center gap-1"><span class="app-indicator"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg></span> Proxy</span>
              <span class="flex items-center gap-1"><span class="app-indicator"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg></span> New tab</span>
              <span class="flex items-center gap-1"><span class="app-indicator"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg></span> New window</span>
              <span class="flex items-center gap-1"><span class="app-indicator">50%</span> Scale</span>
              <span class="flex items-center gap-1"><span class="app-indicator">⌨</span> Keyboard</span>
            </span>
          </div>

          <!-- Groups with their apps (dnd-zone for group reordering) -->
          <div class="space-y-3" use:dndzone={{items: dndGroups, flipDurationMs, type: 'groups', dropTargetStyle: {}}} onconsider={handleGroupDndConsider} onfinalize={handleGroupDndFinalize}>
            {#each dndGroups as group ((group as Group & Record<string, unknown>).id)}
              {@const appsInGroup = dndGroupedApps[group.name] || []}
              <div class="rounded-lg border border-gray-700" animate:flip={{duration: flipDurationMs}}>
                <!-- Group header -->
                <div class="flex items-center gap-3 p-3 bg-gray-700/30 rounded-t-lg cursor-grab active:cursor-grabbing">
                  <!-- Drag handle -->
                  <div class="flex-shrink-0 text-gray-500 hover:text-gray-300">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
                    </svg>
                  </div>

                  <!-- Group icon -->
                  <div class="flex-shrink-0">
                    {#if group.icon?.name}
                      <AppIcon icon={group.icon} name={group.name} color={group.color || '#374151'} size="sm" showBackground={true} />
                    {:else}
                      <span class="w-6 h-6 rounded flex-shrink-0 block" style="background-color: {group.color || '#374151'}"></span>
                    {/if}
                  </div>

                  <!-- Group info -->
                  <div class="flex-1 min-w-0">
                    <span class="font-medium text-white text-sm">{group.name}</span>
                    <span class="text-xs text-gray-500 ml-2">{appsInGroup.length} apps</span>
                  </div>

                  <!-- Group actions -->
                  {#if confirmDeleteGroup?.name === group.name}
                    <div class="flex items-center gap-1">
                      <span class="text-xs text-red-400 mr-1">Delete?</span>
                      <button class="px-2 py-1 text-xs rounded bg-red-600 hover:bg-red-500 text-white"
                              onclick={confirmDeleteGroupAction}>Yes</button>
                      <button class="px-2 py-1 text-xs rounded bg-gray-600 hover:bg-gray-500 text-white"
                              onclick={() => confirmDeleteGroup = null}>No</button>
                    </div>
                  {:else}
                    <div class="flex items-center gap-1 app-actions">
                      <button class="p-1.5 text-gray-400 hover:text-white rounded hover:bg-white/10"
                              onclick={() => editingGroup = group} title="Edit group">
                        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                        </svg>
                      </button>
                      <button class="p-1.5 text-gray-400 hover:text-red-400 rounded hover:bg-white/10"
                              onclick={() => deleteGroup(group)} title="Delete group">
                        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                      </button>
                    </div>
                  {/if}
                </div>

                <!-- Apps in this group (dnd-zone for app reordering + cross-group) -->
                <div class="p-2 space-y-1 min-h-[36px]" use:dndzone={{items: appsInGroup, flipDurationMs, type: 'apps', dropTargetStyle: {}}} onconsider={(e) => handleAppDndConsider(e, group.name)} onfinalize={(e) => handleAppDndFinalize(e, group.name)}>
                  {#if appsInGroup.length === 0}
                    <div class="text-center py-3 text-gray-500 text-sm italic">No apps in this group</div>
                  {/if}
                  {#each appsInGroup as app ((app as App & Record<string, unknown>).id)}
                    <div
                      class="flex items-center gap-3 p-2 rounded-md group/app hover:bg-gray-700/30 cursor-grab active:cursor-grabbing"
                      animate:flip={{duration: flipDurationMs}}
                    >
                      <!-- Drag handle -->
                      <div class="flex-shrink-0 text-gray-600 hover:text-gray-400">
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
                        </svg>
                      </div>
                      <div class="flex-shrink-0">
                        <AppIcon icon={app.icon} name={app.name} color={app.color} size="md" />
                      </div>
                      <div class="flex-1 min-w-0">
                        <div class="flex items-center gap-2 flex-wrap">
                          <span class="font-medium text-white text-sm truncate">{app.name}</span>
                          {#if app.default}
                            <span class="text-xs bg-brand-500/20 text-brand-400 px-1.5 py-0.5 rounded">Default</span>
                          {/if}
                          {#if !app.enabled}
                            <span class="text-xs bg-gray-600 text-gray-400 px-1.5 py-0.5 rounded">Disabled</span>
                          {/if}
                          {#if app.proxy}
                            <span class="app-indicator" title="Proxied through server">
                              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>
                            </span>
                          {/if}
                          {#if app.open_mode && app.open_mode !== 'iframe'}
                            <span class="app-indicator" title="Opens in {app.open_mode.replace('_', ' ')}">
                              {#if app.open_mode === 'new_tab'}
                                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
                              {:else if app.open_mode === 'new_window'}
                                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
                              {:else}
                                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M17 8l4 4m0 0l-4 4m4-4H3" /></svg>
                              {/if}
                            </span>
                          {/if}
                          {#if app.scale && app.scale !== 1}
                            <span class="app-indicator" title="Scaled to {Math.round(app.scale * 100)}%">
                              {Math.round(app.scale * 100)}%
                            </span>
                          {/if}
                        </div>
                        <span class="text-xs text-gray-400 truncate block">{app.url}</span>
                      </div>
                      <!-- App actions -->
                      {#if confirmDeleteApp?.name === app.name}
                        <div class="flex items-center gap-1">
                          <span class="text-xs text-red-400 mr-1">Delete?</span>
                          <button class="px-2 py-1 text-xs rounded bg-red-600 hover:bg-red-500 text-white"
                                  onclick={confirmDeleteAppAction}>Yes</button>
                          <button class="px-2 py-1 text-xs rounded bg-gray-600 hover:bg-gray-500 text-white"
                                  onclick={() => confirmDeleteApp = null}>No</button>
                        </div>
                      {:else}
                        <div class="flex items-center gap-1 opacity-0 group-hover/app:opacity-100 focus-within:opacity-100 transition-opacity app-actions">
                          <button class="p-1.5 text-gray-400 hover:text-white rounded hover:bg-white/10"
                                  tabindex="-1"
                                  onclick={() => editingApp = app} title="Edit">
                            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                            </svg>
                          </button>
                          <button class="p-1.5 text-gray-400 hover:text-red-400 rounded hover:bg-white/10"
                                  tabindex="-1"
                                  onclick={() => deleteApp(app)} title="Delete">
                            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                            </svg>
                          </button>
                        </div>
                      {/if}
                    </div>
                  {/each}
                </div>
              </div>
            {/each}
          </div>

          <!-- Ungrouped apps -->
          {#if (dndGroupedApps[''] || []).length > 0 || localConfig.groups.length > 0}
            {@const ungroupedApps = dndGroupedApps[''] || []}
            <div class="rounded-lg border border-gray-700 border-dashed" class:hidden={ungroupedApps.length === 0 && localConfig.groups.length === 0}>
              <div class="p-3 bg-gray-700/20 rounded-t-lg">
                <span class="text-sm font-medium text-gray-400">Ungrouped</span>
                {#if ungroupedApps.length > 0}
                  <span class="text-xs text-gray-500 ml-2">{ungroupedApps.length} apps</span>
                {:else}
                  <span class="text-xs text-gray-600 ml-2">Drag apps here to ungroup them</span>
                {/if}
              </div>
              <div class="p-2 space-y-1 min-h-[36px]" use:dndzone={{items: ungroupedApps, flipDurationMs, type: 'apps', dropTargetStyle: {}}} onconsider={(e) => handleAppDndConsider(e, '')} onfinalize={(e) => handleAppDndFinalize(e, '')}>
                {#each ungroupedApps as app ((app as App & Record<string, unknown>).id)}
                  <div
                    class="flex items-center gap-3 p-2 rounded-md group/app hover:bg-gray-700/30 cursor-grab active:cursor-grabbing"
                    animate:flip={{duration: flipDurationMs}}
                  >
                    <div class="flex-shrink-0 text-gray-600 hover:text-gray-400">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
                      </svg>
                    </div>
                    <div class="flex-shrink-0">
                      <AppIcon icon={app.icon} name={app.name} color={app.color} size="md" />
                    </div>
                    <div class="flex-1 min-w-0">
                      <div class="flex items-center gap-2 flex-wrap">
                        <span class="font-medium text-white text-sm truncate">{app.name}</span>
                        {#if app.default}
                          <span class="text-xs bg-brand-500/20 text-brand-400 px-1.5 py-0.5 rounded">Default</span>
                        {/if}
                        {#if !app.enabled}
                          <span class="text-xs bg-gray-600 text-gray-400 px-1.5 py-0.5 rounded">Disabled</span>
                        {/if}
                        {#if app.proxy}
                          <span class="app-indicator" title="Proxied through server">
                            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>
                          </span>
                        {/if}
                        {#if app.open_mode && app.open_mode !== 'iframe'}
                          <span class="app-indicator" title="Opens in {app.open_mode.replace('_', ' ')}">
                            {#if app.open_mode === 'new_tab'}
                              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
                            {:else if app.open_mode === 'new_window'}
                              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
                            {:else}
                              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M17 8l4 4m0 0l-4 4m4-4H3" /></svg>
                            {/if}
                          </span>
                        {/if}
                        {#if app.scale && app.scale !== 1}
                          <span class="app-indicator" title="Scaled to {Math.round(app.scale * 100)}%">
                            {Math.round(app.scale * 100)}%
                          </span>
                        {/if}
                      </div>
                      <span class="text-xs text-gray-400 truncate block">{app.url}</span>
                    </div>
                    {#if confirmDeleteApp?.name === app.name}
                      <div class="flex items-center gap-1">
                        <span class="text-xs text-red-400 mr-1">Delete?</span>
                        <button class="px-2 py-1 text-xs rounded bg-red-600 hover:bg-red-500 text-white"
                                onclick={confirmDeleteAppAction}>Yes</button>
                        <button class="px-2 py-1 text-xs rounded bg-gray-600 hover:bg-gray-500 text-white"
                                onclick={() => confirmDeleteApp = null}>No</button>
                      </div>
                    {:else}
                      <div class="flex items-center gap-1 opacity-0 group-hover/app:opacity-100 focus-within:opacity-100 transition-opacity app-actions">
                        <button class="p-1.5 text-gray-400 hover:text-white rounded hover:bg-white/10"
                                tabindex="-1"
                                onclick={() => editingApp = app} title="Edit">
                          <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                          </svg>
                        </button>
                        <button class="p-1.5 text-gray-400 hover:text-red-400 rounded hover:bg-white/10"
                                tabindex="-1"
                                onclick={() => deleteApp(app)} title="Delete">
                          <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                          </svg>
                        </button>
                      </div>
                    {/if}
                  </div>
                {/each}
              </div>
            </div>
          {/if}

          {#if localApps.length === 0 && localConfig.groups.length === 0}
            <div class="text-center py-8 text-gray-400">
              No applications or groups configured. Click "Add App" to get started.
            </div>
          {/if}
        </div>

      <!-- Theme Settings -->
      {:else if activeTab === 'theme'}
        <div class="space-y-6">
          <!-- Variant Mode Selector (Dark / System / Light) -->
          <div class="p-4 rounded-lg" style="background: var(--bg-elevated); border: 1px solid var(--border-subtle);">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3">
                <div class="w-10 h-10 rounded-lg flex items-center justify-center"
                     style="background: linear-gradient(135deg, var(--bg-surface) 50%, var(--bg-overlay) 50%); border: 1px solid var(--border-default);">
                  <svg class="w-5 h-5" style="color: var(--text-muted);" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                  </svg>
                </div>
                <div>
                  <div class="font-medium" style="color: var(--text-primary);">Appearance</div>
                  <div class="text-xs" style="color: var(--text-muted);">Choose dark, light, or follow your system</div>
                </div>
              </div>
              <!-- Three-way segmented control -->
              <div class="flex rounded-lg overflow-hidden" style="border: 1px solid var(--border-default);">
                {#each (['dark', 'system', 'light'] as const) as mode (mode)}
                  <button
                    class="px-3 py-1.5 text-xs font-medium transition-colors"
                    style="
                      background: {$variantMode === mode ? 'var(--accent-primary)' : 'var(--bg-surface)'};
                      color: {$variantMode === mode ? 'white' : 'var(--text-secondary)'};
                    "
                    onclick={() => setVariantMode(mode)}
                  >
                    {#if mode === 'dark'}
                      Dark
                    {:else if mode === 'system'}
                      System
                    {:else}
                      Light
                    {/if}
                  </button>
                {/each}
              </div>
            </div>
          </div>

          <!-- Theme Family Grid -->
          <div>
            <span class="block text-sm font-medium mb-3" style="color: var(--text-secondary);">
              Choose Theme
            </span>
            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {#each $themeFamilies as family (family.id)}
                {@const isSelected = $selectedFamily === family.id}
                {@const isCustom = family.darkTheme ? !family.darkTheme.isBuiltin : family.lightTheme ? !family.lightTheme.isBuiltin : false}
                <div
                  class="relative p-4 rounded-xl text-left transition-all group cursor-pointer"
                  style="
                    background: var(--bg-surface);
                    border: 2px solid {isSelected ? 'var(--accent-primary)' : 'var(--border-subtle)'};
                    box-shadow: {isSelected ? 'var(--shadow-glow)' : 'none'};
                  "
                  onclick={() => setThemeFamily(family.id)}
                  onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setThemeFamily(family.id); } }}
                  role="button"
                  tabindex="0"
                >
                  <!-- Selection indicator / delete button -->
                  <div class="absolute top-3 right-3 flex items-center gap-1">
                    {#if isCustom}
                      <button
                        class="w-5 h-5 rounded-full flex items-center justify-center opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity"
                        style="background: var(--status-error); color: white;"
                        tabindex="-1"
                        onclick={(e: MouseEvent) => { e.stopPropagation(); handleDeleteTheme(family.darkTheme?.id || family.lightTheme?.id || ''); }}
                        title="Delete theme"
                        aria-label="Delete theme"
                      >
                        <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                      </button>
                    {/if}
                    {#if isSelected}
                      <div class="w-5 h-5 rounded-full flex items-center justify-center"
                           style="background: var(--accent-primary);">
                        <svg class="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 20 20">
                          <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                        </svg>
                      </div>
                    {/if}
                  </div>

                  <!-- Dual Preview Swatches (dark left, light right) -->
                  <div class="flex gap-1.5 mb-3">
                    {#if family.darkTheme?.preview && family.lightTheme?.preview}
                      <!-- Dark variant swatch -->
                      <div class="w-10 h-12 rounded-lg overflow-hidden flex flex-col shadow-md"
                           style="border: 1px solid {family.darkTheme.preview.text}20;">
                        <div class="flex-1" style="background: {family.darkTheme.preview.bg};"></div>
                        <div class="h-2" style="background: {family.darkTheme.preview.accent};"></div>
                      </div>
                      <!-- Light variant swatch -->
                      <div class="w-10 h-12 rounded-lg overflow-hidden flex flex-col shadow-md"
                           style="border: 1px solid {family.lightTheme.preview.text}20;">
                        <div class="flex-1" style="background: {family.lightTheme.preview.bg};"></div>
                        <div class="h-2" style="background: {family.lightTheme.preview.accent};"></div>
                      </div>
                    {:else}
                      <!-- Single variant swatch -->
                      {@const theme = family.darkTheme || family.lightTheme}
                      {#if theme?.preview}
                        <div class="w-12 h-12 rounded-lg overflow-hidden flex flex-col shadow-md"
                             style="border: 1px solid {theme.preview.text}20;">
                          <div class="flex-1" style="background: {theme.preview.bg};"></div>
                          <div class="h-2" style="background: {theme.preview.accent};"></div>
                        </div>
                        <div class="flex flex-col gap-1">
                          <div class="w-6 h-5.5 rounded" style="background: {theme.preview.surface}; border: 1px solid {theme.preview.text}15;"></div>
                          <div class="w-6 h-5.5 rounded" style="background: {theme.preview.accent};"></div>
                        </div>
                      {:else}
                        <div class="w-12 h-12 rounded-lg flex items-center justify-center"
                             style="background: var(--bg-elevated); border: 1px solid var(--border-subtle);">
                          <svg class="w-6 h-6" style="color: var(--text-muted);" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
                          </svg>
                        </div>
                      {/if}
                    {/if}
                  </div>

                  <!-- Family Name & Badge -->
                  <div class="flex items-center gap-2">
                    <span class="font-medium" style="color: var(--text-primary);">{family.name}</span>
                    {#if isCustom}
                      <span class="text-[10px] px-1.5 py-0.5 rounded flex-shrink-0"
                            style="background: var(--accent-subtle); color: var(--accent-primary);">
                        Custom
                      </span>
                    {/if}
                  </div>
                  {#if family.description}
                    <div class="text-xs mt-0.5 pr-1" style="color: var(--text-muted);">{family.description}</div>
                  {/if}

                  <!-- Delete confirmation overlay -->
                  {#if confirmDeleteTheme === (family.darkTheme?.id || family.lightTheme?.id)}
                    <div class="absolute inset-0 rounded-xl flex items-center justify-center gap-3 z-10"
                         style="background: var(--bg-overlay); backdrop-filter: blur(4px);"
                         onclick={(e: MouseEvent) => e.stopPropagation()}
                         onkeydown={(e: KeyboardEvent) => e.stopPropagation()}
                         role="presentation">
                      <span class="text-sm font-medium" style="color: var(--text-primary);">Delete?</span>
                      <button class="px-3 py-1 rounded text-sm font-medium"
                              style="background: var(--status-error); color: white;"
                              onclick={(e: MouseEvent) => { e.stopPropagation(); confirmDeleteThemeAction(); }}>Yes</button>
                      <button class="px-3 py-1 rounded text-sm font-medium"
                              style="background: var(--bg-elevated); color: var(--text-primary);"
                              onclick={(e: MouseEvent) => { e.stopPropagation(); confirmDeleteTheme = null; }}>No</button>
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          </div>

          <!-- Current Theme Info -->
          <div class="p-4 rounded-lg" style="background: var(--bg-elevated); border: 1px solid var(--border-subtle);">
            <div class="flex items-center gap-2 text-sm">
              <span style="color: var(--text-muted);">Currently using:</span>
              <span class="font-medium capitalize" style="color: var(--text-primary);">
                {$allThemes.find(t => t.id === $resolvedTheme)?.name || $resolvedTheme} theme
              </span>
              {#if $variantMode === 'system'}
                <span class="text-xs" style="color: var(--text-disabled);">(from system preference)</span>
              {/if}
            </div>
          </div>

          <!-- Theme Customization -->
          <div class="space-y-3">
            {#if !showThemeEditor}
              <button
                class="w-full p-4 rounded-lg text-left transition-all hover:border-brand-500/50 flex items-center gap-3"
                style="background: var(--bg-surface); border: 1px solid var(--border-subtle);"
                onclick={openThemeEditor}
              >
                <div class="w-8 h-8 rounded-lg flex-shrink-0 flex items-center justify-center"
                     style="background: var(--accent-subtle);">
                  <svg class="w-4 h-4" style="color: var(--accent-primary);" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
                  </svg>
                </div>
                <div>
                  <div class="font-medium text-sm" style="color: var(--text-primary);">Customize Current Theme</div>
                  <p class="text-xs mt-0.5" style="color: var(--text-muted);">Tweak colors and save as a new custom theme</p>
                </div>
              </button>
            {:else}
              <!-- Theme Editor Panel -->
              <div class="rounded-lg overflow-hidden" style="border: 1px solid var(--border-default);">
                <div class="flex items-center justify-between p-3" style="background: var(--bg-elevated);">
                  <span class="text-sm font-medium" style="color: var(--text-primary);">Theme Editor</span>
                  <div class="flex items-center gap-2">
                    <button
                      class="px-2 py-1 text-xs rounded transition-colors"
                      style="background: var(--bg-hover); color: var(--text-secondary);"
                      onclick={resetAllThemeVars}
                    >Reset All</button>
                    <button
                      class="p-1 rounded transition-colors"
                      style="color: var(--text-muted);"
                      onclick={closeThemeEditor}
                      aria-label="Close theme editor"
                    >
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  </div>
                </div>

                <div class="p-3 space-y-4" style="background: var(--bg-surface);">
                  {#each Object.entries(themeVariableGroups) as [groupName, vars] (groupName)}
                    <div>
                      <div class="text-xs font-semibold uppercase tracking-wider mb-2" style="color: var(--text-muted);">{groupName}</div>
                      <div class="space-y-2">
                        {#each vars as varName (varName)}
                          {@const isColorVar = !themeEditorVars[varName]?.startsWith('rgba') && !themeEditorVars[varName]?.includes('px')}
                          <div class="flex items-center gap-2">
                            <span class="text-xs w-20 flex-shrink-0" style="color: var(--text-secondary);">{varLabels[varName] || varName.replace('--', '')}</span>
                            {#if isColorVar}
                              <input
                                type="color"
                                value={cssColorToHex(themeEditorVars[varName] || '#000000')}
                                oninput={(e) => updateThemeVar(varName, e.currentTarget.value)}
                                class="w-8 h-8 rounded cursor-pointer"
                              />
                            {/if}
                            <input
                              type="text"
                              value={themeEditorVars[varName] || ''}
                              oninput={(e) => updateThemeVar(varName, e.currentTarget.value)}
                              class="flex-1 px-2 py-1 text-xs rounded font-mono"
                              style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-subtle);"
                            />
                            {#if themeEditorVars[varName] !== themeEditorDefaults[varName]}
                              <button
                                class="p-1 rounded transition-colors flex-shrink-0"
                                style="color: var(--text-muted);"
                                onclick={() => resetThemeVar(varName)}
                                title="Reset to default"
                              >
                                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                </svg>
                              </button>
                            {:else}
                              <div class="w-[22px]"></div>
                            {/if}
                          </div>
                        {/each}
                      </div>
                    </div>
                  {/each}

                  <!-- Save as theme -->
                  <div class="pt-3 space-y-2" style="border-top: 1px solid var(--border-subtle);">
                    <input
                      type="text"
                      bind:value={saveThemeName}
                      placeholder="Theme name..."
                      class="w-full px-3 py-2 text-sm rounded"
                      style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-default);"
                    />
                    <input
                      type="text"
                      bind:value={saveThemeDescription}
                      placeholder="Description (optional)"
                      class="w-full px-3 py-2 text-sm rounded"
                      style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-default);"
                    />
                    <input
                      type="text"
                      bind:value={saveThemeAuthor}
                      placeholder="Author (optional)"
                      class="w-full px-3 py-2 text-sm rounded"
                      style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-default);"
                    />
                    <button
                      class="w-full px-4 py-2 text-sm rounded font-medium transition-colors disabled:opacity-50"
                      style="background: var(--accent-primary); color: var(--bg-base);"
                      disabled={!saveThemeName.trim() || isSavingTheme}
                      onclick={handleSaveTheme}
                    >
                      {isSavingTheme ? 'Saving...' : 'Save Theme'}
                    </button>
                    <p class="text-xs" style="color: var(--text-disabled);">
                      Saves as a CSS file on the server. Changes are live-previewed above.
                    </p>
                  </div>
                </div>
              </div>
            {/if}

          </div>
        </div>
      {/if}

      <!-- Keybindings Settings -->
      {#if activeTab === 'keybindings'}
        <KeybindingsEditor onchange={() => keybindingsChanged = true} />
      {/if}

      <!-- Security Settings -->
      {#if activeTab === 'security'}
        {@const currentMethod = localConfig.auth?.method || 'none'}
        {@const methodChanged = selectedAuthMethod !== currentMethod}
        {@const faFieldsChanged = selectedAuthMethod === 'forward_auth' && currentMethod === 'forward_auth' && (
          methodTrustedProxies !== (localConfig.auth?.trusted_proxies?.join('\n') || '') ||
          faHeaderUser !== (localConfig.auth?.headers?.user || 'Remote-User') ||
          faHeaderEmail !== (localConfig.auth?.headers?.email || 'Remote-Email') ||
          faHeaderGroups !== (localConfig.auth?.headers?.groups || 'Remote-Groups') ||
          faHeaderName !== (localConfig.auth?.headers?.name || 'Remote-Name')
        )}
        {@const showUpdateBtn = methodChanged || faFieldsChanged}
        <div class="space-y-8">
          {#if securitySuccess}
            <div class="p-3 rounded-lg bg-green-500/10 border border-green-500/20 text-green-400 text-sm">
              {securitySuccess}
            </div>
          {/if}

          <!-- Authentication Method -->
          <div>
            <h3 class="text-lg font-semibold text-white mb-1">Authentication Method</h3>
            <p class="text-sm text-gray-400 mb-4">Choose how users authenticate with Muximux</p>

            <div class="space-y-3">
              <!-- Password card -->
              <div
                class="rounded-xl border text-left transition-all overflow-hidden
                       {selectedAuthMethod === 'builtin' ? 'border-brand-500 bg-brand-500/10' : 'border-gray-700 bg-gray-800/50 hover:border-gray-600'}"
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
                      <h3 class="font-semibold text-white">Password authentication</h3>
                      {#if currentMethod === 'builtin'}
                        <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-green-500/20 text-green-400 uppercase tracking-wider">Current</span>
                      {/if}
                    </div>
                    <p class="text-sm text-gray-400 mt-1">Set up a username and password to protect your dashboard</p>
                  </div>
                </button>
                {#if selectedAuthMethod === 'builtin'}
                  <div class="px-4 pb-4 pt-0 ml-14" in:fly={{ y: -8, duration: 200 }}>
                    <div class="border-t border-gray-700 pt-4">
                      {#if currentMethod === 'builtin'}
                        <p class="text-sm text-gray-400 mb-4">Password authentication is active.</p>

                        <!-- Change Password (inline) -->
                        <h4 class="text-sm font-semibold text-white mb-2">Change Password</h4>
                        <div class="max-w-sm space-y-3">
                          <div>
                            <label for="cp-current" class="block text-xs text-gray-400 mb-1">Current password</label>
                            <input
                              id="cp-current"
                              type="password"
                              bind:value={cpCurrent}
                              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white text-sm
                                     focus:outline-none focus:ring-2 focus:ring-brand-500"
                              autocomplete="current-password"
                            />
                          </div>
                          <div>
                            <label for="cp-new" class="block text-xs text-gray-400 mb-1">New password</label>
                            <input
                              id="cp-new"
                              type="password"
                              bind:value={cpNew}
                              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white text-sm
                                     focus:outline-none focus:ring-2 focus:ring-brand-500"
                              placeholder="Minimum 8 characters"
                              autocomplete="new-password"
                            />
                            {#if cpNew.length > 0 && cpNew.length < 8}
                              <p class="text-red-400 text-xs mt-1">Password must be at least 8 characters</p>
                            {/if}
                          </div>
                          <div>
                            <label for="cp-confirm" class="block text-xs text-gray-400 mb-1">Confirm new password</label>
                            <input
                              id="cp-confirm"
                              type="password"
                              bind:value={cpConfirm}
                              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white text-sm
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
                            class="px-4 py-2 bg-brand-600 hover:bg-brand-700 text-white text-sm rounded-md transition-colors disabled:opacity-50 flex items-center gap-2"
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
                        <p class="text-sm text-gray-400">Switch to password authentication using existing users.</p>
                      {:else}
                        <p class="text-sm text-gray-400 mb-3">Create your first user to enable password authentication.</p>
                        <div class="space-y-3 max-w-sm">
                          <div>
                            <label for="setup-username" class="block text-xs text-gray-400 mb-1">Username</label>
                            <input
                              id="setup-username"
                              type="text"
                              bind:value={newUserName}
                              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white text-sm
                                     focus:outline-none focus:ring-2 focus:ring-brand-500"
                              placeholder="admin"
                            />
                          </div>
                          <div>
                            <label for="setup-password" class="block text-xs text-gray-400 mb-1">Password <span class="text-gray-500">(min 8 characters)</span></label>
                            <input
                              id="setup-password"
                              type="password"
                              bind:value={newUserPassword}
                              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white text-sm
                                     focus:outline-none focus:ring-2 focus:ring-brand-500"
                              placeholder="••••••••"
                            />
                          </div>
                          {#if addUserError}
                            <p class="text-red-400 text-xs">{addUserError}</p>
                          {/if}
                          {#if !newUserName.trim() && newUserPassword.length > 0}
                            <p class="text-amber-400 text-xs">Username is required</p>
                          {:else if newUserName.trim() && newUserPassword.length > 0 && newUserPassword.length < 8}
                            <p class="text-amber-400 text-xs">Password must be at least 8 characters ({newUserPassword.length}/8)</p>
                          {/if}
                          <button
                            class="px-4 py-2 bg-brand-600 hover:bg-brand-700 text-white text-sm rounded-md transition-colors disabled:opacity-50 flex items-center gap-2"
                            disabled={addUserLoading || !newUserName.trim() || newUserPassword.length < 8}
                            onclick={async () => {
                              const savedUser = newUserName.trim();
                              const savedPass = newUserPassword;
                              // First user must be admin to manage auth settings
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
                       {selectedAuthMethod === 'forward_auth' ? 'border-brand-500 bg-brand-500/10' : 'border-gray-700 bg-gray-800/50 hover:border-gray-600'}"
              >
                <button class="w-full p-4 flex items-start gap-4" onclick={async () => { selectedAuthMethod = 'forward_auth'; await tick(); document.getElementById('settings-proxies')?.focus(); }}>
                  <div class="w-10 h-10 rounded-lg bg-brand-500/20 flex items-center justify-center flex-shrink-0 mt-0.5">
                    <svg class="w-5 h-5 text-brand-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                    </svg>
                  </div>
                  <div class="flex-1 text-left">
                    <div class="flex items-center gap-2">
                      <h3 class="font-semibold text-white">Auth proxy</h3>
                      {#if currentMethod === 'forward_auth'}
                        <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-green-500/20 text-green-400 uppercase tracking-wider">Current</span>
                      {/if}
                    </div>
                    <p class="text-sm text-gray-400 mt-1">Authelia, Authentik, or another reverse proxy handles authentication</p>
                  </div>
                </button>
                {#if selectedAuthMethod === 'forward_auth'}
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
                      <label for="settings-proxies" class="block text-sm text-gray-400 mb-1">Trusted proxy IPs</label>
                      <textarea
                        id="settings-proxies"
                        bind:value={methodTrustedProxies}
                        class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white text-sm
                               focus:outline-none focus:ring-2 focus:ring-brand-500"
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
                          <label for="settings-header-user" class="block text-xs text-gray-400 mb-1">User header</label>
                          <input id="settings-header-user" type="text" bind:value={faHeaderUser}
                            class="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                        </div>
                        <div>
                          <label for="settings-header-email" class="block text-xs text-gray-400 mb-1">Email header</label>
                          <input id="settings-header-email" type="text" bind:value={faHeaderEmail}
                            class="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                        </div>
                        <div>
                          <label for="settings-header-groups" class="block text-xs text-gray-400 mb-1">Groups header</label>
                          <input id="settings-header-groups" type="text" bind:value={faHeaderGroups}
                            class="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                        </div>
                        <div>
                          <label for="settings-header-name" class="block text-xs text-gray-400 mb-1">Name header</label>
                          <input id="settings-header-name" type="text" bind:value={faHeaderName}
                            class="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:ring-1 focus:ring-brand-500" />
                        </div>
                      </div>
                    {/if}
                  </div>
                {/if}
              </div>

              <!-- No authentication card -->
              <div
                class="rounded-xl border text-left transition-all overflow-hidden
                       {selectedAuthMethod === 'none' ? 'border-amber-500 bg-amber-500/10' : 'border-gray-700 bg-gray-800/50 hover:border-gray-600'}"
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
                      <h3 class="font-semibold text-white">No authentication</h3>
                      {#if currentMethod === 'none'}
                        <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-green-500/20 text-green-400 uppercase tracking-wider">Current</span>
                      {/if}
                    </div>
                    <p class="text-sm text-gray-400 mt-1">Anyone with network access gets full control</p>
                  </div>
                </button>
                {#if selectedAuthMethod === 'none'}
                  <div class="px-4 pb-4 pt-0 ml-14" in:fly={{ y: -8, duration: 200 }}>
                    <div class="border-t border-gray-700 pt-4">
                      <div class="p-4 rounded-lg bg-amber-500/10 border border-amber-500/20">
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
                class="mt-4 px-4 py-2 bg-brand-600 hover:bg-brand-700 text-white text-sm rounded-md transition-colors disabled:opacity-50 flex items-center gap-2"
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
                  <h3 class="text-lg font-semibold text-white mb-1">User Management</h3>
                  <p class="text-sm text-gray-400">Manage dashboard users and roles</p>
                </div>
                <button
                  class="px-3 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md transition-colors flex items-center gap-1.5"
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
                <div class="p-4 rounded-lg bg-gray-800/50 border border-gray-700 mb-4 space-y-3" in:fly={{ y: -10, duration: 150 }}>
                  <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    <div>
                      <label for="new-user-name" class="block text-sm text-gray-400 mb-1">Username</label>
                      <input
                        id="new-user-name"
                        type="text"
                        bind:value={newUserName}
                        class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white text-sm
                               focus:outline-none focus:ring-2 focus:ring-brand-500"
                        placeholder="username"
                      />
                    </div>
                    <div>
                      <label for="new-user-password" class="block text-sm text-gray-400 mb-1">Password</label>
                      <input
                        id="new-user-password"
                        type="password"
                        bind:value={newUserPassword}
                        class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white text-sm
                               focus:outline-none focus:ring-2 focus:ring-brand-500"
                        placeholder="Min 8 characters"
                      />
                    </div>
                  </div>
                  <div>
                    <label for="new-user-role" class="block text-sm text-gray-400 mb-1">Role</label>
                    <select
                      id="new-user-role"
                      bind:value={newUserRole}
                      class="px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white text-sm
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
                      class="px-3 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md transition-colors disabled:opacity-50 flex items-center gap-1.5"
                      disabled={addUserLoading || !newUserName.trim() || newUserPassword.length < 8}
                      onclick={handleAddUser}
                    >
                      {#if addUserLoading}
                        <span class="inline-block w-3 h-3 border-2 border-white/30 border-t-white rounded-full animate-spin"></span>
                      {/if}
                      Add
                    </button>
                    <button
                      class="px-3 py-1.5 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700 transition-colors"
                      onclick={() => showAddUser = false}
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              {/if}

              <!-- User list -->
              {#if securityLoading}
                <div class="text-center py-4 text-gray-400">Loading users...</div>
              {:else}
                <div class="space-y-2">
                  {#each securityUsers as user (user.username)}
                    <div class="flex items-center gap-3 p-3 rounded-lg bg-gray-800/50 border border-gray-700">
                      <div class="w-8 h-8 rounded-full bg-gray-700 flex items-center justify-center text-sm font-medium text-gray-300">
                        {user.username.charAt(0).toUpperCase()}
                      </div>
                      <div class="flex-1 min-w-0">
                        <div class="text-sm font-medium text-white">{user.username}</div>
                        {#if user.email}
                          <div class="text-xs text-gray-500">{user.email}</div>
                        {/if}
                      </div>
                      <select
                        value={user.role}
                        onchange={(e) => handleUpdateUserRole(user.username, e.currentTarget.value)}
                        class="px-2 py-1 text-xs bg-gray-700 border border-gray-600 rounded text-white
                               focus:outline-none focus:ring-1 focus:ring-brand-500"
                      >
                        <option value="admin">Admin</option>
                        <option value="power-user">Power User</option>
                        <option value="user">User</option>
                      </select>
                      {#if confirmDeleteUser === user.username}
                        <div class="flex items-center gap-1.5">
                          <button
                            class="px-2 py-1 text-xs bg-red-600 hover:bg-red-500 text-white rounded"
                            onclick={() => handleDeleteUser(user.username)}
                          >Delete</button>
                          <button
                            class="px-2 py-1 text-xs bg-gray-600 hover:bg-gray-500 text-white rounded"
                            onclick={() => confirmDeleteUser = null}
                          >Cancel</button>
                        </div>
                      {:else}
                        <button
                          class="p-1.5 text-gray-500 hover:text-red-400 rounded transition-colors"
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
      {/if}

      <!-- About -->
      {#if activeTab === 'about'}
        <div class="space-y-6">
          {#if aboutLoading}
            <div class="flex items-center justify-center py-16">
              <svg class="w-6 h-6 text-brand-400 animate-spin" viewBox="0 0 24 24" fill="none">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
              </svg>
              <span class="ml-3 text-gray-400">Loading system info...</span>
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
                    <h3 class="text-lg font-semibold text-white">
                      {updateInfo?.update_available ? 'Update Available' : "You're up to date"}
                    </h3>
                    {#if updateInfo?.update_available}
                      <span class="px-2 py-0.5 text-xs font-medium bg-amber-500/20 text-amber-300 rounded-full">
                        v{updateInfo.latest_version}
                      </span>
                    {/if}
                  </div>
                  <div class="flex flex-wrap gap-x-4 gap-y-1 mt-1.5 text-sm text-gray-400">
                    <span>Current: <span class="text-white font-mono">{systemInfo.version}</span></span>
                    {#if updateInfo}
                      <span>Latest: <span class="text-white font-mono">{updateInfo.latest_version}</span></span>
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
                    class="px-3 py-1.5 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-lg flex items-center gap-1.5 transition-colors flex-shrink-0"
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
              <div class="rounded-xl border border-gray-700 overflow-hidden">
                <button
                  class="w-full flex items-center justify-between p-4 text-left hover:bg-gray-800/50 transition-colors"
                  onclick={() => changelogExpanded = !changelogExpanded}
                >
                  <h3 class="text-sm font-semibold text-white">Release Notes</h3>
                  <svg
                    class="w-4 h-4 text-gray-400 transition-transform {changelogExpanded ? 'rotate-180' : ''}"
                    fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
                  >
                    <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>
                {#if changelogExpanded}
                  <div class="px-4 pb-4 border-t border-gray-700/50">
                    <div class="mt-3 text-sm text-gray-300 leading-relaxed max-h-64 overflow-y-auto changelog-content">
                      <!-- eslint-disable-next-line svelte/no-at-html-tags -- changelog from GitHub release notes, sanitized by marked -->
                      {@html marked.parse(updateInfo.changelog)}
                    </div>
                  </div>
                {/if}
              </div>
            {/if}

            <!-- How to Update (collapsible) -->
            {#if updateInfo}
              <div class="rounded-xl border border-gray-700 overflow-hidden">
                <button
                  class="w-full flex items-center justify-between p-4 text-left hover:bg-gray-800/50 transition-colors"
                  onclick={() => updateInstructionsExpanded = !updateInstructionsExpanded}
                >
                  <h3 class="text-sm font-semibold text-white">How to Update</h3>
                  <svg
                    class="w-4 h-4 text-gray-400 transition-transform {updateInstructionsExpanded ? 'rotate-180' : ''}"
                    fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
                  >
                    <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>
                {#if updateInstructionsExpanded}
                  <div class="border-t border-gray-700/50 divide-y divide-gray-700/50">
                    <!-- Docker -->
                    <div class="p-4">
                      <div class="flex items-center gap-2 mb-2">
                        <svg class="w-5 h-5 text-blue-400" viewBox="0 0 24 24" fill="currentColor">
                          <path d="M13.983 11.078h2.119a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.119a.186.186 0 00-.185.186v1.887c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 00.186-.186V3.574a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.888c0 .102.082.185.185.186m0 2.716h2.118a.187.187 0 00.186-.186V6.29a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.887c0 .102.082.186.185.186m-2.93 0h2.12a.186.186 0 00.184-.186V6.29a.185.185 0 00-.185-.185H8.1a.186.186 0 00-.185.185v1.887c0 .102.083.186.185.186m-2.964 0h2.119a.186.186 0 00.185-.186V6.29a.186.186 0 00-.185-.185H5.136a.186.186 0 00-.186.185v1.887c0 .102.084.186.186.186m5.893 2.715h2.118a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.118a.185.185 0 00-.185.186v1.887c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.186v1.887c0 .102.083.185.185.185m-2.964 0h2.119a.186.186 0 00.185-.185V9.006a.186.186 0 00-.185-.186H5.136a.186.186 0 00-.186.186v1.887c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.186v1.887c0 .102.082.185.185.185M23.763 9.89c-.065-.051-.672-.51-1.954-.51-.338.001-.676.03-1.01.087-.248-1.7-1.653-2.53-1.716-2.566l-.344-.199-.226.327c-.284.438-.49.922-.612 1.43-.23.97-.09 1.882.403 2.661-.595.332-1.55.413-1.744.42H.751a.751.751 0 00-.75.748 11.687 11.687 0 00.692 4.062c.545 1.428 1.355 2.48 2.41 3.124 1.18.723 3.1 1.137 5.275 1.137.983.003 1.963-.086 2.93-.266a12.248 12.248 0 003.823-1.389c.98-.567 1.86-1.288 2.61-2.136 1.252-1.418 1.998-2.997 2.553-4.4h.221c1.372 0 2.215-.549 2.68-1.009.309-.293.55-.65.707-1.046l.098-.288z"/>
                        </svg>
                        <span class="text-sm font-medium text-white">Docker</span>
                        {#if systemInfo.environment === 'docker'}
                          <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-brand-500/20 text-brand-300 rounded uppercase tracking-wider">Your Platform</span>
                        {/if}
                      </div>
                      <pre class="text-xs text-gray-300 bg-gray-900/50 rounded-lg p-3 overflow-x-auto font-mono">cd /path/to/muximux
docker compose pull
docker compose up -d</pre>
                    </div>

                    <!-- Linux -->
                    <div class="p-4">
                      <div class="flex items-center gap-2 mb-2">
                        <svg class="w-5 h-5 text-yellow-400" viewBox="0 0 24 24" fill="currentColor">
                          <path d="M12.504 0c-.155 0-.315.008-.48.021-4.226.333-3.105 4.807-3.17 6.298-.076 1.092-.3 1.953-1.05 3.02-.885 1.051-2.127 2.75-2.716 4.521-.278.832-.41 1.684-.287 2.489a.424.424 0 00-.11.135c-.26.268-.45.6-.663.839-.199.199-.485.267-.797.4-.313.136-.658.269-.864.68-.09.189-.136.394-.132.602 0 .199.027.4.055.536.058.399.116.728.04.97-.249.68-.28 1.145-.106 1.484.174.334.535.47.94.601.81.2 1.91.135 2.774.6.926.466 1.866.67 2.616.47.526-.116.97-.464 1.208-.946.587-.003 1.23-.269 2.26-.334.699-.058 1.574.267 2.577.2.025.134.063.198.114.333l.003.003c.391.778 1.113 1.368 1.884 1.43.199.023.4-.002.64-.078.66-.27.735-.95.791-1.573.042-.468-.017-1.006.017-1.57.265-.112.49-.292.662-.545.272-.352.287-.803.163-1.202-.124-.398-.37-.724-.593-.975-.363-.4-.551-.486-.64-.608-.082-.125-.06-.312-.001-.524.104-.34.349-.608.606-.87.263-.268.545-.565.639-1.014.018-.013.033-.027.05-.04.28-.27.434-.556.469-.96.002-.395-.147-.742-.344-1.075-.2-.34-.432-.588-.595-.85-.115-.2-.131-.529.053-.779.223-.267.333-.485.3-.792-.03-.29-.201-.571-.424-.739-.322-.208-.583-.183-.757-.263-.168-.074-.277-.24-.432-.57-.097-.198-.237-.537-.427-.669-.19-.13-.45-.065-.585.002-.162.074-.27.068-.352.036-.05-.025-.088-.065-.074-.156.15-.4.24-.86.205-1.345-.046-.672-.202-1.349-.392-1.972-.19-.623-.428-1.206-.628-1.67-.36-.873-.663-1.432-.663-1.432z"/>
                        </svg>
                        <span class="text-sm font-medium text-white">Linux</span>
                        {#if systemInfo.environment === 'native' && systemInfo.os === 'linux'}
                          <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-brand-500/20 text-brand-300 rounded uppercase tracking-wider">Your Platform</span>
                        {/if}
                      </div>
                      <pre class="text-xs text-gray-300 bg-gray-900/50 rounded-lg p-3 overflow-x-auto font-mono"># Stop the running instance, then:
curl -LO https://github.com/mescon/Muximux/releases/latest/download/muximux-linux-amd64
chmod +x muximux-linux-amd64
./muximux-linux-amd64</pre>
                      {#if updateInfo.download_urls?.linux_amd64}
                        <div class="flex gap-2 mt-2">
                          <a href={updateInfo.download_urls.linux_amd64} class="text-xs text-brand-400 hover:text-brand-300 transition-colors">Download linux-amd64</a>
                          {#if updateInfo.download_urls?.linux_arm64}
                            <span class="text-gray-600">|</span>
                            <a href={updateInfo.download_urls.linux_arm64} class="text-xs text-brand-400 hover:text-brand-300 transition-colors">Download linux-arm64</a>
                          {/if}
                        </div>
                      {/if}
                    </div>

                    <!-- macOS -->
                    <div class="p-4">
                      <div class="flex items-center gap-2 mb-2">
                        <svg class="w-5 h-5 text-gray-300" viewBox="0 0 24 24" fill="currentColor">
                          <path d="M18.71 19.5c-.83 1.24-1.71 2.45-3.05 2.47-1.34.03-1.77-.79-3.29-.79-1.53 0-2 .77-3.27.82-1.31.05-2.3-1.32-3.14-2.53C4.25 17 2.94 12.45 4.7 9.39c.87-1.52 2.43-2.48 4.12-2.51 1.28-.02 2.5.87 3.29.87.78 0 2.26-1.07 3.8-.91.65.03 2.47.26 3.64 1.98-.09.06-2.17 1.28-2.15 3.81.03 3.02 2.65 4.03 2.68 4.04-.03.07-.42 1.44-1.38 2.83M13 3.5c.73-.83 1.94-1.46 2.94-1.5.13 1.17-.34 2.35-1.04 3.19-.69.85-1.83 1.51-2.95 1.42-.15-1.15.41-2.35 1.05-3.11z"/>
                        </svg>
                        <span class="text-sm font-medium text-white">macOS</span>
                        {#if systemInfo.environment === 'native' && systemInfo.os === 'darwin'}
                          <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-brand-500/20 text-brand-300 rounded uppercase tracking-wider">Your Platform</span>
                        {/if}
                      </div>
                      <pre class="text-xs text-gray-300 bg-gray-900/50 rounded-lg p-3 overflow-x-auto font-mono">curl -LO https://github.com/mescon/Muximux/releases/latest/download/muximux-darwin-arm64
chmod +x muximux-darwin-arm64
./muximux-darwin-arm64</pre>
                      {#if updateInfo.download_urls?.darwin_arm64}
                        <div class="flex gap-2 mt-2">
                          <a href={updateInfo.download_urls.darwin_arm64} class="text-xs text-brand-400 hover:text-brand-300 transition-colors">Download darwin-arm64</a>
                          {#if updateInfo.download_urls?.darwin_amd64}
                            <span class="text-gray-600">|</span>
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
                        <span class="text-sm font-medium text-white">Windows</span>
                        {#if systemInfo.environment === 'native' && systemInfo.os === 'windows'}
                          <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-brand-500/20 text-brand-300 rounded uppercase tracking-wider">Your Platform</span>
                        {/if}
                      </div>
                      <pre class="text-xs text-gray-300 bg-gray-900/50 rounded-lg p-3 overflow-x-auto font-mono"># Download muximux-windows-amd64.exe from the release page
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
              <h3 class="text-sm font-semibold text-white mb-3">System Information</h3>
              <div class="grid grid-cols-3 gap-3 mb-3">
                <div class="rounded-lg bg-gray-800/50 border border-gray-700 p-3 text-center">
                  <div class="flex items-center justify-center gap-1.5 mb-1">
                    {#if systemInfo.environment === 'docker'}
                      <svg class="w-4 h-4 text-blue-400" viewBox="0 0 24 24" fill="currentColor">
                        <path d="M13.983 11.078h2.119a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.119a.186.186 0 00-.185.186v1.887c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 00.186-.186V3.574a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.888c0 .102.082.185.185.186m0 2.716h2.118a.187.187 0 00.186-.186V6.29a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.887c0 .102.082.186.185.186m-2.93 0h2.12a.186.186 0 00.184-.186V6.29a.185.185 0 00-.185-.185H8.1a.186.186 0 00-.185.185v1.887c0 .102.083.186.185.186m-2.964 0h2.119a.186.186 0 00.185-.186V6.29a.186.186 0 00-.185-.185H5.136a.186.186 0 00-.186.185v1.887c0 .102.084.186.186.186m5.893 2.715h2.118a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.118a.185.185 0 00-.185.186v1.887c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.186v1.887c0 .102.083.185.185.185m-2.964 0h2.119a.186.186 0 00.185-.185V9.006a.186.186 0 00-.185-.186H5.136a.186.186 0 00-.186.186v1.887c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.186v1.887c0 .102.082.185.185.185M23.763 9.89c-.065-.051-.672-.51-1.954-.51-.338.001-.676.03-1.01.087-.248-1.7-1.653-2.53-1.716-2.566l-.344-.199-.226.327c-.284.438-.49.922-.612 1.43-.23.97-.09 1.882.403 2.661-.595.332-1.55.413-1.744.42H.751a.751.751 0 00-.75.748 11.687 11.687 0 00.692 4.062c.545 1.428 1.355 2.48 2.41 3.124 1.18.723 3.1 1.137 5.275 1.137.983.003 1.963-.086 2.93-.266a12.248 12.248 0 003.823-1.389c.98-.567 1.86-1.288 2.61-2.136 1.252-1.418 1.998-2.997 2.553-4.4h.221c1.372 0 2.215-.549 2.68-1.009.309-.293.55-.65.707-1.046l.098-.288z"/>
                      </svg>
                    {:else}
                      <svg class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <rect x="2" y="3" width="20" height="14" rx="2" /><path d="M8 21h8m-4-4v4" />
                      </svg>
                    {/if}
                  </div>
                  <div class="text-xs text-gray-500 mb-0.5">Environment</div>
                  <div class="text-sm text-white capitalize">{systemInfo.environment}</div>
                </div>
                <div class="rounded-lg bg-gray-800/50 border border-gray-700 p-3 text-center">
                  <div class="flex items-center justify-center gap-1.5 mb-1">
                    <svg class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
                    </svg>
                  </div>
                  <div class="text-xs text-gray-500 mb-0.5">Platform</div>
                  <div class="text-sm text-white">{systemInfo.os}/{systemInfo.arch}</div>
                </div>
                <div class="rounded-lg bg-gray-800/50 border border-gray-700 p-3 text-center">
                  <div class="flex items-center justify-center gap-1.5 mb-1">
                    <svg class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M12 6v6l4 2" />
                    </svg>
                  </div>
                  <div class="text-xs text-gray-500 mb-0.5">Uptime</div>
                  <div class="text-sm text-white">{systemInfo.uptime}</div>
                </div>
              </div>

              <div class="rounded-lg bg-gray-800/50 border border-gray-700 divide-y divide-gray-700/50">
                <div class="flex items-center justify-between px-4 py-2.5">
                  <span class="text-xs text-gray-500">Data Directory</span>
                  <span class="text-xs text-gray-300 font-mono">{systemInfo.data_dir}</span>
                </div>
                <div class="flex items-center justify-between px-4 py-2.5">
                  <span class="text-xs text-gray-500">Go Version</span>
                  <span class="text-xs text-gray-300 font-mono">{systemInfo.go_version}</span>
                </div>
                <div class="flex items-center justify-between px-4 py-2.5">
                  <span class="text-xs text-gray-500">Build Date</span>
                  <span class="text-xs text-gray-300 font-mono">{systemInfo.build_date}</span>
                </div>
                <div class="flex items-center justify-between px-4 py-2.5">
                  <span class="text-xs text-gray-500">Commit</span>
                  <span class="text-xs text-gray-300 font-mono">{systemInfo.commit.length > 8 ? systemInfo.commit.slice(0, 8) : systemInfo.commit}</span>
                </div>
              </div>
            </div>

            <!-- Links -->
            <div>
              <h3 class="text-sm font-semibold text-white mb-3">Links</h3>
              <div class="flex flex-wrap gap-2">
                <a
                  href={systemInfo.links.github}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-gray-800 hover:bg-gray-700 border border-gray-700 text-gray-300 hover:text-white rounded-lg transition-colors"
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
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-gray-800 hover:bg-gray-700 border border-gray-700 text-gray-300 hover:text-white rounded-lg transition-colors"
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
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-gray-800 hover:bg-gray-700 border border-gray-700 text-gray-300 hover:text-white rounded-lg transition-colors"
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
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-gray-800 hover:bg-gray-700 border border-gray-700 text-gray-300 hover:text-white rounded-lg transition-colors"
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
      {/if}
    </div>
  </div>
</div>

<!-- Add App Modal -->
{#if showAddApp}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
    transition:fade={{ duration: 100 }}
  >
    <div
      class="bg-gray-800 rounded-xl shadow-2xl w-full border border-gray-700 {addAppStep === 'choose' ? 'max-w-2xl' : 'max-w-lg'}"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-gray-700">
        <div class="flex items-center gap-2">
          {#if addAppStep === 'configure'}
            <button
              class="p-1 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
              onclick={() => { addAppStep = 'choose'; addAppSearch = ''; }}
              aria-label="Back"
            >
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
              </svg>
            </button>
          {/if}
          <h3 class="text-lg font-semibold text-white">{addAppStep === 'choose' ? 'Add Application' : 'Configure ' + (newApp.name || 'App')}</h3>
        </div>
        <button
          class="p-1 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => showAddApp = false}
          aria-label="Close"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      {#if addAppStep === 'choose'}
        <!-- Step 1: Choose from popular apps or custom -->
        <div class="p-4 max-h-[65vh] overflow-y-auto">
          <!-- Search -->
          <div class="mb-4">
            <input
              type="text"
              bind:value={addAppSearch}
              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
              placeholder="Search apps..."
            />
          </div>

          <!-- Custom App card -->
          {#if !addAppSearch}
            <button
              class="w-full flex items-center gap-3 p-3 mb-4 rounded-lg border-2 border-dashed border-gray-600 hover:border-brand-500 hover:bg-gray-700/50 transition-colors text-left"
              onclick={startCustomApp}
            >
              <div class="w-10 h-10 rounded-lg bg-gray-700 flex items-center justify-center flex-shrink-0">
                <svg class="w-5 h-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                </svg>
              </div>
              <div>
                <div class="text-sm font-medium text-white">Custom App</div>
                <div class="text-xs text-gray-400">Add any app with a custom URL and icon</div>
              </div>
            </button>
          {/if}

          <!-- Popular apps by category -->
          {#each Object.entries(popularApps) as [category, templates] (category)}
            {@const filtered = addAppSearch ? templates.filter(t => t.name.toLowerCase().includes(addAppSearchLower) || t.description.toLowerCase().includes(addAppSearchLower)) : templates}
            {#if filtered.length > 0}
              <div class="mb-4">
                <h4 class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2">{category}</h4>
                <div class="grid grid-cols-2 gap-2">
                  {#each filtered as template (template.name)}
                    {@const alreadyAdded = localApps.some(a => a.name === template.name)}
                    <button
                      class="flex items-center gap-3 p-2.5 rounded-lg text-left transition-colors hover:bg-gray-700/50 {alreadyAdded ? 'bg-gray-700/30' : 'bg-gray-800/50'}"
                      onclick={() => selectPopularApp(template)}
                      title={template.description}
                    >
                      <div class="flex-shrink-0">
                        <AppIcon icon={{ type: template.iconType || 'dashboard', name: template.icon, file: '', url: '', variant: 'svg', background: template.iconBackground }} name={template.name} color={template.color} size="sm" showBackground={localConfig.navigation.show_icon_background} />
                      </div>
                      <div class="min-w-0">
                        <div class="text-sm font-medium text-white truncate flex items-center gap-1.5">
                          {template.name}
                          {#if alreadyAdded}
                            <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-gray-600 text-gray-400 font-normal flex-shrink-0">added</span>
                          {/if}
                        </div>
                        <div class="text-xs text-gray-500 truncate">{template.description}</div>
                      </div>
                    </button>
                  {/each}
                </div>
              </div>
            {/if}
          {/each}

          {#if addAppSearch && Object.values(popularApps).every(templates => templates.every(t => !t.name.toLowerCase().includes(addAppSearchLower) && !t.description.toLowerCase().includes(addAppSearchLower)))}
            <div class="text-center py-6">
              <p class="text-gray-400 text-sm mb-3">No matching apps found</p>
              <button
                class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md"
                onclick={startCustomApp}
              >
                Add as Custom App
              </button>
            </div>
          {/if}
        </div>
      {:else}
        <!-- Step 2: Configure app details -->
        <div class="p-4 space-y-4 max-h-[60vh] overflow-y-auto">
          <div>
            <label for="app-name" class="block text-sm font-medium text-gray-300 mb-1">Name</label>
            <input
              id="app-name"
              type="text"
              bind:value={newApp.name}
              oninput={() => { delete appErrors.name; appErrors = appErrors; }}
              class="w-full px-3 py-2 bg-gray-700 border rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 {appErrors.name ? 'border-red-500' : 'border-gray-600'}"
              placeholder="My App"
            />
            {#if appErrors.name}<p class="text-red-400 text-xs mt-1">{appErrors.name}</p>{/if}
          </div>
          <div>
            <label for="app-url" class="block text-sm font-medium text-gray-300 mb-1">URL</label>
            <input
              id="app-url"
              type="url"
              bind:value={newApp.url}
              oninput={() => { delete appErrors.url; appErrors = appErrors; }}
              class="w-full px-3 py-2 bg-gray-700 border rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 {appErrors.url ? 'border-red-500' : 'border-gray-600'}"
              placeholder="http://localhost:8080"
            />
            {#if appErrors.url}<p class="text-red-400 text-xs mt-1">{appErrors.url}</p>{/if}
          </div>
          <div>
            <span class="block text-sm font-medium text-gray-300 mb-1">Icon</span>
            <div class="flex items-center gap-3">
              <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('newApp')}>
                <AppIcon icon={newApp.icon} name={newApp.name || 'App'} color={newApp.color} size="lg" />
              </button>
              <button
                class="px-3 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md flex-1 text-left"
                onclick={() => openIconBrowser('newApp')}
              >
                {newApp.icon?.name || 'Choose icon...'}
              </button>
            </div>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label for="app-color" class="block text-sm font-medium text-gray-300 mb-1">Color</label>
              <div class="flex items-center gap-2">
                <input
                  id="app-color"
                  type="color"
                  bind:value={newApp.color}
                  class="w-10 h-10 rounded cursor-pointer"
                />
                <input
                  type="text"
                  bind:value={newApp.color}
                  class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
                />
              </div>
            </div>
            <div>
              <label for="app-group" class="block text-sm font-medium text-gray-300 mb-1">Group</label>
              <select
                id="app-group"
                bind:value={newApp.group}
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
              >
                <option value="">No group</option>
                {#each localConfig.groups as group (group.name)}
                  <option value={group.name}>{group.name}</option>
                {/each}
              </select>
            </div>
          </div>
          <div>
            <label for="app-mode" class="block text-sm font-medium text-gray-300 mb-1">
              Open Mode
              <span class="help-trigger relative ml-1 inline-block align-middle">
                <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                </svg>
                <span class="help-tooltip">
                  <b>Embedded</b> — loads inside Muximux in an iframe. Best for most apps.<br/>
                  <b>New Tab</b> — opens in a separate browser tab.<br/>
                  <b>New Window</b> — opens in a popup window.
                </span>
              </span>
            </label>
            <select
              id="app-mode"
              bind:value={newApp.open_mode}
              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
            >
              {#each openModes as mode (mode.value)}
                <option value={mode.value}>{mode.label} - {mode.description}</option>
              {/each}
            </select>
          </div>
          <div>
            <label for="app-scale" class="block text-sm font-medium text-gray-300 mb-1">
              Scale: {Math.round(newApp.scale * 100)}%
            </label>
            <input
              id="app-scale"
              type="range"
              min="0.5"
              max="2"
              step="0.05"
              bind:value={newApp.scale}
              class="w-full"
            />
          </div>
          <div class="space-y-2">
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={newApp.enabled}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-white">Enabled</span>
                <p class="text-xs text-gray-400">Show this app in the navigation</p>
              </div>
            </label>
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={newApp.default}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-white">Default app</span>
                <p class="text-xs text-gray-400">Automatically load this app on startup instead of the overview</p>
              </div>
            </label>
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={newApp.proxy}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-white">Use reverse proxy</span>
                <p class="text-xs text-gray-400">Route traffic through the built-in Caddy proxy to avoid CORS and mixed-content issues</p>
              </div>
            </label>
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={newApp.force_icon_background}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-white">Force icon background</span>
                <p class="text-xs text-gray-400">Show background even when global icon backgrounds are off</p>
              </div>
            </label>
          </div>
          <div>
            <label for="new-app-min-role" class="block text-sm font-medium text-gray-300 mb-1">Minimum Role</label>
            <select
              id="new-app-min-role"
              bind:value={newApp.min_role}
              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
            >
              <option value="">Everyone (default)</option>
              <option value="power-user">Power User</option>
              <option value="admin">Admin</option>
            </select>
            <p class="text-xs text-gray-400 mt-1">Users below this role won't see this app</p>
          </div>
        </div>
        <div class="flex justify-end gap-2 p-4 border-t border-gray-700">
          <button
            class="px-4 py-2 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
            onclick={() => showAddApp = false}
          >
            Cancel
          </button>
          <button
            class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md"
            onclick={addApp}
          >
            Add App
          </button>
        </div>
      {/if}
    </div>
  </div>
{/if}

<!-- Add Group Modal -->
{#if showAddGroup}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
    transition:fade={{ duration: 100 }}
  >
    <div
      class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-md border border-gray-700"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-gray-700">
        <h3 class="text-lg font-semibold text-white">Add Group</h3>
        <button
          class="p-1 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => showAddGroup = false}
          aria-label="Close"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4">
        <div>
          <label for="group-name" class="block text-sm font-medium text-gray-300 mb-1">Name</label>
          <input
            id="group-name"
            type="text"
            bind:value={newGroup.name}
            oninput={() => { delete groupErrors.name; groupErrors = groupErrors; }}
            class="w-full px-3 py-2 bg-gray-700 border rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 {groupErrors.name ? 'border-red-500' : 'border-gray-600'}"
            placeholder="Media"
          />
          {#if groupErrors.name}<p class="text-red-400 text-xs mt-1">{groupErrors.name}</p>{/if}
        </div>
        <div>
          <span class="block text-sm font-medium text-gray-300 mb-1">Icon</span>
          <div class="flex items-center gap-3">
            <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('newGroup')}>
              <AppIcon icon={newGroup.icon} name={newGroup.name || 'G'} color={newGroup.color} size="lg" />
            </button>
            <button
              class="px-3 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md flex-1 text-left"
              onclick={() => openIconBrowser('newGroup')}
            >
              {newGroup.icon?.name || 'Choose icon...'}
            </button>
          </div>
        </div>
        <div>
          <label for="group-color" class="block text-sm font-medium text-gray-300 mb-1">Color</label>
          <div class="flex items-center gap-2">
            <input
              id="group-color"
              type="color"
              bind:value={newGroup.color}
              class="w-10 h-10 rounded cursor-pointer"
            />
            <input
              type="text"
              bind:value={newGroup.color}
              class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
            />
          </div>
        </div>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-gray-700">
        <button
          class="px-4 py-2 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => showAddGroup = false}
        >
          Cancel
        </button>
        <button
          class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md"
          onclick={addGroup}
        >
          Add Group
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Edit App Modal -->
{#if editingApp}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
    transition:fade={{ duration: 100 }}
  >
    <div
      class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-lg border border-gray-700"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-gray-700">
        <h3 class="text-lg font-semibold text-white">Edit {editingApp.name}</h3>
        <button
          class="p-1 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={closeEditApp}
          aria-label="Close"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4 max-h-[60vh] overflow-y-auto">
        <!-- Identity -->
        <div>
          <label for="edit-app-name" class="block text-sm font-medium text-gray-300 mb-1">
            Name
            <span class="help-trigger relative ml-1 inline-block align-middle">
              <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
              </svg>
              <span class="help-tooltip">
                Displayed in the navigation bar and page title. Also used as a unique identifier — renaming an app creates a new proxy route.
              </span>
            </span>
          </label>
          <input
            id="edit-app-name"
            type="text"
            bind:value={editingApp.name}
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </div>
        <div>
          <label for="edit-app-url" class="block text-sm font-medium text-gray-300 mb-1">
            URL
            <span class="help-trigger relative ml-1 inline-block align-middle">
              <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
              </svg>
              <span class="help-tooltip">
                The full address of the application. Used to load the app in an iframe, or as the link when opened in a new tab/window.
              </span>
            </span>
          </label>
          <input
            id="edit-app-url"
            type="url"
            bind:value={editingApp.url}
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </div>
        <div>
          <span class="block text-sm font-medium text-gray-300 mb-1">Icon</span>
          <div class="flex items-center gap-3">
            <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('editApp')}>
              <AppIcon icon={editingApp.icon} name={editingApp.name} color={editingApp.color} size="lg" />
            </button>
            <div class="flex-1">
              <button
                class="px-3 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md w-full text-left"
                onclick={() => openIconBrowser('editApp')}
              >
                {editingApp.icon?.name || 'Choose icon...'}
              </button>
              <p class="text-xs text-gray-400 mt-1">
                {editingApp.icon?.type === 'dashboard' ? 'Dashboard Icon' : editingApp.icon?.type || 'No icon set'}
              </p>
            </div>
          </div>
          {#if editingApp.icon?.type === 'lucide'}
            <div class="flex items-center gap-4 mt-2">
              <label class="flex items-center gap-2 text-xs text-gray-400">
                Icon color
                <input type="color" value={editingApp!.icon.color || '#ffffff'} oninput={(e) => editingApp!.icon.color = (e.target as HTMLInputElement).value} class="w-8 h-8 rounded cursor-pointer" />
                {#if editingApp!.icon.color}
                  <button class="text-gray-500 hover:text-gray-300" onclick={() => editingApp!.icon.color = ''} title="Reset to theme default">&times;</button>
                {/if}
              </label>
              <label class="flex items-center gap-2 text-xs text-gray-400">
                Background
                <input type="color" value={editingApp!.icon.background || editingApp!.color || '#374151'} oninput={(e) => editingApp!.icon.background = (e.target as HTMLInputElement).value} class="w-8 h-8 rounded cursor-pointer" />
                <button class="text-gray-500 hover:text-gray-300 text-xs" onclick={() => editingApp!.icon.background = 'transparent'} title="Transparent">none</button>
                {#if editingApp!.icon.background}
                  <button class="text-gray-500 hover:text-gray-300" onclick={() => editingApp!.icon.background = ''} title="Reset to app color">&times;</button>
                {/if}
              </label>
            </div>
          {/if}
        </div>
        <div>
          <label for="edit-app-color" class="block text-sm font-medium text-gray-300 mb-1">
            Color
            <span class="help-trigger relative ml-1 inline-block align-middle">
              <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
              </svg>
              <span class="help-tooltip">
                Used as the active tab indicator, icon background, and sidebar accent when "Show App Colors" is enabled.
              </span>
            </span>
          </label>
          <div class="flex items-center gap-2">
            <input
              id="edit-app-color"
              type="color"
              bind:value={editingApp.color}
              class="w-10 h-10 rounded cursor-pointer"
            />
            <input
              type="text"
              bind:value={editingApp.color}
              class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
            />
          </div>
        </div>
        <div>
          <label for="edit-app-group" class="block text-sm font-medium text-gray-300 mb-1">
            Group
            <span class="help-trigger relative ml-1 inline-block align-middle">
              <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
              </svg>
              <span class="help-tooltip">
                Groups organize apps into collapsible sections in the sidebar. Apps with no group appear under "Ungrouped."
              </span>
            </span>
          </label>
          <select
            id="edit-app-group"
            bind:value={editingApp.group}
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            <option value="">No group</option>
            {#each localConfig.groups as group (group.name)}
              <option value={group.name}>{group.name}</option>
            {/each}
          </select>
        </div>

        <!-- Display -->
        <div class="border-t border-gray-700 pt-3">
          <h4 class="text-xs font-medium text-gray-500 uppercase tracking-wide mb-3">Display</h4>
          <div class="space-y-3">
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={editingApp.enabled}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-white">Enabled
                  <span class="help-trigger relative ml-1 inline-block align-middle">
                    <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <span class="help-tooltip">
                      Disabled apps are hidden from non-admin users and excluded from the navigation entirely.
                    </span>
                  </span>
                </span>
                <p class="text-xs text-gray-400">Show this app in the navigation</p>
              </div>
            </label>
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={editingApp.default}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-white">Default app
                  <span class="help-trigger relative ml-1 inline-block align-middle">
                    <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <span class="help-tooltip">
                      Only one app can be the default. Setting this will clear the default flag on any other app.
                    </span>
                  </span>
                </span>
                <p class="text-xs text-gray-400">Automatically load this app on startup instead of the overview</p>
              </div>
            </label>
            <div>
              <label for="edit-app-mode" class="block text-sm font-medium text-gray-300 mb-1">
                Open Mode
                <span class="help-trigger relative ml-1 inline-block align-middle">
                  <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                  </svg>
                  <span class="help-tooltip">
                    <b>Embedded</b> — loads inside Muximux in an iframe. Best for most apps.<br/>
                    <b>New Tab</b> — opens in a separate browser tab.<br/>
                    <b>New Window</b> — opens in a popup window.
                  </span>
                </span>
              </label>
              <select
                id="edit-app-mode"
                bind:value={editingApp.open_mode}
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
              >
                {#each openModes as mode (mode.value)}
                  <option value={mode.value}>{mode.label}</option>
                {/each}
              </select>
            </div>
            <div>
              <label for="edit-app-scale" class="block text-sm font-medium text-gray-300 mb-1">
                Scale: {Math.round(editingApp.scale * 100)}%
                <span class="help-trigger relative ml-1 inline-block align-middle">
                  <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                  </svg>
                  <span class="help-tooltip">
                    Zoom level for the embedded iframe. Useful for apps with small or large UIs. Only applies to iframe open mode.
                  </span>
                </span>
              </label>
              <input
                id="edit-app-scale"
                type="range"
                min="0.5"
                max="2"
                step="0.05"
                bind:value={editingApp.scale}
                class="w-full"
              />
            </div>
          </div>
        </div>

        <!-- Proxy -->
        <div class="border-t border-gray-700 pt-3">
          <h4 class="text-xs font-medium text-gray-500 uppercase tracking-wide mb-3">Proxy</h4>
          <div class="space-y-3">
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={editingApp.proxy}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-white">Use reverse proxy
                  <span class="help-trigger relative ml-1 inline-block align-middle">
                    <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <span class="help-tooltip">
                      Routes all traffic through the built-in Caddy reverse proxy. The app URL is rewritten to a local <code>/proxy/app-name/</code> path, avoiding CORS, mixed-content, and cookie-domain issues.
                    </span>
                  </span>
                </span>
                <p class="text-xs text-gray-400">Route traffic through the built-in proxy to avoid CORS and mixed-content issues</p>
              </div>
            </label>
            {#if editingApp.proxy}
              <div class="ml-7 space-y-3 border-l-2 border-gray-700 pl-4 min-w-0 overflow-hidden">
                <label class="flex items-center gap-3 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={editingApp.proxy_skip_tls_verify !== false}
                    onchange={(e) => { editingApp!.proxy_skip_tls_verify = (e.target as HTMLInputElement).checked ? undefined : false; }}
                    class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
                  />
                  <div>
                    <span class="text-sm text-white">Skip TLS verification
                      <span class="help-trigger relative ml-1 inline-block align-middle">
                        <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                        </svg>
                        <span class="help-tooltip">
                          Enabled by default. Disable this only if the backend has a valid, trusted TLS certificate and you want strict verification.
                        </span>
                      </span>
                    </span>
                    <p class="text-xs text-gray-400">Disable for backends with valid certificates</p>
                  </div>
                </label>
                <div>
                  <span class="block text-sm text-gray-400 mb-1">Custom headers</span>
                  <p class="text-xs text-gray-500 mb-2">Sent to the backend on every proxied request (e.g. Authorization, X-Api-Key)</p>
                  {#each Object.entries(editingApp.proxy_headers ?? {}) as [key, value] (key)}
                    <div class="flex gap-2 mb-2">
                      <input type="text" value={key} placeholder="Header name"
                        class="flex-1 min-w-0 px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-500"
                        onchange={(e) => {
                          const app = editingApp!;
                          const headers = { ...(app.proxy_headers ?? {}) };
                          const oldKey = key;
                          const newKey = (e.target as HTMLInputElement).value.trim();
                          if (newKey && newKey !== oldKey) {
                            delete headers[oldKey];
                            headers[newKey] = value;
                            app.proxy_headers = headers;
                          }
                        }}
                      />
                      <input type="text" value={value} placeholder="Value"
                        class="flex-1 min-w-0 px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-500"
                        onchange={(e) => {
                          const app = editingApp!;
                          const headers = { ...(app.proxy_headers ?? {}) };
                          headers[key] = (e.target as HTMLInputElement).value;
                          app.proxy_headers = headers;
                        }}
                      />
                      <button class="px-2 py-1 text-gray-400 hover:text-red-400" title="Remove header"
                        onclick={() => {
                          const app = editingApp!;
                          const headers = { ...(app.proxy_headers ?? {}) };
                          delete headers[key];
                          app.proxy_headers = Object.keys(headers).length > 0 ? headers : undefined;
                        }}
                      >
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
                      </button>
                    </div>
                  {/each}
                  <button class="text-xs text-brand-400 hover:text-brand-300"
                    onclick={() => {
                      const app = editingApp!;
                      app.proxy_headers = { ...(app.proxy_headers ?? {}), '': '' };
                    }}
                  >+ Add header</button>
                </div>
              </div>
            {/if}
          </div>
        </div>

        <!-- Advanced -->
        <div class="border-t border-gray-700 pt-3">
          <h4 class="text-xs font-medium text-gray-500 uppercase tracking-wide mb-3">Advanced</h4>
          <div class="space-y-3">
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={editingApp.health_check !== false}
                onchange={(e) => {
                  editingApp!.health_check = (e.target as HTMLInputElement).checked ? undefined : false;
                }}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-white">Health check
                  <span class="help-trigger relative ml-1 inline-block align-middle">
                    <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <span class="help-tooltip">
                      Periodically pings the app URL (or health URL if set) and shows a status indicator in the navigation. Enabled by default.
                    </span>
                  </span>
                </span>
                <p class="text-xs text-gray-400">Monitor availability of this app</p>
              </div>
            </label>
            {#if editingApp.health_check !== false}
              <div class="ml-7 pl-4 border-l-2 border-gray-700">
                <label for="edit-app-health-url" class="block text-sm text-gray-400 mb-1">Health check URL</label>
                <input
                  id="edit-app-health-url"
                  type="url"
                  bind:value={editingApp.health_url}
                  placeholder={editingApp.url || 'Uses app URL if empty'}
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                />
                <p class="text-xs text-gray-500 mt-1">Leave blank to use the app URL</p>
              </div>
            {/if}
            <div class="flex items-center gap-3">
              <div class="flex-1">
                <span class="text-sm text-white">Keyboard Shortcut
                  <span class="help-trigger relative ml-1 inline-block align-middle">
                    <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <span class="help-tooltip">
                      Assigns a number key (1–9) to quickly switch to this app. Each number can only be assigned to one app.
                    </span>
                  </span>
                </span>
                <p class="text-xs text-gray-400">Press this number key to switch to this app</p>
              </div>
              <select
                value={editingApp.shortcut ?? ''}
                onchange={(e) => {
                  const val = (e.target as HTMLSelectElement).value;
                  editingApp!.shortcut = val ? parseInt(val) : undefined;
                }}
                class="px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded text-white focus:ring-brand-500 focus:border-brand-500"
              >
                <option value="">None</option>
                {#each [1,2,3,4,5,6,7,8,9] as n (n)}
                  {@const taken = localApps.find(a => a.shortcut === n && a.name !== editingApp?.name)}
                  <option value={n} disabled={!!taken}>{n}{taken ? ` (${taken.name})` : ''}</option>
                {/each}
              </select>
            </div>
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={editingApp.force_icon_background}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-white">Force icon background
                  <span class="help-trigger relative ml-1 inline-block align-middle">
                    <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <span class="help-tooltip">
                      Always show a colored background circle behind this app's icon, even when the global "Show Icon Backgrounds" setting is off.
                    </span>
                  </span>
                </span>
                <p class="text-xs text-gray-400">Show background even when global icon backgrounds are off</p>
              </div>
            </label>
            <div>
              <label for="edit-app-min-role" class="block text-sm font-medium text-gray-300 mb-1">
                Minimum Role
                <span class="help-trigger relative ml-1 inline-block align-middle">
                  <svg class="w-3.5 h-3.5 text-gray-500 cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                  </svg>
                  <span class="help-tooltip">
                    Restricts visibility based on user role. Users below the selected role won't see this app in the navigation or API responses.
                  </span>
                </span>
              </label>
              <select
                id="edit-app-min-role"
                bind:value={editingApp.min_role}
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
              >
                <option value="">Everyone (default)</option>
                <option value="power-user">Power User</option>
                <option value="admin">Admin</option>
              </select>
              <p class="text-xs text-gray-400 mt-1">Users below this role won't see this app</p>
            </div>
          </div>
        </div>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-gray-700">
        <button
          class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md"
          onclick={closeEditApp}
        >
          Done
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Edit Group Modal -->
{#if editingGroup}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
    transition:fade={{ duration: 100 }}
  >
    <div
      class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-md border border-gray-700"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-gray-700">
        <h3 class="text-lg font-semibold text-white">Edit {editingGroup.name}</h3>
        <button
          class="p-1 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={closeEditGroup}
          aria-label="Close"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4">
        <div>
          <label for="edit-group-name" class="block text-sm font-medium text-gray-300 mb-1">Name</label>
          <input
            id="edit-group-name"
            type="text"
            bind:value={editingGroup.name}
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </div>
        <div>
          <span class="block text-sm font-medium text-gray-300 mb-1">Icon</span>
          <div class="flex items-center gap-3">
            <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('editGroup')}>
              <AppIcon icon={editingGroup.icon} name={editingGroup.name} color={editingGroup.color} size="lg" />
            </button>
            <div class="flex-1">
              <button
                class="px-3 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md w-full text-left"
                onclick={() => openIconBrowser('editGroup')}
              >
                {editingGroup.icon?.name || 'Choose icon...'}
              </button>
              <p class="text-xs text-gray-400 mt-1">
                {editingGroup.icon?.type === 'dashboard' ? 'Dashboard Icon' : editingGroup.icon?.type || 'No icon set'}
              </p>
            </div>
          </div>
          {#if editingGroup.icon?.type === 'lucide'}
            <div class="flex items-center gap-4 mt-2">
              <label class="flex items-center gap-2 text-xs text-gray-400">
                Icon color
                <input type="color" value={editingGroup!.icon.color || '#ffffff'} oninput={(e) => editingGroup!.icon.color = (e.target as HTMLInputElement).value} class="w-8 h-8 rounded cursor-pointer" />
                {#if editingGroup!.icon.color}
                  <button class="text-gray-500 hover:text-gray-300" onclick={() => editingGroup!.icon.color = ''} title="Reset to theme default">&times;</button>
                {/if}
              </label>
              <label class="flex items-center gap-2 text-xs text-gray-400">
                Background
                <input type="color" value={editingGroup!.icon.background || editingGroup!.color || '#374151'} oninput={(e) => editingGroup!.icon.background = (e.target as HTMLInputElement).value} class="w-8 h-8 rounded cursor-pointer" />
                <button class="text-gray-500 hover:text-gray-300 text-xs" onclick={() => editingGroup!.icon.background = 'transparent'} title="Transparent">none</button>
                {#if editingGroup!.icon.background}
                  <button class="text-gray-500 hover:text-gray-300" onclick={() => editingGroup!.icon.background = ''} title="Reset to group color">&times;</button>
                {/if}
              </label>
            </div>
          {/if}
        </div>
        <div>
          <label for="edit-group-color" class="block text-sm font-medium text-gray-300 mb-1">Color</label>
          <div class="flex items-center gap-2">
            <input
              id="edit-group-color"
              type="color"
              bind:value={editingGroup.color}
              class="w-10 h-10 rounded cursor-pointer"
            />
            <input
              type="text"
              bind:value={editingGroup.color}
              class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
            />
          </div>
        </div>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-gray-700">
        <button
          class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md"
          onclick={closeEditGroup}
        >
          Done
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Icon Browser Modal -->
{#if showIconBrowser}
  <div
    class="fixed inset-0 z-[70] flex items-center justify-center bg-black/50 p-4"
    transition:fade={{ duration: 100 }}
  >
    <div
      class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-3xl border border-gray-700"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-gray-700">
        <h3 class="text-lg font-semibold text-white">Select Icon</h3>
        <button
          class="p-1 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => { showIconBrowser = false; iconBrowserTarget = null; }}
          aria-label="Close"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <IconBrowser
        selectedIcon={
          iconBrowserTarget === 'editApp' && editingApp?.icon?.type === 'dashboard' ? editingApp.icon.name :
          iconBrowserTarget === 'editGroup' && editingGroup?.icon?.type === 'dashboard' ? editingGroup.icon.name :
          ''
        }
        onselect={handleIconSelect}
        onclose={() => { showIconBrowser = false; iconBrowserTarget = null; }}
      />
    </div>
  </div>
{/if}

<!-- Import Confirmation Modal -->
{#if showImportConfirm && pendingImport}
  <div
    class="fixed inset-0 z-[70] flex items-center justify-center bg-black/50 p-4"
    transition:fade={{ duration: 100 }}
  >
    <div
      class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-md border border-gray-700"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-gray-700">
        <h3 class="text-lg font-semibold text-white">Import Configuration</h3>
        <button
          class="p-1 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={cancelImport}
          aria-label="Close"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4">
        <p class="text-gray-300">
          This will replace your current configuration with the imported settings.
        </p>
        <div class="bg-gray-700/50 rounded-lg p-3 text-sm">
          <div class="text-gray-400">Preview:</div>
          <div class="text-white font-medium">{pendingImport.title}</div>
          <div class="text-gray-400 text-xs mt-1">
            {pendingImport.apps.length} apps, {pendingImport.groups.length} groups
          </div>
        </div>
        <p class="text-yellow-400 text-sm flex items-center gap-2">
          <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          Unsaved changes will be overwritten
        </p>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-gray-700">
        <button
          class="px-4 py-2 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={cancelImport}
        >
          Cancel
        </button>
        <button
          class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md"
          onclick={applyImport}
        >
          Import
        </button>
      </div>
    </div>
  </div>
{/if}
</div>

<style>
  /* Theme-aware overrides: map Tailwind's hardcoded grays to CSS custom properties.
     This makes the Settings UI adapt to light, dark, and custom themes instead of
     being locked to dark-mode gray values. */

  /* Surface backgrounds */
  .settings :global(.bg-gray-800) {
    background-color: var(--bg-surface) !important;
  }
  .settings :global(.bg-gray-700) {
    background-color: var(--bg-elevated) !important;
  }
  .settings :global([class*="bg-gray-700/"]) {
    background-color: var(--bg-hover) !important;
  }
  .settings :global(.bg-gray-600) {
    background-color: var(--bg-overlay) !important;
  }

  /* Borders */
  .settings :global(.border-gray-700) {
    border-color: var(--border-default) !important;
  }
  .settings :global(.border-gray-600) {
    border-color: var(--border-subtle) !important;
  }
  .settings :global(.border-gray-500) {
    border-color: var(--border-strong) !important;
  }

  /* Text */
  .settings :global(.text-white) {
    color: var(--text-primary) !important;
  }
  .settings :global(.text-gray-100),
  .settings :global(.text-gray-200) {
    color: var(--text-primary) !important;
  }
  .settings :global(.text-gray-300) {
    color: var(--text-secondary) !important;
  }
  .settings :global(.text-gray-400) {
    color: var(--text-muted) !important;
  }
  .settings :global(.text-gray-500) {
    color: var(--text-disabled) !important;
  }

  /* Hover backgrounds */
  .settings :global(.hover\:bg-gray-700:hover) {
    background-color: var(--bg-hover) !important;
  }
  .settings :global(.hover\:bg-gray-600:hover) {
    background-color: var(--bg-active) !important;
  }
  .settings :global(.hover\:bg-gray-500:hover) {
    background-color: var(--bg-active) !important;
  }

  /* Hover text */
  .settings :global(.hover\:text-white:hover) {
    color: var(--text-primary) !important;
  }
  .settings :global(.hover\:text-gray-300:hover) {
    color: var(--text-secondary) !important;
  }

  /* Hover borders */
  .settings :global(.hover\:border-gray-600:hover) {
    border-color: var(--border-default) !important;
  }
  .settings :global(.hover\:border-gray-500:hover) {
    border-color: var(--border-strong) !important;
  }

  /* App status indicators (global so they survive DnD reparenting to body) */
  :global(.app-indicator) {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    font-size: 0.875rem;
    line-height: 1;
    padding: 4px 8px;
    border-radius: 4px;
    background: var(--bg-elevated);
    color: var(--text-muted);
    white-space: nowrap;
    flex-shrink: 0;
  }

  /* Drop indicator for intra-group drag-and-drop */
  .settings :global(.drop-indicator) {
    height: 2px;
    background: var(--accent-primary);
    border-radius: 1px;
    margin: 0 8px;
    box-shadow: 0 0 6px var(--accent-primary);
  }

  /* Action button group pill background */
  .app-actions {
    background: var(--bg-overlay, rgba(0, 0, 0, 0.4));
    border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
    border-radius: 6px;
    padding: 2px;
  }

  .app-actions svg {
    width: 1rem;
    height: 1rem;
  }

  /* Help tooltips */
  .help-tooltip {
    display: none;
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    width: 240px;
    padding: 8px 10px;
    border-radius: 8px;
    background: var(--bg-overlay, #1f2937);
    border: 1px solid var(--border-default, #374151);
    color: var(--text-secondary, #d1d5db);
    font-size: 11px;
    line-height: 1.4;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
    z-index: 70;
    pointer-events: none;
  }

  .help-trigger:hover > .help-tooltip {
    display: block;
  }

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

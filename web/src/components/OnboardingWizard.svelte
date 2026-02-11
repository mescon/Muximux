<script lang="ts">
  import { onMount } from 'svelte';
  import { fly, fade } from 'svelte/transition';
  import type { App, Group, NavigationConfig } from '$lib/types';
  import {
    currentStep,
    selectedApps,
    selectedNavigation,
    showLabels,
    selectedGroups,
    nextStep,
    prevStep,
    stepProgress,
    totalSteps
  } from '$lib/onboardingStore';
  import { popularApps, getAllGroups, templateToApp } from '$lib/popularApps';
  import type { PopularAppTemplate } from '$lib/popularApps';
  import AppIcon from './AppIcon.svelte';

  // Props
  let {
    oncomplete
  }: {
    oncomplete?: (detail: { apps: App[]; navigation: NavigationConfig; groups: Group[] }) => void;
  } = $props();

  // Track which apps are selected with their URLs
  let appSelections = $state<Map<string, { selected: boolean; url: string }>>(new Map());

  // Custom app form
  let showCustomApp = $state(false);
  let customApp = $state({
    name: '',
    url: '',
    color: '#22c55e',
    group: ''
  });

  // Initialize app selections
  onMount(() => {
    Object.values(popularApps).flat().forEach(app => {
      appSelections.set(app.name, { selected: false, url: app.defaultUrl });
    });
  });

  // Toggle app selection
  function toggleApp(app: PopularAppTemplate) {
    const current = appSelections.get(app.name);
    if (current) {
      appSelections.set(app.name, { ...current, selected: !current.selected });
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
    const groupsWithApps = new Set<string>();
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

  // Navigation position options
  const navPositions: { value: NavigationConfig['position']; label: string; description: string; icon: string }[] = [
    { value: 'top', label: 'Top Bar', description: 'Horizontal navigation at the top', icon: 'top' },
    { value: 'left', label: 'Left Sidebar', description: 'Vertical sidebar on the left', icon: 'left' },
    { value: 'right', label: 'Right Sidebar', description: 'Vertical sidebar on the right', icon: 'right' },
    { value: 'bottom', label: 'Bottom Dock', description: 'macOS-style dock at the bottom', icon: 'bottom' },
    { value: 'floating', label: 'Floating', description: 'Minimal floating buttons', icon: 'floating' }
  ];

  // Add custom app
  function addCustomApp() {
    if (!customApp.name || !customApp.url) return;

    const newApp: App = {
      name: customApp.name,
      url: customApp.url,
      icon: { type: 'dashboard', name: '', file: '', url: '', variant: '' },
      color: customApp.color,
      group: customApp.group,
      order: selectedCount,
      enabled: true,
      default: false,
      open_mode: 'iframe',
      proxy: false,
      scale: 1,
      disable_keyboard_shortcuts: false
    };

    selectedApps.update(apps => [...apps, newApp]);
    customApp = { name: '', url: '', color: '#22c55e', group: '' };
    showCustomApp = false;
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
    $selectedApps.forEach(app => {
      apps.push({ ...app, order: order++ });
    });

    // Set first app as default if any
    if (apps.length > 0) {
      apps[0].default = true;
    }

    // Build groups
    const groups: Group[] = suggestedGroups.map((name, i) => ({
      name,
      icon: { type: 'lucide', name: '', file: '', url: '', variant: '' },
      color: getGroupColor(name),
      order: i,
      expanded: true
    }));

    // Build navigation config
    const navigation: NavigationConfig = {
      position: $selectedNavigation,
      width: '220px',
      auto_hide: false,
      auto_hide_delay: '3s',
      show_on_hover: true,
      show_labels: $showLabels,
      show_logo: true,
      show_app_colors: true,
      show_icon_background: true,
      show_splash_on_startup: true,
      show_shadow: true
    };

    oncomplete?.({ apps, navigation, groups });
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

  // Step indicators
  const steps = ['Welcome', 'Apps', 'Style', 'Groups', 'Done'];
</script>

<div class="fixed inset-0 z-50 bg-gray-900 overflow-hidden flex flex-col">
  <!-- Progress bar -->
  <div class="flex-shrink-0 px-8 pt-6">
    <div class="max-w-3xl mx-auto">
      <div class="flex items-center justify-between mb-2">
        {#each steps as step, i}
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
              <div class="w-16 sm:w-24 h-0.5 mx-2 {i < $stepProgress ? 'bg-brand-500' : 'bg-gray-700'}"></div>
            {/if}
          </div>
        {/each}
      </div>
      <div class="flex justify-between text-xs text-gray-500">
        {#each steps as step, i}
          <span class="w-8 text-center {i <= $stepProgress ? 'text-gray-300' : ''}">{step}</span>
          {#if i < steps.length - 1}
            <span class="w-16 sm:w-24"></span>
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
            Your unified homelab dashboard. Let's set up your applications in a few quick steps.
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

      <!-- Step 2: Add Apps -->
      {:else if $currentStep === 'apps'}
        <div class="py-6" in:fly={{ x: 30, duration: 300 }} out:fade={{ duration: 150 }}>
          <div class="text-center mb-8">
            <h2 class="text-2xl font-bold text-white mb-2">Add Your Applications</h2>
            <p class="text-gray-400">Select the apps you use and customize their URLs</p>
          </div>

          <!-- App categories -->
          {#each Object.entries(popularApps) as [category, apps]}
            <div class="mb-8">
              <h3 class="text-lg font-semibold text-gray-300 mb-4 flex items-center gap-2">
                <span class="w-2 h-2 rounded-full" style="background-color: {getGroupColor(category)}"></span>
                {category}
              </h3>

              <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                {#each apps as app}
                  {@const selection = appSelections.get(app.name)}
                  <div
                    class="relative p-4 rounded-lg border transition-all cursor-pointer
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
                    <div class="absolute top-3 right-3">
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
                      <!-- App icon -->
                      <AppIcon
                        icon={{ type: 'dashboard', name: app.icon, file: '', url: '', variant: 'svg' }}
                        name={app.name}
                        color={app.color}
                        size="lg"
                      />

                      <div class="flex-1 min-w-0">
                        <h4 class="font-medium text-white">{app.name}</h4>
                        <p class="text-xs text-gray-400 mb-2">{app.description}</p>

                        <!-- URL input (shown when selected) -->
                        {#if selection?.selected}
                          <input
                            type="url"
                            value={selection.url}
                            oninput={(e) => updateAppUrl(app.name, e.currentTarget.value)}
                            onclick={(e) => e.stopPropagation()}
                            class="w-full px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded
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

          <!-- Custom app section -->
          <div class="mt-8 pt-6 border-t border-gray-700">
            {#if showCustomApp}
              <div class="p-4 bg-gray-800/50 rounded-lg border border-gray-700">
                <h4 class="font-medium text-white mb-4">Add Custom Application</h4>
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
                  <div>
                    <label for="custom-group" class="block text-sm text-gray-400 mb-1">Group (optional)</label>
                    <input
                      id="custom-group"
                      type="text"
                      bind:value={customApp.group}
                      class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                             focus:outline-none focus:ring-2 focus:ring-brand-500"
                      placeholder="Utilities"
                    />
                  </div>
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
                class="flex items-center gap-2 px-4 py-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-800"
                onclick={() => showCustomApp = true}
              >
                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                </svg>
                Add custom application
              </button>
            {/if}

            <!-- Custom apps list -->
            {#if $selectedApps.length > 0}
              <div class="mt-4 space-y-2">
                {#each $selectedApps as app}
                  <div class="flex items-center gap-3 p-3 bg-gray-800/50 rounded-lg border border-gray-700">
                    <div class="w-8 h-8 rounded flex items-center justify-center font-bold text-white"
                         style="background-color: {app.color}">
                      {app.name.charAt(0).toUpperCase()}
                    </div>
                    <div class="flex-1">
                      <div class="text-white font-medium">{app.name}</div>
                      <div class="text-xs text-gray-400">{app.url}</div>
                    </div>
                    <button
                      class="p-1 text-gray-400 hover:text-red-400"
                      onclick={() => selectedApps.update(apps => apps.filter(a => a.name !== app.name))}
                      aria-label="Remove app"
                    >
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        </div>

      <!-- Step 3: Navigation Style -->
      {:else if $currentStep === 'navigation'}
        <div class="py-6" in:fly={{ x: 30, duration: 300 }} out:fade={{ duration: 150 }}>
          <div class="text-center mb-8">
            <h2 class="text-2xl font-bold text-white mb-2">Choose Your Navigation Style</h2>
            <p class="text-gray-400">Select how you want to navigate between your apps</p>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 max-w-4xl mx-auto">
            {#each navPositions as pos}
              <button
                class="p-6 rounded-xl border text-left transition-all
                       {$selectedNavigation === pos.value
                         ? 'border-brand-500 bg-brand-500/10'
                         : 'border-gray-700 hover:border-gray-600 bg-gray-800/50'}"
                onclick={() => selectedNavigation.set(pos.value)}
              >
                <!-- Visual preview -->
                <div class="w-full aspect-video mb-4 bg-gray-900 rounded-lg overflow-hidden relative border border-gray-700">
                  {#if pos.value === 'top'}
                    <div class="absolute top-0 left-0 right-0 h-3 bg-gray-700"></div>
                    <div class="absolute top-4 left-2 right-2 bottom-2 bg-gray-800 rounded"></div>
                  {:else if pos.value === 'left'}
                    <div class="absolute top-0 left-0 bottom-0 w-6 bg-gray-700"></div>
                    <div class="absolute top-2 left-8 right-2 bottom-2 bg-gray-800 rounded"></div>
                  {:else if pos.value === 'right'}
                    <div class="absolute top-0 right-0 bottom-0 w-6 bg-gray-700"></div>
                    <div class="absolute top-2 left-2 right-8 bottom-2 bg-gray-800 rounded"></div>
                  {:else if pos.value === 'bottom'}
                    <div class="absolute bottom-0 left-0 right-0 h-3 bg-gray-700"></div>
                    <div class="absolute top-2 left-2 right-2 bottom-4 bg-gray-800 rounded"></div>
                  {:else if pos.value === 'floating'}
                    <div class="absolute top-2 left-2 right-2 bottom-2 bg-gray-800 rounded"></div>
                    <div class="absolute bottom-3 left-1/2 transform -translate-x-1/2 flex gap-1">
                      <div class="w-2 h-2 rounded-full bg-gray-600"></div>
                      <div class="w-2 h-2 rounded-full bg-gray-600"></div>
                      <div class="w-2 h-2 rounded-full bg-gray-600"></div>
                    </div>
                  {/if}
                </div>

                <h3 class="font-semibold text-white mb-1">{pos.label}</h3>
                <p class="text-sm text-gray-400">{pos.description}</p>

                <!-- Selection indicator -->
                {#if $selectedNavigation === pos.value}
                  <div class="mt-3 flex items-center gap-1 text-brand-400 text-sm">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                    </svg>
                    Selected
                  </div>
                {/if}
              </button>
            {/each}
          </div>

          <!-- Show labels toggle -->
          <div class="max-w-md mx-auto mt-8">
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

      <!-- Step 4: Groups -->
      {:else if $currentStep === 'groups'}
        <div class="py-6" in:fly={{ x: 30, duration: 300 }} out:fade={{ duration: 150 }}>
          <div class="text-center mb-8">
            <h2 class="text-2xl font-bold text-white mb-2">Organize with Groups</h2>
            <p class="text-gray-400">Based on your selections, we suggest these groups</p>
          </div>

          {#if suggestedGroups.length > 0}
            <div class="max-w-md mx-auto space-y-3">
              {#each suggestedGroups as group}
                <div class="flex items-center gap-4 p-4 bg-gray-800/50 rounded-lg border border-gray-700">
                  <div class="w-10 h-10 rounded-lg" style="background-color: {getGroupColor(group)}"></div>
                  <div class="flex-1">
                    <div class="font-medium text-white">{group}</div>
                    <div class="text-sm text-gray-400">
                      {[...appSelections.entries()]
                        .filter(([name, sel]) => sel.selected && Object.values(popularApps).flat().find(a => a.name === name)?.group === group)
                        .length} apps
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          {:else}
            <div class="max-w-md mx-auto text-center py-8 text-gray-400">
              <p>No groups needed - all your apps will appear in a single list.</p>
            </div>
          {/if}

          <p class="text-center text-gray-500 text-sm mt-8 max-w-md mx-auto">
            You can customize groups later in Settings
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
                <dt class="text-gray-400">Groups</dt>
                <dd class="text-white">{suggestedGroups.length}</dd>
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
        {/if}
      </div>

      <div>
        {#if $currentStep !== 'welcome' && $currentStep !== 'complete'}
          <button
            class="px-6 py-2 bg-brand-600 hover:bg-brand-700 text-white rounded-md transition-colors disabled:opacity-50"
            disabled={$currentStep === 'apps' && selectedCount + $selectedApps.length === 0}
            onclick={nextStep}
          >
            {$currentStep === 'groups' ? 'Finish' : 'Continue'}
          </button>
        {/if}
      </div>
    </div>
  </div>
</div>

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
</style>

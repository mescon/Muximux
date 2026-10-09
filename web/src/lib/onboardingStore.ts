import { writable, derived, get } from 'svelte/store';
import type { App, Config, Group, NavigationConfig, ThemeConfig } from './types';

export type OnboardingStep = 'welcome' | 'security' | 'apps' | 'navigation' | 'theme' | 'complete';

// Dynamic step order — set by configureSteps()
export const activeStepOrder = writable<OnboardingStep[]>(['welcome', 'apps', 'navigation', 'theme', 'complete']);

export function configureSteps(includeSetup: boolean): void {
  if (includeSetup) {
    activeStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
  } else {
    activeStepOrder.set(['welcome', 'apps', 'navigation', 'theme', 'complete']);
  }
}

export function getStepOrder(): OnboardingStep[] {
  return get(activeStepOrder);
}

export function getTotalSteps(): number {
  return get(activeStepOrder).length;
}

// Current step in the wizard
export const currentStep = writable<OnboardingStep>('welcome');

// Selected apps during onboarding (with user-provided URLs)
export const selectedApps = writable<App[]>([]);

// Selected navigation style
export const selectedNavigation = writable<NavigationConfig['position']>('left');

// Whether to show labels
export const showLabels = writable<boolean>(true);

// Groups to create (based on selected apps)
export const selectedGroups = writable<Group[]>([]);

// Reset onboarding state back to initial step
export function resetOnboarding(): void {
  currentStep.set('welcome');
  selectedApps.set([]);
  selectedNavigation.set('left');
  showLabels.set(true);
  selectedGroups.set([]);
  activeStepOrder.set(['welcome', 'apps', 'navigation', 'theme', 'complete']);
}

// Navigate to next step
export function nextStep(): void {
  const current = get(currentStep);
  const order = get(activeStepOrder);
  const currentIndex = order.indexOf(current);
  if (currentIndex < order.length - 1) {
    currentStep.set(order[currentIndex + 1]);
  }
}

// Navigate to previous step
export function prevStep(): void {
  const current = get(currentStep);
  const order = get(activeStepOrder);
  const currentIndex = order.indexOf(current);
  if (currentIndex > 0) {
    currentStep.set(order[currentIndex - 1]);
  }
}

// Go to specific step
export function goToStep(step: OnboardingStep): void {
  currentStep.set(step);
}

// Get step progress (0-based index)
export const stepProgress = derived([currentStep, activeStepOrder], ([$step, $order]) => {
  return $order.indexOf($step);
});

// mergeOnboardingResult folds the wizard's picks into the loaded config
// instead of replacing it: apps and groups are appended by name (entries
// already in the config win), navigation is spread over the existing block,
// the theme is the wizard's choice and the language is set. Every other
// section (discovery, auth, health, keybindings) is left as loaded.
export function mergeOnboardingResult(
  current: Config,
  picked: { apps: App[]; groups: Group[]; navigation: Partial<NavigationConfig>; theme: ThemeConfig; language: string },
): Config {
  const currentGroups = current.groups ?? [];
  const currentApps = current.apps ?? [];

  const groupNames = new Set(currentGroups.map(g => g.name));
  let nextOrder = currentGroups.reduce((max, g) => Math.max(max, g.order + 1), 0);
  const addedGroups = picked.groups
    .filter(g => !groupNames.has(g.name))
    .map(g => ({ ...g, order: nextOrder++ }));

  // The wizard numbers its own apps from shortcut 1 and marks its first app
  // default; keep the config's claims and drop the wizard's where they clash.
  const appNames = new Set(currentApps.map(a => a.name));
  const takenShortcuts = new Set(currentApps.map(a => a.shortcut).filter(s => s !== undefined));
  const hasDefault = currentApps.some(a => a.default);
  const addedApps = picked.apps
    .filter(a => !appNames.has(a.name))
    .map(a => {
      const app = { ...a };
      if (hasDefault) app.default = false;
      if (app.shortcut !== undefined) {
        if (takenShortcuts.has(app.shortcut)) delete app.shortcut;
        else takenShortcuts.add(app.shortcut);
      }
      return app;
    });

  return {
    ...current,
    language: picked.language,
    navigation: { ...current.navigation, ...picked.navigation },
    theme: picked.theme,
    groups: [...currentGroups, ...addedGroups],
    apps: [...currentApps, ...addedApps],
  };
}


import { writable, derived, get } from 'svelte/store';
import type { App, Group, NavigationConfig } from './types';

export type OnboardingStep = 'welcome' | 'security' | 'apps' | 'navigation' | 'theme' | 'complete';

// Dynamic step order — set by configureSteps()
let activeStepOrder: OnboardingStep[] = ['welcome', 'apps', 'navigation', 'theme', 'complete'];

export function configureSteps(includeSetup: boolean): void {
  if (includeSetup) {
    activeStepOrder = ['welcome', 'security', 'apps', 'navigation', 'theme', 'complete'];
  } else {
    activeStepOrder = ['welcome', 'apps', 'navigation', 'theme', 'complete'];
  }
}

export function getStepOrder(): OnboardingStep[] {
  return activeStepOrder;
}

export function getTotalSteps(): number {
  return activeStepOrder.length;
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
  activeStepOrder = ['welcome', 'apps', 'navigation', 'theme', 'complete'];
}

// Navigate to next step
export function nextStep(): void {
  const current = get(currentStep);
  const currentIndex = activeStepOrder.indexOf(current);
  if (currentIndex < activeStepOrder.length - 1) {
    currentStep.set(activeStepOrder[currentIndex + 1]);
  }
}

// Navigate to previous step
export function prevStep(): void {
  const current = get(currentStep);
  const currentIndex = activeStepOrder.indexOf(current);
  if (currentIndex > 0) {
    currentStep.set(activeStepOrder[currentIndex - 1]);
  }
}

// Go to specific step
export function goToStep(step: OnboardingStep): void {
  currentStep.set(step);
}

// Get step progress (0-based index)
export const stepProgress = derived(currentStep, ($step) => {
  return activeStepOrder.indexOf($step);
});

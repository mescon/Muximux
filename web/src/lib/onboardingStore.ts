import { writable, derived, get } from 'svelte/store';
import type { App, Group, NavigationConfig } from './types';

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

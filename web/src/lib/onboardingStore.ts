import { writable, derived, get } from 'svelte/store';
import type { App, Group, NavigationConfig } from './types';

export type OnboardingStep = 'welcome' | 'apps' | 'navigation' | 'groups' | 'complete';

const ONBOARDING_KEY = 'muximux_onboarded';

// Step order for navigation
const STEP_ORDER: OnboardingStep[] = ['welcome', 'apps', 'navigation', 'groups', 'complete'];

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

// Check if onboarding has been completed
export function isOnboardingComplete(): boolean {
  if (typeof window === 'undefined') return true;
  return localStorage.getItem(ONBOARDING_KEY) === 'true';
}

// Mark onboarding as complete
export function markOnboardingComplete(): void {
  if (typeof window !== 'undefined') {
    localStorage.setItem(ONBOARDING_KEY, 'true');
  }
}

// Reset onboarding (for testing)
export function resetOnboarding(): void {
  if (typeof window !== 'undefined') {
    localStorage.removeItem(ONBOARDING_KEY);
  }
  currentStep.set('welcome');
  selectedApps.set([]);
  selectedNavigation.set('left');
  showLabels.set(true);
  selectedGroups.set([]);
}

// Navigate to next step
export function nextStep(): void {
  const current = get(currentStep);
  const currentIndex = STEP_ORDER.indexOf(current);
  if (currentIndex < STEP_ORDER.length - 1) {
    currentStep.set(STEP_ORDER[currentIndex + 1]);
  }
}

// Navigate to previous step
export function prevStep(): void {
  const current = get(currentStep);
  const currentIndex = STEP_ORDER.indexOf(current);
  if (currentIndex > 0) {
    currentStep.set(STEP_ORDER[currentIndex - 1]);
  }
}

// Go to specific step
export function goToStep(step: OnboardingStep): void {
  currentStep.set(step);
}

// Get step progress (0-based index)
export const stepProgress = derived(currentStep, ($step) => {
  return STEP_ORDER.indexOf($step);
});

// Get total steps
export const totalSteps = STEP_ORDER.length;

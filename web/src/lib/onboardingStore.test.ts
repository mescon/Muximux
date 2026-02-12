import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import {
  currentStep,
  selectedApps,
  selectedNavigation,
  showLabels,
  selectedGroups,
  resetOnboarding,
  nextStep,
  prevStep,
  goToStep,
  stepProgress,
  getTotalSteps,
  configureSteps,
  getStepOrder,
  type OnboardingStep,
} from './onboardingStore';

describe('onboardingStore', () => {
  beforeEach(() => {
    resetOnboarding();
  });

  describe('initial state', () => {
    it('starts at welcome step', () => {
      expect(get(currentStep)).toBe('welcome');
    });

    it('has empty selectedApps', () => {
      expect(get(selectedApps)).toEqual([]);
    });

    it('defaults selectedNavigation to left', () => {
      expect(get(selectedNavigation)).toBe('left');
    });

    it('defaults showLabels to true', () => {
      expect(get(showLabels)).toBe(true);
    });

    it('has empty selectedGroups', () => {
      expect(get(selectedGroups)).toEqual([]);
    });
  });

  describe('getTotalSteps', () => {
    it('equals 5 by default (no setup)', () => {
      expect(getTotalSteps()).toBe(5);
    });

    it('equals 6 when setup is included', () => {
      configureSteps(true);
      expect(getTotalSteps()).toBe(6);
    });

    it('returns to 5 after reset', () => {
      configureSteps(true);
      resetOnboarding();
      expect(getTotalSteps()).toBe(5);
    });
  });

  describe('configureSteps', () => {
    it('includes security step when setup needed', () => {
      configureSteps(true);
      expect(getStepOrder()).toEqual(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
    });

    it('excludes security step when no setup', () => {
      configureSteps(false);
      expect(getStepOrder()).toEqual(['welcome', 'apps', 'navigation', 'theme', 'complete']);
    });
  });

  describe('stepProgress', () => {
    it('returns 0 for welcome step', () => {
      expect(get(stepProgress)).toBe(0);
    });

    it('returns correct index for each step', () => {
      const steps: OnboardingStep[] = ['welcome', 'apps', 'navigation', 'theme', 'complete'];
      for (let i = 0; i < steps.length; i++) {
        currentStep.set(steps[i]);
        expect(get(stepProgress)).toBe(i);
      }
    });
  });

  describe('nextStep', () => {
    it('advances from welcome to apps', () => {
      nextStep();
      expect(get(currentStep)).toBe('apps');
    });

    it('advances through all steps in order', () => {
      const expectedOrder: OnboardingStep[] = ['apps', 'navigation', 'theme', 'complete'];
      for (const expected of expectedOrder) {
        nextStep();
        expect(get(currentStep)).toBe(expected);
      }
    });

    it('does not go past the last step', () => {
      currentStep.set('complete');
      nextStep();
      expect(get(currentStep)).toBe('complete');
    });
  });

  describe('prevStep', () => {
    it('does not go before the first step', () => {
      prevStep();
      expect(get(currentStep)).toBe('welcome');
    });

    it('goes back from apps to welcome', () => {
      currentStep.set('apps');
      prevStep();
      expect(get(currentStep)).toBe('welcome');
    });

    it('goes back from complete to theme', () => {
      currentStep.set('complete');
      prevStep();
      expect(get(currentStep)).toBe('theme');
    });

    it('goes back through all steps', () => {
      currentStep.set('complete');
      const expectedReverse: OnboardingStep[] = ['theme', 'navigation', 'apps', 'welcome'];
      for (const expected of expectedReverse) {
        prevStep();
        expect(get(currentStep)).toBe(expected);
      }
    });
  });

  describe('nextStep with security step', () => {
    it('advances through all steps including security', () => {
      configureSteps(true);
      currentStep.set('welcome');
      const expectedOrder: OnboardingStep[] = ['security', 'apps', 'navigation', 'theme', 'complete'];
      for (const expected of expectedOrder) {
        nextStep();
        expect(get(currentStep)).toBe(expected);
      }
    });
  });

  describe('goToStep', () => {
    it('jumps to a specific step', () => {
      goToStep('theme');
      expect(get(currentStep)).toBe('theme');
    });

    it('jumps to complete', () => {
      goToStep('complete');
      expect(get(currentStep)).toBe('complete');
    });

    it('jumps back to welcome', () => {
      goToStep('complete');
      goToStep('welcome');
      expect(get(currentStep)).toBe('welcome');
    });
  });

  describe('resetOnboarding', () => {
    it('resets all state to defaults', () => {
      // Modify all stores
      currentStep.set('theme');
      selectedApps.set([{
        name: 'Test',
        url: 'http://test.com',
        icon: { type: 'dashboard', name: 'test', file: '', url: '', variant: 'svg' },
        color: '#000',
        group: 'test',
        order: 0,
        enabled: true,
        default: false,
        open_mode: 'iframe',
        proxy: false,
        scale: 1,
        disable_keyboard_shortcuts: false,
      }]);
      selectedNavigation.set('top');
      showLabels.set(false);
      selectedGroups.set([{
        name: 'Group',
        icon: { type: 'dashboard', name: 'test', file: '', url: '', variant: 'svg' },
        color: '#000',
        order: 0,
        expanded: true,
      }]);

      resetOnboarding();

      expect(get(currentStep)).toBe('welcome');
      expect(get(selectedApps)).toEqual([]);
      expect(get(selectedNavigation)).toBe('left');
      expect(get(showLabels)).toBe(true);
      expect(get(selectedGroups)).toEqual([]);
    });
  });
});

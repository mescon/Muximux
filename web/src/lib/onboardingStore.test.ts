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
  mergeOnboardingResult,
  type OnboardingStep,
} from './onboardingStore';
import { makeApp, makeGroup, type Config } from './types';

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

  describe('mergeOnboardingResult', () => {
    function baseConfig(): Config {
      return {
        title: 'Muximux',
        language: 'en',
        navigation: { position: 'top', width: '300px', show_labels: false, max_open_tabs: 3 } as Config['navigation'],
        theme: { family: 'nord', variant: 'dark' },
        discovery: { docker: { enabled: true, endpoint: 'tcp://docker:2376', tls: { enabled: true, ca_cert: '/ca.pem' }, network_strategy: 'container_dns', refresh_interval: '30s', auto_import: 'add' } },
        groups: [makeGroup({ name: 'Media', color: '#111111', order: 0 }), makeGroup({ name: 'Ops', order: 1 })],
        apps: [
          makeApp({ name: 'Plex', url: 'http://plex.old', group: 'Media', default: true, shortcut: 1 }),
          makeApp({ name: 'Grafana', url: 'http://grafana', group: 'Ops', shortcut: 2 }),
        ],
      };
    }

    it('mergeOnboardingResult keeps existing groups and apps and adds the wizard\'s', () => {
      const current = baseConfig();
      const merged = mergeOnboardingResult(current, {
        apps: [
          makeApp({ name: 'Plex', url: 'http://plex.new', group: 'Media', default: true, shortcut: 1 }),
          makeApp({ name: 'Sonarr', url: 'http://sonarr', group: 'Downloads', shortcut: 2 }),
          makeApp({ name: 'Radarr', url: 'http://radarr', group: 'Downloads', shortcut: 3 }),
        ],
        groups: [
          makeGroup({ name: 'Media', color: '#ffffff', order: 0 }),
          makeGroup({ name: 'Downloads', order: 1 }),
        ],
        navigation: { position: 'left', show_labels: true },
        theme: { family: 'default', variant: 'light' },
        language: 'sv',
      });

      // Existing entries win; the wizard's new ones are appended.
      expect(merged.groups.map(g => g.name)).toEqual(['Media', 'Ops', 'Downloads']);
      expect(merged.groups[0].color).toBe('#111111');
      expect(merged.groups[2].order).toBe(2);
      expect(merged.apps.map(a => a.name)).toEqual(['Plex', 'Grafana', 'Sonarr', 'Radarr']);
      expect(merged.apps[0].url).toBe('http://plex.old');
      // One default app, and no shortcut is assigned twice.
      expect(merged.apps.filter(a => a.default).map(a => a.name)).toEqual(['Plex']);
      const shortcuts = merged.apps.map(a => a.shortcut).filter(s => s !== undefined);
      expect(new Set(shortcuts).size).toBe(shortcuts.length);
      expect(merged.apps[3].shortcut).toBe(3);
      // Navigation is a spread over the existing block.
      expect(merged.navigation.position).toBe('left');
      expect(merged.navigation.show_labels).toBe(true);
      expect(merged.navigation.width).toBe('300px');
      expect(merged.navigation.max_open_tabs).toBe(3);
      // Theme is replaced, language set, everything else kept.
      expect(merged.theme).toEqual({ family: 'default', variant: 'light' });
      expect(merged.language).toBe('sv');
      expect(merged.discovery).toEqual(current.discovery);
      expect(merged.title).toBe('Muximux');
      // The input config is not mutated.
      expect(current.apps).toHaveLength(2);
      expect(current.groups).toHaveLength(2);
    });

    it('keeps the wizard default and shortcuts when the config has none', () => {
      const current: Config = { ...baseConfig(), groups: [], apps: [] };
      const merged = mergeOnboardingResult(current, {
        apps: [makeApp({ name: 'Plex', group: 'Media', default: true, shortcut: 1 })],
        groups: [makeGroup({ name: 'Media', order: 0 })],
        navigation: {},
        theme: { family: 'default', variant: 'dark' },
        language: 'en',
      });
      expect(merged.apps[0].default).toBe(true);
      expect(merged.apps[0].shortcut).toBe(1);
      expect(merged.groups[0].order).toBe(0);
      expect(merged.navigation).toEqual(current.navigation);
    });
  });
});

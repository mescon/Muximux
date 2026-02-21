import { describe, it, expect, beforeEach } from 'vitest';
import {
  splitState,
  enableSplit,
  disableSplit,
  setActivePanel,
  setPanelApp,
  updateDividerPosition,
  resetSplit
} from './splitStore.svelte';

const sonarr = { name: 'Sonarr' } as any;
const radarr = { name: 'Radarr' } as any;

describe('splitStore', () => {
  beforeEach(() => {
    resetSplit();
  });

  describe('initial state', () => {
    it('starts disabled', () => {
      expect(splitState.enabled).toBe(false);
      expect(splitState.orientation).toBe('horizontal');
      expect(splitState.activePanel).toBe(0);
      expect(splitState.panels).toEqual([null, null]);
      expect(splitState.dividerPosition).toBe(0.5);
    });
  });

  describe('enableSplit', () => {
    it('enables horizontal split', () => {
      enableSplit('horizontal');
      expect(splitState.enabled).toBe(true);
      expect(splitState.orientation).toBe('horizontal');
      expect(splitState.activePanel).toBe(1);
    });

    it('enables vertical split', () => {
      enableSplit('vertical');
      expect(splitState.enabled).toBe(true);
      expect(splitState.orientation).toBe('vertical');
      expect(splitState.activePanel).toBe(1);
    });

    it('preserves panels[0] when enabling', () => {
      splitState.panels[0] = sonarr;
      enableSplit('horizontal');
      expect(splitState.panels[0]).toEqual(sonarr);
      expect(splitState.panels[1]).toBeNull();
    });

    it('switches orientation when already enabled', () => {
      enableSplit('horizontal');
      expect(splitState.orientation).toBe('horizontal');
      enableSplit('vertical');
      expect(splitState.orientation).toBe('vertical');
      expect(splitState.enabled).toBe(true);
    });
  });

  describe('disableSplit', () => {
    it('keeps activePanel app when disabling', () => {
      splitState.enabled = true;
      splitState.panels[0] = sonarr;
      splitState.panels[1] = radarr;
      splitState.activePanel = 1;
      disableSplit();
      expect(splitState.enabled).toBe(false);
      expect(splitState.panels[0]).toEqual(radarr);
      expect(splitState.panels[1]).toBeNull();
    });

    it('resets divider position when disabling', () => {
      splitState.enabled = true;
      splitState.dividerPosition = 0.3;
      disableSplit();
      expect(splitState.dividerPosition).toBe(0.5);
    });

    it('does nothing when already disabled', () => {
      disableSplit();
      expect(splitState.enabled).toBe(false);
    });
  });

  describe('setActivePanel', () => {
    it('sets the active panel index', () => {
      splitState.enabled = true;
      setActivePanel(1);
      expect(splitState.activePanel).toBe(1);
      setActivePanel(0);
      expect(splitState.activePanel).toBe(0);
    });
  });

  describe('setPanelApp', () => {
    it('sets app in the active panel', () => {
      splitState.enabled = true;
      splitState.activePanel = 0;
      setPanelApp(sonarr);
      expect(splitState.panels[0]).toEqual(sonarr);
    });

    it('moves app if already in the other panel', () => {
      splitState.enabled = true;
      splitState.panels[0] = sonarr;
      splitState.panels[1] = radarr;
      splitState.activePanel = 1;
      setPanelApp(sonarr);
      expect(splitState.panels[1]).toEqual(sonarr);
      expect(splitState.panels[0]).toBeNull();
    });

    it('works in single mode by setting panels[0]', () => {
      setPanelApp(sonarr);
      expect(splitState.panels[0]).toEqual(sonarr);
    });
  });

  describe('updateDividerPosition', () => {
    it('clamps to 0.2-0.8 range', () => {
      splitState.enabled = true;
      updateDividerPosition(0.1);
      expect(splitState.dividerPosition).toBe(0.2);
      updateDividerPosition(0.9);
      expect(splitState.dividerPosition).toBe(0.8);
      updateDividerPosition(0.6);
      expect(splitState.dividerPosition).toBe(0.6);
    });
  });
});

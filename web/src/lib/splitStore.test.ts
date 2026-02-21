import { describe, it, expect, beforeEach } from 'vitest';
import {
  splitState,
  toggleSplit,
  setActivePanel,
  setPanelApp,
  closeSplitPanel,
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

  describe('toggleSplit', () => {
    it('cycles off -> horizontal -> vertical -> off', () => {
      toggleSplit();
      expect(splitState.enabled).toBe(true);
      expect(splitState.orientation).toBe('horizontal');

      toggleSplit();
      expect(splitState.enabled).toBe(true);
      expect(splitState.orientation).toBe('vertical');

      toggleSplit();
      expect(splitState.enabled).toBe(false);
    });

    it('preserves panels[0] when enabling and sets activePanel to 1', () => {
      splitState.panels[0] = sonarr;
      toggleSplit();
      expect(splitState.panels[0]).toEqual(sonarr);
      expect(splitState.panels[1]).toBeNull();
      expect(splitState.activePanel).toBe(1);
    });

    it('keeps activePanel app when disabling', () => {
      splitState.enabled = true;
      splitState.panels[0] = sonarr;
      splitState.panels[1] = radarr;
      splitState.activePanel = 1;
      toggleSplit(); // -> vertical
      toggleSplit(); // -> off
      expect(splitState.enabled).toBe(false);
      expect(splitState.panels[0]).toEqual(radarr);
      expect(splitState.panels[1]).toBeNull();
    });

    it('resets divider position when disabling', () => {
      splitState.enabled = true;
      splitState.dividerPosition = 0.3;
      toggleSplit(); // -> vertical
      toggleSplit(); // -> off
      expect(splitState.dividerPosition).toBe(0.5);
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

  describe('closeSplitPanel', () => {
    it('exits split keeping the other panel app', () => {
      splitState.enabled = true;
      splitState.panels[0] = sonarr;
      splitState.panels[1] = radarr;
      closeSplitPanel(0);
      expect(splitState.enabled).toBe(false);
      expect(splitState.panels[0]).toEqual(radarr);
      expect(splitState.panels[1]).toBeNull();
    });

    it('exits split keeping panel 0 when closing panel 1', () => {
      splitState.enabled = true;
      splitState.panels[0] = sonarr;
      splitState.panels[1] = radarr;
      closeSplitPanel(1);
      expect(splitState.enabled).toBe(false);
      expect(splitState.panels[0]).toEqual(sonarr);
      expect(splitState.panels[1]).toBeNull();
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

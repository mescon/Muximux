import type { App } from './types';

interface SplitState {
  enabled: boolean;
  orientation: 'horizontal' | 'vertical';
  activePanel: 0 | 1;
  panels: [App | null, App | null];
  dividerPosition: number;
}

export const splitState: SplitState = $state({
  enabled: false,
  orientation: 'horizontal',
  activePanel: 0,
  panels: [null, null],
  dividerPosition: 0.5,
});

export function enableSplit(orientation: 'horizontal' | 'vertical'): void {
  if (splitState.enabled) {
    // Already split — just switch orientation
    splitState.orientation = orientation;
    return;
  }
  splitState.enabled = true;
  splitState.orientation = orientation;
  splitState.activePanel = 1;
  // panels[0] keeps current app, panels[1] starts empty
}

export function disableSplit(): void {
  if (!splitState.enabled) return;
  const keepApp = splitState.panels[splitState.activePanel];
  splitState.enabled = false;
  splitState.panels[0] = keepApp;
  splitState.panels[1] = null;
  splitState.activePanel = 0;
  splitState.dividerPosition = 0.5;
}

export function setActivePanel(index: 0 | 1): void {
  splitState.activePanel = index;
}

export function setPanelApp(app: App): void {
  if (!splitState.enabled) {
    splitState.panels[0] = app;
    return;
  }

  // If app is already in the other panel, move it (clear the source)
  const otherPanel = splitState.activePanel === 0 ? 1 : 0;
  if (splitState.panels[otherPanel]?.name === app.name) {
    splitState.panels[otherPanel] = null;
  }

  splitState.panels[splitState.activePanel] = app;
}

export function updateDividerPosition(position: number): void {
  splitState.dividerPosition = Math.min(0.8, Math.max(0.2, position));
}

export function resetSplit(): void {
  splitState.enabled = false;
  splitState.orientation = 'horizontal';
  splitState.activePanel = 0;
  splitState.panels[0] = null;
  splitState.panels[1] = null;
  splitState.dividerPosition = 0.5;
}

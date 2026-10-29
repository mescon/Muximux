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

/** Which pane currently shows the named app, or null. */
export function paneShowing(appName: string): 0 | 1 | null {
  if (splitState.panels[0]?.name === appName) return 0;
  if (splitState.enabled && splitState.panels[1]?.name === appName) return 1;
  return null;
}

/**
 * Focus the pane already showing the app, or place it in `pane`. Never moves
 * an app out of the other pane (unlike setPanelApp). Returns the pane used.
 */
export function focusOrPlace(app: App, pane: 0 | 1): 0 | 1 {
  if (!splitState.enabled) {
    splitState.panels[0] = app;
    return 0;
  }
  const showing = paneShowing(app.name);
  if (showing !== null) {
    splitState.activePanel = showing;
    return showing;
  }
  splitState.activePanel = pane;
  splitState.panels[pane] = app;
  return pane;
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

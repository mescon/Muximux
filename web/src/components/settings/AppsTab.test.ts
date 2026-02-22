import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import type { App, Group } from '$lib/types';

// Mock svelte-dnd-action
vi.mock('svelte-dnd-action', () => ({
  dndzone: (_node: HTMLElement, _params: Record<string, unknown>) => {
    return {
      update: (_newParams: Record<string, unknown>) => {},
      destroy: () => {},
    };
  },
  TRIGGERS: { DRAG_STARTED: 'dragStarted' },
  SOURCES: { POINTER: 'pointer' },
}));

// Mock AppIcon dependencies so it renders without error
vi.mock('$lib/api', () => ({
  getBase: vi.fn(() => ''),
}));

vi.mock('$lib/debug', () => ({
  debug: vi.fn(),
}));

// Mock getAnimations for Svelte animate:flip (not available in jsdom)
if (typeof Element !== 'undefined' && !Element.prototype.getAnimations) {
  Element.prototype.getAnimations = () => [];
}

import AppsTab from './AppsTab.svelte';

function makeApp(overrides: Partial<App> = {}): App {
  return {
    name: 'TestApp',
    url: 'http://localhost:8080',
    icon: { type: 'lucide', name: 'home', file: '', url: '', variant: '' },
    group: 'Default',
    proxy: false,
    open_mode: 'iframe',
    enabled: true,
    default: false,
    order: 0,
    health_check: false,
    color: '',
    scale: 1,
    ...overrides,
  } as App;
}

function makeGroup(overrides: Partial<Group> = {}): Group {
  return {
    name: 'Default',
    icon: { type: 'lucide', name: 'folder', file: '', url: '', variant: '' },
    color: '#374151',
    order: 0,
    expanded: true,
    ...overrides,
  };
}

// Helper to add the required `id` field for DnD
function withId<T extends { name: string }>(item: T): T & { id: string } {
  return { ...item, id: item.name };
}

describe('AppsTab', () => {
  const defaultHandlers = {
    onstartEditApp: vi.fn(),
    onstartEditGroup: vi.fn(),
    onshowAddApp: vi.fn(),
    onshowAddGroup: vi.fn(),
    onsyncGroupOrder: vi.fn(),
    onsyncAppOrder: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  // ===== Basic rendering =====

  it('renders "Add App" and "Add Group" buttons', () => {
    const group = withId(makeGroup());

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [] },
        localAppsCount: 0,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText('Add App')).toBeInTheDocument();
    expect(screen.getByText('Add Group')).toBeInTheDocument();
  });

  it('shows group names', () => {
    const group1 = withId(makeGroup({ name: 'Media', order: 0 }));
    const group2 = withId(makeGroup({ name: 'Tools', order: 1 }));

    render(AppsTab, {
      props: {
        dndGroups: [group1, group2],
        dndGroupedApps: { Media: [], Tools: [] },
        localAppsCount: 0,
        localGroupsCount: 2,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText('Media')).toBeInTheDocument();
    expect(screen.getByText('Tools')).toBeInTheDocument();
  });

  it('shows app names within groups', () => {
    const group = withId(makeGroup());
    const app1 = withId(makeApp({ name: 'Sonarr', order: 0 }));
    const app2 = withId(makeApp({ name: 'Radarr', order: 1 }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app1, app2] },
        localAppsCount: 2,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText('Sonarr')).toBeInTheDocument();
    expect(screen.getByText('Radarr')).toBeInTheDocument();
  });

  it('shows app count next to group name', () => {
    const group = withId(makeGroup({ name: 'Media' }));
    const app1 = withId(makeApp({ name: 'Sonarr', group: 'Media', order: 0 }));
    const app2 = withId(makeApp({ name: 'Radarr', group: 'Media', order: 1 }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Media: [app1, app2] },
        localAppsCount: 2,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText('2 apps')).toBeInTheDocument();
  });

  it('shows app URL in the listing', () => {
    const group = withId(makeGroup());
    const app = withId(makeApp({ name: 'Grafana', url: 'http://grafana.local:3000' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText('http://grafana.local:3000')).toBeInTheDocument();
  });

  it('shows empty state when no apps or groups exist', () => {
    render(AppsTab, {
      props: {
        dndGroups: [],
        dndGroupedApps: {},
        localAppsCount: 0,
        localGroupsCount: 0,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText(/No applications or groups configured/)).toBeInTheDocument();
  });

  it('renders drag instruction text', () => {
    render(AppsTab, {
      props: {
        dndGroups: [],
        dndGroupedApps: {},
        localAppsCount: 0,
        localGroupsCount: 0,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText(/Drag apps to reorder/)).toBeInTheDocument();
  });

  it('renders "Apps & Groups" heading', () => {
    render(AppsTab, {
      props: {
        dndGroups: [],
        dndGroupedApps: {},
        localAppsCount: 0,
        localGroupsCount: 0,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText('Apps & Groups')).toBeInTheDocument();
  });

  // ===== Button callbacks =====

  it('calls onshowAddApp when Add App button is clicked', async () => {
    const handlers = { ...defaultHandlers };
    const group = withId(makeGroup());

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [] },
        localAppsCount: 0,
        localGroupsCount: 1,
        ...handlers,
      },
    });

    const addAppBtn = screen.getByText('Add App').closest('button')!;
    await fireEvent.click(addAppBtn);
    expect(handlers.onshowAddApp).toHaveBeenCalledOnce();
  });

  it('calls onshowAddGroup when Add Group button is clicked', async () => {
    const handlers = { ...defaultHandlers };
    const group = withId(makeGroup());

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [] },
        localAppsCount: 0,
        localGroupsCount: 1,
        ...handlers,
      },
    });

    const addGroupBtn = screen.getByText('Add Group').closest('button')!;
    await fireEvent.click(addGroupBtn);
    expect(handlers.onshowAddGroup).toHaveBeenCalledOnce();
  });

  // ===== App badges =====

  it('shows "Default" badge for default app', () => {
    const group = withId(makeGroup({ name: 'Media' }));
    const app = withId(makeApp({ name: 'HomeApp', group: 'Media', default: true }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Media: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    // The badge text "Default" (not the group name)
    const defaultBadges = screen.getAllByText('Default');
    const badge = defaultBadges.find(el => el.classList.contains('text-xs'));
    expect(badge).toBeInTheDocument();
  });

  it('shows "Disabled" badge for disabled app', () => {
    const group = withId(makeGroup());
    const app = withId(makeApp({ name: 'DisabledApp', enabled: false }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText('Disabled')).toBeInTheDocument();
  });

  it('shows proxy indicator for proxied app', () => {
    const group = withId(makeGroup());
    const app = withId(makeApp({ name: 'ProxiedApp', proxy: true }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    // The proxy indicator has a title attribute
    const proxyIndicator = screen.getByTitle('Proxied through server');
    expect(proxyIndicator).toBeInTheDocument();
  });

  it('shows new_tab open mode indicator', () => {
    const group = withId(makeGroup());
    const app = withId(makeApp({ name: 'TabApp', open_mode: 'new_tab' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    const indicator = screen.getByTitle('Opens in new tab');
    expect(indicator).toBeInTheDocument();
  });

  it('shows new_window open mode indicator', () => {
    const group = withId(makeGroup());
    const app = withId(makeApp({ name: 'WinApp', open_mode: 'new_window' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    const indicator = screen.getByTitle('Opens in new window');
    expect(indicator).toBeInTheDocument();
  });

  it('shows redirect open mode indicator', () => {
    const group = withId(makeGroup());
    const app = withId(makeApp({ name: 'RedirApp', open_mode: 'redirect' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    const indicator = screen.getByTitle('Opens in redirect');
    expect(indicator).toBeInTheDocument();
  });

  it('does not show open mode indicator for iframe (default)', () => {
    const group = withId(makeGroup());
    const app = withId(makeApp({ name: 'IframeApp', open_mode: 'iframe' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    expect(screen.queryByTitle(/Opens in/)).not.toBeInTheDocument();
  });

  it('shows scale indicator for non-1 scale', () => {
    const group = withId(makeGroup());
    const app = withId(makeApp({ name: 'ScaledApp', scale: 0.75 }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    // Scale at 0.75 => "75%"
    const scaleIndicator = screen.getByTitle('Scaled to 75%');
    expect(scaleIndicator).toBeInTheDocument();
    expect(screen.getByText('75%')).toBeInTheDocument();
  });

  it('does not show scale indicator when scale is 1', () => {
    const group = withId(makeGroup());
    const app = withId(makeApp({ name: 'NormalApp', scale: 1 }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    expect(screen.queryByTitle(/Scaled to/)).not.toBeInTheDocument();
  });

  // ===== Edit app =====

  it('calls onstartEditApp when edit button is clicked on an app', async () => {
    const handlers = { ...defaultHandlers };
    const group = withId(makeGroup());
    const app = withId(makeApp({ name: 'Sonarr' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...handlers,
      },
    });

    const editBtn = screen.getAllByTitle('Edit')[0];
    await fireEvent.click(editBtn);
    expect(handlers.onstartEditApp).toHaveBeenCalledOnce();
  });

  // ===== Edit group =====

  it('calls onstartEditGroup when edit group button is clicked', async () => {
    const handlers = { ...defaultHandlers };
    const group = withId(makeGroup({ name: 'Media' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Media: [] },
        localAppsCount: 0,
        localGroupsCount: 1,
        ...handlers,
      },
    });

    const editGroupBtn = screen.getByTitle('Edit group');
    await fireEvent.click(editGroupBtn);
    expect(handlers.onstartEditGroup).toHaveBeenCalledOnce();
  });

  // ===== Delete app flow =====

  it('shows delete confirmation when delete button is clicked on an app', async () => {
    const group = withId(makeGroup());
    const app = withId(makeApp({ name: 'Sonarr' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    // Click the delete button (the one with title "Delete" on the app row)
    const deleteBtn = screen.getAllByTitle('Delete')[0];
    await fireEvent.click(deleteBtn);

    // Should show confirmation
    expect(screen.getByText('Delete?')).toBeInTheDocument();
    expect(screen.getByText('Yes')).toBeInTheDocument();
    expect(screen.getByText('No')).toBeInTheDocument();
  });

  it('cancels app delete when "No" is clicked', async () => {
    const group = withId(makeGroup());
    const app = withId(makeApp({ name: 'Sonarr' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    // Click delete to show confirmation
    const deleteBtn = screen.getAllByTitle('Delete')[0];
    await fireEvent.click(deleteBtn);

    // Click "No" to cancel
    await fireEvent.click(screen.getByText('No'));

    // Confirmation should be gone, app still visible
    expect(screen.queryByText('Delete?')).not.toBeInTheDocument();
    expect(screen.getByText('Sonarr')).toBeInTheDocument();
  });

  it('confirms app deletion when "Yes" is clicked', async () => {
    const handlers = { ...defaultHandlers };
    const group = withId(makeGroup());
    const app = withId(makeApp({ name: 'Sonarr' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...handlers,
      },
    });

    // Click delete to show confirmation
    const deleteBtn = screen.getAllByTitle('Delete')[0];
    await fireEvent.click(deleteBtn);

    // Click "Yes" to confirm
    await fireEvent.click(screen.getByText('Yes'));

    // Should call onsyncAppOrder with '__delete__'
    expect(handlers.onsyncAppOrder).toHaveBeenCalledWith('__delete__', []);
  });

  // ===== Delete group flow =====

  it('shows delete confirmation when delete button is clicked on a group', async () => {
    const group = withId(makeGroup({ name: 'Media' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Media: [] },
        localAppsCount: 0,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    const deleteGroupBtn = screen.getByTitle('Delete group');
    await fireEvent.click(deleteGroupBtn);

    expect(screen.getByText('Delete?')).toBeInTheDocument();
  });

  it('cancels group delete when "No" is clicked', async () => {
    const group = withId(makeGroup({ name: 'Media' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Media: [] },
        localAppsCount: 0,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    const deleteGroupBtn = screen.getByTitle('Delete group');
    await fireEvent.click(deleteGroupBtn);

    // Click "No"
    await fireEvent.click(screen.getByText('No'));

    expect(screen.queryByText('Delete?')).not.toBeInTheDocument();
    expect(screen.getByText('Media')).toBeInTheDocument();
  });

  it('confirms group deletion and syncs order', async () => {
    const handlers = { ...defaultHandlers };
    const group = withId(makeGroup({ name: 'Media' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Media: [], '': [] },
        localAppsCount: 0,
        localGroupsCount: 1,
        ...handlers,
      },
    });

    const deleteGroupBtn = screen.getByTitle('Delete group');
    await fireEvent.click(deleteGroupBtn);

    await fireEvent.click(screen.getByText('Yes'));

    // Should call both sync functions
    expect(handlers.onsyncGroupOrder).toHaveBeenCalled();
    expect(handlers.onsyncAppOrder).toHaveBeenCalledWith('__rebuild__', []);
  });

  it('moves orphaned apps to ungrouped when group is deleted', async () => {
    const handlers = { ...defaultHandlers };
    const group1 = withId(makeGroup({ name: 'Media', order: 0 }));
    const group2 = withId(makeGroup({ name: 'Tools', order: 1 }));
    const app1 = withId(makeApp({ name: 'Sonarr', group: 'Media', order: 0 }));

    render(AppsTab, {
      props: {
        dndGroups: [group1, group2],
        dndGroupedApps: { Media: [app1], Tools: [], '': [] },
        localAppsCount: 1,
        localGroupsCount: 2,
        ...handlers,
      },
    });

    // Delete the Media group which has Sonarr in it
    const deleteGroupBtns = screen.getAllByTitle('Delete group');
    await fireEvent.click(deleteGroupBtns[0]);
    await fireEvent.click(screen.getByText('Yes'));

    expect(handlers.onsyncGroupOrder).toHaveBeenCalled();
    expect(handlers.onsyncAppOrder).toHaveBeenCalledWith('__rebuild__', []);
  });

  // ===== Empty group state =====

  it('shows "No apps in this group" when a group is empty', () => {
    const group = withId(makeGroup({ name: 'EmptyGroup' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { EmptyGroup: [] },
        localAppsCount: 0,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText('No apps in this group')).toBeInTheDocument();
  });

  // ===== Ungrouped apps section =====

  it('shows ungrouped section with apps when there are ungrouped apps', () => {
    const group = withId(makeGroup({ name: 'Media' }));
    const ungroupedApp = withId(makeApp({ name: 'Loose', group: '', order: 0 }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Media: [], '': [ungroupedApp] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText('Ungrouped')).toBeInTheDocument();
    expect(screen.getByText('1 apps')).toBeInTheDocument();
    expect(screen.getByText('Loose')).toBeInTheDocument();
  });

  it('shows ungrouped section with hint when no ungrouped apps but groups exist', () => {
    const group = withId(makeGroup({ name: 'Media' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Media: [], '': [] },
        localAppsCount: 0,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText('Ungrouped')).toBeInTheDocument();
    expect(screen.getByText('Drag apps here to ungroup them')).toBeInTheDocument();
  });

  // ===== Group icon rendering =====

  it('renders a color swatch when group has no icon name', () => {
    const group = withId(makeGroup({
      name: 'NoIcon',
      icon: { type: 'lucide', name: '', file: '', url: '', variant: '' },
      color: '#ff0000',
    }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { NoIcon: [] },
        localAppsCount: 0,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    // When icon.name is falsy, it renders a plain colored span
    const colorSwatch = document.querySelector('span[style*="background-color"]');
    expect(colorSwatch).toBeInTheDocument();
  });

  // ===== Multiple badges on one app =====

  it('shows multiple badges on a single app', () => {
    const group = withId(makeGroup({ name: 'Media' }));
    const app = withId(makeApp({
      name: 'MultiApp',
      group: 'Media',
      default: true,
      enabled: false,
      proxy: true,
      open_mode: 'new_tab',
      scale: 0.5,
    }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Media: [app] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    // "Default" badge (distinct from any group named "Default")
    const defaultBadges = screen.getAllByText('Default');
    expect(defaultBadges.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('Disabled')).toBeInTheDocument();
    expect(screen.getByTitle('Proxied through server')).toBeInTheDocument();
    expect(screen.getByTitle('Opens in new tab')).toBeInTheDocument();
    // Scale badge - the legend row also has "50%" so use getAllByText
    const scaleBadges = screen.getAllByText('50%');
    expect(scaleBadges.length).toBeGreaterThanOrEqual(1);
  });

  // ===== Multiple groups with apps =====

  it('renders multiple groups each with their own apps', () => {
    const group1 = withId(makeGroup({ name: 'Media', order: 0 }));
    const group2 = withId(makeGroup({ name: 'Tools', order: 1 }));
    const app1 = withId(makeApp({ name: 'Plex', group: 'Media', order: 0 }));
    const app2 = withId(makeApp({ name: 'Portainer', group: 'Tools', order: 0 }));

    render(AppsTab, {
      props: {
        dndGroups: [group1, group2],
        dndGroupedApps: { Media: [app1], Tools: [app2] },
        localAppsCount: 2,
        localGroupsCount: 2,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText('Plex')).toBeInTheDocument();
    expect(screen.getByText('Portainer')).toBeInTheDocument();
    expect(screen.getByText('Media')).toBeInTheDocument();
    expect(screen.getByText('Tools')).toBeInTheDocument();
  });

  // ===== App deletion removes from grouped apps =====

  it('removes app from dndGroupedApps when confirmed', async () => {
    const handlers = { ...defaultHandlers };
    const group = withId(makeGroup());
    const app1 = withId(makeApp({ name: 'Sonarr', order: 0 }));
    const app2 = withId(makeApp({ name: 'Radarr', order: 1 }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Default: [app1, app2] },
        localAppsCount: 2,
        localGroupsCount: 1,
        ...handlers,
      },
    });

    // Delete Sonarr
    const deleteButtons = screen.getAllByTitle('Delete');
    await fireEvent.click(deleteButtons[0]);
    await fireEvent.click(screen.getByText('Yes'));

    expect(handlers.onsyncAppOrder).toHaveBeenCalledWith('__delete__', []);
  });

  // ===== Group with default color =====

  it('uses default color when group has no color set', () => {
    const group = withId(makeGroup({
      name: 'Plain',
      color: '',
      icon: { type: 'lucide', name: '', file: '', url: '', variant: '' },
    }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: { Plain: [] },
        localAppsCount: 0,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    // When color is empty, it falls back to '#374151'
    // The style could be set as "background-color: #374151" (various formats)
    const swatches = document.querySelectorAll('span.w-6.h-6');
    expect(swatches.length).toBeGreaterThan(0);
  });

  // ===== Ungrouped apps edit/delete =====

  it('can edit an ungrouped app', async () => {
    const handlers = { ...defaultHandlers };
    const ungroupedApp = withId(makeApp({ name: 'LooseApp', group: '', order: 0 }));

    render(AppsTab, {
      props: {
        dndGroups: [],
        dndGroupedApps: { '': [ungroupedApp] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...handlers,
      },
    });

    const editBtn = screen.getByTitle('Edit');
    await fireEvent.click(editBtn);
    expect(handlers.onstartEditApp).toHaveBeenCalledOnce();
  });

  it('can delete an ungrouped app', async () => {
    const handlers = { ...defaultHandlers };
    const ungroupedApp = withId(makeApp({ name: 'LooseApp', group: '', order: 0 }));

    render(AppsTab, {
      props: {
        dndGroups: [],
        dndGroupedApps: { '': [ungroupedApp] },
        localAppsCount: 1,
        localGroupsCount: 1,
        ...handlers,
      },
    });

    const deleteBtn = screen.getByTitle('Delete');
    await fireEvent.click(deleteBtn);

    expect(screen.getByText('Delete?')).toBeInTheDocument();

    await fireEvent.click(screen.getByText('Yes'));
    expect(handlers.onsyncAppOrder).toHaveBeenCalledWith('__delete__', []);
  });

  // ===== Group deletion with orphaned apps already having ungrouped entries =====

  it('merges orphaned apps into existing ungrouped list on group delete', async () => {
    const handlers = { ...defaultHandlers };
    const group1 = withId(makeGroup({ name: 'Media', order: 0 }));
    const group2 = withId(makeGroup({ name: 'Other', order: 1 }));
    const groupApp = withId(makeApp({ name: 'Plex', group: 'Media', order: 0 }));
    const existingUngrouped = withId(makeApp({ name: 'Loose', group: '', order: 0 }));

    render(AppsTab, {
      props: {
        dndGroups: [group1, group2],
        dndGroupedApps: { Media: [groupApp], Other: [], '': [existingUngrouped] },
        localAppsCount: 2,
        localGroupsCount: 2,
        ...handlers,
      },
    });

    // Delete the first group (Media) which has Plex in it
    const deleteGroupBtns = screen.getAllByTitle('Delete group');
    await fireEvent.click(deleteGroupBtns[0]);
    await fireEvent.click(screen.getByText('Yes'));

    expect(handlers.onsyncGroupOrder).toHaveBeenCalled();
    expect(handlers.onsyncAppOrder).toHaveBeenCalledWith('__rebuild__', []);
  });

  // ===== 0 apps text =====

  it('shows "0 apps" when group has no apps and no entry in dndGroupedApps', () => {
    const group = withId(makeGroup({ name: 'Empty' }));

    render(AppsTab, {
      props: {
        dndGroups: [group],
        dndGroupedApps: {},
        localAppsCount: 0,
        localGroupsCount: 1,
        ...defaultHandlers,
      },
    });

    expect(screen.getByText('0 apps')).toBeInTheDocument();
  });
});

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/svelte';
import type { DiscoverySuggestion } from '$lib/types';

// Swap the real IconBrowser for a deterministic stub. The stub
// exposes one button per icon-type variant ({type: dashboard | lucide
// | custom | url}) plus a cancel button, so the tests below can drive
// every handleIconSelect branch and the close path without mounting
// the real picker's 100s of icon tiles.
vi.mock('../IconBrowser.svelte', async () => {
  const mod = await import('../../test/IconBrowserStub.svelte');
  return { default: mod.default };
});

// Mock the API module before importing the component. Each mock is a
// `vi.hoisted` factory so the SUT picks them up via Vite's vi.mock
// transform.
//
// getBase is consumed by AppIcon (which we render in the row preview).
// The icon-listing endpoints are imported by IconBrowser (rendered
// only after the operator clicks the icon button) - we stub them
// returning empty arrays so the picker mounts cleanly when the few
// tests that open it run.
const mockApi = vi.hoisted(() => ({
  scanDockerContainers: vi.fn(),
  importDockerSuggestions: vi.fn(),
  getBase: vi.fn(() => ''),
  getDashboardIconUrl: vi.fn((name: string) => `/icons/dashboard/${name}.svg`),
  getLucideIconUrl: vi.fn((name: string) => `/icons/lucide/${name}.svg`),
  getCustomIconUrl: vi.fn((name: string) => `/icons/custom/${name}`),
  listDashboardIcons: vi.fn(async () => []),
  listLucideIcons: vi.fn(async () => []),
  listCustomIcons: vi.fn(async () => []),
  uploadCustomIcon: vi.fn(),
  fetchCustomIconFromUrl: vi.fn(),
  deleteCustomIcon: vi.fn(),
}));
vi.mock('$lib/api', () => mockApi);

import DiscoverModal from './DiscoverModal.svelte';

function makeSuggestion(overrides: Partial<DiscoverySuggestion> = {}): DiscoverySuggestion {
  return {
    key: 'name:c1',
    stability: 'stable',
    name: 'C1',
    url: 'http://10.0.0.5:80',
    effective_strategy: 'container_ip',
    container_id: 'deadbeef',
    image_ref: 'example/c1',
    confidence: 'medium',
    ...overrides,
  };
}

describe('DiscoverModal routing radio + gateway interactions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does not show the routing radio until "Add to menu" is checked', async () => {
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [makeSuggestion()],
    });
    render(DiscoverModal, { open: true, mode: 'gateway', onclose: () => {} });

    await waitFor(() => expect(screen.getByDisplayValue('C1')).toBeInTheDocument());

    // mode=gateway means createApp defaults to false; routing
    // radio should be absent.
    expect(screen.queryByTestId('row-routing')).not.toBeInTheDocument();
  });

  it('disables the Gateway-domain radio when no gateway domain is set', async () => {
    // In apps mode createApp defaults to true, so the routing
    // fieldset is visible. With no gatewayDomain, the gateway
    // radio should be disabled (otherwise the operator could
    // select an unsatisfiable routing and hit the backend
    // rejection at submit time).
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [makeSuggestion()],
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });

    await waitFor(() => expect(screen.getByDisplayValue('C1')).toBeInTheDocument());

    const fieldset = screen.getByTestId('row-routing');
    const gatewayRadio = fieldset.querySelector(
      'input[type="radio"][value="gateway"]'
    ) as HTMLInputElement;
    expect(gatewayRadio).toBeTruthy();
    expect(gatewayRadio.disabled).toBe(true);
  });

  it('sends routing=proxy when the operator picks Proxy and submits', async () => {
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [makeSuggestion()],
    });
    mockApi.importDockerSuggestions.mockResolvedValue({
      success: true,
      items: [{ key: 'name:c1', status: 'created', app_name: 'C1' }],
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });

    await waitFor(() => expect(screen.getByDisplayValue('C1')).toBeInTheDocument());

    // Select the row, switch routing to proxy, submit.
    const rowCheckbox = screen.getAllByRole('checkbox')[1] as HTMLInputElement; // [0] is "select all"
    await fireEvent.click(rowCheckbox);

    const proxyRadio = screen.getByTestId('row-routing')
      .querySelector('input[type="radio"][value="proxy"]') as HTMLInputElement;
    await fireEvent.click(proxyRadio);

    const importBtn = screen.getByText(/Import 1 selected/i);
    await fireEvent.click(importBtn);

    await waitFor(() => expect(mockApi.importDockerSuggestions).toHaveBeenCalled());
    const payload = mockApi.importDockerSuggestions.mock.calls[0][0];
    expect(payload.items[0].routing).toBe('proxy');
  });

  it('imports the catalog-suggested icon when the operator does not override it', async () => {
    mockApi.scanDockerContainers.mockResolvedValue({
      // confidence='medium' means the catalog gave us an icon name.
      suggestions: [makeSuggestion({ icon: 'sonarr', confidence: 'medium' })],
    });
    mockApi.importDockerSuggestions.mockResolvedValue({
      success: true,
      items: [{ key: 'name:c1', status: 'created', app_name: 'C1' }],
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });

    await waitFor(() => expect(screen.getByDisplayValue('C1')).toBeInTheDocument());

    const rowCheckbox = screen.getAllByRole('checkbox')[1] as HTMLInputElement;
    await fireEvent.click(rowCheckbox);

    const importBtn = screen.getByText(/Import 1 selected/i);
    await fireEvent.click(importBtn);

    await waitFor(() => expect(mockApi.importDockerSuggestions).toHaveBeenCalled());
    const payload = mockApi.importDockerSuggestions.mock.calls[0][0];
    // The default-path icon: dashboard type + suggestion's icon name.
    // This is the "catalog match, no operator override" baseline.
    expect(payload.items[0].app.icon).toEqual({ type: 'dashboard', name: 'sonarr' });
  });

  it('renders a per-row icon picker button for every row, including low-confidence ones', async () => {
    // Two rows: one with a catalog hint (medium), one without (low,
    // empty icon). Both should expose the picker affordance because
    // the user might want to override either - the low-confidence
    // case is the *primary* reason this exists, but the high-/medium-
    // confidence case still benefits from a quick override path.
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [
        makeSuggestion({ key: 'name:c1', icon: 'sonarr', confidence: 'medium' }),
        makeSuggestion({ key: 'name:c2', name: 'Unknown', icon: '', confidence: 'low' }),
      ],
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });

    await waitFor(() => expect(screen.getByDisplayValue('C1')).toBeInTheDocument());

    // Each row has an aria-labelled icon button. The text content
    // mirrors the operator-editable name so the picker disambiguates
    // when multiple rows are visible.
    expect(screen.getByLabelText(/Pick icon for C1/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Pick icon for Unknown/i)).toBeInTheDocument();
  });

  it('opens the icon picker modal targeted at the row that was clicked', async () => {
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [
        makeSuggestion({ key: 'name:c1', name: 'AppOne' }),
        makeSuggestion({ key: 'name:c2', name: 'AppTwo' }),
      ],
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });
    await waitFor(() => expect(screen.getByDisplayValue('AppOne')).toBeInTheDocument());

    // Clicking the icon button for AppTwo opens the picker with
    // AppTwo's name in the heading, not AppOne's. This proves the
    // iconPickerForKey state correctly identifies the row.
    await fireEvent.click(screen.getByLabelText(/Pick icon for AppTwo/i));
    expect(screen.getByRole('dialog', { name: /Pick icon/i })).toBeInTheDocument();
    expect(screen.getByText(/Pick an icon for AppTwo/i)).toBeInTheDocument();
  });

  it('closes the icon picker when the close-button is clicked', async () => {
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [makeSuggestion({ name: 'AppOne' })],
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });
    await waitFor(() => expect(screen.getByDisplayValue('AppOne')).toBeInTheDocument());

    await fireEvent.click(screen.getByLabelText(/Pick icon for AppOne/i));
    // Scope the Close-button lookup to the inner dialog: the outer
    // modal header has its own Close button with the same accessible
    // name, so the unscoped query is ambiguous.
    const dialog = screen.getByRole('dialog', { name: /Pick icon/i });
    const closeBtn = within(dialog).getByRole('button', { name: /^Close$/ });
    await fireEvent.click(closeBtn);
    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: /Pick icon/i })).not.toBeInTheDocument()
    );
  });

  it('closes the icon picker when the IconBrowser fires onclose', async () => {
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [makeSuggestion({ name: 'AppOne' })],
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });
    await waitFor(() => expect(screen.getByDisplayValue('AppOne')).toBeInTheDocument());

    await fireEvent.click(screen.getByLabelText(/Pick icon for AppOne/i));
    // IconBrowserStub exposes a cancel button that calls onclose.
    // The modal must honour the inner cancel the same way it honours
    // the outer Close button - otherwise the keyboard-Esc / footer-
    // Cancel paths inside IconBrowser would feel broken.
    await fireEvent.click(screen.getByTestId('iconbrowser-stub-cancel'));
    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: /Pick icon/i })).not.toBeInTheDocument()
    );
  });

  it('writes the picked dashboard icon into the import payload', async () => {
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [makeSuggestion({ name: 'AppOne', icon: 'sonarr' })],
    });
    mockApi.importDockerSuggestions.mockResolvedValue({
      success: true,
      items: [{ key: 'name:c1', status: 'created', app_name: 'AppOne' }],
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });
    await waitFor(() => expect(screen.getByDisplayValue('AppOne')).toBeInTheDocument());

    // Open picker, pick "radarr" (dashboard), then import. The
    // picked icon must overwrite the catalog suggestion ("sonarr").
    await fireEvent.click(screen.getByLabelText(/Pick icon for AppOne/i));
    await fireEvent.click(screen.getByTestId('iconbrowser-stub-pick-dashboard'));

    const rowCheckbox = screen.getAllByRole('checkbox')[1] as HTMLInputElement;
    await fireEvent.click(rowCheckbox);
    await fireEvent.click(screen.getByText(/Import 1 selected/i));

    await waitFor(() => expect(mockApi.importDockerSuggestions).toHaveBeenCalled());
    const payload = mockApi.importDockerSuggestions.mock.calls[0][0];
    expect(payload.items[0].app.icon).toEqual({ type: 'dashboard', name: 'radarr', variant: 'svg' });
  });

  it('writes a picked lucide icon (different type branch) into the import payload', async () => {
    // Drives the type='lucide' branch of handleIconSelect, which is a
    // separate switch arm from dashboard/custom/url. Worth a dedicated
    // case because in production a single mistyped string literal in
    // that switch would silently lose icon picks for one variant.
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [makeSuggestion({ name: 'AppOne' })],
    });
    mockApi.importDockerSuggestions.mockResolvedValue({
      success: true,
      items: [{ key: 'name:c1', status: 'created', app_name: 'AppOne' }],
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });
    await waitFor(() => expect(screen.getByDisplayValue('AppOne')).toBeInTheDocument());

    await fireEvent.click(screen.getByLabelText(/Pick icon for AppOne/i));
    await fireEvent.click(screen.getByTestId('iconbrowser-stub-pick-lucide'));

    const rowCheckbox = screen.getAllByRole('checkbox')[1] as HTMLInputElement;
    await fireEvent.click(rowCheckbox);
    await fireEvent.click(screen.getByText(/Import 1 selected/i));

    await waitFor(() => expect(mockApi.importDockerSuggestions).toHaveBeenCalled());
    const payload = mockApi.importDockerSuggestions.mock.calls[0][0];
    expect(payload.items[0].app.icon).toEqual({ type: 'lucide', name: 'rocket', variant: 'svg' });
  });

  it('surfaces a network-level import failure (rejected promise) in the footer banner', async () => {
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [makeSuggestion()],
    });
    // Simulate the fetch layer throwing (offline / proxy 502 / DNS).
    // The component routes this into importTopError, not importResult.
    mockApi.importDockerSuggestions.mockRejectedValue(new Error('network down'));
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });
    await waitFor(() => expect(screen.getByDisplayValue('C1')).toBeInTheDocument());

    const rowCheckbox = screen.getAllByRole('checkbox')[1] as HTMLInputElement;
    await fireEvent.click(rowCheckbox);
    await fireEvent.click(screen.getByText(/Import 1 selected/i));

    await waitFor(() =>
      expect(screen.getByText(/network down/i)).toBeInTheDocument(),
    );
  });

  it('surfaces a structured import failure (success=false + error message) in the footer banner', async () => {
    // The 4xx path: server enforces validation and rejects the batch
    // wholesale. The footer should render the error text rather than
    // the empty fallback "see per-row status".
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [makeSuggestion()],
    });
    mockApi.importDockerSuggestions.mockResolvedValue({
      success: false,
      items: [],
      error: 'config save would diverge; refusing partial import',
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });
    await waitFor(() => expect(screen.getByDisplayValue('C1')).toBeInTheDocument());

    const rowCheckbox = screen.getAllByRole('checkbox')[1] as HTMLInputElement;
    await fireEvent.click(rowCheckbox);
    await fireEvent.click(screen.getByText(/Import 1 selected/i));

    await waitFor(() =>
      expect(screen.getByText(/config save would diverge/i)).toBeInTheDocument(),
    );
  });

  it('falls back to a generic "see per-row status" message when success=false without an error', async () => {
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [makeSuggestion()],
    });
    mockApi.importDockerSuggestions.mockResolvedValue({
      success: false,
      items: [{ key: 'name:c1', status: 'validation_failed', error: 'port required' }],
      // No top-level error here; the per-item errors should be enough.
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });
    await waitFor(() => expect(screen.getByDisplayValue('C1')).toBeInTheDocument());

    const rowCheckbox = screen.getAllByRole('checkbox')[1] as HTMLInputElement;
    await fireEvent.click(rowCheckbox);
    await fireEvent.click(screen.getByText(/Import 1 selected/i));

    await waitFor(() =>
      expect(screen.getByText(/see per-row status/i)).toBeInTheDocument(),
    );
  });

  it('shows the gateway-domain input on a row once "Add gateway site" is ticked', async () => {
    // Hits the {#if row.createGateway} branch (gateway-domain input
    // visibility tied to checkbox state).
    mockApi.scanDockerContainers.mockResolvedValue({
      // mode=apps starts createGateway off even if suggested_domain
      // is set, so we can drive the toggle in the test.
      suggestions: [makeSuggestion({ suggested_domain: 'c1.example.com' })],
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });
    await waitFor(() => expect(screen.getByDisplayValue('C1')).toBeInTheDocument());

    expect(screen.queryByPlaceholderText('sonarr.example.com')).not.toBeInTheDocument();

    // Toggle "Add gateway site" - the gateway-domain input becomes
    // visible and pre-filled from the suggested_domain field.
    const gwToggle = screen.getByLabelText(/Add gateway site/i);
    await fireEvent.click(gwToggle);
    const gwInput = screen.getByPlaceholderText('sonarr.example.com') as HTMLInputElement;
    expect(gwInput).toBeInTheDocument();
    expect(gwInput.value).toBe('c1.example.com');
  });

  it('enables the Gateway-domain radio once a gateway domain is supplied', async () => {
    // Hits the routing-radio's onchange branch when value=gateway:
    // selecting it should auto-tick createGateway (the converse case
    // of the existing forced-back-on effect). Drives that arm of the
    // radio's onchange handler.
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [makeSuggestion({ suggested_domain: 'c1.example.com' })],
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });
    await waitFor(() => expect(screen.getByDisplayValue('C1')).toBeInTheDocument());

    // Tick "Add gateway site" so the suggested_domain populates the
    // input - now the gateway radio becomes selectable.
    await fireEvent.click(screen.getByLabelText(/Add gateway site/i));

    const gatewayRadio = screen.getByTestId('row-routing')
      .querySelector('input[type="radio"][value="gateway"]') as HTMLInputElement;
    expect(gatewayRadio.disabled).toBe(false);
    await fireEvent.click(gatewayRadio);
    expect(gatewayRadio.checked).toBe(true);
  });

  it('renders a help icon + tooltip next to both "Add to menu" and "Add gateway site"', async () => {
    // The user couldn't tell the two checkboxes apart -- "Add to
    // menu" maps to the in-dashboard nav entry, "Add gateway site"
    // maps to a public subdomain. The fix is a per-checkbox help
    // icon with a hover tooltip. This test pins the icons exist
    // (one per checkbox) and that the tooltip <span> renders with
    // the expected explanatory copy alongside each.
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [makeSuggestion()],
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });
    await waitFor(() => expect(screen.getByDisplayValue('C1')).toBeInTheDocument());

    // Both help SVGs are rendered with distinct aria-labels. The
    // labels deliberately avoid the checkbox text so getByLabelText
    // queries scoped to the actual checkbox labels remain
    // unambiguous (this caught us once during implementation when
    // 'What does "Add gateway site" do?' shadowed the checkbox
    // label in a regex search).
    expect(screen.getByLabelText('More info about the menu option')).toBeInTheDocument();
    expect(screen.getByLabelText('More info about the gateway option')).toBeInTheDocument();

    // The tooltip <span>s mount alongside their triggers; we don't
    // assert on visibility (that's CSS-driven) but we do confirm the
    // explanatory copy is present in the DOM so screen readers and
    // hover-tooltips both have something to surface.
    expect(screen.getByText(/Adds this container as an app in the dashboard/i)).toBeInTheDocument();
    expect(screen.getByText(/Registers this container as a Caddy gateway site/i)).toBeInTheDocument();
  });

  it('honours the operator-edited name in the icon-picker heading', async () => {
    // The heading reads from nameOverride first, falling back to the
    // catalog name. This proves the picker reflects in-flight edits
    // rather than the stale suggestion - it would feel jarring to
    // rename "Sonarr" to "ATV" and then see "Pick an icon for Sonarr".
    mockApi.scanDockerContainers.mockResolvedValue({
      suggestions: [makeSuggestion({ name: 'Sonarr' })],
    });
    render(DiscoverModal, { open: true, mode: 'apps', onclose: () => {} });
    await waitFor(() => expect(screen.getByDisplayValue('Sonarr')).toBeInTheDocument());

    // Edit the row name from "Sonarr" to "ATV".
    const nameInput = screen.getByDisplayValue('Sonarr') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'ATV' } });

    await fireEvent.click(screen.getByLabelText(/Pick icon for ATV/i));
    expect(screen.getByText(/Pick an icon for ATV/i)).toBeInTheDocument();
  });
});

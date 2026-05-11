import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';
import type { DiscoverySuggestion } from '$lib/types';

// Mock the API module before importing the component. Each mock is a
// `vi.hoisted` factory so the SUT picks them up via Vite's vi.mock
// transform.
const mockApi = vi.hoisted(() => ({
  scanDockerContainers: vi.fn(),
  importDockerSuggestions: vi.fn(),
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
});

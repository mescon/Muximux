import { describe, it, expect } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { tick } from 'svelte';
import AppForm from './AppForm.svelte';
import type { App, Group } from '$lib/types';

function makeApp(overrides: Partial<App> = {}): App {
  return {
    name: 'TestApp',
    url: 'https://example.com',
    icon: { type: 'dashboard', name: 'x' },
    color: '#22c55e',
    group: '',
    order: 0,
    enabled: true,
    default: false,
    open_mode: 'iframe',
    proxy: false,
    scale: 1,
    ...overrides,
  };
}

const groups: Group[] = [];

describe('AppForm http_action conditional rendering', () => {
  it('hides proxy/scale/default when open_mode is http_action', async () => {
    const app = makeApp({ open_mode: 'http_action' });
    const { queryByText, queryByLabelText } = render(AppForm, { props: { app, mode: 'create', groups, allApps: [] } });
    expect(queryByText(/use reverse proxy/i)).toBeNull();
    expect(queryByLabelText(/scale/i)).toBeNull();
    expect(queryByText(/default app/i)).toBeNull();
  });

  it('shows method dropdown when open_mode is http_action', async () => {
    const app = makeApp({ open_mode: 'http_action' });
    const { getByLabelText } = render(AppForm, { props: { app, mode: 'create', groups, allApps: [] } });
    const methodSelect = getByLabelText(/method/i) as HTMLSelectElement;
    expect(methodSelect.tagName).toBe('SELECT');
    const options = Array.from(methodSelect.options).map((o) => o.value);
    expect(options).toEqual(['GET', 'POST', 'PUT', 'DELETE', 'PATCH']);
  });

  it('shows confirm and show-toast checkboxes when http_action', () => {
    const app = makeApp({ open_mode: 'http_action' });
    const { getByLabelText } = render(AppForm, { props: { app, mode: 'create', groups, allApps: [] } });
    expect(getByLabelText(/confirmation/i)).toBeTruthy();
    expect(getByLabelText(/show toast/i)).toBeTruthy();
  });

  it('shows proxy + scale + default when open_mode is iframe', () => {
    const app = makeApp({ open_mode: 'iframe' });
    const { getByText, getByLabelText } = render(AppForm, { props: { app, mode: 'create', groups, allApps: [] } });
    expect(getByText(/use reverse proxy/i)).toBeTruthy();
    expect(getByLabelText(/scale/i)).toBeTruthy();
    expect(getByText(/default app/i)).toBeTruthy();
  });

  it('switching open_mode from http_action back to iframe restores iframe-only fields', async () => {
    const app = $state(makeApp({ open_mode: 'http_action' }));
    const { getByLabelText, queryByText } = render(AppForm, { props: { app, mode: 'create', groups, allApps: [] } });
    const select = getByLabelText(/open mode/i) as HTMLSelectElement;
    await fireEvent.change(select, { target: { value: 'iframe' } });
    await tick();
    await waitFor(() => {
      expect(queryByText(/use reverse proxy/i)).toBeTruthy();
    });
  });
});

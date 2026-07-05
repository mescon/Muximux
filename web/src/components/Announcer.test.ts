import { describe, it, expect } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';
import Announcer from './Announcer.svelte';
import { announce } from '$lib/announce';

describe('Announcer', () => {
  it('renders a polite status live region', () => {
    const { container } = render(Announcer);
    const region = container.querySelector('[role="status"]');
    expect(region).toBeInTheDocument();
    expect(region?.getAttribute('aria-live')).toBe('polite');
  });

  it('reflects messages published via announce()', async () => {
    const { container } = render(Announcer);
    announce('Moved Sonarr to position 2 of 3');
    await waitFor(() =>
      expect(container.querySelector('[role="status"]')?.textContent).toContain(
        'Moved Sonarr to position 2 of 3'
      )
    );
  });
});

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import ErrorState from './ErrorState.svelte';

describe('ErrorState', () => {
  it('renders with default title "Something went wrong"', () => {
    render(ErrorState);
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
  });

  it('renders custom title and message', () => {
    render(ErrorState, {
      props: { title: 'Not Found', message: 'The page could not be found.' },
    });
    expect(screen.getByText('Not Found')).toBeInTheDocument();
    expect(screen.getByText('The page could not be found.')).toBeInTheDocument();
  });

  it('shows retry button by default', () => {
    render(ErrorState);
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument();
  });

  it('calls onretry when retry button is clicked', async () => {
    const onretry = vi.fn();
    render(ErrorState, { props: { onretry } });

    const button = screen.getByRole('button', { name: /try again/i });
    await fireEvent.click(button);

    expect(onretry).toHaveBeenCalledOnce();
  });

  it('hides retry button when showRetry is false', () => {
    render(ErrorState, { props: { showRetry: false } });
    expect(screen.queryByRole('button', { name: /try again/i })).not.toBeInTheDocument();
  });

  it('uses custom retry label', () => {
    render(ErrorState, { props: { retryLabel: 'Reload' } });
    expect(screen.getByRole('button', { name: /reload/i })).toBeInTheDocument();
  });

  it('renders error icon without crashing', () => {
    const { container } = render(ErrorState, { props: { icon: 'error' } });
    expect(container.querySelector('svg')).toBeInTheDocument();
  });

  it('renders network icon without crashing', () => {
    const { container } = render(ErrorState, { props: { icon: 'network' } });
    expect(container.querySelector('svg')).toBeInTheDocument();
  });

  it('renders notfound icon without crashing', () => {
    const { container } = render(ErrorState, { props: { icon: 'notfound' } });
    expect(container.querySelector('svg')).toBeInTheDocument();
  });

  it('renders empty icon without crashing', () => {
    const { container } = render(ErrorState, { props: { icon: 'empty' } });
    expect(container.querySelector('svg')).toBeInTheDocument();
  });
});

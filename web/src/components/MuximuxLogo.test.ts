import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/svelte';
import MuximuxLogo from './MuximuxLogo.svelte';

describe('MuximuxLogo', () => {
  it('renders an SVG element', () => {
    const { container } = render(MuximuxLogo);
    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();
  });

  it('has correct viewBox', () => {
    const { container } = render(MuximuxLogo);
    const svg = container.querySelector('svg');
    expect(svg).toHaveAttribute('viewBox', '0 0 341 207');
  });

  it('uses currentColor fill', () => {
    const { container } = render(MuximuxLogo);
    const g = container.querySelector('g');
    expect(g).toHaveAttribute('fill', 'currentColor');
  });

  it('applies custom class', () => {
    const { container } = render(MuximuxLogo, { props: { class: 'my-logo' } });
    const svg = container.querySelector('svg');
    expect(svg).toHaveClass('my-logo');
  });

  it('computes width from height using aspect ratio 341/207', () => {
    const { container } = render(MuximuxLogo, { props: { height: '207' } });
    const svg = container.querySelector('svg');
    // width = Math.round(207 * 341 / 207) = 341
    expect(svg).toHaveAttribute('width', '341');
    expect(svg).toHaveAttribute('height', '207');
  });

  it('computes height from width using aspect ratio 207/341', () => {
    const { container } = render(MuximuxLogo, { props: { width: '341' } });
    const svg = container.querySelector('svg');
    // height = Math.round(341 * 207 / 341) = 207
    expect(svg).toHaveAttribute('height', '207');
    expect(svg).toHaveAttribute('width', '341');
  });

  it('defaults height to 28 when neither height nor width provided', () => {
    const { container } = render(MuximuxLogo);
    const svg = container.querySelector('svg');
    expect(svg).toHaveAttribute('height', '28');
  });
});

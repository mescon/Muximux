import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/svelte';
import Skeleton from './Skeleton.svelte';

describe('Skeleton', () => {
  it('renders with default props', () => {
    const { container } = render(Skeleton);
    const div = container.querySelector('div');
    expect(div).toBeInTheDocument();
    // Assert the declared inline style rather than the computed one. Skeleton's
    // contract is that it passes width/height straight through to the style
    // attribute, and jsdom 30 resolves rem against the root font size in
    // getComputedStyle (1rem -> 16px), which toHaveStyle would compare against.
    expect(div?.style.width).toBe('100%');
    expect(div?.style.height).toBe('1rem');
  });

  it('has animate-pulse class', () => {
    const { container } = render(Skeleton);
    const div = container.querySelector('div');
    expect(div).toHaveClass('animate-pulse');
  });

  it('applies rounded-full when circle is true', () => {
    const { container } = render(Skeleton, { props: { circle: true, height: '3rem' } });
    const div = container.querySelector('div');
    expect(div).toHaveClass('rounded-full');
    // When circle is true, width should equal height
    expect(div?.style.width).toBe('3rem');
    expect(div?.style.height).toBe('3rem');
  });

  it('applies rounded (not rounded-full) when circle is false', () => {
    const { container } = render(Skeleton);
    const div = container.querySelector('div');
    expect(div).toHaveClass('rounded');
    expect(div).not.toHaveClass('rounded-full');
  });

  it('applies custom class', () => {
    const { container } = render(Skeleton, { props: { class: 'extra-class' } });
    const div = container.querySelector('div');
    expect(div).toHaveClass('extra-class');
  });

  it('sets aria-hidden to true', () => {
    const { container } = render(Skeleton);
    const div = container.querySelector('div');
    expect(div).toHaveAttribute('aria-hidden', 'true');
  });

  it('applies custom width and height', () => {
    const { container } = render(Skeleton, { props: { width: '50%', height: '2rem' } });
    const div = container.querySelector('div');
    expect(div?.style.width).toBe('50%');
    expect(div?.style.height).toBe('2rem');
  });
});

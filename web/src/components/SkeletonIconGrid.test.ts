import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/svelte';
import SkeletonIconGrid from './SkeletonIconGrid.svelte';

describe('SkeletonIconGrid', () => {
  it('renders default count of 30 items', () => {
    const { container } = render(SkeletonIconGrid);
    const items = container.querySelectorAll('.aspect-square');
    expect(items).toHaveLength(30);
  });

  it('renders custom count of items', () => {
    const { container } = render(SkeletonIconGrid, { props: { count: 12 } });
    const items = container.querySelectorAll('.aspect-square');
    expect(items).toHaveLength(12);
  });

  it('each item contains a Skeleton component', () => {
    const { container } = render(SkeletonIconGrid, { props: { count: 5 } });
    const items = container.querySelectorAll('.aspect-square');
    items.forEach((item) => {
      const skeleton = item.querySelector('[aria-hidden="true"]');
      expect(skeleton).toBeInTheDocument();
      expect(skeleton).toHaveClass('animate-pulse');
    });
  });

  it('renders a grid container', () => {
    const { container } = render(SkeletonIconGrid);
    const grid = container.querySelector('.grid');
    expect(grid).toBeInTheDocument();
  });
});

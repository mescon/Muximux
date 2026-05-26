import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/svelte';
import DockerLogo from './DockerLogo.svelte';

describe('DockerLogo', () => {
  it('renders an inline SVG', () => {
    const { container } = render(DockerLogo);
    const svg = container.querySelector('svg');
    expect(svg).not.toBeNull();
    expect(svg?.getAttribute('viewBox')).toBeTruthy();
  });

  it('honours the size prop', () => {
    const { container } = render(DockerLogo, { props: { size: 'md' } });
    const svg = container.querySelector('svg')!;
    // sm = 12, md = 16, lg = 20 (mirrors HealthIndicator size scale).
    expect(svg.getAttribute('width') || svg.style.width).toContain('16');
  });

  it('passes through the class prop', () => {
    const { container } = render(DockerLogo, { props: { class: 'text-slate-500' } });
    const svg = container.querySelector('svg')!;
    expect(svg.classList.contains('text-slate-500')).toBe(true);
  });
});

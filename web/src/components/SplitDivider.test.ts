import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import SplitDivider from './SplitDivider.svelte';

const defaultProps = {
  orientation: 'horizontal' as const,
  activePanel: 0 as const,
  onresize: vi.fn(),
  ondblclick: vi.fn(),
};

describe('SplitDivider', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset body cursor
    document.body.style.cursor = '';
  });

  afterEach(() => {
    document.body.style.cursor = '';
  });

  // ─── Existing tests (preserved) ───────────────────────────────────────────

  it('renders horizontal divider with correct class', () => {
    const { container } = render(SplitDivider, {
      props: { ...defaultProps, orientation: 'horizontal' },
    });
    const divider = container.querySelector('.split-divider');
    expect(divider).toBeInTheDocument();
    expect(divider).toHaveClass('split-divider-h');
    expect(divider).not.toHaveClass('split-divider-v');
  });

  it('renders vertical divider with correct class', () => {
    const { container } = render(SplitDivider, {
      props: { ...defaultProps, orientation: 'vertical' },
    });
    const divider = container.querySelector('.split-divider');
    expect(divider).toBeInTheDocument();
    expect(divider).toHaveClass('split-divider-v');
    expect(divider).not.toHaveClass('split-divider-h');
  });

  it('applies active-0 class when activePanel is 0', () => {
    const { container } = render(SplitDivider, {
      props: { ...defaultProps, activePanel: 0 },
    });
    const divider = container.querySelector('.split-divider');
    expect(divider).toHaveClass('active-0');
    expect(divider).not.toHaveClass('active-1');
  });

  it('applies active-1 class when activePanel is 1', () => {
    const { container } = render(SplitDivider, {
      props: { ...defaultProps, activePanel: 1 },
    });
    const divider = container.querySelector('.split-divider');
    expect(divider).toHaveClass('active-1');
    expect(divider).not.toHaveClass('active-0');
  });

  it('calls ondblclick on double click', async () => {
    const ondblclick = vi.fn();
    const { container } = render(SplitDivider, {
      props: { ...defaultProps, ondblclick },
    });
    const divider = container.querySelector('.split-divider')!;
    await fireEvent.dblClick(divider);
    expect(ondblclick).toHaveBeenCalledOnce();
  });

  // ─── Dragging class ───────────────────────────────────────────────────────

  describe('dragging state', () => {
    it('does not have dragging class initially', () => {
      const { container } = render(SplitDivider, { props: defaultProps });
      const divider = container.querySelector('.split-divider');
      expect(divider).not.toHaveClass('dragging');
    });

    it('adds dragging class on pointerdown', async () => {
      const { container } = render(SplitDivider, { props: defaultProps });
      const divider = container.querySelector('.split-divider')!;

      // Mock setPointerCapture
      divider.setPointerCapture = vi.fn();

      await fireEvent.pointerDown(divider, { pointerId: 1 });

      expect(divider).toHaveClass('dragging');
    });

    it('removes dragging class on pointerup', async () => {
      const { container } = render(SplitDivider, { props: defaultProps });
      const divider = container.querySelector('.split-divider')!;
      divider.setPointerCapture = vi.fn();

      await fireEvent.pointerDown(divider, { pointerId: 1 });
      expect(divider).toHaveClass('dragging');

      await fireEvent.pointerUp(divider);
      expect(divider).not.toHaveClass('dragging');
    });

    it('removes dragging class on pointercancel', async () => {
      const { container } = render(SplitDivider, { props: defaultProps });
      const divider = container.querySelector('.split-divider')!;
      divider.setPointerCapture = vi.fn();

      await fireEvent.pointerDown(divider, { pointerId: 1 });
      expect(divider).toHaveClass('dragging');

      await fireEvent.pointerCancel(divider);
      expect(divider).not.toHaveClass('dragging');
    });
  });

  // ─── Pointer capture ──────────────────────────────────────────────────────

  describe('pointer capture', () => {
    it('calls setPointerCapture on pointerdown', async () => {
      const { container } = render(SplitDivider, { props: defaultProps });
      const divider = container.querySelector('.split-divider')!;
      divider.setPointerCapture = vi.fn();

      await fireEvent.pointerDown(divider, { pointerId: 42 });

      expect(divider.setPointerCapture).toHaveBeenCalledWith(42);
    });
  });

  // ─── Cursor changes during drag ──────────────────────────────────────────

  describe('cursor changes during drag', () => {
    it('sets body cursor to col-resize for horizontal drag', async () => {
      const { container } = render(SplitDivider, {
        props: { ...defaultProps, orientation: 'horizontal' },
      });
      const divider = container.querySelector('.split-divider')!;
      divider.setPointerCapture = vi.fn();

      await fireEvent.pointerDown(divider, { pointerId: 1 });

      expect(document.body.style.cursor).toBe('col-resize');
    });

    it('sets body cursor to row-resize for vertical drag', async () => {
      const { container } = render(SplitDivider, {
        props: { ...defaultProps, orientation: 'vertical' },
      });
      const divider = container.querySelector('.split-divider')!;
      divider.setPointerCapture = vi.fn();

      await fireEvent.pointerDown(divider, { pointerId: 1 });

      expect(document.body.style.cursor).toBe('row-resize');
    });

    it('resets body cursor on pointerup', async () => {
      const { container } = render(SplitDivider, { props: defaultProps });
      const divider = container.querySelector('.split-divider')!;
      divider.setPointerCapture = vi.fn();

      await fireEvent.pointerDown(divider, { pointerId: 1 });
      expect(document.body.style.cursor).toBe('col-resize');

      await fireEvent.pointerUp(divider);
      expect(document.body.style.cursor).toBe('');
    });

    it('resets body cursor on pointercancel', async () => {
      const { container } = render(SplitDivider, { props: defaultProps });
      const divider = container.querySelector('.split-divider')!;
      divider.setPointerCapture = vi.fn();

      await fireEvent.pointerDown(divider, { pointerId: 1 });
      expect(document.body.style.cursor).toBe('col-resize');

      await fireEvent.pointerCancel(divider);
      expect(document.body.style.cursor).toBe('');
    });
  });

  // ─── Iframe pointer-events during drag ────────────────────────────────────

  describe('iframe pointer-events during drag', () => {
    it('disables pointer-events on iframes during drag', async () => {
      // Add an iframe to the document
      const iframe = document.createElement('iframe');
      document.body.appendChild(iframe);

      try {
        const { container } = render(SplitDivider, { props: defaultProps });
        const divider = container.querySelector('.split-divider')!;
        divider.setPointerCapture = vi.fn();

        await fireEvent.pointerDown(divider, { pointerId: 1 });

        expect(iframe.style.pointerEvents).toBe('none');
      } finally {
        document.body.removeChild(iframe);
      }
    });

    it('restores pointer-events on iframes on pointerup', async () => {
      const iframe = document.createElement('iframe');
      document.body.appendChild(iframe);

      try {
        const { container } = render(SplitDivider, { props: defaultProps });
        const divider = container.querySelector('.split-divider')!;
        divider.setPointerCapture = vi.fn();

        await fireEvent.pointerDown(divider, { pointerId: 1 });
        expect(iframe.style.pointerEvents).toBe('none');

        await fireEvent.pointerUp(divider);
        expect(iframe.style.pointerEvents).toBe('');
      } finally {
        document.body.removeChild(iframe);
      }
    });

    it('restores pointer-events on iframes on pointercancel', async () => {
      const iframe = document.createElement('iframe');
      document.body.appendChild(iframe);

      try {
        const { container } = render(SplitDivider, { props: defaultProps });
        const divider = container.querySelector('.split-divider')!;
        divider.setPointerCapture = vi.fn();

        await fireEvent.pointerDown(divider, { pointerId: 1 });
        expect(iframe.style.pointerEvents).toBe('none');

        await fireEvent.pointerCancel(divider);
        expect(iframe.style.pointerEvents).toBe('');
      } finally {
        document.body.removeChild(iframe);
      }
    });
  });

  // ─── onresize callback during drag ────────────────────────────────────────

  describe('onresize callback during drag', () => {
    it('does not call onresize on pointermove when not dragging', async () => {
      const onresize = vi.fn();
      const { container } = render(SplitDivider, {
        props: { ...defaultProps, onresize },
      });
      const divider = container.querySelector('.split-divider')!;

      await fireEvent.pointerMove(divider, { clientX: 100, clientY: 50 });

      expect(onresize).not.toHaveBeenCalled();
    });

    it('calls onresize on pointermove during horizontal drag', async () => {
      const onresize = vi.fn();
      const { container } = render(SplitDivider, {
        props: { ...defaultProps, orientation: 'horizontal', onresize },
      });
      const divider = container.querySelector('.split-divider')!;
      divider.setPointerCapture = vi.fn();

      // Mock parent element's getBoundingClientRect
      const parent = divider.parentElement!;
      vi.spyOn(parent, 'getBoundingClientRect').mockReturnValue({
        left: 0, top: 0, width: 1000, height: 600,
        right: 1000, bottom: 600, x: 0, y: 0, toJSON: () => {},
      });

      await fireEvent.pointerDown(divider, { pointerId: 1 });
      await fireEvent.pointerMove(divider, { clientX: 400, clientY: 300 });

      expect(onresize).toHaveBeenCalledWith(0.4); // 400/1000
    });

    it('calls onresize on pointermove during vertical drag', async () => {
      const onresize = vi.fn();
      const { container } = render(SplitDivider, {
        props: { ...defaultProps, orientation: 'vertical', onresize },
      });
      const divider = container.querySelector('.split-divider')!;
      divider.setPointerCapture = vi.fn();

      const parent = divider.parentElement!;
      vi.spyOn(parent, 'getBoundingClientRect').mockReturnValue({
        left: 0, top: 0, width: 1000, height: 800,
        right: 1000, bottom: 800, x: 0, y: 0, toJSON: () => {},
      });

      await fireEvent.pointerDown(divider, { pointerId: 1 });
      await fireEvent.pointerMove(divider, { clientX: 200, clientY: 400 });

      expect(onresize).toHaveBeenCalledWith(0.5); // 400/800
    });

    it('stops calling onresize after pointerup', async () => {
      const onresize = vi.fn();
      const { container } = render(SplitDivider, {
        props: { ...defaultProps, orientation: 'horizontal', onresize },
      });
      const divider = container.querySelector('.split-divider')!;
      divider.setPointerCapture = vi.fn();

      const parent = divider.parentElement!;
      vi.spyOn(parent, 'getBoundingClientRect').mockReturnValue({
        left: 0, top: 0, width: 1000, height: 600,
        right: 1000, bottom: 600, x: 0, y: 0, toJSON: () => {},
      });

      await fireEvent.pointerDown(divider, { pointerId: 1 });
      await fireEvent.pointerMove(divider, { clientX: 300, clientY: 100 });
      expect(onresize).toHaveBeenCalledTimes(1);

      await fireEvent.pointerUp(divider);

      // Move again after pointerup
      await fireEvent.pointerMove(divider, { clientX: 500, clientY: 100 });
      expect(onresize).toHaveBeenCalledTimes(1); // no additional call
    });

    it('computes correct position with non-zero container offset', async () => {
      const onresize = vi.fn();
      const { container } = render(SplitDivider, {
        props: { ...defaultProps, orientation: 'horizontal', onresize },
      });
      const divider = container.querySelector('.split-divider')!;
      divider.setPointerCapture = vi.fn();

      const parent = divider.parentElement!;
      vi.spyOn(parent, 'getBoundingClientRect').mockReturnValue({
        left: 200, top: 100, width: 800, height: 600,
        right: 1000, bottom: 700, x: 200, y: 100, toJSON: () => {},
      });

      await fireEvent.pointerDown(divider, { pointerId: 1 });
      await fireEvent.pointerMove(divider, { clientX: 600, clientY: 300 });

      // (600 - 200) / 800 = 0.5
      expect(onresize).toHaveBeenCalledWith(0.5);
    });
  });

  // ─── Multiple drags ───────────────────────────────────────────────────────

  describe('multiple drag cycles', () => {
    it('handles a second drag cycle correctly', async () => {
      const onresize = vi.fn();
      const { container } = render(SplitDivider, {
        props: { ...defaultProps, orientation: 'horizontal', onresize },
      });
      const divider = container.querySelector('.split-divider')!;
      divider.setPointerCapture = vi.fn();

      const parent = divider.parentElement!;
      vi.spyOn(parent, 'getBoundingClientRect').mockReturnValue({
        left: 0, top: 0, width: 1000, height: 600,
        right: 1000, bottom: 600, x: 0, y: 0, toJSON: () => {},
      });

      // First drag
      await fireEvent.pointerDown(divider, { pointerId: 1 });
      await fireEvent.pointerMove(divider, { clientX: 300 });
      await fireEvent.pointerUp(divider);

      expect(onresize).toHaveBeenCalledTimes(1);
      expect(divider).not.toHaveClass('dragging');

      // Second drag
      await fireEvent.pointerDown(divider, { pointerId: 2 });
      expect(divider).toHaveClass('dragging');
      await fireEvent.pointerMove(divider, { clientX: 700 });
      expect(onresize).toHaveBeenCalledTimes(2);
      await fireEvent.pointerUp(divider);
      expect(divider).not.toHaveClass('dragging');
    });
  });
});

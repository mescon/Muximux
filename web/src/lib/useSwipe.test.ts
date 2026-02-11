import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  createSwipeHandlers,
  createEdgeSwipeHandlers,
  isTouchDevice,
  isMobileViewport,
  type SwipeResult,
  type SwipeState,
} from './useSwipe';

describe('useSwipe', () => {
  describe('createSwipeHandlers', () => {
    let onSwipe: ReturnType<typeof vi.fn<(result: SwipeResult) => void>>;
    let handlers: ReturnType<typeof createSwipeHandlers>;

    beforeEach(() => {
      onSwipe = vi.fn<(result: SwipeResult) => void>();
      handlers = createSwipeHandlers(onSwipe);
    });

    function createPointerEvent(
      type: string,
      { clientX = 0, clientY = 0, isPrimary = true, pointerId = 1 } = {}
    ): PointerEvent {
      return {
        type,
        clientX,
        clientY,
        isPrimary,
        pointerId,
        target: document.createElement('div'),
        preventDefault: vi.fn(),
      } as unknown as PointerEvent;
    }

    it('should detect left swipe', () => {
      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 200, clientY: 100 }));
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: 100, clientY: 100 }));

      expect(onSwipe).toHaveBeenCalled();
      const result: SwipeResult = onSwipe.mock.calls[0][0];
      expect(result.direction).toBe('left');
      expect(result.deltaX).toBe(-100);
    });

    it('should detect right swipe', () => {
      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 100, clientY: 100 }));
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: 200, clientY: 100 }));

      expect(onSwipe).toHaveBeenCalled();
      const result: SwipeResult = onSwipe.mock.calls[0][0];
      expect(result.direction).toBe('right');
      expect(result.deltaX).toBe(100);
    });

    it('should detect up swipe', () => {
      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 100, clientY: 200 }));
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: 100, clientY: 100 }));

      expect(onSwipe).toHaveBeenCalled();
      const result: SwipeResult = onSwipe.mock.calls[0][0];
      expect(result.direction).toBe('up');
      expect(result.deltaY).toBe(-100);
    });

    it('should detect down swipe', () => {
      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 100, clientY: 100 }));
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: 100, clientY: 200 }));

      expect(onSwipe).toHaveBeenCalled();
      const result: SwipeResult = onSwipe.mock.calls[0][0];
      expect(result.direction).toBe('down');
      expect(result.deltaY).toBe(100);
    });

    it('should return null direction for short swipes', () => {
      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 100, clientY: 100 }));
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: 110, clientY: 100 }));

      expect(onSwipe).toHaveBeenCalled();
      const result: SwipeResult = onSwipe.mock.calls[0][0];
      expect(result.direction).toBeNull();
    });

    it('should ignore non-primary pointer events', () => {
      handlers.onpointerdown(
        createPointerEvent('pointerdown', { clientX: 200, clientY: 100, isPrimary: false })
      );
      handlers.onpointerup(
        createPointerEvent('pointerup', { clientX: 100, clientY: 100, isPrimary: false })
      );

      expect(onSwipe).not.toHaveBeenCalled();
    });

    it('should call onSwipeMove during swipe', () => {
      const onSwipeMove = vi.fn<(state: SwipeState) => void>();
      handlers = createSwipeHandlers(onSwipe, onSwipeMove);

      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 100, clientY: 100 }));
      handlers.onpointermove(createPointerEvent('pointermove', { clientX: 120, clientY: 100 }));

      expect(onSwipeMove).toHaveBeenCalled();
    });

    it('should handle pointer cancel', () => {
      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 100, clientY: 100 }));
      handlers.onpointercancel();
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: 200, clientY: 100 }));

      expect(onSwipe).not.toHaveBeenCalled();
    });

    it('should respect custom threshold config', () => {
      handlers = createSwipeHandlers(onSwipe, undefined, { threshold: 100 });

      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 100, clientY: 100 }));
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: 150, clientY: 100 }));

      const result: SwipeResult = onSwipe.mock.calls[0][0];
      expect(result.direction).toBeNull(); // 50px < 100px threshold
    });
  });

  describe('createEdgeSwipeHandlers', () => {
    let onSwipeOpen: ReturnType<typeof vi.fn<() => void>>;
    let onSwipeClose: ReturnType<typeof vi.fn<() => void>>;

    beforeEach(() => {
      onSwipeOpen = vi.fn<() => void>();
      onSwipeClose = vi.fn<() => void>();
      // Mock window dimensions
      Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
    });

    function createPointerEvent(
      type: string,
      { clientX = 0, clientY = 0 } = {}
    ): PointerEvent {
      return {
        type,
        clientX,
        clientY,
        isPrimary: true,
        pointerId: 1,
      } as unknown as PointerEvent;
    }

    it('should detect left edge swipe open', () => {
      const handlers = createEdgeSwipeHandlers('left', onSwipeOpen, onSwipeClose);

      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 10, clientY: 100 }));
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: 100, clientY: 100 }));

      expect(onSwipeOpen).toHaveBeenCalled();
      expect(onSwipeClose).not.toHaveBeenCalled();
    });

    it('should detect left edge swipe close', () => {
      const handlers = createEdgeSwipeHandlers('left', onSwipeOpen, onSwipeClose);

      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 10, clientY: 100 }));
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: -50, clientY: 100 }));

      expect(onSwipeClose).toHaveBeenCalled();
      expect(onSwipeOpen).not.toHaveBeenCalled();
    });

    it('should detect right edge swipe open', () => {
      const handlers = createEdgeSwipeHandlers('right', onSwipeOpen, onSwipeClose);

      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 1014, clientY: 100 }));
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: 900, clientY: 100 }));

      expect(onSwipeOpen).toHaveBeenCalled();
    });

    it('should not trigger for non-edge swipes', () => {
      const handlers = createEdgeSwipeHandlers('left', onSwipeOpen, onSwipeClose);

      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 100, clientY: 100 }));
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: 200, clientY: 100 }));

      expect(onSwipeOpen).not.toHaveBeenCalled();
      expect(onSwipeClose).not.toHaveBeenCalled();
    });

    it('should handle pointer cancel', () => {
      const handlers = createEdgeSwipeHandlers('left', onSwipeOpen, onSwipeClose);

      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 10, clientY: 100 }));
      handlers.onpointercancel();
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: 100, clientY: 100 }));

      expect(onSwipeOpen).not.toHaveBeenCalled();
    });

    it('should respect custom edge width', () => {
      const handlers = createEdgeSwipeHandlers('left', onSwipeOpen, onSwipeClose, { edgeWidth: 50 });

      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 40, clientY: 100 }));
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: 150, clientY: 100 }));

      expect(onSwipeOpen).toHaveBeenCalled();
    });
  });

  describe('isTouchDevice', () => {
    it('should return true when ontouchstart exists', () => {
      Object.defineProperty(window, 'ontouchstart', { value: vi.fn(), configurable: true });
      Object.defineProperty(navigator, 'maxTouchPoints', { value: 0, configurable: true });

      expect(isTouchDevice()).toBe(true);
    });

    it('should return true when maxTouchPoints > 0', () => {
      // Remove ontouchstart
      const descriptor = Object.getOwnPropertyDescriptor(window, 'ontouchstart');
      if (descriptor) delete (window as any).ontouchstart;

      Object.defineProperty(navigator, 'maxTouchPoints', { value: 5, configurable: true });

      expect(isTouchDevice()).toBe(true);
    });

    it('should return false for non-touch device', () => {
      const descriptor = Object.getOwnPropertyDescriptor(window, 'ontouchstart');
      if (descriptor) delete (window as any).ontouchstart;

      Object.defineProperty(navigator, 'maxTouchPoints', { value: 0, configurable: true });

      expect(isTouchDevice()).toBe(false);
    });
  });

  describe('isMobileViewport', () => {
    it('should return true for small viewport', () => {
      Object.defineProperty(window, 'innerWidth', { value: 375, configurable: true });
      expect(isMobileViewport()).toBe(true);
    });

    it('should return false for large viewport', () => {
      Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
      expect(isMobileViewport()).toBe(false);
    });

    it('should return false for exactly 640px (breakpoint boundary)', () => {
      Object.defineProperty(window, 'innerWidth', { value: 640, configurable: true });
      expect(isMobileViewport()).toBe(false);
    });

    it('should return true for 639px', () => {
      Object.defineProperty(window, 'innerWidth', { value: 639, configurable: true });
      expect(isMobileViewport()).toBe(true);
    });
  });
});

import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  createSwipeHandlers,
  createEdgeSwipeHandlers,
  createPullToRefreshHandlers,
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

    it('should detect right edge swipe close', () => {
      const handlers = createEdgeSwipeHandlers('right', onSwipeOpen, onSwipeClose);

      // Start at right edge, swipe right (positive deltaX ≥ 50px threshold)
      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 1010, clientY: 100 }));
      handlers.onpointerup(createPointerEvent('pointerup', { clientX: 1080, clientY: 100 }));

      expect(onSwipeClose).toHaveBeenCalled();
      expect(onSwipeOpen).not.toHaveBeenCalled();
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
      if (descriptor) delete (window as unknown as Record<string, unknown>).ontouchstart;

      Object.defineProperty(navigator, 'maxTouchPoints', { value: 5, configurable: true });

      expect(isTouchDevice()).toBe(true);
    });

    it('should return false for non-touch device', () => {
      const descriptor = Object.getOwnPropertyDescriptor(window, 'ontouchstart');
      if (descriptor) delete (window as unknown as Record<string, unknown>).ontouchstart;

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

  describe('createSwipeHandlers - additional coverage', () => {
    it('should not call onSwipeMove before swiping threshold', () => {
      const onSwipe = vi.fn();
      const onSwipeMove = vi.fn();
      const handlers = createSwipeHandlers(onSwipe, onSwipeMove);

      function createPointerEvent(
        type: string,
        { clientX = 0, clientY = 0, isPrimary = true } = {}
      ): PointerEvent {
        return { type, clientX, clientY, isPrimary, pointerId: 1 } as unknown as PointerEvent;
      }

      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 100, clientY: 100 }));
      // Move less than 10px threshold
      handlers.onpointermove(createPointerEvent('pointermove', { clientX: 105, clientY: 100 }));

      expect(onSwipeMove).not.toHaveBeenCalled();
    });

    it('should ignore non-primary pointer in move', () => {
      const onSwipe = vi.fn();
      const onSwipeMove = vi.fn();
      const handlers = createSwipeHandlers(onSwipe, onSwipeMove);

      function createPointerEvent(
        type: string,
        { clientX = 0, clientY = 0, isPrimary = true } = {}
      ): PointerEvent {
        return { type, clientX, clientY, isPrimary, pointerId: 1 } as unknown as PointerEvent;
      }

      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 100, clientY: 100 }));
      handlers.onpointermove(
        createPointerEvent('pointermove', { clientX: 200, clientY: 100, isPrimary: false })
      );

      expect(onSwipeMove).not.toHaveBeenCalled();
    });

    it('should not trigger move handler when no pointer is down', () => {
      const onSwipe = vi.fn();
      const onSwipeMove = vi.fn();
      const handlers = createSwipeHandlers(onSwipe, onSwipeMove);

      handlers.onpointermove(
        { clientX: 200, clientY: 100, isPrimary: true } as unknown as PointerEvent
      );

      expect(onSwipeMove).not.toHaveBeenCalled();
    });

    it('should detect vertical swiping via move callback', () => {
      const onSwipe = vi.fn();
      const onSwipeMove = vi.fn();
      const handlers = createSwipeHandlers(onSwipe, onSwipeMove);

      function createPointerEvent(
        type: string,
        { clientX = 0, clientY = 0, isPrimary = true } = {}
      ): PointerEvent {
        return { type, clientX, clientY, isPrimary, pointerId: 1 } as unknown as PointerEvent;
      }

      handlers.onpointerdown(createPointerEvent('pointerdown', { clientX: 100, clientY: 100 }));
      handlers.onpointermove(createPointerEvent('pointermove', { clientX: 100, clientY: 120 }));

      expect(onSwipeMove).toHaveBeenCalledWith(
        expect.objectContaining({
          startX: 100,
          startY: 100,
          currentX: 100,
          currentY: 120,
          isSwiping: true,
        })
      );
    });
  });

  describe('createPullToRefreshHandlers', () => {
    function createTouchEvent(clientY: number, scrollTop = 0): TouchEvent {
      const target = document.createElement('div');
      Object.defineProperty(target, 'scrollTop', { value: scrollTop, configurable: true });
      return {
        target,
        touches: [{ clientY }],
        preventDefault: vi.fn(),
      } as unknown as TouchEvent;
    }

    it('should trigger refresh when pull exceeds threshold', async () => {
      const onRefresh = vi.fn().mockResolvedValue(undefined);
      const handlers = createPullToRefreshHandlers(onRefresh);

      handlers.ontouchstart(createTouchEvent(0));
      handlers.ontouchmove(createTouchEvent(250)); // Pull 250px / 2.5 resistance = 100px > 80px threshold

      await handlers.ontouchend();

      expect(onRefresh).toHaveBeenCalled();
    });

    it('should not trigger refresh when pull is below threshold', async () => {
      const onRefresh = vi.fn().mockResolvedValue(undefined);
      const handlers = createPullToRefreshHandlers(onRefresh);

      handlers.ontouchstart(createTouchEvent(0));
      handlers.ontouchmove(createTouchEvent(100)); // Pull 100px / 2.5 = 40px < 80px threshold

      await handlers.ontouchend();

      expect(onRefresh).not.toHaveBeenCalled();
    });

    it('should not start pulling when not at top of scroll', () => {
      const onRefresh = vi.fn();
      const handlers = createPullToRefreshHandlers(onRefresh);

      handlers.ontouchstart(createTouchEvent(0, 100)); // scrollTop = 100, not at top

      const result = handlers.ontouchmove(createTouchEvent(200));
      expect(result).toBeUndefined();
    });

    it('should return pull progress during move', () => {
      const onRefresh = vi.fn();
      const handlers = createPullToRefreshHandlers(onRefresh);

      handlers.ontouchstart(createTouchEvent(100));
      const result = handlers.ontouchmove(createTouchEvent(300));

      expect(result).toEqual(
        expect.objectContaining({
          pullDistance: expect.any(Number),
          progress: expect.any(Number),
          isOverThreshold: expect.any(Boolean),
        })
      );
    });

    it('should return null when pulling upward', () => {
      const onRefresh = vi.fn();
      const handlers = createPullToRefreshHandlers(onRefresh);

      handlers.ontouchstart(createTouchEvent(200));
      const result = handlers.ontouchmove(createTouchEvent(100)); // Pulling up

      expect(result).toBeNull();
    });

    it('should cap progress at 1', () => {
      const onRefresh = vi.fn();
      const handlers = createPullToRefreshHandlers(onRefresh);

      handlers.ontouchstart(createTouchEvent(0));
      const result = handlers.ontouchmove(createTouchEvent(500)); // Large pull

      expect(result?.progress).toBeLessThanOrEqual(1);
    });

    it('should report isOverThreshold correctly', () => {
      const onRefresh = vi.fn();
      const handlers = createPullToRefreshHandlers(onRefresh);

      handlers.ontouchstart(createTouchEvent(0));

      // Below threshold
      let result = handlers.ontouchmove(createTouchEvent(100)); // 100/2.5 = 40 < 80
      expect(result?.isOverThreshold).toBe(false);

      // Above threshold
      result = handlers.ontouchmove(createTouchEvent(250)); // 250/2.5 = 100 >= 80
      expect(result?.isOverThreshold).toBe(true);
    });

    it('should respect custom threshold', async () => {
      const onRefresh = vi.fn().mockResolvedValue(undefined);
      const handlers = createPullToRefreshHandlers(onRefresh, { threshold: 200 });

      handlers.ontouchstart(createTouchEvent(0));
      handlers.ontouchmove(createTouchEvent(250)); // 250/2.5 = 100 < 200

      await handlers.ontouchend();

      expect(onRefresh).not.toHaveBeenCalled();
    });

    it('should respect custom resistance', () => {
      const onRefresh = vi.fn();
      const handlers = createPullToRefreshHandlers(onRefresh, { resistance: 5 });

      handlers.ontouchstart(createTouchEvent(0));
      const result = handlers.ontouchmove(createTouchEvent(200)); // 200/5 = 40

      expect(result?.pullDistance).toBe(40);
    });

    it('should prevent default during pull', () => {
      const onRefresh = vi.fn();
      const handlers = createPullToRefreshHandlers(onRefresh);

      handlers.ontouchstart(createTouchEvent(0));
      const event = createTouchEvent(200);
      handlers.ontouchmove(event);

      expect(event.preventDefault).toHaveBeenCalled();
    });

    it('should not accept touch during refresh', async () => {
      let resolveRefresh: () => void;
      const onRefresh = vi.fn(
        () => new Promise<void>((resolve) => { resolveRefresh = resolve; })
      );
      const handlers = createPullToRefreshHandlers(onRefresh);

      // Start and complete a pull that triggers refresh
      handlers.ontouchstart(createTouchEvent(0));
      handlers.ontouchmove(createTouchEvent(500));
      const endPromise = handlers.ontouchend();

      // Try to start another touch while refreshing
      handlers.ontouchstart(createTouchEvent(0));
      const state = handlers.getState();
      expect(state.isRefreshing).toBe(true);

      // Resolve the refresh
      resolveRefresh!();
      await endPromise;

      expect(handlers.getState().isRefreshing).toBe(false);
    });

    it('should not move during refresh', async () => {
      let resolveRefresh: () => void;
      const onRefresh = vi.fn(
        () => new Promise<void>((resolve) => { resolveRefresh = resolve; })
      );
      const handlers = createPullToRefreshHandlers(onRefresh);

      handlers.ontouchstart(createTouchEvent(0));
      handlers.ontouchmove(createTouchEvent(500));
      const endPromise = handlers.ontouchend();

      // Move during refresh should be ignored
      const result = handlers.ontouchmove(createTouchEvent(300));
      expect(result).toBeUndefined();

      resolveRefresh!();
      await endPromise;
    });

    it('should not trigger end when not pulling', async () => {
      const onRefresh = vi.fn();
      const handlers = createPullToRefreshHandlers(onRefresh);

      await handlers.ontouchend();

      expect(onRefresh).not.toHaveBeenCalled();
    });

    it('getState returns current state', () => {
      const onRefresh = vi.fn();
      const handlers = createPullToRefreshHandlers(onRefresh);

      const state = handlers.getState();
      expect(state.isPulling).toBe(false);
      expect(state.isRefreshing).toBe(false);
    });

    it('should reset state after touch end', async () => {
      const onRefresh = vi.fn().mockResolvedValue(undefined);
      const handlers = createPullToRefreshHandlers(onRefresh);

      handlers.ontouchstart(createTouchEvent(0));
      handlers.ontouchmove(createTouchEvent(50)); // Below threshold

      await handlers.ontouchend();

      const state = handlers.getState();
      expect(state.isPulling).toBe(false);
    });

    it('should handle refresh errors gracefully (still resets isRefreshing)', async () => {
      const onRefresh = vi.fn().mockRejectedValue(new Error('refresh failed'));
      const handlers = createPullToRefreshHandlers(onRefresh);

      handlers.ontouchstart(createTouchEvent(0));
      handlers.ontouchmove(createTouchEvent(500));

      // The error propagates from handleTouchEnd but isRefreshing is reset via finally
      try {
        await handlers.ontouchend();
      } catch {
        // Expected error
      }

      expect(handlers.getState().isRefreshing).toBe(false);
    });
  });
});

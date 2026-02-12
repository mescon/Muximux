/**
 * Touch/pointer gesture detection utilities for mobile interactions.
 * Uses Pointer Events API for unified mouse/touch handling.
 */

export interface SwipeState {
  startX: number;
  startY: number;
  currentX: number;
  currentY: number;
  startTime: number;
  isSwiping: boolean;
}

export interface SwipeResult {
  direction: 'left' | 'right' | 'up' | 'down' | null;
  deltaX: number;
  deltaY: number;
  velocity: number;
  duration: number;
}

export interface SwipeConfig {
  /** Minimum distance (px) to trigger a swipe. Default: 50 */
  threshold?: number;
  /** Maximum time (ms) for a valid swipe. Default: 300 */
  maxDuration?: number;
  /** Minimum velocity (px/ms) to trigger swipe. Default: 0.3 */
  minVelocity?: number;
  /** Lock direction after initial movement. Default: true */
  lockAxis?: boolean;
}

const defaultConfig: Required<SwipeConfig> = {
  threshold: 50,
  maxDuration: 300,
  minVelocity: 0.3,
  lockAxis: true,
};

/**
 * Creates swipe detection handlers for an element.
 * Returns event handlers to attach to the element.
 */
export function createSwipeHandlers(
  onSwipe: (result: SwipeResult) => void,
  onSwipeMove?: (state: SwipeState) => void,
  config: SwipeConfig = {}
) {
  const cfg = { ...defaultConfig, ...config };
  let state: SwipeState | null = null;

  function handlePointerDown(e: PointerEvent) {
    // Only handle primary pointer (first finger or left mouse)
    if (!e.isPrimary) return;

    state = {
      startX: e.clientX,
      startY: e.clientY,
      currentX: e.clientX,
      currentY: e.clientY,
      startTime: Date.now(),
      isSwiping: false,
    };
  }

  function handlePointerMove(e: PointerEvent) {
    if (!state || !e.isPrimary) return;

    state.currentX = e.clientX;
    state.currentY = e.clientY;

    const deltaX = state.currentX - state.startX;
    const deltaY = state.currentY - state.startY;
    const absX = Math.abs(deltaX);
    const absY = Math.abs(deltaY);

    // Start swiping once threshold is partially met
    if (!state.isSwiping && (absX > 10 || absY > 10)) {
      state.isSwiping = true;
    }

    if (state.isSwiping && onSwipeMove) {
      onSwipeMove(state);
    }
  }

  function handlePointerUp(e: PointerEvent) {
    if (!state || !e.isPrimary) return;

    const deltaX = e.clientX - state.startX;
    const deltaY = e.clientY - state.startY;
    const absX = Math.abs(deltaX);
    const absY = Math.abs(deltaY);
    const duration = Date.now() - state.startTime;

    // Determine primary direction
    const isHorizontal = absX > absY;
    const distance = isHorizontal ? absX : absY;
    const velocity = distance / duration;

    let direction: SwipeResult['direction'] = null;

    // Check if swipe criteria are met
    if (
      distance >= cfg.threshold &&
      duration <= cfg.maxDuration &&
      velocity >= cfg.minVelocity
    ) {
      if (isHorizontal) {
        direction = deltaX > 0 ? 'right' : 'left';
      } else {
        direction = deltaY > 0 ? 'down' : 'up';
      }
    }

    onSwipe({
      direction,
      deltaX,
      deltaY,
      velocity,
      duration,
    });

    state = null;
  }

  function handlePointerCancel() {
    state = null;
  }

  return {
    onpointerdown: handlePointerDown,
    onpointermove: handlePointerMove,
    onpointerup: handlePointerUp,
    onpointercancel: handlePointerCancel,
  };
}

/**
 * Creates handlers for detecting pull-to-refresh gestures.
 * Only triggers when at the top of a scrollable element.
 */
export function createPullToRefreshHandlers(
  onRefresh: () => void | Promise<void>,
  config: { threshold?: number; resistance?: number } = {}
) {
  const threshold = config.threshold ?? 80;
  const resistance = config.resistance ?? 2.5;

  let startY = 0;
  let currentY = 0;
  let isPulling = false;
  let isRefreshing = false;

  function handleTouchStart(e: TouchEvent) {
    if (isRefreshing) return;

    // Check if at top of scroll
    const target = e.target as HTMLElement;
    const scrollTop = target.scrollTop ?? 0;

    if (scrollTop <= 0) {
      startY = e.touches[0].clientY;
      isPulling = true;
    }
  }

  function handleTouchMove(e: TouchEvent) {
    if (!isPulling || isRefreshing) return;

    currentY = e.touches[0].clientY;
    const pullDistance = (currentY - startY) / resistance;

    if (pullDistance > 0) {
      // Prevent default to stop scroll bounce on iOS
      e.preventDefault();

      // Return pull progress for visual feedback
      return {
        pullDistance,
        progress: Math.min(pullDistance / threshold, 1),
        isOverThreshold: pullDistance >= threshold,
      };
    }

    return null;
  }

  async function handleTouchEnd() {
    if (!isPulling || isRefreshing) return;

    const pullDistance = (currentY - startY) / resistance;

    if (pullDistance >= threshold) {
      isRefreshing = true;
      try {
        await onRefresh();
      } finally {
        isRefreshing = false;
      }
    }

    isPulling = false;
    startY = 0;
    currentY = 0;

    return { isRefreshing };
  }

  return {
    ontouchstart: handleTouchStart,
    ontouchmove: handleTouchMove,
    ontouchend: handleTouchEnd,
    getState: () => ({ isPulling, isRefreshing }),
  };
}

/**
 * Detects if the device supports touch.
 */
export function isTouchDevice(): boolean {
  return 'ontouchstart' in window || navigator.maxTouchPoints > 0;
}

/**
 * Detects if the device is likely a mobile device based on viewport.
 */
export function isMobileViewport(): boolean {
  return window.innerWidth < 640;
}

/**
 * Creates edge swipe detection for opening/closing sidebars.
 * Triggers when swipe starts from screen edge.
 */
export function createEdgeSwipeHandlers(
  edge: 'left' | 'right',
  onSwipeOpen: () => void,
  onSwipeClose: () => void,
  config: { edgeWidth?: number; threshold?: number } = {}
) {
  const edgeWidth = config.edgeWidth ?? 20;
  const threshold = config.threshold ?? 50;

  let startX = 0;
  let startY = 0;
  let isEdgeSwipe = false;

  function handlePointerDown(e: PointerEvent) {
    const x = e.clientX;
    const screenWidth = window.innerWidth;

    // Check if touch starts at edge
    if (
      (edge === 'left' && x <= edgeWidth) ||
      (edge === 'right' && x >= screenWidth - edgeWidth)
    ) {
      isEdgeSwipe = true;
      startX = x;
      startY = e.clientY;
    }
  }

  function handlePointerUp(e: PointerEvent) {
    if (!isEdgeSwipe) return;

    const deltaX = e.clientX - startX;
    const deltaY = e.clientY - startY;
    const absX = Math.abs(deltaX);
    const absY = Math.abs(deltaY);

    // Only handle horizontal swipes
    if (absX > absY && absX >= threshold) {
      if (edge === 'left') {
        if (deltaX > 0) onSwipeOpen();
        else onSwipeClose();
      } else if (deltaX < 0) {
        onSwipeOpen();
      } else {
        onSwipeClose();
      }
    }

    isEdgeSwipe = false;
  }

  function handlePointerCancel() {
    isEdgeSwipe = false;
  }

  return {
    onpointerdown: handlePointerDown,
    onpointerup: handlePointerUp,
    onpointercancel: handlePointerCancel,
  };
}

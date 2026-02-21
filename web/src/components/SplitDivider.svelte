<script lang="ts">
  let {
    orientation,
    onresize,
    ondblclick,
  }: {
    orientation: 'horizontal' | 'vertical';
    onresize: (position: number) => void;
    ondblclick: () => void;
  } = $props();

  let dividerRef = $state<HTMLElement | undefined>(undefined);
  let isDragging = $state(false);

  function handlePointerDown(e: PointerEvent) {
    e.preventDefault();
    isDragging = true;

    const target = e.currentTarget as HTMLElement;
    target.setPointerCapture(e.pointerId);

    // Disable pointer events on iframes during drag
    document.body.style.cursor = orientation === 'horizontal' ? 'col-resize' : 'row-resize';
    document.querySelectorAll('iframe').forEach(f => {
      (f as HTMLElement).style.pointerEvents = 'none';
    });
  }

  function handlePointerMove(e: PointerEvent) {
    if (!isDragging) return;

    const container = dividerRef?.parentElement;
    if (!container) return;

    const rect = container.getBoundingClientRect();
    let position: number;

    if (orientation === 'horizontal') {
      position = (e.clientX - rect.left) / rect.width;
    } else {
      position = (e.clientY - rect.top) / rect.height;
    }

    onresize(position);
  }

  function handlePointerUp() {
    isDragging = false;
    document.body.style.cursor = '';
    document.querySelectorAll('iframe').forEach(f => {
      (f as HTMLElement).style.pointerEvents = '';
    });
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  bind:this={dividerRef}
  class="split-divider {orientation === 'horizontal' ? 'split-divider-h' : 'split-divider-v'}"
  class:dragging={isDragging}
  onpointerdown={handlePointerDown}
  onpointermove={handlePointerMove}
  onpointerup={handlePointerUp}
  onpointercancel={handlePointerUp}
  ondblclick={ondblclick}
></div>

<style>
  .split-divider {
    background: var(--border);
    flex-shrink: 0;
    z-index: 10;
    transition: background 0.15s;
  }

  .split-divider:hover,
  .split-divider.dragging {
    background: var(--accent);
  }

  .split-divider-h {
    width: 4px;
    cursor: col-resize;
  }

  .split-divider-h:hover,
  .split-divider-h.dragging {
    width: 6px;
    margin-left: -1px;
    margin-right: -1px;
  }

  .split-divider-v {
    height: 4px;
    cursor: row-resize;
  }

  .split-divider-v:hover,
  .split-divider-v.dragging {
    height: 6px;
    margin-top: -1px;
    margin-bottom: -1px;
  }
</style>

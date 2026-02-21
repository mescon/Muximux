<script lang="ts">
  let {
    orientation,
    activePanel,
    onresize,
    ondblclick,
  }: {
    orientation: 'horizontal' | 'vertical';
    activePanel: 0 | 1;
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
  class="split-divider"
  class:split-divider-h={orientation === 'horizontal'}
  class:split-divider-v={orientation === 'vertical'}
  class:dragging={isDragging}
  class:active-0={activePanel === 0}
  class:active-1={activePanel === 1}
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
    position: relative;
  }

  .split-divider:hover,
  .split-divider.dragging {
    background: var(--accent);
  }

  /* Horizontal divider */
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

  /* Vertical divider */
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

  /* Active panel accent line — 2px accent on the active side */
  .split-divider-h.active-0 {
    border-left: 2px solid var(--accent-primary);
  }

  .split-divider-h.active-1 {
    border-right: 2px solid var(--accent-primary);
  }

  .split-divider-v.active-0 {
    border-top: 2px solid var(--accent-primary);
  }

  .split-divider-v.active-1 {
    border-bottom: 2px solid var(--accent-primary);
  }
</style>

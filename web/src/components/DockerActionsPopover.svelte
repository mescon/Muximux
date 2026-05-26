<script lang="ts">
  import type { DockerState } from '$lib/types';
  import * as m from '$lib/paraglide/messages.js';

  let { state, appName, onaction, onclose }: {
    state: DockerState;
    appName: string;
    onaction?: (action: 'start' | 'stop' | 'restart') => void;
    onclose?: () => void;
  } = $props();

  // Which actions make sense given the current state:
  //  - running       -> stop, restart (start is a no-op)
  //  - exited / dead -> start
  //  - paused        -> no graceful resume in v1
  //  - restarting / missing -> wait (no actions)
  let allowed = $derived.by((): readonly ('start' | 'stop' | 'restart')[] => {
    switch (state.status) {
      case 'running':
        return ['stop', 'restart'];
      case 'exited':
      case 'dead':
        return ['start'];
      default:
        return [];
    }
  });

  function fire(action: 'start' | 'stop' | 'restart') {
    onaction?.(action);
    onclose?.();
  }
</script>

<div class="docker-actions-popover" role="menu" aria-label="Docker actions for {appName}">
  <header class="docker-popover-header">
    {m.docker_popover_header_running()}
  </header>
  {#if allowed.includes('start')}
    <button class="docker-popover-action" type="button" onclick={() => fire('start')}>
      {m.docker_popover_action_start()}
    </button>
  {/if}
  {#if allowed.includes('stop')}
    <button class="docker-popover-action" type="button" onclick={() => fire('stop')}>
      {m.docker_popover_action_stop()}
    </button>
  {/if}
  {#if allowed.includes('restart')}
    <button class="docker-popover-action" type="button" onclick={() => fire('restart')}>
      {m.docker_popover_action_restart()}
    </button>
  {/if}
</div>

<style>
  .docker-actions-popover {
    position: absolute;
    z-index: 30;
    min-width: 12rem;
    padding: 0.5rem 0;
    background: var(--bg-elevated);
    border: 1px solid var(--border-default);
    border-radius: 0.5rem;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
  }
  .docker-popover-header {
    padding: 0.375rem 0.75rem;
    font-size: 0.6875rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
  }
  .docker-popover-action {
    display: block;
    width: 100%;
    padding: 0.5rem 0.75rem;
    text-align: start;
    background: transparent;
    border: 0;
    color: var(--text-primary);
    font-size: 0.875rem;
    cursor: pointer;
  }
  .docker-popover-action:hover {
    background: var(--bg-hover);
  }
</style>

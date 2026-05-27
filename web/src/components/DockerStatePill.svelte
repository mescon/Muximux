<script lang="ts">
  import type { DockerState } from '$lib/types';
  import * as m from '$lib/paraglide/messages.js';

  let { state, compact = false }: {
    state: DockerState;
    compact?: boolean;
  } = $props();

  // Quiet by default: a running, healthy container shows no dot at all.
  // The dot appears only to flag a state worth a glance -- down (red),
  // unhealthy/paused (amber), or restarting/starting (blue). The Docker
  // logo already signals "tracked container", so absence of a dot reads
  // as "running fine"; the eye is drawn only to what needs attention.
  let nominal = $derived(
    state.status === 'running' &&
    (state.health === 'healthy' || state.health === 'none' || !state.health)
  );

  let label = $derived.by(() => {
    if (state.status === 'running' && state.health === 'unhealthy') return m.docker_state_unhealthy();
    if (state.status === 'running' && state.health === 'starting') return m.docker_state_starting();
    if (state.status === 'running') return m.docker_popover_header_running();
    if (state.status === 'exited' || state.status === 'dead' || state.status === 'missing') return m.docker_state_stopped();
    if (state.status === 'paused') return m.docker_state_paused();
    if (state.status === 'restarting') return m.docker_state_restarting();
    return state.status;
  });

  let color = $derived.by(() => {
    if (state.status === 'exited' || state.status === 'dead' || state.status === 'missing') return 'var(--status-error, #ef4444)';
    if (state.status === 'paused' || state.health === 'unhealthy') return 'var(--status-warning, #f59e0b)';
    if (state.status === 'restarting' || state.health === 'starting') return 'var(--status-info, #3b82f6)';
    return 'var(--status-success, #22c55e)';
  });
</script>

{#if !nominal}
  <span
    class="docker-status-dot"
    class:compact
    style="background: {color};"
    title={label}
    aria-label={label}
    role="img"
  ></span>
{/if}

<style>
  .docker-status-dot {
    display: inline-block;
    width: 0.5rem;
    height: 0.5rem;
    border-radius: 999px;
    flex: none;
    /* A faint ring lifts the dot off both light and dark icons. */
    box-shadow: 0 0 0 1.5px var(--bg-surface, rgba(0, 0, 0, 0.25));
  }
  .docker-status-dot.compact {
    width: 0.4375rem;
    height: 0.4375rem;
    box-shadow: 0 0 0 1px var(--bg-surface, rgba(0, 0, 0, 0.25));
  }
</style>

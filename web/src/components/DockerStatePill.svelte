<script lang="ts">
  import type { DockerState } from '$lib/types';
  import * as m from '$lib/paraglide/messages.js';

  let { state, compact = false }: {
    state: DockerState;
    compact?: boolean;
  } = $props();

  // Render a label only when the state is something other than the
  // healthy-running default. This keeps the "nothing wrong" path
  // visually quiet -- the operator only sees noise when there's
  // something worth a glance.
  let label = $derived.by(() => {
    if (state.status === 'running' && state.health === 'healthy') return '';
    if (state.status === 'exited' || state.status === 'dead') return m.docker_state_stopped();
    if (state.status === 'paused') return m.docker_state_paused();
    if (state.status === 'restarting') return m.docker_state_restarting();
    if (state.status === 'running' && state.health === 'starting') return m.docker_state_starting();
    if (state.status === 'running' && state.health === 'unhealthy') return m.docker_state_unhealthy();
    if (state.status === 'missing') return m.docker_state_stopped();
    return '';
  });

  // State-driven background. Keep these as CSS custom property
  // references so themes can override.
  let bg = $derived.by(() => {
    if (state.status === 'exited' || state.status === 'dead' || state.status === 'missing') return 'var(--status-danger, #dc2626)';
    if (state.status === 'paused') return 'var(--status-warning, #d97706)';
    if (state.status === 'restarting') return 'var(--status-info, #2563eb)';
    if (state.health === 'unhealthy') return 'var(--status-danger, #dc2626)';
    if (state.health === 'starting') return 'var(--status-info, #2563eb)';
    return 'var(--bg-elevated)';
  });
</script>

{#if label}
  <span
    class="docker-state-pill"
    class:compact
    style="background: {bg};"
    title={label}
  >
    {label}
  </span>
{/if}

<style>
  .docker-state-pill {
    display: inline-block;
    padding: 0.125rem 0.5rem;
    border-radius: 999px;
    color: #fff;
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.025em;
    line-height: 1.2;
  }
  .docker-state-pill.compact {
    padding: 0 0.375rem;
    font-size: 0.625rem;
  }
</style>

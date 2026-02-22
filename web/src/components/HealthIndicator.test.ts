import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

const { mockHealthData, mockTriggerHealthCheck } = vi.hoisted(() => {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const { writable } = require('svelte/store');
  return {
    mockHealthData: writable(new Map()),
    mockTriggerHealthCheck: vi.fn(),
  };
});

vi.mock('$lib/healthStore', () => ({
  healthData: mockHealthData,
}));
vi.mock('$lib/api', () => ({
  triggerHealthCheck: (...args: unknown[]) => mockTriggerHealthCheck(...args),
}));

import HealthIndicator from './HealthIndicator.svelte';

function makeHealth(overrides: Record<string, unknown> = {}) {
  return {
    name: 'TestApp',
    status: 'healthy' as const,
    response_time_ms: 50,
    last_check: new Date().toISOString(),
    uptime_percent: 100,
    check_count: 10,
    success_count: 10,
    ...overrides,
  };
}

describe('HealthIndicator', () => {
  beforeEach(() => {
    mockHealthData.set(new Map());
    vi.clearAllMocks();
  });

  // ─── Existing tests (preserved) ───────────────────────────────────────────

  it('renders status dot', () => {
    const { container } = render(HealthIndicator, { props: { appName: 'TestApp' } });
    const dot = container.querySelector('span.rounded-full');
    expect(dot).toBeInTheDocument();
  });

  it('shows healthy class when health status is healthy', () => {
    mockHealthData.set(new Map([['TestApp', makeHealth()]]));

    const { container } = render(HealthIndicator, { props: { appName: 'TestApp' } });
    const dot = container.querySelector('span.rounded-full');
    expect(dot).toHaveClass('health-dot-healthy');
  });

  it('shows unhealthy class when health status is unhealthy', () => {
    mockHealthData.set(new Map([
      ['TestApp', makeHealth({
        status: 'unhealthy',
        response_time_ms: 0,
        last_error: 'Connection refused',
        uptime_percent: 50,
        success_count: 5,
      })],
    ]));

    const { container } = render(HealthIndicator, { props: { appName: 'TestApp' } });
    const dot = container.querySelector('span.rounded-full');
    expect(dot).toHaveClass('health-dot-unhealthy');
  });

  it('shows unknown class when no health data exists', () => {
    const { container } = render(HealthIndicator, { props: { appName: 'UnknownApp' } });
    const dot = container.querySelector('span.rounded-full');
    expect(dot).toHaveClass('health-dot-unknown');
  });

  it('applies correct size class for sm', () => {
    const { container } = render(HealthIndicator, { props: { appName: 'TestApp', size: 'sm' } });
    const dot = container.querySelector('span.rounded-full');
    expect(dot).toHaveClass('w-2', 'h-2');
  });

  it('applies correct size class for md', () => {
    const { container } = render(HealthIndicator, { props: { appName: 'TestApp', size: 'md' } });
    const dot = container.querySelector('span.rounded-full');
    expect(dot).toHaveClass('w-3', 'h-3');
  });

  it('applies correct size class for lg', () => {
    const { container } = render(HealthIndicator, { props: { appName: 'TestApp', size: 'lg' } });
    const dot = container.querySelector('span.rounded-full');
    expect(dot).toHaveClass('w-4', 'h-4');
  });

  // ─── Default size ─────────────────────────────────────────────────────────

  it('defaults to sm size when not specified', () => {
    const { container } = render(HealthIndicator, { props: { appName: 'TestApp' } });
    const dot = container.querySelector('span.rounded-full');
    expect(dot).toHaveClass('w-2', 'h-2');
  });

  // ─── Tooltip display ──────────────────────────────────────────────────────

  describe('tooltip', () => {
    it('does not show tooltip initially', () => {
      mockHealthData.set(new Map([['TestApp', makeHealth()]]));
      const { container } = render(HealthIndicator, { props: { appName: 'TestApp' } });

      expect(container.querySelector('.health-tooltip')).not.toBeInTheDocument();
    });

    it('shows tooltip on mouseenter when showTooltip is true and health data exists', async () => {
      mockHealthData.set(new Map([['TestApp', makeHealth()]]));

      const { container } = render(HealthIndicator, { props: { appName: 'TestApp', showTooltip: true } });

      const wrapper = container.querySelector('.inline-flex')!;

      // Mock getBoundingClientRect on the dot element
      const dot = container.querySelector('span.rounded-full')!;
      vi.spyOn(dot, 'getBoundingClientRect').mockReturnValue({
        left: 100,
        top: 200,
        width: 8,
        height: 8,
        right: 108,
        bottom: 208,
        x: 100,
        y: 200,
        toJSON: () => {},
      });

      await fireEvent.mouseEnter(wrapper);

      await waitFor(() => {
        expect(container.ownerDocument.querySelector('.health-tooltip')).toBeInTheDocument();
      });
    });

    it('hides tooltip on mouseleave', async () => {
      mockHealthData.set(new Map([['TestApp', makeHealth()]]));

      const { container } = render(HealthIndicator, { props: { appName: 'TestApp', showTooltip: true } });

      const wrapper = container.querySelector('.inline-flex')!;

      const dot = container.querySelector('span.rounded-full')!;
      vi.spyOn(dot, 'getBoundingClientRect').mockReturnValue({
        left: 100, top: 200, width: 8, height: 8,
        right: 108, bottom: 208, x: 100, y: 200, toJSON: () => {},
      });

      await fireEvent.mouseEnter(wrapper);
      await waitFor(() => {
        expect(container.ownerDocument.querySelector('.health-tooltip')).toBeInTheDocument();
      });

      await fireEvent.mouseLeave(wrapper);
      await waitFor(() => {
        expect(container.ownerDocument.querySelector('.health-tooltip')).not.toBeInTheDocument();
      });
    });

    it('does not show tooltip when showTooltip is false', async () => {
      mockHealthData.set(new Map([['TestApp', makeHealth()]]));

      const { container } = render(HealthIndicator, { props: { appName: 'TestApp', showTooltip: false } });

      const wrapper = container.querySelector('.inline-flex')!;
      await fireEvent.mouseEnter(wrapper);

      // No tooltip should appear
      expect(container.ownerDocument.querySelector('.health-tooltip')).not.toBeInTheDocument();
    });

    it('does not show tooltip when no health data exists', async () => {
      const { container } = render(HealthIndicator, { props: { appName: 'TestApp', showTooltip: true } });

      const wrapper = container.querySelector('.inline-flex')!;
      await fireEvent.mouseEnter(wrapper);

      expect(container.ownerDocument.querySelector('.health-tooltip')).not.toBeInTheDocument();
    });
  });

  // ─── Tooltip content ──────────────────────────────────────────────────────

  describe('tooltip content', () => {
    async function openTooltip(healthOverrides: Record<string, unknown> = {}) {
      const health = makeHealth(healthOverrides);
      mockHealthData.set(new Map([['TestApp', health]]));

      const { container } = render(HealthIndicator, { props: { appName: 'TestApp', showTooltip: true } });

      const wrapper = container.querySelector('.inline-flex')!;
      const dot = container.querySelector('span.rounded-full')!;
      vi.spyOn(dot, 'getBoundingClientRect').mockReturnValue({
        left: 100, top: 200, width: 8, height: 8,
        right: 108, bottom: 208, x: 100, y: 200, toJSON: () => {},
      });

      await fireEvent.mouseEnter(wrapper);
      return container;
    }

    it('shows "Healthy" status label for healthy status', async () => {
      const container = await openTooltip({ status: 'healthy' });

      await waitFor(() => {
        const tooltip = container.ownerDocument.querySelector('.health-tooltip');
        expect(tooltip).toBeInTheDocument();
        expect(tooltip!.textContent).toContain('Healthy');
      });
    });

    it('shows "Unhealthy" status label for unhealthy status', async () => {
      const container = await openTooltip({ status: 'unhealthy', response_time_ms: 0 });

      await waitFor(() => {
        const tooltip = container.ownerDocument.querySelector('.health-tooltip');
        expect(tooltip!.textContent).toContain('Unhealthy');
      });
    });

    it('shows response time when > 0', async () => {
      const container = await openTooltip({ response_time_ms: 250 });

      await waitFor(() => {
        const tooltip = container.ownerDocument.querySelector('.health-tooltip');
        expect(tooltip!.textContent).toContain('Response:');
        expect(tooltip!.textContent).toContain('250ms');
      });
    });

    it('formats response time in seconds when >= 1000ms', async () => {
      const container = await openTooltip({ response_time_ms: 2500 });

      await waitFor(() => {
        const tooltip = container.ownerDocument.querySelector('.health-tooltip');
        expect(tooltip!.textContent).toContain('2.5s');
      });
    });

    it('does not show response time row when response_time_ms is 0', async () => {
      const container = await openTooltip({ response_time_ms: 0 });

      await waitFor(() => {
        const tooltip = container.ownerDocument.querySelector('.health-tooltip');
        expect(tooltip).toBeInTheDocument();
        // Should not contain "Response:" text
        const detailRows = tooltip!.querySelectorAll('.health-detail-row');
        const responseRow = Array.from(detailRows).find(r => r.textContent?.includes('Response:'));
        expect(responseRow).toBeFalsy();
      });
    });

    it('shows uptime percentage badge when check_count > 0', async () => {
      const container = await openTooltip({ uptime_percent: 95, check_count: 20, success_count: 19 });

      await waitFor(() => {
        const badge = container.ownerDocument.querySelector('.health-uptime-badge');
        expect(badge).toBeInTheDocument();
        expect(badge!.textContent).toContain('95%');
      });
    });

    it('shows uptime check counts when check_count > 0', async () => {
      const container = await openTooltip({ check_count: 20, success_count: 18 });

      await waitFor(() => {
        const tooltip = container.ownerDocument.querySelector('.health-tooltip');
        expect(tooltip!.textContent).toContain('Uptime: 18/20 checks');
      });
    });

    it('does not show uptime badge/count when check_count is 0', async () => {
      const container = await openTooltip({ check_count: 0, success_count: 0 });

      await waitFor(() => {
        const badge = container.ownerDocument.querySelector('.health-uptime-badge');
        expect(badge).not.toBeInTheDocument();

        const tooltip = container.ownerDocument.querySelector('.health-tooltip');
        expect(tooltip!.textContent).not.toContain('Uptime:');
      });
    });

    it('shows last check time', async () => {
      const container = await openTooltip();

      await waitFor(() => {
        const tooltip = container.ownerDocument.querySelector('.health-tooltip');
        expect(tooltip!.textContent).toContain('Checked:');
      });
    });

    it('shows "Never" for last check when timestamp is empty', async () => {
      const container = await openTooltip({ last_check: '' });

      await waitFor(() => {
        const tooltip = container.ownerDocument.querySelector('.health-tooltip');
        expect(tooltip!.textContent).toContain('Never');
      });
    });

    it('shows error message when last_error is set', async () => {
      const container = await openTooltip({ last_error: 'Connection timeout', status: 'unhealthy' });

      await waitFor(() => {
        const errorEl = container.ownerDocument.querySelector('.health-error');
        expect(errorEl).toBeInTheDocument();
        expect(errorEl!.textContent).toContain('Connection timeout');
      });
    });

    it('does not show error when last_error is empty', async () => {
      const container = await openTooltip({ last_error: undefined });

      await waitFor(() => {
        const errorEl = container.ownerDocument.querySelector('.health-error');
        expect(errorEl).not.toBeInTheDocument();
      });
    });

    it('shows Check Now button in tooltip', async () => {
      const container = await openTooltip();

      await waitFor(() => {
        const btn = container.ownerDocument.querySelector('.health-check-btn');
        expect(btn).toBeInTheDocument();
        expect(btn!.textContent).toContain('Check Now');
      });
    });
  });

  // ─── Check Now button ─────────────────────────────────────────────────────

  describe('Check Now button', () => {
    it('calls triggerHealthCheck when clicked', async () => {
      const newHealth = makeHealth({ response_time_ms: 100 });
      mockTriggerHealthCheck.mockResolvedValueOnce(newHealth);
      mockHealthData.set(new Map([['TestApp', makeHealth()]]));

      const { container } = render(HealthIndicator, { props: { appName: 'TestApp', showTooltip: true } });

      const wrapper = container.querySelector('.inline-flex')!;
      const dot = container.querySelector('span.rounded-full')!;
      vi.spyOn(dot, 'getBoundingClientRect').mockReturnValue({
        left: 100, top: 200, width: 8, height: 8,
        right: 108, bottom: 208, x: 100, y: 200, toJSON: () => {},
      });

      await fireEvent.mouseEnter(wrapper);

      await waitFor(() => {
        expect(container.ownerDocument.querySelector('.health-check-btn')).toBeInTheDocument();
      });

      const checkBtn = container.ownerDocument.querySelector('.health-check-btn') as HTMLButtonElement;

      // Create a proper MouseEvent with stopPropagation
      await fireEvent.click(checkBtn);

      await waitFor(() => {
        expect(mockTriggerHealthCheck).toHaveBeenCalledWith('TestApp');
      });
    });

    it('shows "Checking..." while check is in progress', async () => {
      mockTriggerHealthCheck.mockImplementation(() => new Promise(() => {}));
      mockHealthData.set(new Map([['TestApp', makeHealth()]]));

      const { container } = render(HealthIndicator, { props: { appName: 'TestApp', showTooltip: true } });

      const wrapper = container.querySelector('.inline-flex')!;
      const dot = container.querySelector('span.rounded-full')!;
      vi.spyOn(dot, 'getBoundingClientRect').mockReturnValue({
        left: 100, top: 200, width: 8, height: 8,
        right: 108, bottom: 208, x: 100, y: 200, toJSON: () => {},
      });

      await fireEvent.mouseEnter(wrapper);

      await waitFor(() => {
        expect(container.ownerDocument.querySelector('.health-check-btn')).toBeInTheDocument();
      });

      const checkBtn = container.ownerDocument.querySelector('.health-check-btn') as HTMLButtonElement;
      await fireEvent.click(checkBtn);

      await waitFor(() => {
        expect(checkBtn.textContent).toContain('Checking...');
        expect(checkBtn).toBeDisabled();
      });
    });
  });

  // ─── formatResponseTime logic ─────────────────────────────────────────────

  describe('formatResponseTime (via tooltip content)', () => {
    async function getResponseText(ms: number): Promise<string> {
      const health = makeHealth({ response_time_ms: ms });
      mockHealthData.set(new Map([['TestApp', health]]));

      const { container } = render(HealthIndicator, { props: { appName: 'TestApp', showTooltip: true } });

      const wrapper = container.querySelector('.inline-flex')!;
      const dot = container.querySelector('span.rounded-full')!;
      vi.spyOn(dot, 'getBoundingClientRect').mockReturnValue({
        left: 100, top: 200, width: 8, height: 8,
        right: 108, bottom: 208, x: 100, y: 200, toJSON: () => {},
      });

      await fireEvent.mouseEnter(wrapper);

      let text = '';
      await waitFor(() => {
        const tooltip = container.ownerDocument.querySelector('.health-tooltip');
        text = tooltip!.textContent || '';
      });
      return text;
    }

    it('formats sub-second times as milliseconds', async () => {
      const text = await getResponseText(450);
      expect(text).toContain('450ms');
    });

    it('rounds sub-second times', async () => {
      const text = await getResponseText(123.7);
      expect(text).toContain('124ms');
    });

    it('formats 1000ms+ as seconds with one decimal', async () => {
      const text = await getResponseText(1500);
      expect(text).toContain('1.5s');
    });

    it('formats exactly 1000ms as 1.0s', async () => {
      const text = await getResponseText(1000);
      expect(text).toContain('1.0s');
    });
  });

  // ─── formatLastCheck logic ────────────────────────────────────────────────

  describe('formatLastCheck (via tooltip content)', () => {
    async function getCheckedText(timestamp: string): Promise<string> {
      const health = makeHealth({ last_check: timestamp });
      mockHealthData.set(new Map([['TestApp', health]]));

      const { container } = render(HealthIndicator, { props: { appName: 'TestApp', showTooltip: true } });

      const wrapper = container.querySelector('.inline-flex')!;
      const dot = container.querySelector('span.rounded-full')!;
      vi.spyOn(dot, 'getBoundingClientRect').mockReturnValue({
        left: 100, top: 200, width: 8, height: 8,
        right: 108, bottom: 208, x: 100, y: 200, toJSON: () => {},
      });

      await fireEvent.mouseEnter(wrapper);

      let text = '';
      await waitFor(() => {
        const tooltip = container.ownerDocument.querySelector('.health-tooltip');
        text = tooltip!.textContent || '';
      });
      return text;
    }

    it('shows "Never" for empty timestamp', async () => {
      const text = await getCheckedText('');
      expect(text).toContain('Never');
    });

    it('shows seconds ago for recent timestamps', async () => {
      const fiveSecsAgo = new Date(Date.now() - 5000).toISOString();
      const text = await getCheckedText(fiveSecsAgo);
      expect(text).toMatch(/\ds ago/);
    });

    it('shows minutes ago for timestamps minutes old', async () => {
      const fiveMinAgo = new Date(Date.now() - 300000).toISOString();
      const text = await getCheckedText(fiveMinAgo);
      expect(text).toMatch(/\dm ago/);
    });

    it('shows hours ago for timestamps hours old', async () => {
      const twoHoursAgo = new Date(Date.now() - 7200000).toISOString();
      const text = await getCheckedText(twoHoursAgo);
      expect(text).toMatch(/\dh ago/);
    });
  });

  // ─── Status class application ─────────────────────────────────────────────

  describe('getStatusClass', () => {
    it('uses health-dot-healthy for healthy status', () => {
      mockHealthData.set(new Map([['TestApp', makeHealth({ status: 'healthy' })]]));
      const { container } = render(HealthIndicator, { props: { appName: 'TestApp' } });
      expect(container.querySelector('.health-dot-healthy')).toBeInTheDocument();
    });

    it('uses health-dot-unhealthy for unhealthy status', () => {
      mockHealthData.set(new Map([['TestApp', makeHealth({ status: 'unhealthy' })]]));
      const { container } = render(HealthIndicator, { props: { appName: 'TestApp' } });
      expect(container.querySelector('.health-dot-unhealthy')).toBeInTheDocument();
    });

    it('uses health-dot-unknown for unknown status', () => {
      mockHealthData.set(new Map([['TestApp', makeHealth({ status: 'unknown' })]]));
      const { container } = render(HealthIndicator, { props: { appName: 'TestApp' } });
      expect(container.querySelector('.health-dot-unknown')).toBeInTheDocument();
    });

    it('uses health-dot-unknown when no health data present', () => {
      const { container } = render(HealthIndicator, { props: { appName: 'NoApp' } });
      expect(container.querySelector('.health-dot-unknown')).toBeInTheDocument();
    });
  });
});

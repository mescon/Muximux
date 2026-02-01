import { writable, derived } from 'svelte/store';
import type { AppHealth, HealthStatus } from './api';
import { fetchAllAppHealth } from './api';

// Store for all app health data
export const healthData = writable<Map<string, AppHealth>>(new Map());

// Loading state
export const healthLoading = writable<boolean>(false);

// Error state
export const healthError = writable<string | null>(null);

// Polling interval ID
let pollInterval: ReturnType<typeof setInterval> | null = null;

// Fetch health data for all apps
export async function refreshHealth(): Promise<void> {
  healthLoading.set(true);
  healthError.set(null);

  try {
    const healthList = await fetchAllAppHealth();
    const healthMap = new Map<string, AppHealth>();
    for (const health of healthList) {
      healthMap.set(health.name, health);
    }
    healthData.set(healthMap);
  } catch (e) {
    healthError.set(e instanceof Error ? e.message : 'Failed to fetch health data');
  } finally {
    healthLoading.set(false);
  }
}

// Start polling health data
export function startHealthPolling(intervalMs: number = 30000): void {
  // Clear existing interval if any
  stopHealthPolling();

  // Fetch immediately
  refreshHealth();

  // Start polling
  pollInterval = setInterval(refreshHealth, intervalMs);
}

// Stop polling
export function stopHealthPolling(): void {
  if (pollInterval) {
    clearInterval(pollInterval);
    pollInterval = null;
  }
}

// Get health status for a specific app
export function getAppHealthStatus(appName: string): HealthStatus {
  let status: HealthStatus = 'unknown';
  healthData.subscribe((data) => {
    const health = data.get(appName);
    if (health) {
      status = health.status;
    }
  })();
  return status;
}

// Derived store for getting health by app name
export function createAppHealthStore(appName: string) {
  return derived(healthData, ($healthData) => $healthData.get(appName) || null);
}

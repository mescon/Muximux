import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';
import {
  healthData,
  healthLoading,
  healthError,
  refreshHealth,
  startHealthPolling,
  stopHealthPolling,
  getAppHealthStatus,
  createAppHealthStore,
} from './healthStore';

// Mock the api module
vi.mock('./api', () => ({
  fetchAllAppHealth: vi.fn(),
}));

import { fetchAllAppHealth } from './api';

const mockFetchAllAppHealth = vi.mocked(fetchAllAppHealth);

describe('healthStore', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    // Reset stores
    healthData.set(new Map());
    healthLoading.set(false);
    healthError.set(null);
    stopHealthPolling();
  });

  afterEach(() => {
    vi.useRealTimers();
    stopHealthPolling();
  });

  describe('refreshHealth', () => {
    it('should set loading state during fetch', async () => {
      mockFetchAllAppHealth.mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve([]), 100))
      );

      const promise = refreshHealth();

      expect(get(healthLoading)).toBe(true);

      vi.advanceTimersByTime(100);
      await promise;

      expect(get(healthLoading)).toBe(false);
    });

    it('should populate healthData with results', async () => {
      mockFetchAllAppHealth.mockResolvedValue([
        { name: 'App1', status: 'healthy', response_time: 50 },
        { name: 'App2', status: 'unhealthy', response_time: 0 },
      ]);

      await refreshHealth();

      const data = get(healthData);
      expect(data.size).toBe(2);
      expect(data.get('App1')).toEqual({
        name: 'App1',
        status: 'healthy',
        response_time: 50,
      });
      expect(data.get('App2')?.status).toBe('unhealthy');
    });

    it('should handle fetch errors', async () => {
      mockFetchAllAppHealth.mockRejectedValue(new Error('Network error'));

      await refreshHealth();

      expect(get(healthError)).toBe('Network error');
      expect(get(healthLoading)).toBe(false);
    });

    it('should clear error on successful fetch', async () => {
      healthError.set('Previous error');

      mockFetchAllAppHealth.mockResolvedValue([]);

      await refreshHealth();

      expect(get(healthError)).toBeNull();
    });

    it('should handle non-Error exceptions', async () => {
      mockFetchAllAppHealth.mockRejectedValue('String error');

      await refreshHealth();

      expect(get(healthError)).toBe('Failed to fetch health data');
    });
  });

  describe('startHealthPolling', () => {
    it('should fetch immediately on start', async () => {
      mockFetchAllAppHealth.mockResolvedValue([]);

      startHealthPolling(5000);

      expect(mockFetchAllAppHealth).toHaveBeenCalledTimes(1);
    });

    it('should poll at specified interval', async () => {
      mockFetchAllAppHealth.mockResolvedValue([]);

      startHealthPolling(5000);

      // Initial fetch
      expect(mockFetchAllAppHealth).toHaveBeenCalledTimes(1);

      // Wait for first interval
      vi.advanceTimersByTime(5000);
      expect(mockFetchAllAppHealth).toHaveBeenCalledTimes(2);

      // Wait for second interval
      vi.advanceTimersByTime(5000);
      expect(mockFetchAllAppHealth).toHaveBeenCalledTimes(3);
    });

    it('should use default interval if not specified', async () => {
      mockFetchAllAppHealth.mockResolvedValue([]);

      startHealthPolling();

      expect(mockFetchAllAppHealth).toHaveBeenCalledTimes(1);

      // Default is 30000ms
      vi.advanceTimersByTime(30000);
      expect(mockFetchAllAppHealth).toHaveBeenCalledTimes(2);
    });

    it('should clear previous interval when starting new polling', async () => {
      mockFetchAllAppHealth.mockResolvedValue([]);

      startHealthPolling(1000);
      expect(mockFetchAllAppHealth).toHaveBeenCalledTimes(1);

      // Start new polling before first interval fires
      startHealthPolling(2000);
      expect(mockFetchAllAppHealth).toHaveBeenCalledTimes(2);

      // Advance past old interval - should not trigger extra call
      vi.advanceTimersByTime(1000);
      expect(mockFetchAllAppHealth).toHaveBeenCalledTimes(2);

      // New interval should trigger
      vi.advanceTimersByTime(1000);
      expect(mockFetchAllAppHealth).toHaveBeenCalledTimes(3);
    });
  });

  describe('stopHealthPolling', () => {
    it('should stop polling when called', async () => {
      mockFetchAllAppHealth.mockResolvedValue([]);

      startHealthPolling(1000);
      expect(mockFetchAllAppHealth).toHaveBeenCalledTimes(1);

      stopHealthPolling();

      // Advance time - should not trigger more calls
      vi.advanceTimersByTime(5000);
      expect(mockFetchAllAppHealth).toHaveBeenCalledTimes(1);
    });

    it('should be safe to call multiple times', () => {
      expect(() => {
        stopHealthPolling();
        stopHealthPolling();
        stopHealthPolling();
      }).not.toThrow();
    });
  });

  describe('getAppHealthStatus', () => {
    it('should return health status for known app', () => {
      const healthMap = new Map();
      healthMap.set('App1', { name: 'App1', status: 'healthy', response_time: 50 });
      healthData.set(healthMap);

      const status = getAppHealthStatus('App1');
      expect(status).toBe('healthy');
    });

    it('should return unknown for unknown app', () => {
      healthData.set(new Map());

      const status = getAppHealthStatus('UnknownApp');
      expect(status).toBe('unknown');
    });

    it('should return correct status for unhealthy app', () => {
      const healthMap = new Map();
      healthMap.set('BadApp', { name: 'BadApp', status: 'unhealthy', response_time: 0 });
      healthData.set(healthMap);

      const status = getAppHealthStatus('BadApp');
      expect(status).toBe('unhealthy');
    });
  });

  describe('createAppHealthStore', () => {
    it('should create a derived store for specific app', () => {
      const healthMap = new Map();
      healthMap.set('MyApp', { name: 'MyApp', status: 'healthy', response_time: 100 });
      healthData.set(healthMap);

      const myAppHealth = createAppHealthStore('MyApp');
      const health = get(myAppHealth);

      expect(health).toEqual({
        name: 'MyApp',
        status: 'healthy',
        response_time: 100,
      });
    });

    it('should return null for unknown app', () => {
      healthData.set(new Map());

      const unknownHealth = createAppHealthStore('Unknown');
      expect(get(unknownHealth)).toBeNull();
    });

    it('should update when healthData changes', () => {
      const myAppHealth = createAppHealthStore('DynamicApp');

      // Initially null
      expect(get(myAppHealth)).toBeNull();

      // Add app health data
      const healthMap = new Map();
      healthMap.set('DynamicApp', { name: 'DynamicApp', status: 'healthy', response_time: 25 });
      healthData.set(healthMap);

      // Should now have data
      expect(get(myAppHealth)?.status).toBe('healthy');

      // Update status
      healthMap.set('DynamicApp', { name: 'DynamicApp', status: 'unhealthy', response_time: 0 });
      healthData.set(healthMap);

      expect(get(myAppHealth)?.status).toBe('unhealthy');
    });
  });
});

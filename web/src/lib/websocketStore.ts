import { writable, get } from 'svelte/store';
import type { AppHealth } from './api';
import type { DockerState } from './types';
import { healthData } from './healthStore';
import { applyDockerStateChange } from './dockerStateStore';
import { debug } from './debug';

// Event types from server
export type EventType = 'config_updated' | 'health_changed' | 'app_health_changed' | 'log_entry' | 'docker_state_changed';

export interface WebSocketEvent {
  type: EventType;
  payload: unknown;
}

// Connection state
export type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'error';

// Stores
export const connectionState = writable<ConnectionState>('disconnected');
export const lastError = writable<string | null>(null);

// Event handlers
type EventHandler = (payload: unknown) => void;
const eventHandlers: Map<EventType, Set<EventHandler>> = new Map();

// WebSocket instance
let ws: WebSocket | null = null;
let reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
let reconnectAttempts = 0;
const maxReconnectAttempts = 10;
const baseReconnectDelay = 1000;

// Calculate reconnect delay with exponential backoff
function getReconnectDelay(): number {
  const delay = Math.min(baseReconnectDelay * Math.pow(2, reconnectAttempts), 30000);
  return delay + Math.random() * 1000; // Add jitter
}

// Connect to WebSocket server
export function connect(): void {
  if (ws && (ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN)) {
    return;
  }

  const protocol = globalThis.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = globalThis.location.host;
  const basePath = (globalThis as unknown as Record<string, string>).__MUXIMUX_BASE__ || '';
  const url = `${protocol}//${host}${basePath}/ws`;
  debug('ws', 'connecting', url);

  // Only set 'connecting' on first attempt, not during retries
  if (reconnectAttempts === 0) {
    connectionState.set('connecting');
  }
  lastError.set(null);

  try {
    ws = new WebSocket(url);

    ws.onopen = () => {
      connectionState.set('connected');
      debug('ws', 'connected');
      reconnectAttempts = 0;
    };

    ws.onclose = () => {
      ws = null;

      // Attempt to reconnect
      if (reconnectAttempts < maxReconnectAttempts) {
        // Only transition to 'disconnected' on the first close, not during retries
        if (reconnectAttempts === 0) {
          connectionState.set('disconnected');
        }
        const delay = getReconnectDelay();
        debug('ws', 'reconnecting', { attempt: reconnectAttempts + 1, delay: Math.round(delay) });
        reconnectTimeout = setTimeout(() => {
          reconnectAttempts++;
          connect();
        }, delay);
      } else {
        debug('ws', 'gave up after', maxReconnectAttempts, 'attempts');
        lastError.set('Failed to connect after multiple attempts');
        connectionState.set('error');
      }
    };

    ws.onerror = () => {
      debug('ws', 'connection error');
      lastError.set('WebSocket connection error');
    };

    ws.onmessage = (event) => {
      try {
        // Handle multiple messages (may be newline-separated)
        const messages = event.data.split('\n').filter((m: string) => m.trim());
        for (const msg of messages) {
          const parsed: WebSocketEvent = JSON.parse(msg);
          handleEvent(parsed);
        }
      } catch (e) {
        console.error('Error parsing WebSocket message', e);
      }
    };
  } catch {
    debug('ws', 'failed to create connection');
    lastError.set('Failed to create WebSocket connection');
    connectionState.set('error');
  }
}

// Disconnect from WebSocket server
export function disconnect(): void {
  if (reconnectTimeout) {
    clearTimeout(reconnectTimeout);
    reconnectTimeout = null;
  }
  reconnectAttempts = maxReconnectAttempts; // Prevent reconnection

  if (ws) {
    ws.close();
    ws = null;
  }
  connectionState.set('disconnected');
}

// Handle incoming events
function handleEvent(event: WebSocketEvent): void {
  debug('ws', 'event', event.type);
  // Built-in event handling
  switch (event.type) {
    case 'health_changed':
      // Update all health data
      if (Array.isArray(event.payload)) {
        const healthMap = new Map<string, AppHealth>();
        for (const health of event.payload as AppHealth[]) {
          healthMap.set(health.name, health);
        }
        healthData.set(healthMap);
      }
      break;

    case 'app_health_changed': {
      // Update single app health. Build a new Map rather than
      // mutating the existing one and returning the same reference -
      // returning the same Map silently breaks any downstream
      // consumer that memoises by reference equality (e.g. $derived
      // chains that compare the previous value). The bulk
      // health_changed handler above already does this; the two
      // should agree on the contract.
      const { app, health } = event.payload as { app: string; health: AppHealth };
      healthData.update((data) => {
        const next = new Map(data);
        next.set(app, health);
        return next;
      });
      break;
    }

    case 'docker_state_changed': {
      // Update the per-app docker-state map. applyDockerStateChange
      // builds a new Map (rather than mutating) so $derived chains that
      // memoise by reference re-run -- same contract as app_health_changed.
      const { app_name, state } = event.payload as { app_name: string; state: DockerState };
      applyDockerStateChange(app_name, state);
      break;
    }

    case 'config_updated':
      // Config updates are handled by registered handlers
      break;
  }

  // Notify registered handlers
  const handlers = eventHandlers.get(event.type);
  if (handlers) {
    for (const handler of handlers) {
      try {
        handler(event.payload);
      } catch (e) {
        console.error('Error in event handler', e);
      }
    }
  }
}

// Register an event handler
export function on(eventType: EventType, handler: EventHandler): () => void {
  if (!eventHandlers.has(eventType)) {
    eventHandlers.set(eventType, new Set());
  }
  eventHandlers.get(eventType)!.add(handler);

  // Return unsubscribe function
  return () => {
    eventHandlers.get(eventType)?.delete(handler);
  };
}

// Check if connected
export function isConnected(): boolean {
  return get(connectionState) === 'connected';
}

// Test-only export: lets unit tests drive handleEvent without a socket.
export const __test_handleEvent__ = handleEvent;

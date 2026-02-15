import { writable, get } from 'svelte/store';
import type { AppHealth } from './api';
import { healthData } from './healthStore';

// Event types from server
export type EventType = 'config_updated' | 'health_changed' | 'app_health_changed' | 'log_entry';

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

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  const url = `${protocol}//${host}/ws`;

  connectionState.set('connecting');
  lastError.set(null);

  try {
    ws = new WebSocket(url);

    ws.onopen = () => {
      console.log('WebSocket connected');
      connectionState.set('connected');
      reconnectAttempts = 0;
    };

    ws.onclose = (event) => {
      console.log('WebSocket disconnected', event.code, event.reason);
      connectionState.set('disconnected');
      ws = null;

      // Attempt to reconnect
      if (reconnectAttempts < maxReconnectAttempts) {
        const delay = getReconnectDelay();
        console.log(`Reconnecting in ${Math.round(delay / 1000)}s...`);
        reconnectTimeout = setTimeout(() => {
          reconnectAttempts++;
          connect();
        }, delay);
      } else {
        lastError.set('Failed to connect after multiple attempts');
        connectionState.set('error');
      }
    };

    ws.onerror = (event) => {
      console.error('WebSocket error', event);
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
  } catch (e) {
    console.error('Error creating WebSocket', e);
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
  // Skip verbose logging for high-frequency log_entry events
  if (event.type !== 'log_entry') {
    console.log('WebSocket event:', event.type, event.payload);
  }

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
      // Update single app health
      const { app, health } = event.payload as { app: string; health: AppHealth };
      healthData.update((data) => {
        data.set(app, health);
        return data;
      });
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

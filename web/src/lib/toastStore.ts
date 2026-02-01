/**
 * Toast Store - Manages toast notifications
 */

import { writable, derived } from 'svelte/store';

export type ToastType = 'success' | 'error' | 'warning' | 'info';

export interface Toast {
  id: string;
  type: ToastType;
  message: string;
  title?: string;
  duration: number; // ms, 0 = no auto-dismiss
  dismissible: boolean;
}

interface ToastOptions {
  title?: string;
  duration?: number;
  dismissible?: boolean;
}

const DEFAULT_DURATION = 4000;
const MAX_TOASTS = 5;

// Store for all toasts
const toastsStore = writable<Toast[]>([]);

// Generate unique IDs
let idCounter = 0;
function generateId(): string {
  return `toast-${++idCounter}-${Date.now()}`;
}

// Add a toast
function addToast(type: ToastType, message: string, options: ToastOptions = {}): string {
  const id = generateId();
  const toast: Toast = {
    id,
    type,
    message,
    title: options.title,
    duration: options.duration ?? DEFAULT_DURATION,
    dismissible: options.dismissible ?? true,
  };

  toastsStore.update(toasts => {
    // Remove oldest if at max capacity
    const updated = [...toasts, toast];
    if (updated.length > MAX_TOASTS) {
      return updated.slice(-MAX_TOASTS);
    }
    return updated;
  });

  // Auto-dismiss after duration
  if (toast.duration > 0) {
    setTimeout(() => {
      dismiss(id);
    }, toast.duration);
  }

  return id;
}

// Dismiss a toast by ID
function dismiss(id: string) {
  toastsStore.update(toasts => toasts.filter(t => t.id !== id));
}

// Clear all toasts
function clear() {
  toastsStore.set([]);
}

// Export the store and actions
export const toasts = {
  subscribe: toastsStore.subscribe,

  success(message: string, options?: ToastOptions) {
    return addToast('success', message, options);
  },

  error(message: string, options?: ToastOptions) {
    return addToast('error', message, { duration: 6000, ...options });
  },

  warning(message: string, options?: ToastOptions) {
    return addToast('warning', message, options);
  },

  info(message: string, options?: ToastOptions) {
    return addToast('info', message, options);
  },

  dismiss,
  clear,
};

// Convenience function for promise-based operations
export async function withToast<T>(
  promise: Promise<T>,
  options: {
    loading?: string;
    success?: string | ((result: T) => string);
    error?: string | ((err: Error) => string);
  }
): Promise<T> {
  let loadingId: string | undefined;

  if (options.loading) {
    loadingId = toasts.info(options.loading, { duration: 0, dismissible: false });
  }

  try {
    const result = await promise;
    if (loadingId) toasts.dismiss(loadingId);

    if (options.success) {
      const message = typeof options.success === 'function'
        ? options.success(result)
        : options.success;
      toasts.success(message);
    }

    return result;
  } catch (err) {
    if (loadingId) toasts.dismiss(loadingId);

    if (options.error) {
      const message = typeof options.error === 'function'
        ? options.error(err as Error)
        : options.error;
      toasts.error(message);
    }

    throw err;
  }
}

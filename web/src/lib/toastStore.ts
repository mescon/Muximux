/**
 * Toast Store - Thin wrapper around svelte-sonner
 * Keeps the existing toasts.success(msg) / toasts.error(msg) API
 */

import { toast } from 'svelte-sonner';

export type ToastType = 'success' | 'error' | 'warning' | 'info';

interface ToastOptions {
  title?: string;
  duration?: number;
  dismissible?: boolean;
}

export const toasts = {
  success(message: string, options?: ToastOptions) {
    return toast.success(message, {
      description: options?.title,
      duration: options?.duration ?? 4000,
    });
  },

  error(message: string, options?: ToastOptions) {
    return toast.error(message, {
      description: options?.title,
      duration: options?.duration ?? 6000,
    });
  },

  warning(message: string, options?: ToastOptions) {
    return toast.warning(message, {
      description: options?.title,
      duration: options?.duration ?? 4000,
    });
  },

  info(message: string, options?: ToastOptions) {
    return toast.info(message, {
      description: options?.title,
      duration: options?.duration ?? 4000,
    });
  },

  dismiss(id?: string | number) {
    toast.dismiss(id);
  },

  clear() {
    toast.dismiss();
  },
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
  return toast.promise(promise, {
    loading: options.loading ?? 'Loading...',
    success: (r: T) => typeof options.success === 'function' ? options.success(r) : options.success ?? 'Done',
    error: (e: Error) => typeof options.error === 'function' ? options.error(e) : options.error ?? 'Failed',
  });
}

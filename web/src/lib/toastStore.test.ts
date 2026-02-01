import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';
import { toasts, withToast, type Toast } from './toastStore';

describe('toastStore', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    toasts.clear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe('adding toasts', () => {
    it('should add a success toast', () => {
      toasts.success('Operation completed');
      const currentToasts = get(toasts);

      expect(currentToasts).toHaveLength(1);
      expect(currentToasts[0].type).toBe('success');
      expect(currentToasts[0].message).toBe('Operation completed');
      expect(currentToasts[0].dismissible).toBe(true);
    });

    it('should add an error toast with longer duration', () => {
      toasts.error('Something went wrong');
      const currentToasts = get(toasts);

      expect(currentToasts).toHaveLength(1);
      expect(currentToasts[0].type).toBe('error');
      expect(currentToasts[0].duration).toBe(6000);
    });

    it('should add a warning toast', () => {
      toasts.warning('Be careful');
      const currentToasts = get(toasts);

      expect(currentToasts).toHaveLength(1);
      expect(currentToasts[0].type).toBe('warning');
    });

    it('should add an info toast', () => {
      toasts.info('Just letting you know');
      const currentToasts = get(toasts);

      expect(currentToasts).toHaveLength(1);
      expect(currentToasts[0].type).toBe('info');
    });

    it('should support custom title', () => {
      toasts.success('Details here', { title: 'Success!' });
      const currentToasts = get(toasts);

      expect(currentToasts[0].title).toBe('Success!');
    });

    it('should support custom duration', () => {
      toasts.info('Quick message', { duration: 1000 });
      const currentToasts = get(toasts);

      expect(currentToasts[0].duration).toBe(1000);
    });

    it('should support non-dismissible toasts', () => {
      toasts.info('Loading...', { dismissible: false, duration: 0 });
      const currentToasts = get(toasts);

      expect(currentToasts[0].dismissible).toBe(false);
    });

    it('should return toast id', () => {
      const id = toasts.success('Test');

      expect(id).toMatch(/^toast-\d+-\d+$/);
    });
  });

  describe('dismissing toasts', () => {
    it('should dismiss toast by id', () => {
      const id = toasts.success('Will be dismissed');
      expect(get(toasts)).toHaveLength(1);

      toasts.dismiss(id);
      expect(get(toasts)).toHaveLength(0);
    });

    it('should auto-dismiss after duration', () => {
      toasts.success('Auto dismiss', { duration: 2000 });
      expect(get(toasts)).toHaveLength(1);

      vi.advanceTimersByTime(2000);
      expect(get(toasts)).toHaveLength(0);
    });

    it('should not auto-dismiss when duration is 0', () => {
      toasts.info('Permanent', { duration: 0 });
      expect(get(toasts)).toHaveLength(1);

      vi.advanceTimersByTime(10000);
      expect(get(toasts)).toHaveLength(1);
    });

    it('should handle dismissing non-existent id gracefully', () => {
      toasts.success('Test');
      toasts.dismiss('non-existent-id');

      expect(get(toasts)).toHaveLength(1);
    });
  });

  describe('clearing toasts', () => {
    it('should clear all toasts', () => {
      toasts.success('One');
      toasts.error('Two');
      toasts.warning('Three');
      expect(get(toasts)).toHaveLength(3);

      toasts.clear();
      expect(get(toasts)).toHaveLength(0);
    });
  });

  describe('max toasts limit', () => {
    it('should limit to 5 toasts', () => {
      for (let i = 0; i < 7; i++) {
        toasts.info(`Toast ${i}`, { duration: 0 });
      }

      const currentToasts = get(toasts);
      expect(currentToasts).toHaveLength(5);
      // Should keep the most recent ones
      expect(currentToasts[0].message).toBe('Toast 2');
      expect(currentToasts[4].message).toBe('Toast 6');
    });
  });

  describe('unique IDs', () => {
    it('should generate unique IDs for each toast', () => {
      const id1 = toasts.success('First');
      const id2 = toasts.success('Second');
      const id3 = toasts.success('Third');

      expect(id1).not.toBe(id2);
      expect(id2).not.toBe(id3);
      expect(id1).not.toBe(id3);
    });
  });
});

describe('withToast', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    toasts.clear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('should show loading toast while promise is pending', async () => {
    let resolvePromise: (value: string) => void;
    const promise = new Promise<string>((resolve) => {
      resolvePromise = resolve;
    });

    const resultPromise = withToast(promise, {
      loading: 'Loading...',
      success: 'Done!',
    });

    expect(get(toasts)).toHaveLength(1);
    expect(get(toasts)[0].message).toBe('Loading...');

    resolvePromise!('result');
    await resultPromise;

    // Loading toast should be dismissed, success shown
    const currentToasts = get(toasts);
    expect(currentToasts).toHaveLength(1);
    expect(currentToasts[0].type).toBe('success');
    expect(currentToasts[0].message).toBe('Done!');
  });

  it('should return the promise result', async () => {
    const promise = Promise.resolve({ data: 'test' });

    const result = await withToast(promise, {
      success: 'Success',
    });

    expect(result).toEqual({ data: 'test' });
  });

  it('should show error toast on failure', async () => {
    const promise = Promise.reject(new Error('Something broke'));

    await expect(
      withToast(promise, {
        loading: 'Working...',
        error: 'Operation failed',
      })
    ).rejects.toThrow('Something broke');

    const currentToasts = get(toasts);
    expect(currentToasts).toHaveLength(1);
    expect(currentToasts[0].type).toBe('error');
    expect(currentToasts[0].message).toBe('Operation failed');
  });

  it('should support function for success message', async () => {
    const promise = Promise.resolve({ count: 5 });

    await withToast(promise, {
      success: (result) => `Processed ${result.count} items`,
    });

    const currentToasts = get(toasts);
    expect(currentToasts[0].message).toBe('Processed 5 items');
  });

  it('should support function for error message', async () => {
    const promise = Promise.reject(new Error('Network timeout'));

    await expect(
      withToast(promise, {
        error: (err) => `Error: ${err.message}`,
      })
    ).rejects.toThrow();

    const currentToasts = get(toasts);
    expect(currentToasts[0].message).toBe('Error: Network timeout');
  });

  it('should work without loading toast', async () => {
    const promise = Promise.resolve('done');

    await withToast(promise, {
      success: 'Completed',
    });

    expect(get(toasts)).toHaveLength(1);
    expect(get(toasts)[0].type).toBe('success');
  });
});

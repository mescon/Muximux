import { describe, it, expect, vi, beforeEach } from 'vitest';

// Use vi.hoisted so the mock is available when vi.mock factory runs
const { mockToast } = vi.hoisted(() => {
  const mockToast = Object.assign(
    vi.fn(),
    {
      success: vi.fn().mockReturnValue(1),
      error: vi.fn().mockReturnValue(2),
      warning: vi.fn().mockReturnValue(3),
      info: vi.fn().mockReturnValue(4),
      dismiss: vi.fn(),
      promise: vi.fn(),
    }
  );
  return { mockToast };
});

vi.mock('svelte-sonner', () => ({
  toast: mockToast,
}));

import { toasts, withToast } from './toastStore';

describe('toastStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('adding toasts', () => {
    it('should call toast.success with message', () => {
      toasts.success('Operation completed');
      expect(mockToast.success).toHaveBeenCalledWith('Operation completed', {
        description: undefined,
        duration: 4000,
      });
    });

    it('should call toast.error with longer default duration', () => {
      toasts.error('Something went wrong');
      expect(mockToast.error).toHaveBeenCalledWith('Something went wrong', {
        description: undefined,
        duration: 6000,
      });
    });

    it('should call toast.warning', () => {
      toasts.warning('Be careful');
      expect(mockToast.warning).toHaveBeenCalledWith('Be careful', {
        description: undefined,
        duration: 4000,
      });
    });

    it('should call toast.info', () => {
      toasts.info('Just letting you know');
      expect(mockToast.info).toHaveBeenCalledWith('Just letting you know', {
        description: undefined,
        duration: 4000,
      });
    });

    it('should pass title as description', () => {
      toasts.success('Details here', { title: 'Success!' });
      expect(mockToast.success).toHaveBeenCalledWith('Details here', {
        description: 'Success!',
        duration: 4000,
      });
    });

    it('should pass custom duration', () => {
      toasts.info('Quick message', { duration: 1000 });
      expect(mockToast.info).toHaveBeenCalledWith('Quick message', {
        description: undefined,
        duration: 1000,
      });
    });

    it('should return toast id', () => {
      const id = toasts.success('Test');
      expect(id).toBe(1);
    });
  });

  describe('dismissing toasts', () => {
    it('should call toast.dismiss with id', () => {
      toasts.dismiss(42);
      expect(mockToast.dismiss).toHaveBeenCalledWith(42);
    });

    it('should call toast.dismiss without id', () => {
      toasts.dismiss();
      expect(mockToast.dismiss).toHaveBeenCalledWith(undefined);
    });
  });

  describe('clearing toasts', () => {
    it('should call toast.dismiss without arguments', () => {
      toasts.clear();
      expect(mockToast.dismiss).toHaveBeenCalledWith();
    });
  });
});

describe('withToast', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should call toast.promise with the promise', async () => {
    const promise = Promise.resolve('result');
    mockToast.promise.mockResolvedValue('result');

    await withToast(promise, {
      loading: 'Loading...',
      success: 'Done!',
      error: 'Failed',
    });

    expect(mockToast.promise).toHaveBeenCalledWith(promise, expect.objectContaining({
      loading: 'Loading...',
    }));
  });

  it('should handle function-based success message', async () => {
    const promise = Promise.resolve({ count: 5 });
    mockToast.promise.mockResolvedValue({ count: 5 });

    await withToast(promise, {
      success: (result) => `Processed ${result.count} items`,
    });

    const call = mockToast.promise.mock.calls[0];
    const successFn = call[1].success;
    expect(successFn({ count: 5 })).toBe('Processed 5 items');
  });

  it('should handle function-based error message', async () => {
    const error = new Error('Network timeout');
    const promise = Promise.reject(error);
    promise.catch(() => {}); // Prevent unhandled rejection
    mockToast.promise.mockRejectedValue(error);

    await expect(
      withToast(promise, {
        error: (err) => `Error: ${err.message}`,
      })
    ).rejects.toThrow();

    const call = mockToast.promise.mock.calls[0];
    const errorFn = call[1].error;
    expect(errorFn(new Error('Network timeout'))).toBe('Error: Network timeout');
  });

  it('should use defaults when options not provided', async () => {
    const promise = Promise.resolve('ok');
    mockToast.promise.mockResolvedValue('ok');

    await withToast(promise, {});

    const call = mockToast.promise.mock.calls[0];
    expect(call[1].loading).toBe('Loading...');
    expect(call[1].success('ok')).toBe('Done');
    expect(call[1].error(new Error('fail'))).toBe('Failed');
  });
});

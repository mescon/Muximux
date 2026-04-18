import { z } from 'zod';

export const appSchema = z.object({
  name: z.string().min(1, 'Name is required').max(100, 'Name must be under 100 characters'),
  url: z.string().min(1, 'URL is required').url('Must be a valid URL'),
});

export const groupSchema = z.object({
  name: z.string().min(1, 'Name is required').max(50, 'Name must be under 50 characters'),
});

/** Extract field errors from a Zod result into a flat Record */
export function extractErrors(result: z.ZodSafeParseResult<unknown>): Record<string, string> {
  if (result.success) return {};
  const errors: Record<string, string> = {};
  for (const issue of result.error.issues) {
    const field = issue.path[0]?.toString();
    if (field && !errors[field]) {
      errors[field] = issue.message;
    }
  }
  return errors;
}

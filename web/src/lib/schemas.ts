import { z } from 'zod';
import { isSafeAppUrl } from './appUrl';

export const appSchema = z.object({
  name: z.string().min(1, 'Name is required').max(100, 'Name must be under 100 characters'),
  // Matches AppFrame's iframe src allowlist and the backend
  // config.validateAppURL: an absolute http(s) URL, or a single-slash
  // same-origin path (e.g. a proxied app). Anything else is rejected.
  url: z.string().min(1, 'URL is required').refine(isSafeAppUrl, 'Must be an http(s) URL or a same-origin path'),
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

import { describe, it, expect } from 'vitest';
import type { z } from 'zod';
import { appSchema, groupSchema, extractErrors } from './schemas';

describe('schemas', () => {
  describe('appSchema', () => {
    it('validates a valid app', () => {
      const result = appSchema.safeParse({ name: 'MyApp', url: 'http://example.com' });
      expect(result.success).toBe(true);
    });

    it('rejects empty name', () => {
      const result = appSchema.safeParse({ name: '', url: 'http://example.com' });
      expect(result.success).toBe(false);
    });

    it('rejects name over 100 characters', () => {
      const result = appSchema.safeParse({ name: 'a'.repeat(101), url: 'http://example.com' });
      expect(result.success).toBe(false);
    });

    it('accepts name at exactly 100 characters', () => {
      const result = appSchema.safeParse({ name: 'a'.repeat(100), url: 'http://example.com' });
      expect(result.success).toBe(true);
    });

    it('rejects empty URL', () => {
      const result = appSchema.safeParse({ name: 'MyApp', url: '' });
      expect(result.success).toBe(false);
    });

    it('rejects invalid URL', () => {
      const result = appSchema.safeParse({ name: 'MyApp', url: 'not-a-url' });
      expect(result.success).toBe(false);
    });

    it('accepts valid https URL', () => {
      const result = appSchema.safeParse({ name: 'MyApp', url: 'https://example.com' });
      expect(result.success).toBe(true);
    });

    it('rejects missing fields', () => {
      const result = appSchema.safeParse({});
      expect(result.success).toBe(false);
    });
  });

  describe('groupSchema', () => {
    it('validates a valid group', () => {
      const result = groupSchema.safeParse({ name: 'MyGroup' });
      expect(result.success).toBe(true);
    });

    it('rejects empty name', () => {
      const result = groupSchema.safeParse({ name: '' });
      expect(result.success).toBe(false);
    });

    it('rejects name over 50 characters', () => {
      const result = groupSchema.safeParse({ name: 'a'.repeat(51) });
      expect(result.success).toBe(false);
    });

    it('accepts name at exactly 50 characters', () => {
      const result = groupSchema.safeParse({ name: 'a'.repeat(50) });
      expect(result.success).toBe(true);
    });
  });

  describe('extractErrors', () => {
    it('returns empty object for successful parse', () => {
      const result = appSchema.safeParse({ name: 'MyApp', url: 'http://example.com' });
      expect(extractErrors(result)).toEqual({});
    });

    it('returns field errors for failed parse', () => {
      const result = appSchema.safeParse({ name: '', url: '' });
      const errors = extractErrors(result);
      expect(errors.name).toBeDefined();
      expect(errors.url).toBeDefined();
    });

    it('returns first error message per field', () => {
      const result = appSchema.safeParse({ name: '', url: 'not-a-url' });
      const errors = extractErrors(result);
      expect(errors.name).toBe('Name is required');
      expect(errors.url).toBe('Must be a valid URL');
    });

    it('only includes the first error for each field', () => {
      // An empty string triggers both min(1) and url() checks for URL
      const result = appSchema.safeParse({ name: 'ok', url: '' });
      const errors = extractErrors(result);
      // Should only have one error for url
      expect(typeof errors.url).toBe('string');
    });

    it('handles no field path gracefully', () => {
      // Create a mock result with an issue that has an empty path
      const mockResult = {
        success: false as const,
        error: {
          issues: [
            { path: [], message: 'Root error', code: 'custom' as const },
          ],
        },
      };
      const errors = extractErrors(mockResult as z.SafeParseReturnType<unknown, unknown>);
      // Empty path should result in no field key being set
      expect(Object.keys(errors)).toHaveLength(0);
    });
  });
});

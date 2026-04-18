import { describe, it, expect } from 'vitest';
import { openModes, ALL_IFRAME_PERMISSIONS, resolvePermissions } from './constants';

describe('constants', () => {
  describe('resolvePermissions', () => {
    it('returns empty list for undefined', () => {
      expect(resolvePermissions(undefined)).toEqual([]);
    });

    it('returns empty list for empty array', () => {
      expect(resolvePermissions([])).toEqual([]);
    });

    it('expands "all" sentinel to the full permission list', () => {
      expect(resolvePermissions(['all'])).toEqual([...ALL_IFRAME_PERMISSIONS]);
    });

    it('treats "none" sentinel as empty list', () => {
      expect(resolvePermissions(['none'])).toEqual([]);
    });

    it('returns deny when both "all" and "none" are present', () => {
      expect(resolvePermissions(['all', 'none'])).toEqual([]);
    });

    it('passes known permissions through unchanged', () => {
      expect(resolvePermissions(['camera', 'microphone'])).toEqual(['camera', 'microphone']);
    });

    it('filters out unknown permission values', () => {
      expect(resolvePermissions(['camera', 'not-a-real-permission'])).toEqual(['camera']);
    });
  });

  describe('openModes', () => {
    it('should have exactly 4 entries', () => {
      expect(openModes).toHaveLength(4);
    });

    it('should have value, label, and description on each entry', () => {
      for (const mode of openModes) {
        expect(mode).toHaveProperty('value');
        expect(mode).toHaveProperty('label');
        expect(mode).toHaveProperty('description');
        expect(typeof mode.value).toBe('string');
        expect(typeof mode.label).toBe('string');
        expect(typeof mode.description).toBe('string');
      }
    });

    it('should contain all expected open mode values', () => {
      const values = openModes.map(m => m.value);
      expect(values).toContain('iframe');
      expect(values).toContain('new_tab');
      expect(values).toContain('new_window');
      expect(values).toContain('redirect');
    });

    it('should have non-empty labels and descriptions', () => {
      for (const mode of openModes) {
        expect(mode.label.length).toBeGreaterThan(0);
        expect(mode.description.length).toBeGreaterThan(0);
      }
    });
  });
});

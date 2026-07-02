import { describe, it, expect } from 'vitest';
import { renderChangelog } from './changelog';

describe('renderChangelog', () => {
  it('renders ordinary markdown to HTML', async () => {
    const html = await renderChangelog('# Release 1.0\n\n- Added a thing');
    expect(html).toContain('<h1');
    expect(html).toContain('Release 1.0');
    expect(html).toContain('<li>');
  });

  it('strips <script> tags from untrusted changelog HTML', async () => {
    const html = await renderChangelog('# Notes\n\n<script>alert(document.domain)</script>');
    expect(html).not.toContain('<script');
    expect(html).not.toContain('alert(document.domain)');
    // The legitimate heading still survives sanitization.
    expect(html).toContain('Notes');
  });

  it('strips event-handler attributes and javascript: URLs', async () => {
    const html = await renderChangelog('<img src=x onerror="alert(1)">\n\n[click](javascript:alert(1))');
    expect(html.toLowerCase()).not.toContain('onerror');
    expect(html.toLowerCase()).not.toContain('javascript:');
  });

  it('returns empty string for empty input', async () => {
    expect(await renderChangelog('')).toBe('');
  });
});

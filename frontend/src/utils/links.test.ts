import { describe, expect, it } from 'vitest';
import { normalizeOptionalExternalLink } from './links';

describe('normalizeOptionalExternalLink', () => {
  it('accepts blank input as no link', () => {
    expect(normalizeOptionalExternalLink('   ')).toBeNull();
  });

  it('accepts standard https links', () => {
    expect(normalizeOptionalExternalLink('https://example.com/docs')).toBe('https://example.com/docs');
  });

  it('rejects non-http protocols', () => {
    expect(normalizeOptionalExternalLink('javascript:alert(1)')).toBeNull();
  });

  it('rejects credential-bearing links', () => {
    expect(normalizeOptionalExternalLink('https://user:pass@example.com/docs')).toBeNull();
  });
});

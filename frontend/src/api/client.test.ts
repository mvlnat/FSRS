import { afterEach, describe, expect, it, vi } from 'vitest';
import { getDecks, logout } from './client';

describe('api client', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns undefined for successful empty responses', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response('', { status: 200, headers: { 'Content-Type': 'text/plain' } }),
    ));

    await expect(logout()).resolves.toBeUndefined();
  });

  it('parses successful JSON responses', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response('[]', { status: 200, headers: { 'Content-Type': 'application/json' } }),
    ));

    await expect(getDecks()).resolves.toEqual([]);
  });
});

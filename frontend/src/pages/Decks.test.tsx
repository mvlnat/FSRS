import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import type { Deck, DeckWithStats } from '../types';
import * as api from '../api/client';
import { Decks } from './Decks';

vi.mock('../api/client', () => ({
  getDecks: vi.fn(),
  getStudyStats: vi.fn(),
  getDueCalendar: vi.fn(),
  createDeck: vi.fn(),
  exportDeck: vi.fn(),
  importDeck: vi.fn(),
}));

const mockedApi = vi.mocked(api);

const zeroStats: api.StudyStats = {
  totalReviews: 0,
  reviewsLast24Hours: 0,
  reviewsLast7Days: 0,
  avgRating: 0,
  retentionRate: 0,
};

const createdDeck: Deck = {
  id: 'deck-1',
  user_id: 'user-1',
  name: 'Biology',
  description: 'Cells',
  created_at: '2026-04-14T00:00:00Z',
};

const deckWithStats: DeckWithStats = {
  ...createdDeck,
  stats: {
    total: 3,
    new: 2,
    due: 1,
    learning: 0,
  },
};

function mockDownloadApis() {
  const originalCreateObjectURL = URL.createObjectURL;
  const originalRevokeObjectURL = URL.revokeObjectURL;
  const createObjectURL = vi.fn<(blob: Blob | MediaSource) => string>();
  createObjectURL.mockImplementation(() => 'blob:deck-export');
  const revokeObjectURL = vi.fn<(url: string) => void>();
  revokeObjectURL.mockImplementation(() => undefined);
  const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);

  Object.defineProperty(URL, 'createObjectURL', {
    configurable: true,
    writable: true,
    value: createObjectURL,
  });
  Object.defineProperty(URL, 'revokeObjectURL', {
    configurable: true,
    writable: true,
    value: revokeObjectURL,
  });

  return {
    createObjectURL,
    revokeObjectURL,
    clickSpy,
    restore() {
      clickSpy.mockRestore();
      Object.defineProperty(URL, 'createObjectURL', {
        configurable: true,
        writable: true,
        value: originalCreateObjectURL,
      });
      Object.defineProperty(URL, 'revokeObjectURL', {
        configurable: true,
        writable: true,
        value: originalRevokeObjectURL,
      });
    },
  };
}

function renderDecks() {
  return render(
    <MemoryRouter>
      <Decks />
    </MemoryRouter>
  );
}

describe('Decks', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedApi.getStudyStats.mockResolvedValue(zeroStats);
    mockedApi.getDueCalendar.mockResolvedValue([]);
  });

  it('clears a stale load error after a successful create-triggered reload', async () => {
    const user = userEvent.setup();

    mockedApi.getDecks.mockRejectedValueOnce(new Error('Failed to load decks'));
    mockedApi.getDecks.mockResolvedValueOnce([deckWithStats]);
    mockedApi.createDeck.mockResolvedValue(createdDeck);

    renderDecks();

    await screen.findByText('Failed to load decks');
    await user.click(screen.getByRole('button', { name: 'New Deck' }));
    await user.type(screen.getByLabelText('Name'), 'Biology');
    await user.type(screen.getByLabelText('Description'), 'Cells');
    await user.click(screen.getByRole('button', { name: 'Create Deck' }));

    await waitFor(() => {
      expect(mockedApi.createDeck).toHaveBeenCalledWith('Biology', 'Cells');
    });
    await screen.findByText('Biology');
    expect(screen.queryByText('Failed to load decks')).not.toBeInTheDocument();
  });

  it('imports a deck file and reloads the deck list', async () => {
    const user = userEvent.setup();
    const importedDeck: DeckWithStats = {
      id: 'deck-2',
      user_id: 'user-1',
      name: 'Imported Deck',
      description: 'Imported cards',
      created_at: '2026-04-14T00:05:00Z',
      stats: {
        total: 1,
        new: 1,
        due: 1,
        learning: 0,
      },
    };
    const importPayload: api.DeckExport = {
      name: 'Imported Deck',
      description: 'Imported cards',
      cards: [
        {
          front: 'Imported question',
          back: 'Imported answer',
          link: 'https://example.com/imported',
        },
      ],
    };

    mockedApi.getDecks.mockResolvedValueOnce([]).mockResolvedValueOnce([importedDeck]);
    mockedApi.importDeck.mockResolvedValue(importedDeck);

    renderDecks();

    await screen.findByText('No decks yet. Create your first deck to get started!');

    const fileInput = document.querySelector('input[type="file"]');
    if (!(fileInput instanceof HTMLInputElement)) {
      throw new Error('expected hidden import file input');
    }

    const file = new File([JSON.stringify(importPayload)], 'imported-deck.json', {
      type: 'application/json',
    });

    await user.upload(fileInput, file);

    await waitFor(() => {
      expect(mockedApi.importDeck).toHaveBeenCalledWith(importPayload);
    });
    await screen.findByText('Imported Deck');
    expect(screen.getByText('Total: 1')).toBeInTheDocument();
    expect(fileInput.value).toBe('');
  });

  it('does not submit whitespace-only deck names', async () => {
    const user = userEvent.setup();

    mockedApi.getDecks.mockResolvedValue([]);

    renderDecks();

    await screen.findByText('No decks yet. Create your first deck to get started!');
    await user.click(screen.getByRole('button', { name: 'New Deck' }));
    await user.type(screen.getByLabelText('Name'), '   ');

    expect(screen.getByRole('button', { name: 'Create Deck' })).toBeDisabled();
    expect(mockedApi.createDeck).not.toHaveBeenCalled();
  });

  it('exports a deck as a downloadable json file', async () => {
    const user = userEvent.setup();
    const download = mockDownloadApis();
    const deckName = '../Biology:/Deck?';
    const exportPayload: api.DeckExport = {
      name: deckName,
      description: 'Cells',
      cards: [
        {
          front: 'What is a cell?',
          back: 'The basic unit of life.',
          link: 'https://example.com/cell',
        },
      ],
    };

    mockedApi.getDecks.mockResolvedValue([
      {
        ...deckWithStats,
        name: deckName,
      },
    ]);
    mockedApi.exportDeck.mockResolvedValue(exportPayload);

    try {
      renderDecks();

      await screen.findByText(deckName);
      await user.click(screen.getByRole('button', { name: 'Export' }));

      await waitFor(() => {
        expect(mockedApi.exportDeck).toHaveBeenCalledWith('deck-1');
      });
      expect(download.createObjectURL).toHaveBeenCalledTimes(1);

      const blob = download.createObjectURL.mock.calls[0]?.[0];
      if (!(blob instanceof Blob)) {
        throw new Error('expected export download blob');
      }
      expect(await blob.text()).toBe(JSON.stringify(exportPayload, null, 2));

      await waitFor(() => {
        expect(download.clickSpy).toHaveBeenCalledTimes(1);
      });
      const [anchor] = download.clickSpy.mock.instances as unknown as HTMLAnchorElement[];
      expect(anchor.download).toBe('Biology_Deck.json');
      expect(anchor.href).toBe('blob:deck-export');
      expect(download.revokeObjectURL).toHaveBeenCalledWith('blob:deck-export');
    } finally {
      download.restore();
    }
  });

  it('renders the due calendar with per-day deck details', async () => {
    const user = userEvent.setup();
    const today = new Date();
    const todayKey = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`;

    mockedApi.getDecks.mockResolvedValue([deckWithStats]);
    mockedApi.getDueCalendar.mockResolvedValue([
      {
        date: todayKey,
        total: 3,
        decks: [
          { deck_id: 'deck-1', deck_name: 'Biology', count: 2 },
          { deck_id: 'deck-2', deck_name: 'History', count: 1 },
        ],
      },
    ]);

    renderDecks();

    await screen.findByRole('heading', { name: 'Due Calendar' });
    await screen.findByText('3 cards due this month');

    await user.click(screen.getByRole('button', { name: `View due cards for ${todayKey}` }));

    expect(screen.getByRole('link', { name: 'Biology, 2 cards due' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'History, 1 card due' })).toBeInTheDocument();
    expect(screen.getByText('3 cards due', { selector: '.due-calendar-detail-total' })).toBeInTheDocument();
  });
});

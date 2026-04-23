import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import type { Card, CardWithState, Deck, Tag } from '../types';
import * as api from '../api/client';
import { DeckEdit } from './DeckEdit';

vi.mock('../api/client', () => ({
  getDeck: vi.fn(),
  getCards: vi.fn(),
  getTags: vi.fn(),
  updateDeck: vi.fn(),
  createCard: vi.fn(),
  updateCard: vi.fn(),
  createTag: vi.fn(),
  deleteTag: vi.fn(),
  deleteCard: vi.fn(),
  deleteDeck: vi.fn(),
}));

const mockedApi = vi.mocked(api);

const baseDeck: Deck = {
  id: 'deck-1',
  user_id: 'user-1',
  name: 'Biology',
  description: 'Cells',
  fuzz_enabled: false,
  created_at: '2026-04-14T00:00:00Z',
};

const initialCards: CardWithState[] = [
  {
    id: 'card-1',
    deck_id: 'deck-1',
    front: 'Existing question',
    back: 'Existing answer',
    link: '',
    created_at: '2026-04-14T00:00:00Z',
    tags: [],
    state: {
      id: 'state-1',
      card_id: 'card-1',
      due: '2026-04-15T00:00:00Z',
      stability: 0,
      difficulty: 0,
      elapsed_days: 0,
      scheduled_days: 0,
      reps: 0,
      lapses: 0,
      state: 0,
      last_review: null,
    },
  },
];

const noTags: Tag[] = [];
const biologyTags: Tag[] = [
  {
    id: 'tag-1',
    deck_id: 'deck-1',
    name: 'Cells',
    created_at: '2026-04-14T00:00:00Z',
  },
];

function renderDeckEdit() {
  return render(
    <MemoryRouter initialEntries={['/decks/deck-1']}>
      <Routes>
        <Route path="/decks/:id" element={<DeckEdit />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('DeckEdit', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('reloads the deck after saving deck settings', async () => {
    const updatedDeck: Deck = { ...baseDeck, name: 'Biology Updated', description: 'Updated cells', fuzz_enabled: true };
    const user = userEvent.setup();

    mockedApi.getDeck.mockResolvedValueOnce(baseDeck).mockResolvedValueOnce(updatedDeck);
    mockedApi.getCards.mockResolvedValue(initialCards);
    mockedApi.getTags.mockResolvedValue(noTags);
    mockedApi.updateDeck.mockResolvedValue(updatedDeck);

    renderDeckEdit();

    await screen.findByRole('heading', { name: 'Biology' });
    await user.click(screen.getByRole('tab', { name: 'Settings' }));

    const nameInput = await screen.findByLabelText('Deck Name');
    await user.clear(nameInput);
    await user.type(nameInput, 'Biology Updated');

    const descriptionInput = screen.getByLabelText('Description');
    await user.clear(descriptionInput);
    await user.type(descriptionInput, 'Updated cells');
    await user.click(screen.getByLabelText(/Enable Fuzz For Long-Term Reviews/));

    await user.click(screen.getByRole('button', { name: 'Save Changes' }));

    await waitFor(() => {
      expect(mockedApi.updateDeck).toHaveBeenCalledWith('deck-1', 'Biology Updated', 'Updated cells', true);
    });
    await waitFor(() => {
      expect(mockedApi.getDeck).toHaveBeenCalledTimes(2);
    });
    await screen.findByRole('heading', { name: 'Biology Updated' });
  });

  it('reloads cards after adding a new card', async () => {
    const newCard: Card = {
      id: 'card-2',
      deck_id: 'deck-1',
      front: 'Added question',
      back: 'Added answer',
      link: '',
      created_at: '2026-04-14T00:05:00Z',
    };
    const updatedCards: CardWithState[] = [
      ...initialCards,
      {
        ...newCard,
        tags: [],
      },
    ];
    const user = userEvent.setup();

    mockedApi.getDeck.mockResolvedValue(baseDeck);
    mockedApi.getCards.mockResolvedValueOnce(initialCards).mockResolvedValueOnce(updatedCards);
    mockedApi.getTags.mockResolvedValue(noTags);
    mockedApi.createCard.mockResolvedValue(newCard);

    renderDeckEdit();

    await screen.findByRole('tab', { name: 'Cards (1)' });
    await user.click(screen.getByRole('button', { name: 'Add Card' }));

    await user.type(screen.getByLabelText('Front'), 'Added question');
    await user.type(screen.getByLabelText('Back'), 'Added answer');
    await user.click(screen.getByRole('button', { name: 'Add Card' }));

    await waitFor(() => {
      expect(mockedApi.createCard).toHaveBeenCalledWith('deck-1', 'Added question', 'Added answer', '');
    });
    await waitFor(() => {
      expect(mockedApi.getCards).toHaveBeenCalledTimes(2);
    });
    await screen.findByRole('tab', { name: 'Cards (2)' });
    expect(screen.getByText('Added question')).toBeInTheDocument();
  });

  it('shows a duplicate-title warning while adding a card', async () => {
    const user = userEvent.setup();

    mockedApi.getDeck.mockResolvedValue(baseDeck);
    mockedApi.getCards.mockResolvedValue(initialCards);
    mockedApi.getTags.mockResolvedValue(noTags);

    renderDeckEdit();

    await screen.findByRole('tab', { name: 'Cards (1)' });
    await user.click(screen.getByRole('button', { name: 'Add Card' }));
    await user.type(screen.getByLabelText('Front'), 'Existing question\nwith extra detail');

    const warning = await screen.findByRole('status');
    expect(within(warning).getByText('Possible duplicate in this deck')).toBeInTheDocument();
    expect(within(warning).getByText(/Existing answer/)).toBeInTheDocument();
  });

  it('saves edited tags through the card update request', async () => {
    const user = userEvent.setup();

    mockedApi.getDeck.mockResolvedValue(baseDeck);
    mockedApi.getCards.mockResolvedValue(initialCards);
    mockedApi.getTags.mockResolvedValue(biologyTags);
    mockedApi.updateCard.mockResolvedValue(initialCards[0]);

    renderDeckEdit();

    await screen.findByText('Existing question');
    await user.click(screen.getByText('Existing question'));
    await user.click(screen.getByRole('button', { name: 'Cells' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => {
      expect(mockedApi.updateCard).toHaveBeenCalledWith(
        'card-1',
        'Existing question',
        'Existing answer',
        '',
        ['tag-1'],
      );
    });
  });

  it('shows a duplicate-title warning while editing a card', async () => {
    const user = userEvent.setup();
    const cardsWithPotentialDuplicate: CardWithState[] = [
      initialCards[0],
      {
        id: 'card-2',
        deck_id: 'deck-1',
        front: 'Different question',
        back: 'Second answer',
        link: '',
        created_at: '2026-04-14T00:01:00Z',
        tags: [],
      },
    ];

    mockedApi.getDeck.mockResolvedValue(baseDeck);
    mockedApi.getCards.mockResolvedValue(cardsWithPotentialDuplicate);
    mockedApi.getTags.mockResolvedValue(noTags);

    renderDeckEdit();

    await screen.findByText('Different question');
    await user.click(screen.getByText('Different question'));

    const editForm = document.querySelector('.card-edit');
    expect(editForm).not.toBeNull();
    const frontInput = within(editForm as HTMLElement).getAllByRole('textbox')[0];

    await user.clear(frontInput);
    await user.type(frontInput, 'Existing question');

    const warning = await screen.findByRole('status');
    expect(within(warning).getByText('Possible duplicate in this deck')).toBeInTheDocument();
    expect(within(warning).getByText(/Existing answer/)).toBeInTheDocument();
  });

  it('sorts alphabetically by the visible first-line title', async () => {
    const user = userEvent.setup();
    const titleSortedCards: CardWithState[] = [
      {
        id: 'card-1',
        deck_id: 'deck-1',
        front: '**Beta** topic',
        back: 'Beta answer',
        link: '',
        created_at: '2026-04-14T00:00:00Z',
        tags: [],
      },
      {
        id: 'card-2',
        deck_id: 'deck-1',
        front: 'Alpha topic',
        back: 'Alpha answer',
        link: '',
        created_at: '2026-04-14T00:01:00Z',
        tags: [],
      },
    ];

    mockedApi.getDeck.mockResolvedValue(baseDeck);
    mockedApi.getCards.mockResolvedValue(titleSortedCards);
    mockedApi.getTags.mockResolvedValue(noTags);

    const { container } = renderDeckEdit();

    await screen.findByText('Beta topic');
    await user.selectOptions(screen.getByLabelText('Sort cards'), 'alpha');

    const titles = Array.from(container.querySelectorAll('.card-preview-text')).map((node) =>
      node.textContent?.trim(),
    );
    expect(titles).toEqual(['Alpha topic', 'Beta topic']);
  });

  it('exposes card editor controls with accessible names', async () => {
    const linkedCards: CardWithState[] = [
      {
        ...initialCards[0],
        link: 'https://example.com/card',
        tags: biologyTags,
      },
    ];

    mockedApi.getDeck.mockResolvedValue(baseDeck);
    mockedApi.getCards.mockResolvedValue(linkedCards);
    mockedApi.getTags.mockResolvedValue(biologyTags);

    renderDeckEdit();

    await screen.findByRole('button', { name: /Existing question.*Cells.*New/i });
    expect(screen.getByLabelText('Search cards')).toBeInTheDocument();
    expect(screen.getByLabelText('Sort cards')).toBeInTheDocument();
    expect(screen.getByLabelText('Filter cards by tag')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open link for Existing question' })).toHaveAttribute(
      'href',
      'https://example.com/card',
    );

    await userEvent.setup().click(screen.getByRole('tab', { name: 'Settings' }));
    expect(screen.getByRole('button', { name: 'Delete tag Cells' })).toBeInTheDocument();
  });

  it('supports keyboard navigation between editor tabs', async () => {
    const user = userEvent.setup();

    mockedApi.getDeck.mockResolvedValue(baseDeck);
    mockedApi.getCards.mockResolvedValue(initialCards);
    mockedApi.getTags.mockResolvedValue(noTags);

    renderDeckEdit();

    const cardsTab = await screen.findByRole('tab', { name: 'Cards (1)' });
    const settingsTab = screen.getByRole('tab', { name: 'Settings' });

    cardsTab.focus();
    await user.keyboard('{ArrowRight}');

    expect(settingsTab).toHaveFocus();
    expect(settingsTab).toHaveAttribute('aria-selected', 'true');
    expect(settingsTab).toHaveAttribute('tabindex', '0');
    expect(cardsTab).toHaveAttribute('tabindex', '-1');
    expect(screen.getByLabelText('Deck Name')).toBeInTheDocument();
  });

  it('clears a stale error after a successful deck reload', async () => {
    const updatedDeck: Deck = { ...baseDeck, name: 'Recovered Deck', description: 'Recovered description' };
    const user = userEvent.setup();

    mockedApi.getDeck.mockResolvedValueOnce(baseDeck).mockResolvedValueOnce(updatedDeck);
    mockedApi.getCards.mockResolvedValue(initialCards);
    mockedApi.getTags.mockResolvedValue(noTags);
    mockedApi.updateDeck
      .mockRejectedValueOnce(new Error('Failed to update deck'))
      .mockResolvedValueOnce(updatedDeck);

    renderDeckEdit();

    await screen.findByRole('heading', { name: 'Biology' });
    await user.click(screen.getByRole('tab', { name: 'Settings' }));

    const nameInput = await screen.findByLabelText('Deck Name');
    await user.clear(nameInput);
    await user.type(nameInput, 'Recovered Deck');

    const descriptionInput = screen.getByLabelText('Description');
    await user.clear(descriptionInput);
    await user.type(descriptionInput, 'Recovered description');

    await user.click(screen.getByRole('button', { name: 'Save Changes' }));
    await screen.findByText('Failed to update deck');

    await user.click(screen.getByRole('button', { name: 'Save Changes' }));

    await screen.findByRole('heading', { name: 'Recovered Deck' });
    expect(screen.queryByText('Failed to update deck')).not.toBeInTheDocument();
  });

  it('shows the load error instead of a false not-found state when startup requests fail', async () => {
    mockedApi.getDeck.mockRejectedValue(new Error('backend unavailable'));
    mockedApi.getCards.mockResolvedValue(initialCards);
    mockedApi.getTags.mockResolvedValue(noTags);

    renderDeckEdit();

    await screen.findByRole('heading', { name: 'Unable to Load Deck' });
    expect(screen.getByText('backend unavailable')).toBeInTheDocument();
    expect(screen.queryByText('Deck not found')).not.toBeInTheDocument();
  });
});

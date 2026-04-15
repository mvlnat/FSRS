import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import type { CardState, CardWithState } from '../types';
import * as api from '../api/client';
import { Study } from './Study';

vi.mock('../api/client', () => ({
  getDueCards: vi.fn(),
  reviewCard: vi.fn(),
}));

const mockedApi = vi.mocked(api);

const studyCard: CardWithState = {
  id: 'card-1',
  deck_id: 'deck-1',
  front: 'What is FSRS?',
  back: 'A spaced repetition scheduler.',
  link: '',
  created_at: '2026-04-14T00:00:00Z',
  tags: [],
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function renderStudy() {
  return render(
    <MemoryRouter initialEntries={['/study/deck-1']}>
      <Routes>
        <Route path="/study/:deckId" element={<Study />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('Study', () => {
  it('submits a revealed card only once while the review request is in flight', async () => {
    const pendingReview = deferred<CardState>();
    const user = userEvent.setup();

    mockedApi.getDueCards.mockResolvedValueOnce([studyCard]).mockResolvedValueOnce([]);
    mockedApi.reviewCard.mockReturnValueOnce(pendingReview.promise);

    renderStudy();

    await screen.findByText('What is FSRS?');
    await user.click(screen.getByRole('button', { name: /show answer/i }));

    const goodButton = screen.getByRole('button', { name: /good/i });
    await user.click(goodButton);

    await waitFor(() => {
      expect(mockedApi.reviewCard).toHaveBeenCalledTimes(1);
    });
    expect(goodButton).toBeDisabled();

    await user.click(goodButton);
    expect(mockedApi.reviewCard).toHaveBeenCalledTimes(1);

    pendingReview.resolve({
      id: 'state-1',
      card_id: 'card-1',
      due: '2026-04-15T00:00:00Z',
      stability: 1,
      difficulty: 1,
      elapsed_days: 0,
      scheduled_days: 1,
      reps: 1,
      lapses: 0,
      state: 2,
      last_review: '2026-04-14T00:00:00Z',
    });

    await screen.findByText('Session Complete!');
  });
});

import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import type { CardState, CardWithState } from '../types';
import * as api from '../api/client';
import { Study } from './Study';

vi.mock('../api/client', () => ({
  getStudySession: vi.fn(),
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

const followUpCard: CardWithState = {
  id: 'card-2',
  deck_id: 'deck-1',
  front: 'What is spaced repetition?',
  back: 'Timed recall for memory retention.',
  link: '',
  created_at: '2026-04-14T00:01:00Z',
  tags: [],
};

const trailingCard: CardWithState = {
  id: 'card-3',
  deck_id: 'deck-1',
  front: 'Trailing card question',
  back: 'Trailing card answer',
  link: '',
  created_at: '2026-04-14T00:02:00Z',
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

    mockedApi.getStudySession
      .mockResolvedValueOnce({
        due_cards: [studyCard],
        pending_learning_cards: [],
      })
      .mockResolvedValueOnce({
        due_cards: [],
        pending_learning_cards: [],
      });
    mockedApi.reviewCard.mockReturnValueOnce(pendingReview.promise);

    renderStudy();

    await screen.findByText('What is FSRS?');
    await userEvent.setup().click(screen.getByRole('button', { name: /show answer/i }));

    const goodButton = screen.getByRole('button', { name: /good/i });
    fireEvent.click(goodButton);
    fireEvent.click(goodButton);

    await waitFor(() => {
      expect(mockedApi.reviewCard).toHaveBeenCalledTimes(1);
    });
    await screen.findByRole('heading', { name: 'Saving Answers' });
    expect(screen.queryByText('Session Complete!')).not.toBeInTheDocument();

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

  it('shows a retry state instead of a false completion message when the due-card load fails', async () => {
    mockedApi.getStudySession.mockRejectedValueOnce(new Error('backend unavailable'));

    renderStudy();

    await screen.findByRole('heading', { name: 'Unable to Load Session' });
    expect(screen.getByText('backend unavailable')).toBeInTheDocument();
    expect(screen.queryByText('Session Complete!')).not.toBeInTheDocument();
  });

  it('does not render unsafe card links', async () => {
    const user = userEvent.setup();

    mockedApi.getStudySession.mockResolvedValueOnce({
      due_cards: [
        { ...studyCard, link: 'javascript:alert(1)' },
      ],
      pending_learning_cards: [],
    });

    renderStudy();

    await screen.findByText('What is FSRS?');
    await user.click(screen.getByRole('button', { name: /show answer/i }));

    expect(screen.queryByRole('link', { name: 'Open Link' })).not.toBeInTheDocument();
  });

  it('preserves cards that become due while another review is still submitting', async () => {
    const pendingSecondReview = deferred<CardState>();

    mockedApi.getStudySession.mockResolvedValueOnce({
      due_cards: [studyCard, followUpCard, trailingCard],
      pending_learning_cards: [],
    });
    mockedApi.reviewCard
      .mockResolvedValueOnce({
        id: 'state-1',
        card_id: studyCard.id,
        due: new Date(Date.now() + 1000).toISOString(),
        stability: 1,
        difficulty: 1,
        elapsed_days: 0,
        scheduled_days: 0,
        reps: 1,
        lapses: 0,
        state: 1,
        last_review: new Date().toISOString(),
      })
      .mockReturnValueOnce(pendingSecondReview.promise);

    renderStudy();

    await screen.findByText('What is FSRS?');
    fireEvent.click(screen.getByRole('button', { name: /show answer/i }));
    fireEvent.click(screen.getByRole('button', { name: /good/i }));
    await waitFor(() => {
      expect(mockedApi.reviewCard).toHaveBeenCalledWith(studyCard.id, 3);
    });
    expect(screen.getByText('What is spaced repetition?')).toBeInTheDocument();
    expect(screen.getByText('Due Now: 2')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /show answer/i }));
    fireEvent.click(screen.getByRole('button', { name: /good/i }));
    await waitFor(() => {
      expect(mockedApi.reviewCard).toHaveBeenCalledWith(followUpCard.id, 3);
    });
    expect(screen.getByText('Due Now: 1')).toBeInTheDocument();
    expect(screen.getByText('Saving: 1')).toBeInTheDocument();

    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 1100));
    });

    await waitFor(() => {
      expect(screen.getByText('Due Now: 2')).toBeInTheDocument();
      expect(screen.getByText('Saving: 1')).toBeInTheDocument();
    });

    await act(async () => {
      pendingSecondReview.resolve({
        id: 'state-2',
        card_id: followUpCard.id,
        due: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
        stability: 2,
        difficulty: 1,
        elapsed_days: 1,
        scheduled_days: 1,
        reps: 1,
        lapses: 0,
        state: 2,
        last_review: new Date().toISOString(),
      });
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(screen.getByText('Due Now: 2')).toBeInTheDocument();
      expect(screen.getByText('Saving: 0')).toBeInTheDocument();
    });
  });

  it('restores pending learning cards after a reload and returns them when due', async () => {
    mockedApi.getStudySession.mockResolvedValueOnce({
      due_cards: [],
      pending_learning_cards: [{
        ...studyCard,
        state: {
          id: 'state-1',
          card_id: studyCard.id,
          due: new Date(Date.now() + 1000).toISOString(),
          stability: 1,
          difficulty: 1,
          elapsed_days: 0,
          scheduled_days: 0,
          reps: 1,
          lapses: 0,
          state: 1,
          last_review: new Date().toISOString(),
        },
      }],
    });

    renderStudy();

    await screen.findByRole('heading', { name: 'Next Review Soon' });
    expect(screen.queryByText('Session Complete!')).not.toBeInTheDocument();
    expect(screen.getByText('Learning Queue: 1')).toBeInTheDocument();

    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 1100));
    });

    await screen.findByText('What is FSRS?');
    expect(screen.getByText('Due Now: 1')).toBeInTheDocument();
    expect(screen.getByText('Learning Queue: 0')).toBeInTheDocument();
  });
});

import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';
import App from './App';
import { createMockApiServer } from './test/mockApiServer';
import type { CardWithState, Deck, User } from './types';

describe('App end-to-end flows', () => {
  let restoreServer: (() => void) | undefined;

  afterEach(() => {
    restoreServer?.();
    restoreServer = undefined;
    window.history.replaceState({}, '', '/');
  });

  it('supports registering, logging in, creating a deck, and adding a card', async () => {
    const server = createMockApiServer();
    restoreServer = server.restore;

    window.history.replaceState({}, '', '/register');

    const user = userEvent.setup();
    render(<App />);

    expect(screen.getByRole('link', { name: 'Skip to content' })).toHaveAttribute('href', '#main-content');
    await screen.findByRole('heading', { name: 'Register' });

    await user.type(screen.getByLabelText('Email'), 'ada@example.com');
    await user.type(screen.getByLabelText('Password'), 'password123');
    await user.type(screen.getByLabelText('Confirm Password'), 'password123');
    await user.click(screen.getByRole('button', { name: 'Register' }));

    await screen.findByRole('heading', { name: 'Login' });
    expect(screen.getByText('If the email is available, a verification email has been sent.')).toBeInTheDocument();

    await user.type(screen.getByLabelText('Email'), 'ada@example.com');
    await user.type(screen.getByLabelText('Password'), 'password123');
    await user.click(screen.getByRole('button', { name: 'Login' }));

    await screen.findByRole('heading', { name: 'My Decks' });
    expect(screen.getByText('ada@example.com')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'New Deck' }));
    await user.type(screen.getByLabelText('Name'), 'Biology');
    await user.type(screen.getByLabelText('Description'), 'Cells and memory');
    await user.click(screen.getByRole('button', { name: 'Create Deck' }));

    await screen.findByText('Biology');
    await user.click(screen.getByRole('link', { name: 'Edit' }));

    await screen.findByRole('heading', { name: 'Biology' });
    await user.click(screen.getByRole('button', { name: 'Add Card' }));

    await user.type(screen.getByLabelText('Front'), 'What is FSRS?');
    await user.type(screen.getByLabelText('Back'), 'A spaced repetition scheduler.');
    await user.click(screen.getByRole('button', { name: 'Add Card' }));

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: 'Cards (1)' })).toBeInTheDocument();
    });
    expect(screen.getByText('What is FSRS?')).toBeInTheDocument();
  });

  it('supports completing a study session for an authenticated user', async () => {
    const currentUser: User = {
      id: 'user-1',
      email: 'grace@example.com',
    };
    const deck: Deck = {
      id: 'deck-1',
      user_id: currentUser.id,
      name: 'Biology',
      description: 'Cells and memory',
      fuzz_enabled: false,
      created_at: '2026-04-16T00:00:00Z',
    };
    const dueCard: CardWithState = {
      id: 'card-1',
      deck_id: deck.id,
      front: 'What is spaced repetition?',
      back: 'Timed recall for memory retention.',
      link: 'https://example.com/guide',
      created_at: '2026-04-16T00:00:00Z',
      tags: [],
    };

    const server = createMockApiServer({
      currentUser,
      users: [{ user: currentUser, password: 'password123' }],
      decks: [deck],
      cardsByDeck: {
        [deck.id]: [dueCard],
      },
    });
    restoreServer = server.restore;

    window.history.replaceState({}, '', `/study/${deck.id}`);

    const user = userEvent.setup();
    render(<App />);

    await screen.findByText('What is spaced repetition?');
    await user.click(screen.getByRole('button', { name: /show answer/i }));

    expect(screen.getByText('Timed recall for memory retention.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open Link' })).toHaveAttribute('href', 'https://example.com/guide');

    await user.click(screen.getByRole('button', { name: /good/i }));

    await screen.findByRole('heading', { name: 'Session Complete!' });
    expect(screen.getByText("You've reviewed all due cards.")).toBeInTheDocument();
  });

  it('renders the about page without requiring authentication', async () => {
    const server = createMockApiServer();
    restoreServer = server.restore;

    window.history.replaceState({}, '', '/about');

    render(<App />);

    await screen.findByRole('heading', { name: 'How Study Works' });
    expect(screen.getByText(/comes back in about 1 minute/i)).toBeInTheDocument();
    expect(screen.getByText(/short-term learning steps are enabled/i)).toBeInTheDocument();
  });

  it('supports requesting a password reset from the login page', async () => {
    const server = createMockApiServer();
    restoreServer = server.restore;

    window.history.replaceState({}, '', '/login');

    const user = userEvent.setup();
    render(<App />);

    await screen.findByRole('heading', { name: 'Login' });

    await user.click(screen.getByRole('link', { name: 'Forgot your password?' }));

    await screen.findByRole('heading', { name: 'Forgot Password' });
    await user.type(screen.getByLabelText('Email'), 'ada@example.com');
    await user.click(screen.getByRole('button', { name: 'Send Reset Link' }));

    expect(await screen.findByText('If the account exists, a password reset email has been sent.')).toBeInTheDocument();
  });
});

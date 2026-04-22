import type { StudyStats } from '../api/client';
import type { CardState, CardWithState, Deck, DeckWithStats, DueCalendarDay, StudySession, Tag, User } from '../types';

type MockUserRecord = {
  password: string;
  user: User;
};

type MockApiServerOptions = {
  currentUser?: User | null;
  users?: MockUserRecord[];
  decks?: Deck[];
  cardsByDeck?: Record<string, CardWithState[]>;
  tagsByDeck?: Record<string, Tag[]>;
  studyStats?: StudyStats;
  dueCalendar?: DueCalendarDay[];
};

type MockState = {
  currentUser: User | null;
  users: Map<string, MockUserRecord>;
  decks: Deck[];
  cardsByDeck: Map<string, CardWithState[]>;
  tagsByDeck: Map<string, Tag[]>;
  studyStats: StudyStats;
  dueCalendar: DueCalendarDay[];
  ids: {
    deck: number;
    card: number;
    state: number;
    tag: number;
    user: number;
  };
};

const defaultStudyStats: StudyStats = {
  totalReviews: 0,
  reviewsLast24Hours: 0,
  reviewsLast7Days: 0,
  avgRating: 0,
  retentionRate: 0,
};

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function createJSONResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      'Content-Type': 'application/json',
    },
  });
}

function createTextResponse(body: string, status: number): Response {
  return new Response(body, {
    status,
    headers: {
      'Content-Type': 'text/plain',
    },
  });
}

function createNoContentResponse(): Response {
  return new Response(null, { status: 204 });
}

function normalizeEmail(email: string): string {
  return email.trim().toLowerCase();
}

function computeDeckStats(cards: CardWithState[]): DeckWithStats['stats'] {
  const now = Date.now();

  return cards.reduce<DeckWithStats['stats']>((stats, card) => {
    stats.total += 1;

    if (!card.state || card.state.state === 0) {
      stats.new += 1;
    }

    if (card.state && (card.state.state === 1 || card.state.state === 3)) {
      stats.learning += 1;
    }

    if (!card.state || new Date(card.state.due).getTime() <= now) {
      stats.due += 1;
    }

    return stats;
  }, {
    total: 0,
    new: 0,
    due: 0,
    learning: 0,
  });
}

function parseDeckID(pathname: string): string | null {
  const match = pathname.match(/^\/api\/decks\/([^/]+)$/);
  return match?.[1] ?? null;
}

function parseDeckResource(pathname: string, resource: 'cards' | 'tags'): string | null {
  const match = pathname.match(new RegExp(`^/api/decks/([^/]+)/${resource}$`));
  return match?.[1] ?? null;
}

function parseStudyDeckID(pathname: string): string | null {
  const match = pathname.match(/^\/api\/study\/([^/]+)$/);
  return match?.[1] ?? null;
}

function parseStudySessionDeckID(pathname: string): string | null {
  const match = pathname.match(/^\/api\/study\/([^/]+)\/session$/);
  return match?.[1] ?? null;
}

function parseReviewCardID(pathname: string): string | null {
  const match = pathname.match(/^\/api\/study\/([^/]+)\/review$/);
  return match?.[1] ?? null;
}

function isDueForStudy(card: CardWithState): boolean {
  if (!card.state) {
    return true;
  }

  return new Date(card.state.due).getTime() <= Date.now();
}

function isPendingLearningCard(card: CardWithState): boolean {
  if (!card.state) {
    return false;
  }

  const isLearningState = card.state.state === 1 || card.state.state === 3;
  return isLearningState && card.state.scheduled_days === 0 && new Date(card.state.due).getTime() > Date.now();
}

function findCard(state: MockState, cardID: string): CardWithState | undefined {
  for (const cards of state.cardsByDeck.values()) {
    const card = cards.find((candidate) => candidate.id === cardID);
    if (card) {
      return card;
    }
  }

  return undefined;
}

async function parseBody(init?: RequestInit): Promise<Record<string, unknown>> {
  if (!init?.body || typeof init.body !== 'string') {
    return {};
  }

  return JSON.parse(init.body) as Record<string, unknown>;
}

function requireUser(state: MockState): Response | null {
  if (!state.currentUser) {
    return createTextResponse('Unauthorized', 401);
  }

  return null;
}

export function createMockApiServer(options: MockApiServerOptions = {}) {
  const users = new Map<string, MockUserRecord>();
  for (const record of options.users ?? []) {
    users.set(normalizeEmail(record.user.email), clone(record));
  }

  const state: MockState = {
    currentUser: options.currentUser ? clone(options.currentUser) : null,
    users,
    decks: clone(options.decks ?? []),
    cardsByDeck: new Map(
      Object.entries(options.cardsByDeck ?? {}).map(([deckID, cards]) => [deckID, clone(cards)]),
    ),
    tagsByDeck: new Map(
      Object.entries(options.tagsByDeck ?? {}).map(([deckID, tags]) => [deckID, clone(tags)]),
    ),
    studyStats: clone(options.studyStats ?? defaultStudyStats),
    dueCalendar: clone(options.dueCalendar ?? []),
    ids: {
      deck: (options.decks?.length ?? 0) + 1,
      card: Object.values(options.cardsByDeck ?? {}).reduce((count, cards) => count + cards.length, 0) + 1,
      state: 1,
      tag: Object.values(options.tagsByDeck ?? {}).reduce((count, tags) => count + tags.length, 0) + 1,
      user: (options.users?.length ?? 0) + 1,
    },
  };

  const originalFetch = globalThis.fetch;

  const mockFetch: typeof fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const requestURL = input instanceof Request ? input.url : String(input);
    const url = new URL(requestURL);
    const pathname = url.pathname;
    const method = (init?.method ?? (input instanceof Request ? input.method : 'GET')).toUpperCase();

    if (pathname === '/api/auth/register' && method === 'POST') {
      const body = await parseBody(init);
      const email = normalizeEmail(String(body.email ?? ''));
      const password = String(body.password ?? '');

      if (!state.users.has(email)) {
        const user = { id: `user-${state.ids.user++}`, email };
        state.users.set(email, { password, user });
      }

      return createJSONResponse({
        message: 'If the email is available, the account is ready to sign in.',
      }, 202);
    }

    if (pathname === '/api/auth/login' && method === 'POST') {
      const body = await parseBody(init);
      const email = normalizeEmail(String(body.email ?? ''));
      const password = String(body.password ?? '');
      const record = state.users.get(email);

      if (!record || record.password !== password) {
        return createTextResponse('Invalid credentials', 401);
      }

      state.currentUser = clone(record.user);
      return createJSONResponse(record.user);
    }

    if (pathname === '/api/auth/me' && method === 'GET') {
      if (!state.currentUser) {
        return createTextResponse('Unauthorized', 401);
      }

      return createJSONResponse(state.currentUser);
    }

    if (pathname === '/api/auth/logout' && method === 'POST') {
      state.currentUser = null;
      return createNoContentResponse();
    }

    const unauthorized = requireUser(state);
    if (unauthorized) {
      return unauthorized;
    }

    if (pathname === '/api/decks' && method === 'GET') {
      const decksWithStats = state.decks
        .filter((deck) => deck.user_id === state.currentUser!.id)
        .map((deck) => ({
          ...deck,
          stats: computeDeckStats(state.cardsByDeck.get(deck.id) ?? []),
        }));
      return createJSONResponse(decksWithStats);
    }

    if (pathname === '/api/decks' && method === 'POST') {
      const body = await parseBody(init);
      const deck: Deck = {
        id: `deck-${state.ids.deck++}`,
        user_id: state.currentUser!.id,
        name: String(body.name ?? ''),
        description: String(body.description ?? ''),
        fuzz_enabled: Boolean(body.fuzz_enabled ?? false),
        created_at: new Date().toISOString(),
      };

      state.decks.push(deck);
      state.cardsByDeck.set(deck.id, []);
      state.tagsByDeck.set(deck.id, []);
      return createJSONResponse(deck, 201);
    }

    if (pathname === '/api/study/stats' && method === 'GET') {
      return createJSONResponse(state.studyStats);
    }

    if (pathname === '/api/study/schedule' && method === 'GET') {
      return createJSONResponse(state.dueCalendar);
    }

    const deckID = parseDeckID(pathname);
    if (deckID && method === 'GET') {
      const deck = state.decks.find((candidate) => candidate.id === deckID && candidate.user_id === state.currentUser!.id);
      if (!deck) {
        return createTextResponse('Deck not found', 404);
      }
      return createJSONResponse(deck);
    }

    const cardsDeckID = parseDeckResource(pathname, 'cards');
    if (cardsDeckID && method === 'GET') {
      return createJSONResponse(state.cardsByDeck.get(cardsDeckID) ?? []);
    }

    if (cardsDeckID && method === 'POST') {
      const body = await parseBody(init);
      const card: CardWithState = {
        id: `card-${state.ids.card++}`,
        deck_id: cardsDeckID,
        front: String(body.front ?? ''),
        back: String(body.back ?? ''),
        link: String(body.link ?? ''),
        created_at: new Date().toISOString(),
        tags: [],
      };

      const cards = state.cardsByDeck.get(cardsDeckID) ?? [];
      cards.unshift(card);
      state.cardsByDeck.set(cardsDeckID, cards);
      return createJSONResponse(card, 201);
    }

    const tagsDeckID = parseDeckResource(pathname, 'tags');
    if (tagsDeckID && method === 'GET') {
      return createJSONResponse(state.tagsByDeck.get(tagsDeckID) ?? []);
    }

    if (tagsDeckID && method === 'POST') {
      const body = await parseBody(init);
      const tag: Tag = {
        id: `tag-${state.ids.tag++}`,
        deck_id: tagsDeckID,
        name: String(body.name ?? ''),
        created_at: new Date().toISOString(),
      };

      const tags = state.tagsByDeck.get(tagsDeckID) ?? [];
      tags.push(tag);
      state.tagsByDeck.set(tagsDeckID, tags);
      return createJSONResponse(tag, 201);
    }

    const studySessionDeckID = parseStudySessionDeckID(pathname);
    if (studySessionDeckID && method === 'GET') {
      const cards = state.cardsByDeck.get(studySessionDeckID) ?? [];
      const session: StudySession = {
        due_cards: cards.filter(isDueForStudy),
        pending_learning_cards: cards.filter(isPendingLearningCard),
      };

      return createJSONResponse(session);
    }

    const studyDeckID = parseStudyDeckID(pathname);
    if (studyDeckID && method === 'GET') {
      return createJSONResponse((state.cardsByDeck.get(studyDeckID) ?? []).filter(isDueForStudy));
    }

    const reviewCardID = parseReviewCardID(pathname);
    if (reviewCardID && method === 'POST') {
      const card = findCard(state, reviewCardID);
      if (!card) {
        return createTextResponse('Card not found', 404);
      }

      const previousReps = card.state?.reps ?? 0;
      const nextState: CardState = {
        id: card.state?.id ?? `state-${state.ids.state++}`,
        card_id: card.id,
        due: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
        stability: 1,
        difficulty: 1,
        elapsed_days: 0,
        scheduled_days: 1,
        reps: previousReps + 1,
        lapses: card.state?.lapses ?? 0,
        state: 2,
        last_review: new Date().toISOString(),
      };

      card.state = nextState;
      return createJSONResponse(nextState);
    }

    throw new Error(`Unhandled request in mock API server: ${method} ${pathname}`);
  }) as typeof fetch;

  globalThis.fetch = mockFetch;

  return {
    state,
    fetchMock: mockFetch,
    restore() {
      globalThis.fetch = originalFetch;
    },
  };
}

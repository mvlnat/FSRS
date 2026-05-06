import type { CardState, CardWithState, Deck, DeckWithStats, StudySession, Tag } from '../types';
import type { StudyStats } from '../api/client';
import { DEMO_USER, DEMO_DECKS, DEMO_CARDS, DEMO_TAGS, DEMO_STATS, getDemoCalendar } from './demoData';

type DemoState = {
  decks: DeckWithStats[];
  cardsByDeck: Map<string, CardWithState[]>;
  tagsByDeck: Map<string, Tag[]>;
  stats: StudyStats;
  ids: { card: number; state: number; tag: number; deck: number };
};

const DEMO_MODE_KEY = 'fsrs-demo-mode';

let originalFetch: typeof fetch | null = null;
let demoState: DemoState | null = null;

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function createJSONResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function createTextResponse(body: string, status: number): Response {
  return new Response(body, {
    status,
    headers: { 'Content-Type': 'text/plain' },
  });
}

function createNoContentResponse(): Response {
  return new Response(null, { status: 204 });
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
  }, { total: 0, new: 0, due: 0, learning: 0 });
}

function isDueForStudy(card: CardWithState): boolean {
  if (!card.state) return true;
  return new Date(card.state.due).getTime() <= Date.now();
}

function isPendingLearningCard(card: CardWithState): boolean {
  if (!card.state) return false;
  const isLearning = card.state.state === 1 || card.state.state === 3;
  return isLearning && card.state.scheduled_days === 0 && new Date(card.state.due).getTime() > Date.now();
}

function parseDeckID(pathname: string): string | null {
  const match = pathname.match(/^\/api\/decks\/([^/]+)$/);
  return match?.[1] ?? null;
}

function parseDeckResource(pathname: string, resource: string): string | null {
  const match = pathname.match(new RegExp(`^/api/decks/([^/]+)/${resource}$`));
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

async function parseBody(init?: RequestInit): Promise<Record<string, unknown>> {
  if (!init?.body || typeof init.body !== 'string') return {};
  return JSON.parse(init.body) as Record<string, unknown>;
}

function findCard(state: DemoState, cardID: string): CardWithState | undefined {
  for (const cards of state.cardsByDeck.values()) {
    const card = cards.find((c) => c.id === cardID);
    if (card) return card;
  }
  return undefined;
}

function initDemoState(): DemoState {
  return {
    decks: clone(DEMO_DECKS),
    cardsByDeck: new Map(Object.entries(clone(DEMO_CARDS))),
    tagsByDeck: new Map(Object.entries(clone(DEMO_TAGS))),
    stats: clone(DEMO_STATS),
    ids: { card: 100, state: 100, tag: 100, deck: 100 },
  };
}

const demoFetch: typeof fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
  if (!demoState) {
    demoState = initDemoState();
  }

  const requestURL = input instanceof Request ? input.url : String(input);
  const url = new URL(requestURL, window.location.origin);
  const pathname = url.pathname;
  const method = (init?.method ?? (input instanceof Request ? input.method : 'GET')).toUpperCase();

  // Auth endpoints
  if (pathname === '/api/auth/me' && method === 'GET') {
    return createJSONResponse(DEMO_USER);
  }

  if (pathname === '/api/auth/logout' && method === 'POST') {
    return createNoContentResponse();
  }

  // Decks
  if (pathname === '/api/decks' && method === 'GET') {
    const decksWithStats = demoState.decks.map((deck) => ({
      ...deck,
      stats: computeDeckStats(demoState!.cardsByDeck.get(deck.id) ?? []),
    }));
    return createJSONResponse(decksWithStats);
  }

  if (pathname === '/api/decks' && method === 'POST') {
    // In demo mode, creating a deck should trigger signup prompt
    // Return success but the UI will intercept this
    const body = await parseBody(init);
    const deck: Deck = {
      id: `demo-deck-${demoState.ids.deck++}`,
      user_id: DEMO_USER.id,
      name: String(body.name ?? ''),
      description: String(body.description ?? ''),
      fuzz_enabled: Boolean(body.fuzz_enabled ?? false),
      new_card_front_template: String(body.new_card_front_template ?? ''),
      new_card_back_template: String(body.new_card_back_template ?? ''),
      created_at: new Date().toISOString(),
    };
    const deckWithStats: DeckWithStats = { ...deck, stats: { total: 0, new: 0, due: 0, learning: 0 } };
    demoState.decks.push(deckWithStats);
    demoState.cardsByDeck.set(deck.id, []);
    demoState.tagsByDeck.set(deck.id, []);
    return createJSONResponse(deckWithStats, 201);
  }

  // Study stats
  if (pathname === '/api/study/stats' && method === 'GET') {
    return createJSONResponse(demoState.stats);
  }

  if (pathname === '/api/study/schedule' && method === 'GET') {
    return createJSONResponse(getDemoCalendar());
  }

  // Single deck
  const deckID = parseDeckID(pathname);
  if (deckID && method === 'GET') {
    const deck = demoState.decks.find((d) => d.id === deckID);
    if (!deck) return createTextResponse('Deck not found', 404);
    return createJSONResponse(deck);
  }

  if (deckID && method === 'PUT') {
    const deck = demoState.decks.find((d) => d.id === deckID);
    if (!deck) return createTextResponse('Deck not found', 404);
    const body = await parseBody(init);
    if (body.name !== undefined) deck.name = String(body.name);
    if (body.description !== undefined) deck.description = String(body.description);
    if (body.fuzz_enabled !== undefined) deck.fuzz_enabled = Boolean(body.fuzz_enabled);
    return createJSONResponse(deck);
  }

  if (deckID && method === 'DELETE') {
    const idx = demoState.decks.findIndex((d) => d.id === deckID);
    if (idx >= 0) {
      demoState.decks.splice(idx, 1);
      demoState.cardsByDeck.delete(deckID);
      demoState.tagsByDeck.delete(deckID);
    }
    return createNoContentResponse();
  }

  // Cards
  const cardsDeckID = parseDeckResource(pathname, 'cards');
  if (cardsDeckID && method === 'GET') {
    return createJSONResponse(demoState.cardsByDeck.get(cardsDeckID) ?? []);
  }

  if (cardsDeckID && method === 'POST') {
    const body = await parseBody(init);
    const card: CardWithState = {
      id: `demo-card-${demoState.ids.card++}`,
      deck_id: cardsDeckID,
      front: String(body.front ?? ''),
      back: String(body.back ?? ''),
      link: String(body.link ?? ''),
      created_at: new Date().toISOString(),
      tags: [],
    };
    const cards = demoState.cardsByDeck.get(cardsDeckID) ?? [];
    cards.unshift(card);
    demoState.cardsByDeck.set(cardsDeckID, cards);
    return createJSONResponse(card, 201);
  }

  // Tags
  const tagsDeckID = parseDeckResource(pathname, 'tags');
  if (tagsDeckID && method === 'GET') {
    return createJSONResponse(demoState.tagsByDeck.get(tagsDeckID) ?? []);
  }

  if (tagsDeckID && method === 'POST') {
    const body = await parseBody(init);
    const tag: Tag = {
      id: `demo-tag-${demoState.ids.tag++}`,
      deck_id: tagsDeckID,
      name: String(body.name ?? ''),
      created_at: new Date().toISOString(),
    };
    const tags = demoState.tagsByDeck.get(tagsDeckID) ?? [];
    tags.push(tag);
    demoState.tagsByDeck.set(tagsDeckID, tags);
    return createJSONResponse(tag, 201);
  }

  // Study session
  const studySessionDeckID = parseStudySessionDeckID(pathname);
  if (studySessionDeckID && method === 'GET') {
    const cards = demoState.cardsByDeck.get(studySessionDeckID) ?? [];
    const session: StudySession = {
      due_cards: cards.filter(isDueForStudy),
      pending_learning_cards: cards.filter(isPendingLearningCard),
    };
    return createJSONResponse(session);
  }

  // Review card
  const reviewCardID = parseReviewCardID(pathname);
  if (reviewCardID && method === 'POST') {
    const card = findCard(demoState, reviewCardID);
    if (!card) return createTextResponse('Card not found', 404);

    const body = await parseBody(init);
    const rating = Number(body.rating ?? 3);
    const previousReps = card.state?.reps ?? 0;

    // Simple scheduling: higher rating = longer interval
    const intervalMinutes = rating === 1 ? 1 : rating === 2 ? 5 : rating === 3 ? 10 : 60 * 24;
    const nextDue = new Date(Date.now() + intervalMinutes * 60 * 1000);

    const nextState: CardState = {
      id: card.state?.id ?? `demo-state-${demoState.ids.state++}`,
      card_id: card.id,
      due: nextDue.toISOString(),
      stability: Math.min(10, (card.state?.stability ?? 1) + (rating - 2) * 0.5),
      difficulty: Math.max(1, Math.min(10, (card.state?.difficulty ?? 5) - (rating - 2.5) * 0.2)),
      elapsed_days: 0,
      scheduled_days: rating === 4 ? 1 : 0,
      reps: previousReps + 1,
      lapses: rating === 1 ? (card.state?.lapses ?? 0) + 1 : (card.state?.lapses ?? 0),
      state: rating === 4 ? 2 : rating === 1 ? 3 : 1,
      last_review: new Date().toISOString(),
    };

    card.state = nextState;
    demoState.stats.totalReviews += 1;
    demoState.stats.reviewsLast24Hours += 1;

    return createJSONResponse(nextState);
  }

  // Card update/delete
  const cardMatch = pathname.match(/^\/api\/cards\/([^/]+)$/);
  if (cardMatch) {
    const cardID = cardMatch[1];
    if (method === 'PUT') {
      const card = findCard(demoState, cardID);
      if (!card) return createTextResponse('Card not found', 404);
      const body = await parseBody(init);
      if (body.front !== undefined) card.front = String(body.front);
      if (body.back !== undefined) card.back = String(body.back);
      if (body.link !== undefined) card.link = String(body.link);
      // Reset state on edit
      delete card.state;
      return createJSONResponse(card);
    }
    if (method === 'DELETE') {
      for (const [deckId, cards] of demoState.cardsByDeck.entries()) {
        const idx = cards.findIndex((c) => c.id === cardID);
        if (idx >= 0) {
          cards.splice(idx, 1);
          demoState.cardsByDeck.set(deckId, cards);
          break;
        }
      }
      return createNoContentResponse();
    }
  }

  // Tag delete
  const tagMatch = pathname.match(/^\/api\/tags\/([^/]+)$/);
  if (tagMatch && method === 'DELETE') {
    const tagID = tagMatch[1];
    for (const [deckId, tags] of demoState.tagsByDeck.entries()) {
      const idx = tags.findIndex((t) => t.id === tagID);
      if (idx >= 0) {
        tags.splice(idx, 1);
        demoState.tagsByDeck.set(deckId, tags);
        break;
      }
    }
    return createNoContentResponse();
  }

  // Fallback
  console.warn('[Demo API] Unhandled request:', method, pathname);
  return createTextResponse('Not found', 404);
};

export function enableDemoMode(): void {
  if (originalFetch) return; // Already enabled
  originalFetch = globalThis.fetch;
  demoState = initDemoState();
  globalThis.fetch = demoFetch;
  sessionStorage.setItem(DEMO_MODE_KEY, 'true');
}

export function disableDemoMode(): void {
  if (originalFetch) {
    globalThis.fetch = originalFetch;
    originalFetch = null;
  }
  demoState = null;
  sessionStorage.removeItem(DEMO_MODE_KEY);
}

export function isDemoModeEnabled(): boolean {
  return originalFetch !== null;
}

// Initialize demo mode if it was active before page reload
export function initDemoModeFromStorage(): boolean {
  if (sessionStorage.getItem(DEMO_MODE_KEY) === 'true') {
    enableDemoMode();
    return true;
  }
  return false;
}

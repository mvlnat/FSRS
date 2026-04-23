import type { User, Deck, DeckWithStats, Card, CardWithState, CardState, StudySession, Tag, DueCalendarDay } from '../types';

const API_BASE = import.meta.env.PROD ? '/api' : 'http://localhost:8080/api';
const UNAUTHORIZED_EVENT = 'fsrs:unauthorized';
let latestRequestId = 0;

class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

function notifyUnauthorized(requestId: number) {
  window.dispatchEvent(new CustomEvent(UNAUTHORIZED_EVENT, { detail: { requestId } }));
}

export function onUnauthorized(callback: (requestId: number) => void) {
  const handler = (event: Event) => {
    const requestId = event instanceof CustomEvent && typeof event.detail?.requestId === 'number'
      ? event.detail.requestId
      : 0;
    callback(requestId);
  };

  window.addEventListener(UNAUTHORIZED_EVENT, handler);

  return () => {
    window.removeEventListener(UNAUTHORIZED_EVENT, handler);
  };
}

export function getLatestRequestId() {
  return latestRequestId;
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const requestId = ++latestRequestId;
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });

  if (!response.ok) {
    const text = await response.text();
    if (response.status === 401) {
      notifyUnauthorized(requestId);
    }
    throw new ApiError(response.status, text || response.statusText);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  if (!text) {
    return undefined as T;
  }

  const contentType = response.headers.get('Content-Type') ?? '';
  if (contentType.includes('application/json')) {
    return JSON.parse(text) as T;
  }

  return text as T;
}

export interface ApiMessageResponse {
  message: string;
}

// Auth
export async function register(email: string, password: string): Promise<void> {
  return request<void>('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export async function login(email: string, password: string): Promise<User> {
  return request<User>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export async function logout(): Promise<void> {
  return request<void>('/auth/logout', { method: 'POST' });
}

export async function getMe(): Promise<User> {
  return request<User>('/auth/me');
}

export async function requestPasswordReset(email: string): Promise<ApiMessageResponse> {
  return request<ApiMessageResponse>('/auth/password-reset/request', {
    method: 'POST',
    body: JSON.stringify({ email }),
  });
}

export async function confirmPasswordReset(token: string, password: string): Promise<ApiMessageResponse> {
  return request<ApiMessageResponse>('/auth/password-reset/confirm', {
    method: 'POST',
    body: JSON.stringify({ token, password }),
  });
}

export async function confirmEmailVerification(token: string): Promise<ApiMessageResponse> {
  return request<ApiMessageResponse>('/auth/verify-email/confirm', {
    method: 'POST',
    body: JSON.stringify({ token }),
  });
}

// Decks
export async function getDecks(): Promise<DeckWithStats[]> {
  return request<DeckWithStats[]>('/decks');
}

export async function getDeck(id: string): Promise<Deck> {
  return request<Deck>(`/decks/${id}`);
}

export async function createDeck(name: string, description: string): Promise<Deck> {
  return request<Deck>('/decks', {
    method: 'POST',
    body: JSON.stringify({ name, description, fuzz_enabled: false }),
  });
}

export async function updateDeck(id: string, name: string, description: string, fuzzEnabled: boolean = false): Promise<Deck> {
  return request<Deck>(`/decks/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name, description, fuzz_enabled: fuzzEnabled }),
  });
}

export async function deleteDeck(id: string): Promise<void> {
  return request<void>(`/decks/${id}`, { method: 'DELETE' });
}

// Import/Export
export interface DeckExport {
  name: string;
  description: string;
  fuzz_enabled?: boolean;
  cards: { front: string; back: string; link?: string }[];
}

export async function exportDeck(id: string): Promise<DeckExport> {
  return request<DeckExport>(`/decks/${id}/export`);
}

export async function importDeck(data: DeckExport): Promise<DeckWithStats> {
  return request<DeckWithStats>('/decks/import', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

// Cards
export async function getCards(deckId: string): Promise<CardWithState[]> {
  return request<CardWithState[]>(`/decks/${deckId}/cards`);
}

export async function createCard(deckId: string, front: string, back: string, link: string = ''): Promise<Card> {
  return request<Card>(`/decks/${deckId}/cards`, {
    method: 'POST',
    body: JSON.stringify({ front, back, link }),
  });
}

export async function updateCard(
  id: string,
  front: string,
  back: string,
  link: string = '',
  tagIds?: string[],
): Promise<Card> {
  return request<Card>(`/cards/${id}`, {
    method: 'PUT',
    body: JSON.stringify({
      front,
      back,
      link,
      ...(tagIds !== undefined ? { tag_ids: tagIds } : {}),
    }),
  });
}

export async function deleteCard(id: string): Promise<void> {
  return request<void>(`/cards/${id}`, { method: 'DELETE' });
}

// Study
export async function getDueCards(deckId: string): Promise<CardWithState[]> {
  return request<CardWithState[]>(`/study/${deckId}`);
}

export async function getStudySession(deckId: string): Promise<StudySession> {
  return request<StudySession>(`/study/${deckId}/session`);
}

export async function reviewCard(cardId: string, rating: number): Promise<CardState> {
  return request<CardState>(`/study/${cardId}/review`, {
    method: 'POST',
    body: JSON.stringify({ rating }),
  });
}

// Study Stats
export interface StudyStats {
  totalReviews: number;
  reviewsLast24Hours: number;
  reviewsLast7Days: number;
  avgRating: number;
  retentionRate: number;
}

export async function getStudyStats(): Promise<StudyStats> {
  return request<StudyStats>('/study/stats');
}

export async function getDueCalendar(start: string, end: string, timezone: string): Promise<DueCalendarDay[]> {
  const params = new URLSearchParams({ start, end, timezone });
  return request<DueCalendarDay[]>(`/study/schedule?${params.toString()}`);
}

// Tags
export async function getTags(deckId: string): Promise<Tag[]> {
  return request<Tag[]>(`/decks/${deckId}/tags`);
}

export async function createTag(deckId: string, name: string): Promise<Tag> {
  return request<Tag>(`/decks/${deckId}/tags`, {
    method: 'POST',
    body: JSON.stringify({ name }),
  });
}

export async function deleteTag(tagId: string): Promise<void> {
  return request<void>(`/tags/${tagId}`, { method: 'DELETE' });
}

import type { User, Deck, DeckWithStats, Card, CardWithState, CardState, DeckStats, Tag } from '../types';

const API_BASE = import.meta.env.PROD ? '/api' : 'http://localhost:8080/api';

class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
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
    throw new ApiError(response.status, text || response.statusText);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}

// Auth
export async function register(email: string, password: string): Promise<User> {
  return request<User>('/auth/register', {
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
    body: JSON.stringify({ name, description }),
  });
}

export async function updateDeck(id: string, name: string, description: string): Promise<Deck> {
  return request<Deck>(`/decks/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name, description }),
  });
}

export async function deleteDeck(id: string): Promise<void> {
  return request<void>(`/decks/${id}`, { method: 'DELETE' });
}

export async function getDeckStats(id: string): Promise<DeckStats> {
  return request<DeckStats>(`/decks/${id}/stats`);
}

// Import/Export
export interface DeckExport {
  name: string;
  description: string;
  cards: { front: string; back: string }[];
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

export async function getCard(id: string): Promise<Card> {
  return request<Card>(`/cards/${id}`);
}

export async function createCard(deckId: string, front: string, back: string, link: string = ''): Promise<Card> {
  return request<Card>(`/decks/${deckId}/cards`, {
    method: 'POST',
    body: JSON.stringify({ front, back, link }),
  });
}

export async function updateCard(id: string, front: string, back: string, link: string = ''): Promise<Card> {
  return request<Card>(`/cards/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ front, back, link }),
  });
}

export async function deleteCard(id: string): Promise<void> {
  return request<void>(`/cards/${id}`, { method: 'DELETE' });
}

// Study
export async function getDueCards(deckId: string): Promise<CardWithState[]> {
  return request<CardWithState[]>(`/study/${deckId}`);
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

export async function setCardTags(cardId: string, tagIds: string[]): Promise<Tag[]> {
  return request<Tag[]>(`/cards/${cardId}/tags`, {
    method: 'PUT',
    body: JSON.stringify({ tag_ids: tagIds }),
  });
}

export interface User {
  id: string;
  email: string;
}

export interface Deck {
  id: string;
  user_id: string;
  name: string;
  description: string;
  fuzz_enabled: boolean;
  new_card_front_template: string;
  new_card_back_template: string;
  created_at: string;
}

export interface DeckStats {
  total: number;
  new: number;
  due: number;
  learning: number;
}

export interface DeckWithStats extends Deck {
  stats: DeckStats;
}

export interface Card {
  id: string;
  deck_id: string;
  front: string;
  back: string;
  link: string;
  created_at: string;
}

export interface CardState {
  id: string;
  card_id: string;
  due: string;
  stability: number;
  difficulty: number;
  elapsed_days: number;
  scheduled_days: number;
  reps: number;
  lapses: number;
  state: number; // 0=New, 1=Learning, 2=Review, 3=Relearning
  last_review: string | null;
}

export interface StudySession {
  due_cards: CardWithState[];
  pending_learning_cards: CardWithState[];
}

export interface CardWithState extends Card {
  state?: CardState;
  tags: Tag[];
}

export interface Tag {
  id: string;
  deck_id: string;
  name: string;
  created_at: string;
}

export interface Review {
  id: string;
  card_id: string;
  rating: number; // 1=Again, 2=Hard, 3=Good, 4=Easy
  reviewed_at: string;
}

export interface DueCalendarDeck {
  deck_id: string;
  deck_name: string;
  count: number;
}

export interface DueCalendarDay {
  date: string;
  total: number;
  decks: DueCalendarDeck[];
}

export type Rating = 1 | 2 | 3 | 4;

export const RATING_LABELS: Record<Rating, string> = {
  1: 'Again',
  2: 'Hard',
  3: 'Good',
  4: 'Easy',
};

import type { DeckWithStats, CardWithState, Tag, DueCalendarDay } from '../types';
import type { StudyStats } from '../api/client';

export const DEMO_USER = {
  id: 'demo-user',
  email: 'demo@example.com',
};

const now = new Date().toISOString();

export const DEMO_DECKS: DeckWithStats[] = [
  {
    id: 'demo-deck-spanish',
    user_id: DEMO_USER.id,
    name: 'Spanish Basics',
    description: 'Learn common Spanish words and phrases',
    fuzz_enabled: false,
    new_card_front_template: '',
    new_card_back_template: '',
    created_at: now,
    stats: { total: 5, new: 2, due: 2, learning: 1 },
  },
  {
    id: 'demo-deck-capitals',
    user_id: DEMO_USER.id,
    name: 'World Capitals',
    description: 'Test your geography knowledge',
    fuzz_enabled: false,
    new_card_front_template: '',
    new_card_back_template: '',
    created_at: now,
    stats: { total: 5, new: 3, due: 1, learning: 1 },
  },
];

export const DEMO_CARDS: Record<string, CardWithState[]> = {
  'demo-deck-spanish': [
    {
      id: 'demo-card-sp-1',
      deck_id: 'demo-deck-spanish',
      front: 'Hello',
      back: 'Hola',
      link: '',
      created_at: now,
      tags: [],
      state: {
        id: 'state-sp-1',
        card_id: 'demo-card-sp-1',
        due: now,
        stability: 4.5,
        difficulty: 5.0,
        elapsed_days: 1,
        scheduled_days: 1,
        reps: 2,
        lapses: 0,
        state: 2, // Review
        last_review: new Date(Date.now() - 86400000).toISOString(),
      },
    },
    {
      id: 'demo-card-sp-2',
      deck_id: 'demo-deck-spanish',
      front: 'Thank you',
      back: 'Gracias',
      link: '',
      created_at: now,
      tags: [],
      state: {
        id: 'state-sp-2',
        card_id: 'demo-card-sp-2',
        due: now,
        stability: 2.0,
        difficulty: 5.5,
        elapsed_days: 0,
        scheduled_days: 0,
        reps: 1,
        lapses: 0,
        state: 1, // Learning
        last_review: new Date(Date.now() - 300000).toISOString(),
      },
    },
    {
      id: 'demo-card-sp-3',
      deck_id: 'demo-deck-spanish',
      front: 'Goodbye',
      back: 'Adiós',
      link: '',
      created_at: now,
      tags: [],
      state: {
        id: 'state-sp-3',
        card_id: 'demo-card-sp-3',
        due: now,
        stability: 3.0,
        difficulty: 5.0,
        elapsed_days: 2,
        scheduled_days: 2,
        reps: 3,
        lapses: 0,
        state: 2, // Review
        last_review: new Date(Date.now() - 172800000).toISOString(),
      },
    },
    {
      id: 'demo-card-sp-4',
      deck_id: 'demo-deck-spanish',
      front: 'Please',
      back: 'Por favor',
      link: '',
      created_at: now,
      tags: [],
      // No state = New card
    },
    {
      id: 'demo-card-sp-5',
      deck_id: 'demo-deck-spanish',
      front: 'Yes / No',
      back: 'Sí / No',
      link: '',
      created_at: now,
      tags: [],
      // No state = New card
    },
  ],
  'demo-deck-capitals': [
    {
      id: 'demo-card-cap-1',
      deck_id: 'demo-deck-capitals',
      front: 'What is the capital of France?',
      back: 'Paris',
      link: '',
      created_at: now,
      tags: [],
      state: {
        id: 'state-cap-1',
        card_id: 'demo-card-cap-1',
        due: now,
        stability: 5.0,
        difficulty: 4.5,
        elapsed_days: 3,
        scheduled_days: 3,
        reps: 4,
        lapses: 0,
        state: 2, // Review
        last_review: new Date(Date.now() - 259200000).toISOString(),
      },
    },
    {
      id: 'demo-card-cap-2',
      deck_id: 'demo-deck-capitals',
      front: 'What is the capital of Japan?',
      back: 'Tokyo',
      link: '',
      created_at: now,
      tags: [],
      state: {
        id: 'state-cap-2',
        card_id: 'demo-card-cap-2',
        due: now,
        stability: 1.5,
        difficulty: 5.0,
        elapsed_days: 0,
        scheduled_days: 0,
        reps: 1,
        lapses: 0,
        state: 1, // Learning
        last_review: new Date(Date.now() - 600000).toISOString(),
      },
    },
    {
      id: 'demo-card-cap-3',
      deck_id: 'demo-deck-capitals',
      front: 'What is the capital of Brazil?',
      back: 'Brasília',
      link: '',
      created_at: now,
      tags: [],
      // New card
    },
    {
      id: 'demo-card-cap-4',
      deck_id: 'demo-deck-capitals',
      front: 'What is the capital of Australia?',
      back: 'Canberra',
      link: '',
      created_at: now,
      tags: [],
      // New card
    },
    {
      id: 'demo-card-cap-5',
      deck_id: 'demo-deck-capitals',
      front: 'What is the capital of Egypt?',
      back: 'Cairo',
      link: '',
      created_at: now,
      tags: [],
      // New card
    },
  ],
};

export const DEMO_TAGS: Record<string, Tag[]> = {
  'demo-deck-spanish': [],
  'demo-deck-capitals': [],
};

export const DEMO_STATS: StudyStats = {
  totalReviews: 15,
  reviewsLast24Hours: 5,
  reviewsLast7Days: 12,
  avgRating: 3.2,
  retentionRate: 0.85,
};

export function getDemoCalendar(): DueCalendarDay[] {
  const days: DueCalendarDay[] = [];
  const today = new Date();

  for (let i = 0; i < 14; i++) {
    const date = new Date(today);
    date.setDate(date.getDate() + i);
    const dateStr = date.toISOString().split('T')[0];

    // Simulate some due cards over the next 2 weeks
    const total = i === 0 ? 4 : Math.max(0, 3 - Math.floor(i / 3));

    days.push({
      date: dateStr,
      total,
      decks: total > 0 ? [
        { deck_id: 'demo-deck-spanish', deck_name: 'Spanish Basics', count: Math.ceil(total / 2) },
        { deck_id: 'demo-deck-capitals', deck_name: 'World Capitals', count: Math.floor(total / 2) },
      ].filter(d => d.count > 0) : [],
    });
  }

  return days;
}

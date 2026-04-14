-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Decks
CREATE TABLE decks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_decks_user_id ON decks(user_id);

-- Cards
CREATE TABLE cards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deck_id UUID NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    front TEXT NOT NULL,
    back TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_cards_deck_id ON cards(deck_id);

-- Card states (FSRS scheduling data)
-- state: 0=New, 1=Learning, 2=Review, 3=Relearning
CREATE TABLE card_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id UUID NOT NULL UNIQUE REFERENCES cards(id) ON DELETE CASCADE,
    due TIMESTAMPTZ NOT NULL,
    stability FLOAT DEFAULT 0,
    difficulty FLOAT DEFAULT 0,
    elapsed_days INT DEFAULT 0,
    scheduled_days INT DEFAULT 0,
    reps INT DEFAULT 0,
    lapses INT DEFAULT 0,
    state INT DEFAULT 0,
    last_review TIMESTAMPTZ
);

CREATE INDEX idx_card_states_due ON card_states(due);

-- Reviews (history log)
CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id UUID NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 4),
    reviewed_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_reviews_card_id ON reviews(card_id);

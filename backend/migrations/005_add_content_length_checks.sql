ALTER TABLE decks
    DROP CONSTRAINT IF EXISTS decks_description_length_check;

ALTER TABLE decks
    ADD CONSTRAINT decks_description_length_check
    CHECK (description IS NULL OR char_length(description) <= 100000);

ALTER TABLE cards
    DROP CONSTRAINT IF EXISTS cards_front_length_check,
    DROP CONSTRAINT IF EXISTS cards_back_length_check,
    DROP CONSTRAINT IF EXISTS cards_link_length_check;

ALTER TABLE cards
    ADD CONSTRAINT cards_front_length_check CHECK (char_length(front) <= 100000),
    ADD CONSTRAINT cards_back_length_check CHECK (char_length(back) <= 100000),
    ADD CONSTRAINT cards_link_length_check CHECK (char_length(link) <= 8192);

ALTER TABLE decks
ADD COLUMN IF NOT EXISTS new_card_front_template TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS new_card_back_template TEXT NOT NULL DEFAULT '';

ALTER TABLE decks
    DROP CONSTRAINT IF EXISTS decks_new_card_front_template_length_check,
    DROP CONSTRAINT IF EXISTS decks_new_card_back_template_length_check;

ALTER TABLE decks
    ADD CONSTRAINT decks_new_card_front_template_length_check
    CHECK (char_length(new_card_front_template) <= 100000),
    ADD CONSTRAINT decks_new_card_back_template_length_check
    CHECK (char_length(new_card_back_template) <= 100000);

-- Add link column to cards table
ALTER TABLE cards ADD COLUMN IF NOT EXISTS link TEXT DEFAULT '';

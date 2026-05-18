-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS picture_url TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS picture_url;

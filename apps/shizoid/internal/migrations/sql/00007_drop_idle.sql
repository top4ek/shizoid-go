-- +goose Up
ALTER TABLE chats DROP COLUMN idle_days;
ALTER TABLE chats DROP COLUMN idle_poked_at;

-- +goose Down
ALTER TABLE chats ADD COLUMN idle_days INTEGER;
ALTER TABLE chats ADD COLUMN idle_poked_at TIMESTAMPTZ;

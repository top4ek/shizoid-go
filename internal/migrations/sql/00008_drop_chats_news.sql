-- +goose Up
ALTER TABLE chats DROP COLUMN news;

-- +goose Down
ALTER TABLE chats ADD COLUMN news TEXT;

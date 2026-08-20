-- +goose Up
ALTER TABLE chats ADD COLUMN news TEXT;

-- +goose Down
ALTER TABLE chats DROP COLUMN news;

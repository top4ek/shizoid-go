-- +goose Up
UPDATE chats SET generation_mode = 2;
ALTER TABLE chats ALTER COLUMN generation_mode SET DEFAULT 2;

-- +goose Down
UPDATE chats SET generation_mode = 0 WHERE generation_mode = 2;
ALTER TABLE chats ALTER COLUMN generation_mode SET DEFAULT 0;

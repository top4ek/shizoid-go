-- +goose Up
UPDATE chats SET generation_mode = 0 WHERE generation_mode = 2;

-- +goose Down
-- no-op: cannot restore which chats were neural

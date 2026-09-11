-- +goose Up
ALTER TABLE participations ADD COLUMN greeted_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE participations DROP COLUMN greeted_at;

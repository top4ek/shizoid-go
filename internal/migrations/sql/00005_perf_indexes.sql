-- +goose Up
-- Serve FetchPair's "replies ORDER BY count DESC" without a sort and idle
-- queries filtering on (chat_id, active_at); the old single-column indexes
-- are prefixes of the new ones and become redundant.
CREATE INDEX index_replies_on_pair_id_count ON replies (pair_id, count DESC);
DROP INDEX index_replies_on_pair_id;
CREATE INDEX index_participations_on_chat_id_active_at ON participations (chat_id, active_at);
DROP INDEX index_participations_on_chat_id;

-- +goose Down
CREATE INDEX index_replies_on_pair_id ON replies (pair_id);
DROP INDEX index_replies_on_pair_id_count;
CREATE INDEX index_participations_on_chat_id ON participations (chat_id);
DROP INDEX index_participations_on_chat_id_active_at;

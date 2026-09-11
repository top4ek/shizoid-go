-- +goose Up
-- +goose StatementBegin
CREATE TABLE summary_jobs (
  id         BIGSERIAL PRIMARY KEY,
  chat_id    BIGINT      NOT NULL,
  kind       SMALLINT    NOT NULL,
  payload    JSONB       NOT NULL DEFAULT '{}'::jsonb,
  attempts   INTEGER     NOT NULL DEFAULT 0,
  run_after  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT summary_jobs_chat_kind_unique UNIQUE (chat_id, kind)
);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX index_summary_jobs_on_run_after ON summary_jobs (run_after);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS summary_jobs;
-- +goose StatementEnd

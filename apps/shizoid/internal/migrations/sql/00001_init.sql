-- +goose Up
-- +goose StatementBegin
CREATE TABLE chats (
  id              BIGINT PRIMARY KEY,
  kind            VARCHAR(32),
  random          SMALLINT NOT NULL DEFAULT 0,
  eightball       BOOLEAN NOT NULL DEFAULT false,
  greeting        BOOLEAN NOT NULL DEFAULT false,
  winner          VARCHAR,
  locale          VARCHAR(5) NOT NULL DEFAULT 'ru',
  generation_mode SMALLINT NOT NULL DEFAULT 0,
  title           VARCHAR,
  first_name      VARCHAR,
  last_name       VARCHAR,
  username        VARCHAR,
  active_at       TIMESTAMPTZ,
  idle_days       INTEGER,
  captcha_enabled_at TIMESTAMPTZ,
  captcha_greeting VARCHAR,
  system_prompt   TEXT,
  memory          TEXT,
  idle_poked_at   TIMESTAMPTZ,
  memory_summarized_at TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE users (
  id               BIGINT PRIMARY KEY,
  is_bot           BOOLEAN,
  first_name       VARCHAR,
  last_name        VARCHAR,
  username         VARCHAR,
  language_code    VARCHAR,
  captcha_solved_at TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE participations (
  id               BIGSERIAL PRIMARY KEY,
  chat_id          BIGINT NOT NULL,
  user_id          BIGINT NOT NULL,
  score            INTEGER NOT NULL DEFAULT 0,
  active_at        TIMESTAMPTZ,
  left_at          TIMESTAMPTZ,
  captcha_solved_at TIMESTAMPTZ,
  captcha_requested_at TIMESTAMPTZ,
  captcha_correct_emoji VARCHAR,
  captcha_message_id INTEGER,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT participations_chat_user_unique UNIQUE (chat_id, user_id)
);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX index_participations_on_chat_id ON participations (chat_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE words (
  id   BIGSERIAL PRIMARY KEY,
  word VARCHAR NOT NULL,
  CONSTRAINT words_word_unique UNIQUE (word)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE pairs (
  id        BIGSERIAL PRIMARY KEY,
  chat_id   BIGINT NOT NULL,
  first_id  BIGINT,
  second_id BIGINT,
  CONSTRAINT pairs_chat_first_second_unique UNIQUE NULLS NOT DISTINCT (chat_id, first_id, second_id)
);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX index_pairs_on_chat_id ON pairs (chat_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE replies (
  id      BIGSERIAL PRIMARY KEY,
  pair_id BIGINT NOT NULL REFERENCES pairs (id) ON DELETE CASCADE,
  word_id BIGINT,
  count   INTEGER NOT NULL DEFAULT 0,
  CONSTRAINT replies_pair_word_unique UNIQUE NULLS NOT DISTINCT (pair_id, word_id)
);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX index_replies_on_pair_id ON replies (pair_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE winners (
  id         BIGSERIAL PRIMARY KEY,
  chat_id    BIGINT NOT NULL,
  user_id    BIGINT NOT NULL,
  date       DATE NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT winners_chat_date_unique UNIQUE (chat_id, date)
);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX index_winners_on_chat_id ON winners (chat_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE greetings (
  id      BIGSERIAL PRIMARY KEY,
  chat_id BIGINT NOT NULL,
  text    VARCHAR NOT NULL,
  CONSTRAINT greetings_chat_unique UNIQUE (chat_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE messages (
  id         BIGSERIAL PRIMARY KEY,
  chat_id    BIGINT NOT NULL,
  user_id    BIGINT NOT NULL,
  text       VARCHAR NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX index_messages_on_chat_id_created_at ON messages (chat_id, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS greetings;
DROP TABLE IF EXISTS winners;
DROP TABLE IF EXISTS replies;
DROP TABLE IF EXISTS pairs;
DROP TABLE IF EXISTS words;
DROP TABLE IF EXISTS participations;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS chats;
-- +goose StatementEnd

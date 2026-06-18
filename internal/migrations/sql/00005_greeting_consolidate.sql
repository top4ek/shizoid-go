-- +goose Up
ALTER TABLE chats ADD COLUMN greeting_text TEXT;

UPDATE chats c
SET greeting_text = g.text
FROM greetings g
WHERE c.id = g.chat_id AND g.text IS NOT NULL AND g.text != '';

DROP TABLE greetings;
ALTER TABLE chats DROP COLUMN greeting;

ALTER TABLE participations ADD COLUMN greeting_message_id INTEGER;

-- +goose Down
ALTER TABLE participations DROP COLUMN greeting_message_id;

ALTER TABLE chats ADD COLUMN greeting BOOLEAN NOT NULL DEFAULT false;
UPDATE chats SET greeting = (greeting_text IS NOT NULL);

CREATE TABLE greetings (
  id      BIGSERIAL PRIMARY KEY,
  chat_id BIGINT NOT NULL,
  text    VARCHAR NOT NULL,
  CONSTRAINT greetings_chat_unique UNIQUE (chat_id)
);

INSERT INTO greetings (chat_id, text)
SELECT id, greeting_text FROM chats WHERE greeting_text IS NOT NULL;

ALTER TABLE chats DROP COLUMN greeting_text;

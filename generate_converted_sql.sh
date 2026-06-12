#!/usr/bin/env bash
# Generate converted.sql from a Ruby legacy database and optionally load it
# into a fresh Go database.
#
# Usage:
#   ./generate_converted_sql.sh --config build/dev/config.yaml \
#       --dump shizoid_production.dump --apply
#
# From an existing legacy database (podman postgres on the host):
#   ./generate_converted_sql.sh --config migrate-config.yaml \
#       --app-config build/dev/config.yaml \
#       --pg-container postgresql \
#       --skip-restore \
#       --legacy-dsn "host=127.0.0.1 user=postgres password=... dbname=shizoid_production" \
#       --binary ./shizoid --apply
#
# Re-import an existing converted.sql without regenerating:
#   ./generate_converted_sql.sh --config build/dev/config.yaml \
#       --apply-only --out scripts/converted.sql
#
# Requires: pg_restore, createdb, dropdb, psql, go (for -migrate-only)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
LEGACY_DB="${LEGACY_DB:-shizoid_legacy}"
OUT="${OUT:-$ROOT/scripts/converted.sql}"
CONFIG=""
APP_CONFIG=""
DUMP=""
BINARY=""
PG_CONTAINER=""
GRANT_USER=""
SKIP_RESTORE=0
SKIP_DROP=0
APPLY=0
APPLY_ONLY=0
LEGACY_DSN=""

usage() {
  sed -n '2,20p' "$0" >&2
  echo "  --apply          drop target DB, run goose, import generated SQL" >&2
  echo "  --apply-only     skip generation; only reset target DB and import --out" >&2
  echo "  --binary PATH    shizoid binary for -migrate-only (default: go run ./cmd/app)" >&2
  echo "  --pg-container   run psql/dropdb/createdb via podman exec (import via podman cp)" >&2
  echo "  --grant-user     grant table access after import (app DB user)" >&2
  echo "  --app-config     read --grant-user from database.user in bot config" >&2
  echo "  --skip-drop      with --apply: skip dropdb/createdb (debug only)" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config) CONFIG="$2"; shift 2 ;;
    --app-config) APP_CONFIG="$2"; shift 2 ;;
    --dump) DUMP="$2"; shift 2 ;;
    --legacy-db) LEGACY_DB="$2"; shift 2 ;;
    --out) OUT="$2"; shift 2 ;;
    --binary) BINARY="$2"; shift 2 ;;
    --pg-container) PG_CONTAINER="$2"; shift 2 ;;
    --grant-user) GRANT_USER="$2"; shift 2 ;;
    --skip-restore) SKIP_RESTORE=1; shift ;;
    --legacy-dsn) LEGACY_DSN="$2"; SKIP_RESTORE=1; shift 2 ;;
    --apply) APPLY=1; shift ;;
    --apply-only) APPLY_ONLY=1; APPLY=1; shift ;;
    --skip-drop) SKIP_DROP=1; shift ;;
    -h|--help) usage ;;
    *) echo "unknown arg: $1" >&2; usage ;;
  esac
done

if [[ "$APPLY" -eq 1 && -z "$CONFIG" ]]; then
  echo "--config is required with --apply" >&2
  exit 1
fi

if [[ "$APPLY_ONLY" -eq 1 && ! -f "$OUT" ]]; then
  echo "converted SQL not found: $OUT" >&2
  exit 1
fi

if [[ -n "$PG_CONTAINER" ]] && ! command -v podman >/dev/null; then
  echo "podman is required with --pg-container" >&2
  exit 1
fi

parse_yaml_db() {
  local file="$1"
  python3 - "$file" <<'PY'
import sys

section = None
values = {}
for raw in open(sys.argv[1], encoding="utf-8"):
    line = raw.rstrip()
    if not line or line.lstrip().startswith("#"):
        continue
    if line.endswith(":") and not line.startswith(" "):
        section = line[:-1]
        continue
    if section != "database" or ":" not in line:
        continue
    key, val = line.strip().split(":", 1)
    values[key] = val.strip().strip('"').strip("'")

host = values.get("host", "localhost")
if host in {"postgres", "db", "postgresql"}:
    host = "127.0.0.1"
print(host)
print(values.get("port", "5432"))
print(values.get("user", "shizoid"))
print(values.get("password", ""))
print(values.get("name", "shizoid"))
PY
}

if [[ -n "$CONFIG" ]]; then
  mapfile -t DB < <(parse_yaml_db "$CONFIG")
  DB_HOST="${DB[0]}"
  DB_PORT="${DB[1]}"
  DB_USER="${DB[2]}"
  DB_PASS="${DB[3]}"
  DB_NAME="${DB[4]}"
else
  DB_HOST="${DB_HOST:-127.0.0.1}"
  DB_PORT="${DB_PORT:-5432}"
  DB_USER="${DB_USER:-shizoid}"
  DB_PASS="${DB_PASS:-}"
  DB_NAME="${DB_NAME:-shizoid}"
fi

if [[ -n "$APP_CONFIG" ]]; then
  mapfile -t APP_DB < <(parse_yaml_db "$APP_CONFIG")
  GRANT_USER="${GRANT_USER:-${APP_DB[2]}}"
fi

export PGPASSWORD="$DB_PASS"

if [[ -z "$LEGACY_DSN" ]]; then
  LEGACY_DSN="host=$DB_HOST port=$DB_PORT user=$DB_USER password=$DB_PASS dbname=$LEGACY_DB"
fi

if [[ -z "$BINARY" ]]; then
  BINARY=(go run "$ROOT/cmd/app")
else
  BINARY=("$BINARY")
fi

pg_psql() {
  if [[ -n "$PG_CONTAINER" ]]; then
    podman exec -i -e PGPASSWORD "$PG_CONTAINER" psql -h 127.0.0.1 -U "$DB_USER" -v ON_ERROR_STOP=1 "$@"
  else
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -v ON_ERROR_STOP=1 "$@"
  fi
}

pg_psql_dsn() {
  local dsn="$1"
  shift
  if [[ -n "$PG_CONTAINER" ]]; then
    podman exec -i "$PG_CONTAINER" psql "$dsn" -v ON_ERROR_STOP=1 "$@"
  else
    psql "$dsn" -v ON_ERROR_STOP=1 "$@"
  fi
}

pg_dropdb() {
  if [[ -n "$PG_CONTAINER" ]]; then
    podman exec -e PGPASSWORD "$PG_CONTAINER" dropdb -h 127.0.0.1 -U "$DB_USER" "$@"
  else
    dropdb -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$@"
  fi
}

pg_createdb() {
  if [[ -n "$PG_CONTAINER" ]]; then
    podman exec -e PGPASSWORD "$PG_CONTAINER" createdb -h 127.0.0.1 -U "$DB_USER" "$@"
  else
    createdb -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$@"
  fi
}

pg_restore() {
  if [[ -n "$PG_CONTAINER" ]]; then
    podman exec -i -e PGPASSWORD "$PG_CONTAINER" pg_restore -h 127.0.0.1 -U "$DB_USER" "$@"
  else
    pg_restore -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$@"
  fi
}

grant_app_user() {
  local user="$1"
  [[ -n "$user" ]] || return 0
  if [[ "$user" == "$DB_USER" ]]; then
    return 0
  fi
  echo "granting access to $user..."
  pg_psql -d "$DB_NAME" <<-SQL
		ALTER DATABASE ${DB_NAME} OWNER TO ${user};
		GRANT ALL ON SCHEMA public TO ${user};
		GRANT ALL ON ALL TABLES IN SCHEMA public TO ${user};
		GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO ${user};
		ALTER DEFAULT PRIVILEGES FOR ROLE ${DB_USER} IN SCHEMA public GRANT ALL ON TABLES TO ${user};
		ALTER DEFAULT PRIVILEGES FOR ROLE ${DB_USER} IN SCHEMA public GRANT ALL ON SEQUENCES TO ${user};
SQL
}

emit_copy() {
  local label="$1"
  local table="$2"
  local columns="$3"
  local query="$4"

  echo "-- $label"
  echo "COPY $table ($columns) FROM stdin;"
  pg_psql_dsn "$LEGACY_DSN" -c "COPY ($query) TO STDOUT"
  echo '\.'
  echo
}

apply_to_target() {
  echo "applying $OUT to $DB_NAME on $DB_HOST..."

  local db_owner="${GRANT_USER:-$DB_USER}"

  if [[ "$SKIP_DROP" -eq 0 ]]; then
    echo "terminating connections to $DB_NAME..."
    pg_psql -d postgres -c \
      "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$DB_NAME' AND pid <> pg_backend_pid();" \
      >/dev/null || true
    echo "recreating database $DB_NAME..."
    pg_dropdb --if-exists "$DB_NAME"
    pg_createdb -O "$db_owner" "$DB_NAME"
  fi

  echo "running goose migrations..."
  (cd "$ROOT" && "${BINARY[@]}" -config "$CONFIG" -migrate-only)

  echo "importing data..."
  if [[ -n "$PG_CONTAINER" ]]; then
    local container_sql="/tmp/converted.sql"
    echo "copying SQL into $PG_CONTAINER:$container_sql..."
    podman cp "$OUT" "$PG_CONTAINER:$container_sql"
    pg_psql -d "$DB_NAME" -f "$container_sql"
    podman exec "$PG_CONTAINER" rm -f "$container_sql"
  else
    pg_psql -d "$DB_NAME" -f "$OUT"
  fi

  grant_app_user "$GRANT_USER"

  local null_first
  null_first=$(pg_psql -d "$DB_NAME" -tAc "SELECT count(*) FROM pairs WHERE first_id IS NULL")
  echo "null-first pairs in target: $null_first"
  if [[ "$null_first" -eq 0 ]]; then
    echo "error: no null-first pairs imported — check legacy data and LEFT JOIN in pairs query" >&2
    exit 1
  fi
  echo "apply done"
}

if [[ "$APPLY_ONLY" -eq 0 ]]; then
  if [[ "$SKIP_RESTORE" -eq 0 ]]; then
    [[ -n "$DUMP" ]] || { echo "--dump is required unless --skip-restore or --apply-only" >&2; exit 1; }
    [[ -f "$DUMP" ]] || { echo "dump not found: $DUMP" >&2; exit 1; }
    echo "restoring dump into $LEGACY_DB..."
    pg_dropdb --if-exists "$LEGACY_DB"
    pg_createdb "$LEGACY_DB"
    pg_restore --no-owner --no-acl -d "$LEGACY_DB" "$DUMP"
  fi

  legacy_null_first=$(pg_psql_dsn "$LEGACY_DSN" -tAc \
    "SELECT count(*) FROM pairs WHERE first_id IS NULL AND data_bank_id IS NULL")
  echo "legacy null-first pairs: $legacy_null_first"

  mkdir -p "$(dirname "$OUT")"
  echo "writing $OUT ..."

  {
    cat <<'HEADER'
-- Generated by generate_converted_sql.sh
-- Apply to an empty Go database (goose migrations already applied).
-- Bayan / data_banks corpus pairs are skipped; duplicate words are merged.

BEGIN;
SET LOCAL synchronous_commit = off;

HEADER

    emit_copy "words (deduplicated)" "words" "id, word" \
      "SELECT DISTINCT ON (word) id, word FROM words ORDER BY word, id"

    emit_copy "chats" "chats" \
      "id, kind, random, eightball, greeting, winner, locale, generation_mode, title, first_name, last_name, username, active_at, created_at" \
      "SELECT c.telegram_id AS id, c.kind, c.random, c.eightball, c.greeting, c.winner, c.locale, 0::smallint AS generation_mode, c.title, c.first_name, c.last_name, c.username, c.active_at, c.created_at FROM chats c"

    emit_copy "users" "users" \
      "id, is_bot, first_name, last_name, username, language_code, captcha_solved_at, created_at, updated_at" \
      "SELECT id, is_bot, first_name, last_name, username, language_code, NULL::timestamptz AS captcha_solved_at, created_at, updated_at FROM users"

    emit_copy "participations" "participations" \
      "id, chat_id, user_id, score, active_at, left_at, created_at, updated_at" \
      "SELECT p.id, c.telegram_id AS chat_id, p.user_id, p.score, p.active_at, CASE WHEN p.\"left\" THEN COALESCE(p.active_at, p.updated_at) END AS left_at, p.created_at, p.updated_at FROM participations p JOIN chats c ON c.id = p.chat_id"

    emit_copy "pairs (deduplicated)" "pairs" "id, chat_id, first_id, second_id" \
      "WITH word_map AS (SELECT id AS old_id, MIN(id) OVER (PARTITION BY word) AS new_id FROM words) SELECT DISTINCT ON (c.telegram_id, f1.new_id, f2.new_id) p.id, c.telegram_id AS chat_id, f1.new_id AS first_id, f2.new_id AS second_id FROM pairs p JOIN chats c ON c.id = p.chat_id LEFT JOIN word_map f1 ON f1.old_id = p.first_id LEFT JOIN word_map f2 ON f2.old_id = p.second_id WHERE p.data_bank_id IS NULL ORDER BY c.telegram_id, f1.new_id, f2.new_id, p.id"

    emit_copy "replies (aggregated)" "replies" "pair_id, word_id, count" \
      "WITH word_map AS (SELECT id AS old_id, MIN(id) OVER (PARTITION BY word) AS new_id FROM words), pair_map AS (SELECT p.id AS old_id, FIRST_VALUE(p.id) OVER (PARTITION BY c.telegram_id, f1.new_id, f2.new_id ORDER BY p.id) AS new_id FROM pairs p JOIN chats c ON c.id = p.chat_id LEFT JOIN word_map f1 ON f1.old_id = p.first_id LEFT JOIN word_map f2 ON f2.old_id = p.second_id WHERE p.data_bank_id IS NULL) SELECT pm.new_id AS pair_id, wm.new_id AS word_id, SUM(r.count) AS count FROM replies r JOIN pairs p ON p.id = r.pair_id AND p.data_bank_id IS NULL JOIN pair_map pm ON pm.old_id = r.pair_id LEFT JOIN word_map wm ON wm.old_id = r.word_id GROUP BY pm.new_id, wm.new_id"

    emit_copy "winners" "winners" "id, chat_id, user_id, date, created_at" \
      "SELECT w.id, c.telegram_id AS chat_id, w.user_id, w.date, w.created_at::timestamptz AS created_at FROM winners w JOIN chats c ON c.id = w.chat_id"

    emit_copy "greetings" "greetings" "id, chat_id, text" \
      "SELECT g.id, c.telegram_id AS chat_id, g.text FROM greetings g JOIN chats c ON c.id = g.chat_id"

    cat <<'FOOTER'
SELECT setval('words_id_seq',          (SELECT COALESCE(MAX(id), 1) FROM words));
SELECT setval('pairs_id_seq',          (SELECT COALESCE(MAX(id), 1) FROM pairs));
SELECT setval('replies_id_seq',        (SELECT COALESCE(MAX(id), 1) FROM replies));
SELECT setval('participations_id_seq', (SELECT COALESCE(MAX(id), 1) FROM participations));
SELECT setval('winners_id_seq',        (SELECT COALESCE(MAX(id), 1) FROM winners));
SELECT setval('greetings_id_seq',      (SELECT COALESCE(MAX(id), 1) FROM greetings));

COMMIT;
FOOTER

  } > "$OUT"

  ls -lh "$OUT"
  if [[ "$legacy_null_first" -gt 0 ]]; then
    echo "generated SQL from legacy with $legacy_null_first null-first pairs"
  else
    echo "warning: legacy has no null-first pairs" >&2
  fi
fi

if [[ "$APPLY" -eq 1 ]]; then
  apply_to_target
fi

echo "done"

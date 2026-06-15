# shizoid-go

A Telegram chatter bot in Go — a rewrite of the Ruby [top4ek/shizoid](https://github.com/top4ek/shizoid).

It learns from chat messages and replies with a modified Markov-chain generator,
plus extras: 8-ball, /me, daily "winner" draw, captcha for new members, per-chat
greetings, and more. Localization is per-chat.

WARNING: neuroslop ahead (Opus and Composer are used).

## How it works

- Every incoming message first ensures the chat, user and participation rows
  exist (one transaction), then learning/scoring happens in background goroutines.
- Generation has switchable per-chat Markov modes via `/generation`:
  - `classic` — trigram Markov walk (pair of words → reply); chat admins may set.
  - `simplified` — bigram walk (only the second word seeds the next reply; more nonsensical); chat admins may set.
- When `neural.reply` providers are configured, every reply first tries an
  OpenAI-compatible LLM; on failure (unavailable, error, or no slots) it falls
  back to the chat's Markov mode (`classic` or `simplified`).
- The Markov "context" for reply fallback is derived from the
  most recent messages stored per chat (byte budget = max `context_size` across
  `neural.reply` providers, or 16 KiB when no reply providers are configured).

Migrations and locale strings are **embedded in the binary** at build time — you
do not need to ship SQL or YAML files separately.

## Requirements

- Go 1.25+
- PostgreSQL 18+ (uses `UNIQUE NULLS NOT DISTINCT`)
- Docker (optional, for production deployment)

## Configuration

Application settings live in a YAML file next to the binary (default name: `config.yaml`).
See [`build/prod/config.yaml-example`](build/prod/config.yaml-example) for production
and [`build/dev/config.yaml-example`](build/dev/config.yaml-example) for local development.

| Section | Key | Default | Purpose |
| --- | --- | --- | --- |
| (top-level) | `app_env` | `production` | `development` or `dev` for console logs; otherwise JSON |
| (top-level) | `log_level` | — | `debug`, `info`, `warn`, `error` |
| `telegram` | `token` | — | Bot token (required) |
| `app` | `bot_owners` | — | Owner Telegram user IDs |
| `database` | `*` | — | Postgres host/port/name/user/password |
| `app` | `generation_mode` | `classic` | Default mode for new chats |
| `app` | `winner_cron` | `20 4 * * *` | Daily winner draw (04:20) |
| `app` | `memory_cron` | `0 */6 * * *` | Memory summarization for all active chats (messages since last `memory_summarized_at`) |
| `app` | `allow_to_all` | `false` | Reply in all chats without `/start` |
| `app` | `app_prompt` / `summary_prompt` | see example | Neural system / memory prompts |
| `telegram` | `webhook_url` | — | Webhook mode URL; empty = long polling (`deleteWebhook` on startup) |
| `telegram` | `webhook_secret_token` | — | Secret for webhook requests (`setWebhook` + header check); auto-generated in webhook mode if omitted |
| `sentry` | `dsn` | — | Enables Sentry when set |
| `neural` | `reply` / `summary` | — | Provider fallback chains for LLM replies and memory summarization |
| `neural.*` | `context_size` | — | Per-model UTF-8 byte budget for API payload; max across `reply` caps DB history; max across `summary` caps memory input |
| `neural.*` | `sampling` | — | Optional chat/completions sampling (`temperature`, `top_p`, `top_k`, `min_p`, `presence_penalty`, `repetition_penalty`; sent as `repeat_penalty` to llama.cpp) |

Pass `-config path/to/config.yaml` if the file is not named `config.yaml`.

Migrations run automatically on startup (managed by [goose](https://github.com/pressly/goose)).

## Production (Docker)

What you need before starting:

1. A Telegram bot token from [@BotFather](https://t.me/BotFather)
2. A server with Docker and Docker Compose
3. Your Telegram user ID in `app.bot_owners` (send `/ids` to the bot after `/start`)

Steps:

```bash
cd build/prod
cp config.yaml-example config.yaml   # edit: telegram.token, database.password, bot_owners
cp .env-example .env                 # edit: POSTGRES_PASSWORD (must match config.yaml)
docker compose pull                  # or build locally (see below)
docker compose up -d
```

Then open your group chat in Telegram and send `/start` to activate the bot.

**Webhook mode:** set `telegram.webhook_url` in `config.yaml`, expose `app.bind_to`
(default `8095`) on the host, and add a `ports` mapping to `docker-compose.yaml`.
On startup the bot calls Telegram `setWebhook` with that URL and a
`webhook_secret_token` (auto-generated if omitted). With an empty `webhook_url` it calls `deleteWebhook`
and runs long polling.

**Update to a new version:**

```bash
cd build/prod
docker compose pull
docker compose up -d
```

The running version (git commit) is shown in `/status`.

**Build the image locally:**

```bash
docker build -f build/prod/Containerfile \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  -t top4ek/shizoid-go .
```

Official images are published to [Docker Hub](https://hub.docker.com/r/top4ek/shizoid-go) on every push to `main`.

## Development (Docker)

Hot reload with reflex + Delve debugger:

```bash
cp build/dev/config.yaml-example build/dev/config.yaml   # then edit values
cp build/dev/.env-example build/dev/.env                 # postgres + llama only
docker compose up --build
```

Docker infra (`postgres`, `llama`) uses `build/dev/.env` for `POSTGRES_*` and `LLAMA_ARG_*` variables.

## Run locally (without Docker)

```bash
cp build/dev/config.yaml-example build/dev/config.yaml
go run ./cmd/app -config build/dev/config.yaml
```

## Data migration (Ruby → Go)

One-off script [`generate_converted_sql.sh`](generate_converted_sql.sh) — not part of
the Go app. Bayan / `data_banks` corpus pairs are skipped; duplicate words are merged.

Each `--apply` run **drops and recreates** the target database (`database.name` from
config), runs goose migrations, then imports data. Stop the bot before applying on
production. The legacy source database is never modified.

```bash
# Full cycle: restore dump → generate SQL → apply to target DB
./generate_converted_sql.sh \
  --config build/dev/config.yaml \
  --dump shizoid_production.dump \
  --apply

# From an already-restored legacy database (e.g. shizoid_production on the server)
# Use a separate migrate config with a superuser (postgres) for dropdb/createdb.
# --app-config supplies the bot's database.user for GRANT after import.
./generate_converted_sql.sh \
  --config migrate-config.yaml \
  --app-config build/prod/config.yaml \
  --pg-container postgresql \
  --skip-restore \
  --legacy-dsn "host=127.0.0.1 user=postgres password=... dbname=shizoid_production" \
  --binary ./shizoid \
  --apply

# Re-import an existing converted.sql without regenerating
./generate_converted_sql.sh \
  --config migrate-config.yaml \
  --app-config build/prod/config.yaml \
  --pg-container postgresql \
  --apply-only \
  --out scripts/converted.sql
```

`--legacy-dsn` is a PostgreSQL connection string to the **source** Ruby database.
When omitted, the script reads from `shizoid_legacy` on the same host as `database`
in config.

| Flag | Purpose |
| --- | --- |
| `--pg-container` | Run `psql`/`dropdb`/`createdb` via `podman exec`; import via `podman cp` |
| `--app-config` | Read bot `database.user` and grant access after import |
| `--grant-user` | Same as `--app-config` but explicit (overrides) |
| `--binary` | Compiled `shizoid` for `-migrate-only` instead of `go run` |

When import runs as `postgres` but the bot connects as `shizoid`, pass
`--app-config` (or `--grant-user shizoid`) so the app can read the tables.

Schema-only migration (no data import):

```bash
go run ./cmd/app -config build/dev/config.yaml -migrate-only
```

## Test

```bash
go test ./...
```

In the dev container, `reflex` re-runs the package's tests on every file change.

## Develop

- Handlers live in `internal/handlers/<name>`; register them in
  `internal/handlers/handlers.go`.
- Data access is in `internal/models` (raw SQL, no ORM).
- Text generation/learning is in `internal/generator`.
- Localized strings are embedded YAML in `internal/locale/locales/`.
- Schema changes: add a new goose migration in `internal/migrations/sql/`.

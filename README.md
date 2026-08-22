# shizoid-go

A Telegram chatter bot in Go — a rewrite of the Ruby [top4ek/shizoid](https://github.com/top4ek/shizoid).

It learns from chat messages and replies with a modified Markov-chain generator,
plus extras: 8-ball, /me, daily "winner" draw, captcha for new members, per-chat
greetings, and more. Localization is per-chat.

WARNING: neuroslop ahead (Opus and Composer are used).

## How it works

- Every incoming message first ensures the chat, user and participation rows
  exist (one transaction), then learning/scoring happens in background goroutines.
- Generation has switchable per-chat modes via `/generation`:
  - `neural` (default) — when `neural.reply` providers are configured, tries an
    OpenAI-compatible LLM first; on failure (unavailable, error, or no slots)
    falls back to classic Markov.
  - `classic` — trigram Markov walk (pair of words → reply); no LLM.
  - `simplified` — bigram walk (only the second word seeds the next reply; more
    nonsensical); no LLM.
- The Markov "context" for reply fallback is derived from the
  most recent messages stored per chat (byte budget = max `context_size` across
  `neural.reply` providers, or 16 KiB when no reply providers are configured).

Migrations and locale strings are **embedded in the binary** at build time — you
do not need to ship SQL or YAML files separately.

### Summary queue

Work that needs the `neural.summary` chain (the daily winner announcement and
chat memory summarization) is never run inline, because the model it needs may
be busy, rate-limited or down.
Instead each unit of work is written to the `summary_jobs` table first, and a
worker (`summary_queue_cron`, `@every 1m`) drains it:

- One pending job per chat per kind.
  Re-queueing the same kind replaces the pending one, so a fresh draw supersedes
  a stale one rather than queueing behind it.
- A failed job is retried with exponential backoff - 1m, 2m, 4m, 8m, then every
  10m - and one failure stops the rest of the drain, since the chain is a single
  backend and the jobs behind it would only rediscover the same outage.
- Claiming a job leases it for 10 minutes rather than removing it, so a crash
  mid-job makes it due again instead of losing it.
- A job is only ever given up on when it expires (20 h for an announcement, 24 h
  for a memory summary), which is logged at `error`.

Consequences worth knowing:

- The winner announcement waits **as a whole**: with a summary chain configured
  nothing is posted at 01:20 if the model is unreachable, and the full message
  (ceremony, result line and year table) goes out when it answers.
  Without a summary chain the plain announcement is posted immediately as before.
- That wait is bounded: the model gets the first **2 hours**, after which the
  announcement is posted without a ceremony - just the result line and the year
  table - and the drop is logged at `warn`.
  So a draw is always announced the same day, whatever the model does.
- While a summary chain is configured, the daily message prune keeps everything
  newer than `chats.memory_summarized_at`, so a long outage cannot delete history
  before the summarizer has read it.

## Requirements

- Go 1.27+
- PostgreSQL 18+ (uses `UNIQUE NULLS NOT DISTINCT`); the app connects via [pgx](https://github.com/jackc/pgx)
- Docker (optional: production deployment and integration tests)

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
| `app` | `generation_mode` | `neural` | Default mode for new chats |
| `app` | `bind_to` | `3000` | Webhook/health HTTP port (prod example uses `8095`) |
| `app` | `locale` | `ru` | Default locale for new chats |
| `app` | `winner_cron` | `20 1 * * *` | Daily winner draw (01:20); the announcement is written by the `summary` chain when one is configured, and queued until it answers |
| `app` | `captcha_cron` | `@every 1m` | Expiry sweep for pending captchas |
| `app` | `memory_cron` | `0 */3 * * *` | Queues memory summarization for all active chats (messages since last `memory_summarized_at`) |
| `app` | `summary_queue_cron` | `@every 1m` | Drains the deferred `summary` chain work (see [Summary queue](#summary-queue)) |
| `app` | `allow_to_all` | `false` | Reply in all chats without `/start` |
| `app` | `prompts.*` | see example | Named blocks the system prompts are assembled from; shared blocks (`chat_role`, `chat_format`, `chat_length`, `telegram_markup`, `precedence`) reach every prompt that uses them |
| `telegram` | `webhook_url` | — | Webhook mode URL; empty = long polling (`deleteWebhook` on startup) |
| `telegram` | `webhook_secret_token` | — | Secret for webhook requests (`setWebhook` + header check); auto-generated in webhook mode if omitted |
| `sentry` | `dsn` | — | Enables Sentry when set |
| `neural` | `reply` / `summary` | — | Provider fallback chains for LLM replies and for memory summarization plus the daily winner announcement |
| `neural.*` | `context_size` | — | Per-model UTF-8 byte budget for API payload; max across `reply` caps DB history; max across `summary` caps memory input |
| `neural.*` | `sampling` | — | Optional chat/completions sampling (`temperature`, `top_p`, `top_k`, `min_p`, `presence_penalty`, `repetition_penalty`; sent as `repeat_penalty` to llama.cpp) |

### Retired config keys

These `app` keys were removed and are now **silently ignored** — an unknown key does
not fail the load, so a config still carrying one falls back to the built-in default
without any warning. Check a deployed `config.yaml` when upgrading past this change.

| Removed key (and env var) | Replacement |
| --- | --- |
| `app_prompt` / `APP_APP_PROMPT` | `app.prompts.chat_role`, `chat_memory`, `chat_format`, `telegram_markup`, `chat_length`, `precedence` |
| `summary_prompt` / `APP_SUMMARY_PROMPT` | `app.prompts.summary_role`, `summary_rules`, `summary_format` |
| `idle_prompt` / `APP_IDLE_PROMPT` | none — the idle/poke feature was removed |
| `idle_cron` / `APP_IDLE_CRON` | none — the idle/poke feature was removed |
| `news_cron` / `APP_NEWS_CRON` | none — the daily news issue was removed |
| `prompts.news_role` / `APP_PROMPT_NEWS_ROLE` | `app.prompts.winner_role` |
| `prompts.news_source` / `APP_PROMPT_NEWS_SOURCE` | `app.prompts.winner_data` |
| `prompts.news_format` / `APP_PROMPT_NEWS_FORMAT` | `app.prompts.winner_format` |
| `prompts.news_tone` / `APP_PROMPT_NEWS_TONE` | `app.prompts.winner_tone` |

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
(`8095` in the prod example) on the host, and add a `ports` mapping to `docker-compose.yaml`.
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

Developer commands are defined in [`Taskfile.yml`](Taskfile.yml) — the single source
of truth, run locally and in CI via [Task](https://taskfile.dev). See `task --list`
for everything available.

Hot reload with reflex + Delve debugger:

```bash
task dev
```

On first run it copies `build/dev/.env-example` to `build/dev/.env` (and the container
entrypoint copies `config.yaml-example` to `config.yaml`) — edit both with your values
(`telegram.token`, `POSTGRES_*`, `LLAMA_ARG_*`). `task dev-down` stops the stack.

Without Task: `docker compose up --build` after copying the two files yourself.
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
task              # build + vet + test
task test         # unit + integration (integration spins up Postgres via testcontainers, needs docker)
task test-short   # unit tests only
task ci           # everything CI runs: gofmt check, go vet, golangci-lint, go test -race, govulncheck
```

CI runs the same task commands, so a green `task ci` locally means a green pipeline
(`task lint-install` installs the pinned golangci-lint if you don't have it).
Without Task: `go test ./...` / `go test -short ./...`.

In the dev container, `reflex` re-runs the package's tests on every file change.

## Develop

- Handlers live in `internal/handlers/<name>`; register them in
  `internal/handlers/handlers.go` and declare the required permission
  (`roleEveryone`/`roleAdmin`/`roleOwner`) plus ready/enabled flags there —
  the registry gate enforces them centrally.
- Data access is in `internal/models` (raw SQL over pgx, no ORM); repositories
  hang off `models.Store`, reachable in handlers via `app.Store()`.
- Text generation/learning is in `internal/generator`.
- Localized strings are embedded YAML in `internal/locale/locales/`.
- Schema changes: add a new goose migration in `internal/migrations/sql/`.

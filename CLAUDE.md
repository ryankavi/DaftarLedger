# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

Run app (also runs migrations on startup):

```bash
go run ./main.go
```

Build / vet / format:

```bash
go build ./...
go vet ./...
gofmt -w .
```

Tests (none exist yet; `-run` filters a single test by name once added):

```bash
go test ./...
go test ./internal/db -run TestName
```

## Database (Postgres 16 in Docker, port 5433)

Port 5433 is deliberate — avoids collision with a native Postgres on 5432. The connection string in `main.go` is hard-coded to `localhost:5433` / `postgres` / `secret` / `gopgtest`.

First-time setup:

```bash
docker run --name pg-container -e POSTGRES_PASSWORD=secret -p 5433:5432 -v payclone_pgdata:/var/lib/postgresql/data -d postgres:16
docker exec pg-container createdb -U postgres gopgtest
```

Subsequent runs: `docker start pg-container`.

Reset: `docker rm -f pg-container && docker volume rm payclone_pgdata`.

psql: `docker exec -it pg-container psql -U postgres` then `\c gopgtest`.

## Architecture

Single-binary Go service, currently bootstrap-only (no HTTP layer yet). Flow:

1. `main.go` opens a `*sql.DB` with `lib/pq` and hands it to `internal/db.SetUpDB`.
2. `internal/db/db.go` pings, then runs `golang-migrate` with source `file://migrations` and the same live `*sql.DB` wrapped via `postgres.WithInstance`. Migrations run on every startup; `migrate.ErrNoChange` is swallowed.
3. Migration files live in `migrations/` as numbered pairs (`NNNNNN_name.up.sql` / `.down.sql`). The path is relative, so the app must be run from the repo root for `file://migrations` to resolve.

Schema (ledger-style double-entry):

- `users` — identity.
- `accounts` — owned by a user, typed, currency-scoped.
- `transactions` — header row with `idempotency_key UNIQUE`, `transaction_status` enum (`PENDING|POSTED|REVERSED`), `posted_at`.
- `entries` — line items referencing `accounts(account_id)` and a `transaction_id`, with `entry_direction` enum (`DEBIT|CREDIT`) and `amount BIGINT` (store minor units, not floats). Sum of debits/credits per transaction is the balancing invariant to preserve when writing transaction logic.

`internal/models/models.go` holds plain Go structs mirroring the schema; nullable SQL columns are `*string`. DB enum columns are represented by named string types (`AccountType`, `EntryDirection`, `TransactionStatus`) with exported consts (e.g. `DirectionDebit`, `StatusPosted`) — use the consts, not bare string literals, at insert/compare sites. No ORM, no repository layer yet — add queries against the raw `*sql.DB`.

## Conventions worth knowing

- Connection string is inline in `main.go` with a `// Move to env var` TODO — treat env-var extraction as expected next step rather than a refactor.
- New schema changes go in a new numbered migration pair; don't edit existing ones (migrate tracks applied version in `schema_migrations`).
- Enum types are created via `DO $$ BEGIN ... EXCEPTION WHEN duplicate_object ... $$` so migrations stay idempotent against partially-applied state.

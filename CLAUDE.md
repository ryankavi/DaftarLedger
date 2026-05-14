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
go test ./internal/repository -run TestName
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

## On session start
                   
Read `PLAN.md` before answering the first user prompt of the session.

## Architecture

Single-binary Go service, currently bootstrap-only (no HTTP layer yet). Layered:

- `internal/db/` — bootstrap only. `SetUpDB` pings + runs migrations. No queries.
- `internal/repository/` — per-entity CRUD against `*sql.DB`. One file per entity (`users.go`, `accounts.go`, `transactions.go`, `entries.go`). Primitives only — no multi-row orchestration, no business rules.
- `internal/service/` — orchestration. `transfer.go` (currently scaffolded, commented out) owns the `BEGIN/COMMIT` across the txn header + 2 entries, idempotency precheck, account locking (`SELECT ... FOR UPDATE` in canonical order), currency validation.
- `internal/models/` — plain structs mirroring the schema.

Startup flow:

1. `main.go` opens a `*sql.DB` with `lib/pq` and hands it to `internal/db.SetUpDB`.
2. `internal/db/db.go` pings, then runs `golang-migrate` with source `file://migrations` and the same live `*sql.DB` wrapped via `postgres.WithInstance`. Migrations run on every startup; `migrate.ErrNoChange` is swallowed.
3. Migration files live in `migrations/` as numbered pairs (`NNNNNN_name.up.sql` / `.down.sql`). The path is relative, so the app must be run from the repo root for `file://migrations` to resolve.

Schema (ledger-style double-entry):

- `users` — identity.
- `accounts` — owned by a user, typed, currency-scoped.
- `transactions` — header row. `idempotency_key TEXT UNIQUE NOT NULL`, `external_id UUID` (nullable, **informational tag — not unique**, may appear on multiple txns), `transaction_status` enum (`PENDING|POSTED|REVERSED`), `posted_at TIMESTAMPTZ` (NULL until status flips to POSTED). Clock source for `posted_at` is the DB (`NOW()` in the UPDATE), not the service — avoids instance clock drift.
- `entries` — line items referencing `accounts(account_id)` and a `transaction_id`, with `entry_direction` enum (`DEBIT|CREDIT`) and `amount BIGINT` (store minor units, not floats). `effective_at TIMESTAMPTZ NOT NULL` is caller-supplied at insert (service layer passes `time.Now()` by default, non-NOW for backdating). Sum of debits/credits per transaction is the balancing invariant — enforced in service layer, inside a single `*sql.Tx`.

`internal/models/models.go` holds plain Go structs mirroring the schema; nullable SQL columns are pointers (`*string`, `*time.Time` — e.g. `Transaction.PostedAt`). DB enum columns are represented by named string types (`AccountType`, `EntryDirection`, `TransactionStatus`) with exported consts (e.g. `DirectionDebit`, `StatusPosted`) — use the consts, not bare string literals, at insert/compare sites.

### Balance + sign convention

`repository.GetAccountBalance` returns `SUM(DEBIT) - SUM(CREDIT)` (net-debit, minor units). Positive = net inflow on debit side, negative = net inflow on credit side. Per double-entry rules, asset accounts have DEBIT as their normal balance side (balance reads positive after inflow); liability/revenue accounts have CREDIT (balance reads negative after inflow). The service/display layer flips sign per `AccountType` when rendering to users. Optional `asOf *time.Time` filters `effective_at <= asOf`; nil = current balance.

## Conventions worth knowing

- Connection string is inline in `main.go` with a `// Move to env var` TODO — treat env-var extraction as expected next step rather than a refactor.
- New schema changes go in a new numbered migration pair; don't edit existing ones (migrate tracks applied version in `schema_migrations`).
- Enum types are created via `DO $$ BEGIN ... EXCEPTION WHEN duplicate_object ... $$` so migrations stay idempotent against partially-applied state.
- Repo funcs currently take `*sql.DB` directly. When the service layer needs to reuse them inside a `*sql.Tx`, swap the param type to a `DBTX` interface (`QueryRowContext` / `QueryContext` / `ExecContext`) — both `*sql.DB` and `*sql.Tx` satisfy it. Don't pre-refactor until a service call needs it.
- Ledger is append-only: no `DeleteEntry`, no `DeleteTransaction`, no mutation of historical entries. Reversal = new transaction with opposing entries, or `UpdateTransactionStatus → REVERSED`. `effective_at` does not get updated when a payment "settles" — if settlement is a distinct event, model it as its own timestamp/status, not by mutating past rows.

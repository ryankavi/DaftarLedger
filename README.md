# PayClone

A **double-entry ledger service** — a payment-app backend (think Venmo/Cash App
rails) written in Go on PostgreSQL, with a React/MUI single-page frontend that
exercises every endpoint. It is a *simulated* ledger: there's no real bank, card
network, or ACH behind it, but the money-movement model, invariants, and safety
guarantees are the real thing.

- **Backend:** single Go binary, `database/sql` + `lib/pq`, HTTP under `/api`, JWT auth.
- **Frontend:** Vite + React 19 + TypeScript + MUI v9 (`frontend/`), one panel per endpoint.
- **Database:** PostgreSQL 16 in Docker on **port 5433**.

> Coming back after a while? Read [`PLAN.md`](./PLAN.md) (full roadmap + design
> decisions) and [`CLAUDE.md`](./CLAUDE.md) (conventions). [`CHEATSHEET.md`](./CHEATSHEET.md)
> has a deeper feature/tech-stack writeup. This README is the operational quickstart.

---

## Quick start (backend)

The app connects to Postgres on **port 5433** to avoid clashing with a native
Postgres on 5432. Docker must be running.

**1. First-time database setup** (creates the container + the `gopgtest` database):

```bash
docker run --name pg-container -e POSTGRES_PASSWORD=secret -p 5433:5432 -v payclone_pgdata:/var/lib/postgresql/data -d postgres:16
# wait a few seconds for Postgres to accept connections, then:
docker exec pg-container createdb -U postgres gopgtest
```

**2. Configure environment.** Copy `.env.example` to `.env` and fill it in. The
only strictly required value for local dev is `JWT_SECRET` (the server **refuses
to boot** without it); `DATABASE_URL` has a dev fallback pointing at the container
above.

```bash
cp .env.example .env
# set JWT_SECRET, e.g.:  openssl rand -base64 48
```

**3. Run the app** (migrations run automatically on startup):

```bash
go run ./main.go
```

The API comes up on **`http://localhost:8080`**, all routes under `/api`.

### Day-to-day

| Action | Command |
| --- | --- |
| Start an existing DB container | `docker start pg-container` |
| Run the service | `go run ./main.go` |
| Build / vet / format | `go build ./...` · `go vet ./...` · `gofmt -w .` |
| Stop the service | `Ctrl-C` (graceful shutdown drains in-flight requests) |

### Reset the database (start from scratch)

This drops **all data** and the volume; the next `go run` re-runs every migration
and re-seeds the platform accounts.

```bash
docker rm -f pg-container && docker volume rm payclone_pgdata
# then redo first-time setup (docker run + createdb) above
```

---

## Environment variables

Documented in [`.env.example`](./.env.example). `.env` is gitignored; real
environment variables take precedence over the file (so an EC2 host's exported
vars win).

| Variable | Required | Purpose |
| --- | --- | --- |
| `JWT_SECRET` | **Yes** — fatal if empty | HS256 signing key for auth tokens. Generate a strong random value. |
| `DATABASE_URL` | No (dev fallback) | Postgres connection string. Defaults to the local 5433 container. |
| `BOOTSTRAP_ADMIN_EMAIL` + `BOOTSTRAP_ADMIN_PASSWORD` | No (both-or-neither) | If both set, idempotently ensures an admin user on startup. Setting only one is a fatal misconfig. `signup` only ever creates regular users, so this is the only way an admin first exists. |

Token TTL (15m) and shutdown timeout (10s) are constants in `main.go`.

---

## Inspecting the database

Watch the ledger change as you click around the frontend or hit the API. Open a
psql shell inside the container:

```bash
docker exec -it pg-container psql -U postgres
```

```sql
\c gopgtest        -- connect to the database
\dt                -- list tables
\d accounts        -- describe the accounts table
```

Useful queries:

```sql
-- Who exists, and their accounts (note the seeded system/platform rows)
SELECT user_id, email, role FROM users;
SELECT account_id, account_type, owner_id, currency, account_status FROM accounts;

-- Recent transactions (header rows) — newest first
SELECT transaction_id, transaction_type, transaction_status, posted_at
FROM transactions ORDER BY created_at DESC LIMIT 10;

-- The two balanced entries behind each transaction (amounts are MINOR UNITS / cents)
SELECT transaction_id, account_id, entry_direction, amount
FROM entries ORDER BY created_at DESC LIMIT 20;

-- Net balance of one account (raw net-debit, minor units): SUM(DEBIT) - SUM(CREDIT)
SELECT COALESCE(SUM(CASE WHEN entry_direction = 'DEBIT' THEN amount ELSE -amount END), 0) AS balance_minor
FROM entries WHERE account_id = '<account-uuid>';

-- Prove the double-entry invariant: every transaction must net to zero
SELECT transaction_id,
       SUM(CASE WHEN entry_direction = 'DEBIT' THEN amount ELSE -amount END) AS imbalance
FROM entries GROUP BY transaction_id HAVING SUM(CASE WHEN entry_direction = 'DEBIT' THEN amount ELSE -amount END) <> 0;
-- (returns zero rows when the ledger is healthy)

-- Which migrations have been applied
SELECT * FROM schema_migrations;
```

> **Amounts are stored as minor units** (cents) in `BIGINT`. `1000` = `$10.00`.
> The frontend converts to/from dollars at the edge.

---

## Running the frontend

```bash
cd frontend
npm install
npm run dev        # Vite dev server (default http://localhost:5173)
```

The dev server **proxies `/api/*` to the Go backend on `:8080`** (see
`vite.config.ts`), so run the backend alongside it. Same-origin proxy means no
CORS config is needed in dev. Build for production with `npm run build` (the real
typecheck is `tsc -b`, run from `frontend/` — `npm run build` does this).

The frontend hardcodes the seeded platform-account UUIDs in
`frontend/src/platformAccounts.ts` and prefills the relevant fields, so e.g. a
deposit needs only a destination wallet + an amount.

---

## Tests

Tests use **testcontainers-go** — they spin up a real Postgres container per
package, so Docker must be running.

```bash
go test -p 1 ./...                              # full suite (serialized — see below)
go test ./internal/repository -run TestName     # one package, one test
```

Use **`-p 1`** for the full suite: it serializes the package test binaries so only
one Postgres container starts at a time. Plain `go test ./...` runs the
container-using packages in parallel, and Docker Desktop on Windows intermittently
fails testcontainers' provider init under that concurrency. A single package
doesn't need `-p 1`.

---

## How it works (orientation)

Layered single binary. Start at `main.go` and follow the wiring:

`main.go` → loads `.env`, opens the DB, runs migrations (`internal/db`), builds the
logger + JWT authenticator, optionally bootstraps an admin, starts the HTTP server.

| Layer | Path | Responsibility |
| --- | --- | --- |
| Bootstrap | `internal/db/` | Ping + run migrations. No queries. |
| Repository | `internal/repository/` | Per-entity CRUD primitives against a `DBTX` (works for both `*sql.DB` and `*sql.Tx`). No business rules, no `BEGIN/COMMIT`. |
| Service | `internal/service/` | Orchestration: the `Post` engine (one transaction = header + 2 entries), `Reverse`, account lifecycle, auth (`Login`/`EnsureAdmin`). Owns all `BEGIN/COMMIT`, locking, idempotency, validation. |
| HTTP | `internal/http/` (package `httpapi`) | Handlers, routing under `/api`, JWT middleware, auth callbacks, domain-error → HTTP-status mapping. |
| Models / Errors | `internal/models/`, `internal/errors/` | Structs mirroring the schema; domain sentinel errors. |
| Auth core | `internal/auth/` | DB-free HS256 JWT mint/verify (alg pinned). |
| Migrations | `migrations/` | Numbered `NNNNNN_name.{up,down}.sql` pairs, applied on startup. |

**Conventions that bite if you forget them** (full list in `CLAUDE.md`):

- **Append-only ledger** — never `DELETE` or mutate entries/transactions. A
  correction is a *new* opposing (reversal) transaction.
- **Minor units, never floats** — amounts are `BIGINT` cents.
- **`posted_at` is set by the DB** (`NOW()`), not the app clock.
- **Repo takes `DBTX`, only the service touches transactions** — don't add
  `BEGIN/COMMIT` to the repo layer.
- **Use the enum consts** (`models.StatusPosted`, `models.DirectionDebit`, …), not
  bare strings.
- **New schema = a new migration pair.** Never edit an applied migration.
- **Run from the repo root** so `file://migrations` resolves.

### Sign convention (the one thing people re-derive every time)

`repository.GetAccountBalance` returns raw `SUM(DEBIT) - SUM(CREDIT)` (net-debit,
minor units). Asset accounts (`TREASURY`, `CARD_SETTLEMENT`, `ACH_CLEARING`,
`EXTERNAL`) are DEBIT-normal → funded balance reads positive. Liability/revenue
accounts (`USER_CASH`, `FEE_REVENUE`) are CREDIT-normal → funded balance reads
*negative* (correct double-entry, not a bug). The service flips the sign per
account type before showing it to users.

---

## Seeded platform accounts (simulated ledger)

PayClone is a **simulated** double-entry ledger — there's no real bank, card
network, or ACH rail behind it. But a real ledger still needs the *platform-side*
accounts those rails settle against: an `EXTERNAL` account standing in for "the
outside world," a `TREASURY`, a `FEE_REVENUE` book, plus card/ACH clearing
accounts. Money never appears from nowhere — a deposit is `EXTERNAL → USER_CASH`,
a withdrawal is `USER_CASH → EXTERNAL`, a fee is `USER_CASH → FEE_REVENUE`. Every
one of those flows needs a platform account on the other side of the entry.

End users can't create those accounts (a `USER` may only open a `USER_CASH` wallet;
platform types are admin-only), and a fresh deploy shouldn't require an operator to
hand-insert rows before the demo works. So migration
`000007_seed_platform_accounts` seeds them automatically on startup, with **fixed
UUIDs**:

| Account type      | Seeded id                              |
| ----------------- | -------------------------------------- |
| `EXTERNAL`        | `a0000000-0000-0000-0000-000000000001` |
| `TREASURY`        | `a0000000-0000-0000-0000-000000000002` |
| `FEE_REVENUE`     | `a0000000-0000-0000-0000-000000000003` |
| `CARD_SETTLEMENT` | `a0000000-0000-0000-0000-000000000004` |
| `ACH_CLEARING`    | `a0000000-0000-0000-0000-000000000005` |

They're owned by a deterministic "system" user that can never log in (its bcrypt
hash is unusable). Because the ids are fixed and stable across deploys, the
frontend hardcodes them (`frontend/src/platformAccounts.ts`) and prefills the
relevant fields — so you can deposit into your wallet with nothing but an amount.

---

## Service features

A deeper writeup (description + impact for each, plus per-tech-stack benefit) is in
[`CHEATSHEET.md`](./CHEATSHEET.md). In brief:

1. **Double-entry ledger.** Every transaction posts a balanced DEBIT + CREDIT of
   equal magnitude, so total debits always equal total credits. Money can never be
   created or destroyed — only moved — and any imbalance is a detectable bug.

2. **ACID transactions.** Each post writes its header row and both entries inside a
   single SQL transaction. A failure mid-write rolls everything back — you never
   get a half-posted transaction or an unbalanced pair.

3. **Idempotency.** Each request carries a unique idempotency key; the engine
   prechecks it and a `UNIQUE` constraint backstops races (the losing caller
   catches the `23505` violation and re-fetches). Safe retries — a double-click or
   a dropped response can't double-charge anyone.

4. **Authorization layer.** Stateless HS256 JWTs are minted at login/signup and
   verified in middleware on every protected route (the algorithm is pinned to
   defend against alg-confusion). Handlers then enforce ownership and role —
   users move only their own money; fees/reversals are admin-only.

5. **Deadlock-safe locking.** When a transfer locks both accounts
   (`SELECT … FOR UPDATE`), it acquires them in a canonical order (UUIDs sorted
   lexicographically). Concurrent transfers on the same pair can't deadlock by
   grabbing locks in opposite orders.

6. **Append-only & reversible.** Ledger rows are never updated or deleted; a
   mistake is corrected by posting a new transaction with opposing entries, and an
   original can be reversed only once. The result is a complete, immutable audit
   trail.

7. **Integer money.** Amounts are stored as minor units (cents) in `BIGINT`, never
   floating-point — so there's no binary-float rounding drift and totals reconcile
   to the penny.

8. **DB-authoritative time.** `posted_at` is stamped by Postgres (`NOW()`), not the
   application clock — one trusted clock immune to drift/skew across multiple app
   instances.

9. **Auto-migrations.** `golang-migrate` applies the versioned, idempotent
   migration files on every startup, so a fresh DB or a new instance self-provisions
   to the correct schema (including the seeded platform accounts) with no manual step.

10. **Typed domain errors.** The service returns sentinel errors (insufficient
    funds, currency mismatch, account closed, forbidden, …) that one mapping layer
    translates to precise HTTP status codes (400/403/404/409/422), masking 5xx
    bodies. Callers get accurate, machine-readable failures; internals stay hidden.

11. **Funds & currency guards.** User-sourced flows check available balance before
    posting, and a transfer requires both accounts to share a currency (no implicit
    FX) — enforced in the service layer inside the transaction.

12. **Graceful shutdown.** A signal-cancellable context lets in-flight requests
    drain before the process exits (10s timeout), so a deploy or restart doesn't
    sever an active money movement.

---

## API reference

The HTTP surface is described in [`openapi.yaml`](./openapi.yaml) (mirrored by
`frontend/src/types.ts`). Paste it into the [Swagger editor](https://editor.swagger.io/)
to browse. All routes are served under `/api` (e.g. `POST /api/signup`,
`POST /api/transactions/transfer`, `GET /api/accounts/{id}/balance`).

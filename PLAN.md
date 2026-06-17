# PayClone Service Layer Plan

Fresh-Claude brief. Read `CLAUDE.md` first for repo-wide conventions; this file is the roadmap and the constraints that came out of the prior design conversation. Working dir: `C:\Users\ryan\Documents\PayClone`. Go + Postgres 16 (Docker, port 5433). Caveman mode (terse) is the user's preferred response style — see `~/.claude/plugins/cache/caveman/...`.

---

## Current state

- `internal/db/` — bootstrap only (`SetUpDB`: ping + run migrations). No queries here.
- `internal/repository/` — per-entity primitives. Each function takes `ctx context.Context` and `db DBTX` (interface satisfied by both `*sql.DB` and `*sql.Tx`). Files: `users.go`, `accounts.go`, `entries.go`, `transactions.go`, `dbtx.go`. `LockAccount` exists for `SELECT ... FOR UPDATE`. `users.go` adds `GetUserByEmail` (login lookup, returns hash + role); `CreateUser` writes `password_hash` and returns `role`. Unit tests landed (`*_test.go`).
- `internal/service/`:
  - `post.go` — single public engine `Post(ctx, db, p, postType)` plus `Reverse(ctx, db, ReverseParams)`. No per-flow wrappers — callers invoke `Post` directly with the appropriate `PostType`. Engine handles BeginTx → idempotency precheck (with type-conflict check) → canonical-order lock → per-flow type guard + direction → status guard (rejects non-OPEN accounts, **reversals bypass** via `reversalPostTypes` map) → currency match → funds check → CreateTransaction → 2 entries → POSTED → Commit. 23505 race fallback after CreateTransaction (also runs the type-conflict check on refetch). 10 PostType values: 5 forward + 5 reversal.
  - `Reverse` looks up the original via `repository.GetTransaction`, blocks unless `StatusPosted`, blocks if `repository.TransactionHasReversal` returns true, swaps from/to from the original entries, and calls `Post` with the inverse PostType from the package-private `reversalOf` map. The map covers forward → reversal and reversal → forward (so reversal-of-reversal works as a normal Post).
  - `accounts.go` — `CreateUserWithAccount` (signup, no auth; bcrypt-hashes the password then creates user+account in one txn), `CreateAccount` (authenticated; `ownerID` from auth context, never request body; single-statement, no `BeginTx`), `CloseAccount` / `FreezeAccount` / `ReopenAccount` all share shape: BeginTx → LockAccount → GetAccount → status guard → (CloseAccount also: GetAccountBalance + zero check) → UpdateAccountStatus → Commit. Status guards: Close rejects `CLOSED`, Freeze rejects `!= OPEN`, Reopen rejects `!= FROZEN` (so `CLOSED` is terminal). `validateEmail` uses `net/mail` (rejects empty, >254 chars, display-name form). 23505 catches on `users.email` (`ErrEmailTaken`) and `accounts(owner_id, account_type)` (`ErrAccountTypeExists`).
  - `balance.go` — `needsFundsCheck`, `availableBalance` helpers (CREDIT-normal sign flip).
  - `auth.go` — `Login(ctx, db, email, password)`: `GetUserByEmail` → `bcrypt.CompareHashAndPassword`; unknown-email and wrong-password both collapse to `ErrInvalidCredentials` (no enumeration). DB-only — the HTTP layer mints the token. `EnsureAdmin(ctx, db, email, password)`: bcrypt-hashes, then `repository.UpsertAdmin` (idempotent `INSERT … ON CONFLICT (email) → role=ADMIN`) — the startup admin-bootstrap path.
  - Service unit tests landed (`post_test.go`, `reverse_test.go`, `accounts_test.go`, `balance_test.go`, `helpers_test.go`, `main_test.go`). Tests use `testcontainers-go` (real Postgres per package), shared schema, per-test `truncateAll`.
  - `PostBookFee` is commented out — `FEE_REVENUE → TREASURY` is not a balanced double-entry pair as modeled (both sides decrease). Needs an equity account or a redefined model. Decide before re-enabling.
- `internal/errors/` — domain sentinels: `ErrInsufficientFunds`, `ErrCurrencyMismatch`, `ErrAccountNotFound`, `ErrSameAccount`, `ErrAccountClosed`, `ErrAccountNotOpen`, `ErrAccountNotFrozen`, `ErrBalanceNotZero`, `ErrInvalidAmount`, `ErrInvalidAccountType`, `ErrIdempotencyConflict`, `ErrNotReversible`, `ErrAlreadyReversed`, `ErrInvalidEmail`, `ErrEmailTaken`, `ErrAccountTypeExists`, `ErrForbidden`, `ErrTransactionNotFound`, `ErrInvalidCredentials` (generic login 401, maps in `toHTTPStatus`). Imported as `errx` because `package errors` clashes with stdlib name.
- `internal/models/` — plain structs. `Transaction.PostedAt` is `*time.Time` (NULL until POSTED). `external_id` is informational, **non-unique**. `idempotency_key` is `UNIQUE NOT NULL`. `User` carries `PasswordHash` + `Role`; `UserRole` named type with `RoleUser` / `RoleAdmin` consts (matches the `user_role` enum).
- `internal/http/` (package `httpapi` — directory named `http` but package renamed to avoid stdlib clash):
  - `server.go` — `*Server` struct (db, logger, auth, mux), `NewServer(db, logger, authenticator)`, `routes()` (builds an `api` mux — public routes plus a `protected` sub-mux gated behind `authMiddleware` — and mounts it under `/api/` via `http.StripPrefix`, so handler patterns stay clean), `ServeHTTP` (delegates to mux), `Run(ctx, addr, shutdownTimeout)` with graceful shutdown via goroutine + `select` on `ctx.Done()` vs. error channel.
  - `handlers.go` — all 12 handler methods on `*Server`, **complete**. Per-handler DTO structs colocated. `signup`/`login` issue a token via `s.auth.Sign`. The five forward transaction flows share a `postTransaction` helper (PostType + an authz callback injected per route); `writeTransaction` builds the txn response DTO. Edge currency validation `^[A-Z]{3}$`. Authz helpers: `assertOwns` (ownership → `ErrForbidden` / `ErrAccountNotFound`) and `isAdmin` (real role check → `RoleFromCtx == models.RoleAdmin`). Admin-gated routes: deposit, assess-fee, refund-fee, reverse.
  - `errors.go` — `toHTTPStatus(err) int` (domain error → 400/403/404/409/422, `default` 500) + `writeError(w, logger, msg, err)` (resolves status, logs with it, masks 5xx response bodies). Every handler routes errors through it.
  - `context.go` — request-scoped identity. Unexported `ctxKey int` type (collision-safe), `userIDKey` + `roleKey`. `WithUserID` / `UserIDFromCtx (string, bool)` and `WithRole` / `RoleFromCtx (models.UserRole, bool)` — both return `false` on missing or empty.
  - `middleware.go` — `authMiddleware(next http.Handler) http.Handler`. **Authentication only** (identity, not permission). Requires `Authorization: Bearer <jwt>`, verifies via `Authenticator.Parse` (signature + `exp` + HS256 pin), injects verified `sub` + `role` via `WithUserID` / `WithRole`; `401` on missing/malformed/invalid/expired. **No header fallback** — a token is the only way to establish identity. Standard `http.Handler → http.Handler` shape, wraps the whole protected sub-mux.
  - Routes are all served under `/api` (the whole `api` mux is mounted via `s.mux.Handle("/api/", http.StripPrefix("/api", api))`, so CloudFront's `/api/*` behavior reaches the server while handler patterns stay prefix-free): `POST /api/signup` (public), `POST /api/login` (public), `POST /api/accounts`, `GET /api/accounts`, `GET /api/accounts/{id}/balance`, `POST /api/accounts/{id}/{close|freeze|reopen}`, `POST /api/transactions/{transfer|deposit|withdraw|assess-fee|refund-fee}`, `POST /api/transactions/{id}/reverse`.
- `internal/auth/` — DB-free token core (unit-tested, no container). `Authenticator{secret, ttl}`; `New(secret, ttl)` returns `ErrEmptySecret` on a blank secret (the hook `main` uses to fail fast); `Sign(userID, role)` mints an **HS256** JWT (`sub`/`role`/`iat`/`exp`); `Parse` verifies signature + `exp` and **pins HS256 via `WithValidMethods`** (alg-confusion / `alg:none` defense). Lib `golang-jwt/jwt/v5` — chosen over hand-rolled `crypto/hmac` (we'd own verify-path security), `lestrrat-go/jwx` (heavier; only with JWKS), and PASETO. Revisit RS256 if verifiers ever split from the issuer.
- `main.go` loads `.env` via `godotenv` (real environment vars take precedence) → opens DB from `DATABASE_URL` (dev fallback) → runs migrations → builds `slog.Logger` (JSON handler) → builds `auth.New(JWT_SECRET, TOKEN_TTL = 15m)` (**fatal if `JWT_SECRET` unset**) → bootstraps an admin via `service.EnsureAdmin` when `BOOTSTRAP_ADMIN_EMAIL` + `BOOTSTRAP_ADMIN_PASSWORD` are both set (idempotent; fatal if only one is set) → signal-cancellable ctx via `signal.NotifyContext` → `httpapi.NewServer(database, logger, authenticator)` → `srv.Run(ctx, ":8080", SHUTDOWN_TIME)`. Env config is documented in committed `.env.example`; `.env` is gitignored.

Schema is double-entry ledger. Sum of debits == sum of credits per transaction is the balancing invariant. Append-only — no `Delete*` for entries/transactions, no row mutation; reversal is always a new opposing transaction.

---

## Hard constraints carried forward (do not violate)

1. **DBTX in repo.** Repo functions take `db DBTX`. Only the service layer touches `BeginTx` / `Commit` / `Rollback`. Don't add `BeginTx` to `DBTX`.
2. **Repo is dumb.** No business rules, no balanced-entry checks, no idempotency logic, no `BEGIN/COMMIT`. Service composes primitives.
3. **Append-only ledger.** Never `DELETE` entries/transactions, never mutate `effective_at`/`amount`/`direction` after insert.
4. **`posted_at` set by DB.** `UpdateTransactionStatus` sets `posted_at = NOW()` via CASE clause when status flips to `POSTED`. Service does not pass timestamps for posting.
5. **Bare enum strings forbidden at call sites.** Use `models.StatusPosted`, `models.DirectionDebit`, etc. — not `"POSTED"`, `"DEBIT"`.
6. **Nullable columns are pointers.** `*string`, `*time.Time`. Pass `nil` for NULL. Do not deref at call site — `database/sql` handles nil internally.
7. **Idempotency**: precheck via `GetTransactionByIdempotencyKey`. DB `UNIQUE` is the backstop. Losing-race caller catches `pq` error code `23505` and re-fetches. Type-conflict check on both precheck and refetch — same `idempotency_key` reused across post types returns `ErrIdempotencyConflict`.
8. **Lock order**: when locking multiple accounts, sort their UUIDs lexicographically and lock smaller-first to prevent deadlock. Go string `<`/`>` is byte-by-byte and total — sufficient as canonical order. Pattern is in `post.go`.
9. **Idempotent migrations.** New schema = new numbered pair under `migrations/`, never edit applied ones. Enum types via `DO $$ BEGIN ... EXCEPTION WHEN duplicate_object ... $$`.

---

## Sign convention recap

`repository.GetAccountBalance` returns `SUM(DEBIT) - SUM(CREDIT)` (raw net-debit, minor units). Repo is convention-neutral — service layer translates via `availableBalance` in `balance.go`.

- **Asset** account types (`TREASURY`, `CARD_SETTLEMENT`, `ACH_CLEARING`, `EXTERNAL`): DEBIT-normal. Funded → raw is positive. No flip.
- **Liability/revenue** types (`USER_CASH`, `FEE_REVENUE`): CREDIT-normal. Funded → raw is *negative* (this is correct double-entry, not a bug). Flip sign before comparing to a positive `Amount`.

Account-type roles:

| Type | Role | Source of transfer? | Funds check? |
|---|---|---|---|
| `USER_CASH` | User wallet (liability) | Yes — every user-initiated send | **Yes** |
| `EXTERNAL` | Outside-world counterparty | Yes (deposits) | No (unbounded) |
| `CARD_SETTLEMENT` | Card-network rail | Internal only | No |
| `ACH_CLEARING` | ACH rail | Internal only | No |
| `FEE_REVENUE` | Earned fees | Yes (refund only) | No (unbounded for refund path) |
| `TREASURY` | Platform reserves | Internal only | No (audited separately) |

Per-flow direction matrix (each row is `from / to` per `post.go` switch):

| PostType | From → To | from dir | to dir | Funds check? |
|---|---|---|---|---|
| `PostCashTransfer` | USER_CASH → USER_CASH | DEBIT | CREDIT | Yes |
| `PostDeposit` | EXTERNAL → USER_CASH | DEBIT | CREDIT | No |
| `PostWithdraw` | USER_CASH → EXTERNAL | DEBIT | CREDIT | Yes |
| `PostAssessFee` | USER_CASH → FEE_REVENUE | DEBIT | CREDIT | Yes |
| `PostRefundFee` | FEE_REVENUE → USER_CASH | DEBIT | CREDIT | No |
| `PostCashTransferReversal` | USER_CASH → USER_CASH | DEBIT | CREDIT | Yes |
| `PostDepositReversal` | USER_CASH → EXTERNAL | DEBIT | CREDIT | Yes |
| `PostWithdrawReversal` | EXTERNAL → USER_CASH | DEBIT | CREDIT | No |
| `PostAssessFeeReversal` | FEE_REVENUE → USER_CASH | DEBIT | CREDIT | No |
| `PostRefundFeeReversal` | USER_CASH → FEE_REVENUE | DEBIT | CREDIT | Yes |
| `PostBookFee` (disabled) | FEE_REVENUE → TREASURY | — | — | — |

All current types use `DEBIT/CREDIT`. Future asset↔asset flows (e.g. `TREASURY → EXTERNAL`, `CARD_SETTLEMENT → TREASURY`) will need `CREDIT/DEBIT` — the engine's per-case direction setting is the source of truth, not a universal pattern.

---

## Remaining work

HTTP handlers, the **JWT auth slice, and handler tests are complete** — see the `internal/http/` and `internal/auth/` inventory under **Current state** above. The `internal/http/*_test.go` suite (29 tests; testcontainers Postgres + `httptest`, white-box so it mints tokens via `srv.auth`) covers every route's happy path plus its main error (`401` missing/invalid token, `403` non-owner / non-admin, `404`, `409`/`422` domain errors) and the DTO shapes (`[]` not `null`, `204` no-body). What's left are a few follow-ups the auth work surfaced.

### Auth follow-ups (surfaced by the completed JWT slice)

The auth slice is enforced end to end: `signup`/`login` issue HS256 tokens, `authMiddleware` verifies them (no header bypass), and `isAdmin` gates the four admin routes on the verified role. Per-endpoint policy now lives in code (`ownsFrom` / `adminOnly` per route in `handlers.go`). What's left are the loose ends that work exposed:

- **Login timing side-channel.** `service.Login` skips bcrypt on the unknown-email path, so "no such user" returns measurably faster than "wrong password" — a (minor) enumeration oracle despite the generic error. Equalize by running a dummy `bcrypt.CompareHashAndPassword` against a fixed hash on the miss path.
- **freeze / reopen → admin (deferred).** Both stay user-owner today — correct while `freeze` is self-service only (the sole freeze path is the owner freezing their own account, so owner-reopen is safe). When a compliance/fraud hold lands, split `FROZEN` into self-lock vs compliance-hold and move the *compliance* freeze/unfreeze to admin — don't overload the single state. `freezeAccount` carries a `TODO` marker for this.

**Out of scope (unchanged):** stateless access token only — no refresh, no revocation (a stolen token is valid until `exp`, hence the short **15m** TTL). Refresh tokens + a denylist (or server-side sessions), rate limiting, CORS, request-logging middleware, OpenAPI, WebSocket/SSE — all later work.

---

## Small tasks (hygiene)

Low-risk, do anytime — none block the remaining HTTP steps (tests, auth).

- **`Clock` interface.** Inject a clock so tests can run deterministically. Currently `post.go` calls `time.Now()` direct at the entry-creation site. Skeleton:
  ```go
  type Clock interface{ Now() time.Time }
  type realClock struct{}
  func (realClock) Now() time.Time { return time.Now() }
  ```
  Implies converting service from package-level functions to methods on a `*Service` struct. Not blocking — current tests pass with real time. Defer until a flake forces it.
- **Structured logging in service layer.** `slog.Logger` is now threaded into `*Server` (HTTP layer) — extend the same pattern into the service layer after Clock refactor. INFO on each successful post, WARN on domain errors, ERROR on infra errors.
- **Metrics.** Counters per post type, latency histogram per post type. Prometheus client lib. Wire into the engine, not each wrapper.
- **`models/README.md` polish.** Stale: FEE_REVENUE row says "never a source" but `PostRefundFee` makes it a source. Missing `account_status` enum, `UNIQUE(owner_id, account_type)`, lifecycle state docs.
- **`service/README.md` refresh.** Stale: lists unimplemented flows (card-rail land, ACH land, capital injection) and omits the actual surface (`Post` engine, `PostType` enum, `Reverse`, account lifecycle ops). Replace flow table with the per-flow direction matrix already in this PLAN.
- **`PostBookFee` decision.** Either model an equity account so FEE_REVENUE → TREASURY balances, or delete the commented code.

---

## Permanently out of scope

- `sqlx` / `pgx` migration — current `database/sql` + `lib/pq` is fine.
- Bulk insert helpers (`CreateEntries`) — only 2 entries per transaction, premature.
- Multi-currency FX conversion — currently currency must match across both accounts; no conversion path. Add when a real cross-currency flow lands.
- Per-counterparty `EXTERNAL` accounts — single shared `EXTERNAL` is fine for now. When real rails land, split into one row per bank link / card network for reconciliation.

When in doubt, re-read this file's "Hard constraints" section before coding.

# PayClone Service Layer Plan

Fresh-Claude brief. Read `CLAUDE.md` first for repo-wide conventions; this file is the roadmap and the constraints that came out of the prior design conversation. Working dir: `C:\Users\ryan\Documents\PayClone`. Go + Postgres 16 (Docker, port 5433). Caveman mode (terse) is the user's preferred response style — see `~/.claude/plugins/cache/caveman/...`.

---

## Current state

- `internal/db/` — bootstrap only (`SetUpDB`: ping + run migrations). No queries here.
- `internal/repository/` — per-entity primitives. Each function takes `ctx context.Context` and `db DBTX` (interface satisfied by both `*sql.DB` and `*sql.Tx`). Files: `users.go`, `accounts.go`, `entries.go`, `transactions.go`, `dbtx.go`. `LockAccount` exists for `SELECT ... FOR UPDATE`. Unit tests landed (`*_test.go`).
- `internal/service/`:
  - `post.go` — single public engine `Post(ctx, db, p, postType)` plus `Reverse(ctx, db, ReverseParams)`. No per-flow wrappers — callers invoke `Post` directly with the appropriate `PostType`. Engine handles BeginTx → idempotency precheck (with type-conflict check) → canonical-order lock → per-flow type guard + direction → status guard (rejects non-OPEN accounts, **reversals bypass** via `reversalPostTypes` map) → currency match → funds check → CreateTransaction → 2 entries → POSTED → Commit. 23505 race fallback after CreateTransaction (also runs the type-conflict check on refetch). 10 PostType values: 5 forward + 5 reversal.
  - `Reverse` looks up the original via `repository.GetTransaction`, blocks unless `StatusPosted`, blocks if `repository.TransactionHasReversal` returns true, swaps from/to from the original entries, and calls `Post` with the inverse PostType from the package-private `reversalOf` map. The map covers forward → reversal and reversal → forward (so reversal-of-reversal works as a normal Post).
  - `accounts.go` — `CreateUserWithAccount` (signup, no auth; user+account in one txn), `CreateAccount` (authenticated; `ownerID` from auth context, never request body; single-statement, no `BeginTx`), `CloseAccount` / `FreezeAccount` / `ReopenAccount` all share shape: BeginTx → LockAccount → GetAccount → status guard → (CloseAccount also: GetAccountBalance + zero check) → UpdateAccountStatus → Commit. Status guards: Close rejects `CLOSED`, Freeze rejects `!= OPEN`, Reopen rejects `!= FROZEN` (so `CLOSED` is terminal). `validateEmail` uses `net/mail` (rejects empty, >254 chars, display-name form). 23505 catches on `users.email` (`ErrEmailTaken`) and `accounts(owner_id, account_type)` (`ErrAccountTypeExists`).
  - `balance.go` — `needsFundsCheck`, `availableBalance` helpers (CREDIT-normal sign flip).
  - Service unit tests landed (`post_test.go`, `reverse_test.go`, `accounts_test.go`, `balance_test.go`, `helpers_test.go`, `main_test.go`). Tests use `testcontainers-go` (real Postgres per package), shared schema, per-test `truncateAll`.
  - `PostBookFee` is commented out — `FEE_REVENUE → TREASURY` is not a balanced double-entry pair as modeled (both sides decrease). Needs an equity account or a redefined model. Decide before re-enabling.
- `internal/errors/` — domain sentinels: `ErrInsufficientFunds`, `ErrCurrencyMismatch`, `ErrAccountNotFound`, `ErrSameAccount`, `ErrAccountClosed`, `ErrAccountNotOpen`, `ErrAccountNotFrozen`, `ErrBalanceNotZero`, `ErrInvalidAmount`, `ErrInvalidAccountType`, `ErrIdempotencyConflict`, `ErrNotReversible`, `ErrAlreadyReversed`, `ErrInvalidEmail`, `ErrEmailTaken`, `ErrAccountTypeExists`, `ErrForbidden`, `ErrTransactionNotFound`. Imported as `errx` because `package errors` clashes with stdlib name.
- `internal/models/` — plain structs. `Transaction.PostedAt` is `*time.Time` (NULL until POSTED). `external_id` is informational, **non-unique**. `idempotency_key` is `UNIQUE NOT NULL`.
- `internal/http/` (package `httpapi` — directory named `http` but package renamed to avoid stdlib clash):
  - `server.go` — `*Server` struct (db, logger, mux), `NewServer`, `routes()` (registers all 11 routes), `ServeHTTP` (delegates to mux), `Run(ctx, addr, shutdownTimeout)` with graceful shutdown via goroutine + `select` on `ctx.Done()` vs. error channel.
  - `handlers.go` — all 11 handler methods on `*Server`, **complete**. Per-handler DTO structs colocated. The five forward transaction flows share a `postTransaction` helper (PostType + an authz callback injected per route); `writeTransaction` builds the txn response DTO. Edge currency validation `^[A-Z]{3}$`. Authz helpers: `assertOwns` (ownership → `ErrForbidden` / `ErrAccountNotFound`) and `isAdmin` (stub: `userID == "ADMIN_USER"`).
  - `errors.go` — `toHTTPStatus(err) int` (domain error → 400/403/404/409/422, `default` 500) + `writeError(w, logger, msg, err)` (resolves status, logs with it, masks 5xx response bodies). Every handler routes errors through it.
  - `context.go` — request-scoped identity. Unexported `ctxKey int` type (collision-safe), `userIDKey`. `WithUserID(ctx, id)` / `UserIDFromCtx(ctx) (string, bool)` (returns `false` on missing or empty). Landed.
  - `middleware.go` — `authMiddleware(next http.Handler) http.Handler`. **Authentication only** (identity, not permission). Stub: reads `Authorization: Bearer <id>`, falls back to raw `X-User-ID`; 401 if neither present; injects id via `WithUserID`. Standard `http.Handler → http.Handler` signature so it composes and can wrap a whole sub-mux. Landed.
  - Routes registered: `POST /signup` (public), `POST /accounts`, `GET /accounts`, `GET /accounts/{id}/balance`, `POST /accounts/{id}/{close|freeze|reopen}`, `POST /transactions/{transfer|deposit|withdraw|assess-fee|refund-fee}`, `POST /transactions/{id}/reverse`.
- `main.go` opens DB → runs migrations → builds `slog.Logger` (JSON handler) → builds signal-cancellable ctx via `signal.NotifyContext` → `httpapi.NewServer(database, logger)` → `srv.Run(ctx, ":8080", SHUTDOWN_TIME)`. Connection string still inline (env-var TODO unchanged).

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

HTTP handlers are complete — see the `internal/http/` inventory under **Current state** above (all 11 handlers, shared `toHTTPStatus`/`writeError`, `assertOwns`/`isAdmin` authz stubs, untrusted header authn stub). Two steps remain. **Do Step 1 before Step 2** — auth is security-critical, and a tested, committed handler layer is the bisectable foundation the auth swap builds on.

### Step 1 — Handler tests

`httptest` + the `testcontainers` real-Postgres pattern from `internal/service/`. Extract a `newTestServer(t) *Server` helper. One test per route × happy path + the main error case. Coverage targets: `401` (no identity in ctx), `403` (`assertOwns` / `isAdmin` reject), `404` (missing account / transaction), the `409`/`422` domain errors (e.g. `ErrBalanceNotZero`, `ErrInsufficientFunds`, `ErrAccountNotOpen`), and the success-path DTOs (snake_case fields, `204` no-body on lifecycle ops, empty list → `[]` not `null`). To exercise authz, set identity via the stub header (`X-User-ID`) until Step 2 swaps in real tokens.

### Step 2 — Real JWT authn + authz (one slice)

Replaces the header stub. Identity is a **prerequisite** for issuing tokens, so this is one ordered slice, not parallel tracks. Same middleware shape and ctx keys throughout → handlers don't change (they still read `UserIDFromCtx`), which is the whole point of the original design.

1. **Migration (credentials + roles).** New numbered pair: add `password_hash TEXT NOT NULL` and a `role` enum (`USER | ADMIN`, default `USER`) to `users`. Idempotent enum via the `DO $$ BEGIN ... EXCEPTION WHEN duplicate_object ... $$` pattern.
2. **Identity.** `signup` DTO gains `password`; service hashes with **bcrypt** (`golang.org/x/crypto/bcrypt`) before insert. Add `repository.GetUserByEmail` (returns the hash + role); extend `models.User`. Min-length check at the edge.
3. **`POST /login` (public, like `signup`).** email+password → `bcrypt.CompareHashAndPassword` → sign an **HS256** JWT (`golang-jwt/jwt/v5`). Claims: `sub`=user_id, `role`, `exp`, `iat`. Secret from env `JWT_SECRET`, **fail-fast on startup if unset**. Return the **same** `401` for unknown-email and wrong-password (don't leak which emails exist).
4. **Verify in `authMiddleware` (swap the stub impl).** Parse `Authorization: Bearer <jwt>`, verify signature + `exp`, extract `sub` + `role` → ctx (existing `userIDKey` + a new `roleKey`). `401` on missing/invalid/expired. **Delete the `X-User-ID` / raw-bearer fallback** — leaving it in is a full auth bypass (anyone sets a header and becomes anyone). This is the single most important line to remove.
5. **Real authz.** Add `RoleFromCtx`; replace the `isAdmin` magic-string stub (`userID == "ADMIN_USER"`) with a role check. Optionally add a `requireAdmin(next) http.Handler` wrapper for the wholly-admin routes (deposit, assess-fee, refund-fee, reverse); ownership (`assertOwns`) stays in-handler since it's per-resource. Per-endpoint policy:

   | Route | Auth | Rule |
   |---|---|---|
   | `POST /signup`, `POST /login` | Public | none |
   | `POST /accounts` | User | `ownerID` from ctx, never body |
   | `GET /accounts` | User | filtered to ctx owner (no `assertOwns` — only returns your own) |
   | `GET /accounts/{id}/balance` | User (owner) | `assertOwns` |
   | `POST /accounts/{id}/{close\|freeze\|reopen}` | User (owner) | `assertOwns` (freeze/reopen may move to admin once fraud holds land — single `FROZEN` state can't distinguish self-lock from compliance hold) |
   | `POST /transactions/transfer` | User | owns `from_account_id`; `to` can be anyone |
   | `POST /transactions/withdraw` | User | owns `from_account_id` |
   | `POST /transactions/deposit` | Admin/system | `from` is EXTERNAL — payment ingress worker only |
   | `POST /transactions/assess-fee` | Admin/system | platform-initiated, never user-callable |
   | `POST /transactions/refund-fee` | Admin/system | support/ops authorized |
   | `POST /transactions/{id}/reverse` | Admin/system | default admin-only; user self-reverse is abuse risk |

**Caveats / out of scope for this slice:** stateless access token only — **no refresh, no revocation**; a stolen token is valid until `exp`, so keep `exp` short. Refresh tokens + a denylist (or server-side sessions) are later work. Also deferred: rate limiting, CORS, request-logging middleware, OpenAPI, WebSocket/SSE.

---

## Small tasks (hygiene)

Low-risk, do anytime — none block the remaining HTTP steps (tests, auth).

- **Move DB conn string to env var.** `main.go` has the inline `localhost:5433` / `postgres` / `secret` / `gopgtest` string with a `// Move to env var` TODO. Read `DATABASE_URL` from env (default to current dev string if unset). One commit.
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

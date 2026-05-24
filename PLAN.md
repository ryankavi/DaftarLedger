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
- `internal/errors/` — domain sentinels: `ErrInsufficientFunds`, `ErrCurrencyMismatch`, `ErrAccountNotFound`, `ErrSameAccount`, `ErrAccountClosed`, `ErrAccountNotOpen`, `ErrAccountNotFrozen`, `ErrBalanceNotZero`, `ErrInvalidAmount`, `ErrInvalidAccountType`, `ErrIdempotencyConflict`, `ErrNotReversible`, `ErrAlreadyReversed`, `ErrInvalidEmail`, `ErrEmailTaken`, `ErrAccountTypeExists`. Imported as `errx` because `package errors` clashes with stdlib name.
- `internal/models/` — plain structs. `Transaction.PostedAt` is `*time.Time` (NULL until POSTED). `external_id` is informational, **non-unique**. `idempotency_key` is `UNIQUE NOT NULL`.
- No `internal/http/` yet. `main.go` only opens DB + runs migrations.

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

## Next: HTTP layer (`internal/http/`)

Service surface stable + tested. HTTP is the next layer.

### Surface to expose

Map service funcs to routes. Service signatures already take `ownerID` separately — handler reads it from auth context (stub for now, real middleware later), never from request body.

| Method + path | Service call | Notes |
|---|---|---|
| `POST /signup` | `CreateUserWithAccount` | No auth. Body: `email`, `account_type`, `currency`. |
| `POST /accounts` | `CreateAccount` | Auth required. `ownerID` from ctx. |
| `POST /accounts/{id}/close` | `CloseAccount` | Auth. |
| `POST /accounts/{id}/freeze` | `FreezeAccount` | Auth. |
| `POST /accounts/{id}/reopen` | `ReopenAccount` | Auth. |
| `POST /transfers` | `Post` (CASH_TRANSFER) | Body picks one PostType per route or accepts `type` field — pick one. |
| `POST /deposits` | `Post` (DEPOSIT) | |
| `POST /withdrawals` | `Post` (WITHDRAW) | |
| `POST /fees/assess` | `Post` (FEE_ASSESSMENT) | |
| `POST /fees/refund` | `Post` (FEE_REFUND) | |
| `POST /transactions/{id}/reverse` | `Reverse` | Body: `idempotency_key`, optional `memo`. |
| `GET /accounts/{id}` | `repository.GetAccount` via service passthrough | Read path. |
| `GET /accounts/{id}/balance` | `repository.GetAccountBalance` + sign flip | Apply `availableBalance` for display. |

Decide: one `POST /transactions` with `type` field vs. one route per flow. Per-flow routes give cleaner validation and OpenAPI; single endpoint is fewer handlers. Lean per-flow.

### Concrete next steps

1. **Pick a router.** `net/http` + `http.ServeMux` (Go 1.22 path params) is enough; no need for chi/gorilla yet.
2. **Request validation at the edge.** Currency `^[A-Z]{3}$`. Amount > 0. `idempotency_key` non-empty. Reject malformed JSON with 400.
3. **Domain error → HTTP status helper.** Single `func toHTTPStatus(err error) int` using `errors.Is`:
   - 400: `ErrInvalidAmount`, `ErrSameAccount`, `ErrInvalidAccountType`, `ErrCurrencyMismatch`, `ErrInvalidEmail`
   - 404: `ErrAccountNotFound`
   - 409: `ErrIdempotencyConflict`, `ErrAlreadyReversed`, `ErrEmailTaken`, `ErrAccountTypeExists`, `ErrBalanceNotZero`
   - 422: `ErrInsufficientFunds`, `ErrNotReversible`, `ErrAccountClosed`, `ErrAccountNotOpen`, `ErrAccountNotFrozen`
   - 500: everything else
4. **Auth context stub.** `func ownerFromCtx(ctx) (string, error)` — pulls from a `context.Value` key. Middleware writes the key; for now hardcode/test mode reads a header. Real JWT/session later.
5. **Wire into `main.go`.** Open DB → migrate → build mux → `http.ListenAndServe`. Graceful shutdown via `http.Server.Shutdown` on SIGINT/SIGTERM.
6. **JSON DTOs separate from `internal/models` structs.** Don't marshal DB structs directly — add request/response types in `internal/http`. Decouples wire format from schema.
7. **Handler tests** with `httptest.NewServer` against the same `testcontainers` Postgres. One test per route × happy path + main error case.

### Out of scope for HTTP slice 1

- Real auth (JWT/session). Stub via header for now.
- Rate limiting, CORS, request logging middleware — add after slice 1.
- OpenAPI generation.
- WebSocket / SSE.

---

## Small tasks (hygiene)

Low-risk, do anytime — none block HTTP work.

- **Move DB conn string to env var.** `main.go` has the inline `localhost:5433` / `postgres` / `secret` / `gopgtest` string with a `// Move to env var` TODO. Read `DATABASE_URL` from env (default to current dev string if unset). One commit.
- **`Clock` interface.** Inject a clock so tests can run deterministically. Currently `post.go` calls `time.Now()` direct at the entry-creation site. Skeleton:
  ```go
  type Clock interface{ Now() time.Time }
  type realClock struct{}
  func (realClock) Now() time.Time { return time.Now() }
  ```
  Implies converting service from package-level functions to methods on a `*Service` struct. Not blocking — current tests pass with real time. Defer until a flake forces it.
- **Structured logging.** Stdlib `log/slog`. Thread `slog.Logger` through `*Service` (after Clock refactor). INFO on each successful post, WARN on domain errors, ERROR on infra errors.
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

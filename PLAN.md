# PayClone Service Layer Plan

Fresh-Claude brief. Read `CLAUDE.md` first for repo-wide conventions; this file is the service-layer roadmap and the constraints that came out of the prior design conversation. Working dir: `C:\Users\ryan\Documents\PayClone`. Go + Postgres 16 (Docker, port 5433). Caveman mode (terse) is the user's preferred response style — see `~/.claude/plugins/cache/caveman/...`.

---

## Current state

- `internal/db/` — bootstrap only (`SetUpDB`: ping + run migrations). No queries here.
- `internal/repository/` — per-entity primitives. Each function takes `ctx context.Context` and `db DBTX` (interface satisfied by both `*sql.DB` and `*sql.Tx`). Files: `users.go`, `accounts.go`, `entries.go`, `transactions.go`, `dbtx.go`. `LockAccount` exists for `SELECT ... FOR UPDATE`.
- `internal/service/`:
  - `post.go` — single public engine `Post(ctx, db, p, postType)` plus `Reverse(ctx, db, ReverseParams)`. No per-flow wrappers — callers invoke `Post` directly with the appropriate `PostType`. Engine handles BeginTx → idempotency precheck (with type-conflict check) → canonical-order lock → per-flow type guard + direction → status guard (rejects non-OPEN accounts, **reversals bypass** via `reversalPostTypes` map) → currency match → funds check → CreateTransaction → 2 entries → POSTED → Commit. 23505 race fallback after CreateTransaction (also runs the type-conflict check on refetch). 10 PostType values: 5 forward + 5 reversal.
  - `Reverse` looks up the original via `repository.GetTransaction`, blocks unless `StatusPosted`, blocks if `repository.TransactionHasReversal` returns true, swaps from/to from the original entries, and calls `Post` with the inverse PostType from the package-private `reversalOf` map. The map covers forward → reversal and reversal → forward (so reversal-of-reversal works as a normal Post).
  - `accounts.go` — `CreateUserWithAccount` (signup, no auth; user+account in one txn), `CreateAccount` (authenticated; `ownerID` from auth context, never request body; single-statement, no `BeginTx`), `CloseAccount` / `FreezeAccount` / `ReopenAccount` all share shape: BeginTx → LockAccount → GetAccount → status guard → (CloseAccount also: GetAccountBalance + zero check) → UpdateAccountStatus → Commit. Status guards: Close rejects `CLOSED`, Freeze rejects `!= OPEN`, Reopen rejects `!= FROZEN` (so `CLOSED` is terminal). `validateEmail` uses `net/mail` (rejects empty, >254 chars, display-name form). 23505 catches on `users.email` (`ErrEmailTaken`) and `accounts(owner_id, account_type)` (`ErrAccountTypeExists`).
  - `balance.go` — `needsFundsCheck`, `availableBalance` helpers (CREDIT-normal sign flip).
  - `PostBookFee` is commented out — `FEE_REVENUE → TREASURY` is not a balanced double-entry pair as modeled (both sides decrease). Needs an equity account or a redefined model. Decide before re-enabling.
- `internal/errors/` — domain sentinels: `ErrInsufficientFunds`, `ErrCurrencyMismatch`, `ErrAccountNotFound`, `ErrSameAccount`, `ErrAccountClosed`, `ErrAccountNotOpen`, `ErrAccountNotFrozen`, `ErrBalanceNotZero`, `ErrInvalidAmount`, `ErrInvalidAccountType`, `ErrIdempotencyConflict`, `ErrNotReversible`, `ErrAlreadyReversed`, `ErrInvalidEmail`, `ErrEmailTaken`, `ErrAccountTypeExists`. Imported as `errx` because `package errors` clashes with stdlib name.
- `internal/models/` — plain structs. `Transaction.PostedAt` is `*time.Time` (NULL until POSTED). `external_id` is informational, **non-unique**. `idempotency_key` is `UNIQUE NOT NULL`.

Schema is double-entry ledger. Sum of debits == sum of credits per transaction is the balancing invariant. Append-only — no `Delete*` for entries/transactions, no row mutation; reversal is always a new opposing transaction (option A — option B status-flip was rejected because balances wouldn't move).

---

## Hard constraints carried forward (do not violate)

1. **DBTX swap is done.** Repo functions take `db DBTX`. Only the service layer touches `BeginTx` / `Commit` / `Rollback`. Don't add `BeginTx` to `DBTX`.
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

Account-type roles (also documented in `internal/models/README.md`):

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

## Goals (priority order)

### P1 — Service-layer remainder

1. **`Clock` interface for `time.Now()`.** Inject a clock so tests can run deterministically. Skeleton:
   ```go
   type Clock interface{ Now() time.Time }
   type realClock struct{}
   func (realClock) Now() time.Time { return time.Now() }
   ```
   Replace direct `time.Now()` calls in `post.go` with `s.clock.Now()`. This implies converting service from package-level functions to methods on a `*Service` struct (or pass the clock through `PostParams`/`context.Context` — pick one; struct is more idiomatic Go for this kind of dependency).

2. **Tests.** Use `testcontainers-go` (real Postgres per test). Mocks of `DBTX` defeat the entire reason DBTX exists — they don't validate SQL or `FOR UPDATE` lock semantics. Tests to write first:
   - Each post type happy path (5 forward flows).
   - Reversal happy path per forward type (5 reversal flows).
   - Reversal-of-reversal (chains back to forward via `reversalOf` map).
   - Reversal blocked when original is not `POSTED` → `ErrNotReversible`.
   - Reversal blocked when original already has a reversal → `ErrAlreadyReversed`.
   - Currency mismatch.
   - Insufficient funds (USER_CASH source — including refund where new source is USER_CASH).
   - Idempotent retry (call twice with same `idempotency_key`, second returns same txn).
   - Idempotency type-conflict (same key, different post type → `ErrIdempotencyConflict`).
   - Concurrent posts crossing same account pair (verify no deadlock).
   - Account-not-found.
   - Account-type guard rejection per flow (forward + reversal).

---

## Out of scope (do not add yet)

- HTTP handlers / routing — separate `internal/http` layer when service is stable.
- Auth, permissions, rate limiting — middleware concern.
- Caching — premature.
- `sqlx` / `pgx` migration — current `database/sql` + `lib/pq` is fine.
- Bulk insert helpers (`CreateEntries`) — only 2 entries per transaction, premature.
- Multi-currency FX conversion — currently currency must match across both accounts; no conversion path. Add when a real cross-currency flow lands.
- Per-counterparty `EXTERNAL` accounts — single shared `EXTERNAL` is fine for now. When real rails land, split into one row per bank link / card network for reconciliation.

### Deferred until HTTP layer lands

These come from `.todo` — record so they aren't forgotten when API work starts.

- **Auth context supplies `ownerID`.** `CreateAccount` (authenticated path) reads `ownerID` from session/JWT, not request body. Prevents IDOR. `CreateUserWithAccount` (signup path) takes email instead — no auth required, no `ownerID` exists yet.
- **Currency stays 3-letter ISO 4217 string in DB.** Schema column is `VARCHAR(3) NOT NULL`. Validate the 3-letter format at the HTTP/request layer (regex `^[A-Z]{3}$` plus optional ISO 4217 allowlist). Service layer assumes input is already valid format — currency mismatch in flows is a different check (compares stored values, not format).
- **Domain error → HTTP status mapping.** Build a single helper at the HTTP edge: `ErrInsufficientFunds` → 422, `ErrIdempotencyConflict` → 409, `ErrAlreadyReversed` → 409, `ErrEmailTaken` → 409, `ErrAccountTypeExists` → 409, `ErrBalanceNotZero` → 409, `ErrAccountNotFound` → 404, `ErrNotReversible` → 422, `ErrAccountClosed`/`ErrAccountNotOpen`/`ErrAccountNotFrozen` → 422, `ErrInvalidAmount`/`ErrSameAccount`/`ErrInvalidAccountType`/`ErrCurrencyMismatch`/`ErrInvalidEmail` → 400. Use `errors.Is` because service layer wraps with `%w`.

---

## Recommended execution order

1. P1 #1 (clock) before tests.
2. P1 #2 (tests) — close the loop on service correctness before HTTP exposes any of it.
3. After P1: HTTP layer + auth in `internal/http`. Out of scope for this plan.

When in doubt, re-read this file's "Hard constraints" section before coding.

---

## Small tasks (hygiene)

Low-risk, do anytime — none block the P1 work.

- **Move DB conn string to env var.** `main.go` has the inline `localhost:5433` / `postgres` / `secret` / `gopgtest` string with a `// Move to env var` TODO. Read `DATABASE_URL` from env (default to current dev string if unset). One commit.
- **Structured logging.** Stdlib `log/slog` (Go 1.21+). Thread `slog.Logger` through `*Service`. Log at INFO on each successful post (txn ID, post type, amount, currency), WARN on domain errors, ERROR on infra errors.
- **Metrics.** Counters per post type (`payclone_post_total{type="DEPOSIT",result="success"}`), latency histogram per post type. Prometheus client lib. Wire into the engine, not each wrapper.
- **`models/README.md` polish.** Currently stale: FEE_REVENUE row says "never a source" but `PostRefundFee` makes it a source. Missing `account_status` enum, `UNIQUE(owner_id, account_type)`, lifecycle state docs.
- **Service `README.md` refresh.** Currently stale: lists unimplemented flows (card-rail land, ACH land, capital injection) and omits the actual surface (`Post` engine, `PostType` enum, `Reverse`, account lifecycle ops). Replace flow table with the per-flow direction matrix already in this PLAN.

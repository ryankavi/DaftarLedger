# PayClone Service Layer Plan

Fresh-Claude brief. Read `CLAUDE.md` first for repo-wide conventions; this file is the service-layer roadmap and the constraints that came out of the prior design conversation. Working dir: `C:\Users\ryan\Documents\PayClone`. Go + Postgres 16 (Docker, port 5433). Caveman mode (terse) is the user's preferred response style — see `~/.claude/plugins/cache/caveman/...`.

---

## Current state

- `internal/db/` — bootstrap only (`SetUpDB`: ping + run migrations). No queries here.
- `internal/repository/` — per-entity primitives. Each function takes `ctx context.Context` and `db DBTX` (interface satisfied by both `*sql.DB` and `*sql.Tx`). Files: `users.go`, `accounts.go`, `entries.go`, `transactions.go`, `dbtx.go`. `LockAccount` exists for `SELECT ... FOR UPDATE`.
- `internal/service/` — `transfer.go` orchestrates `USER_CASH → USER_CASH` peer transfer. Has: idempotency precheck, canonical-order account lock, currency match, funds check (USER_CASH only), PENDING-then-POSTED txn flow, atomic via `*sql.Tx`.
- `internal/errors/` — domain sentinels: `ErrInsufficientFunds`, `ErrCurrencyMismatch`, `ErrAccountNotFound`, `ErrSameAccount`, `ErrInvalidAmount`. Imported as `errx` because `package errors` clashes with stdlib name.
- `internal/models/` — plain structs. `Transaction.PostedAt` is `*time.Time` (NULL until POSTED). `external_id` is informational, **non-unique**. `idempotency_key` is `UNIQUE NOT NULL`.

Schema is double-entry ledger. Sum of debits == sum of credits per transaction is the balancing invariant. Append-only — no `Delete*` for entries/transactions, no row mutation; reversal is a new opposing transaction or a status flip to `REVERSED`.

---

## Hard constraints carried forward (do not violate)

1. **DBTX swap is done.** Repo functions take `db DBTX`. Only the service layer touches `BeginTx` / `Commit` / `Rollback`. Don't add `BeginTx` to `DBTX`.
2. **Repo is dumb.** No business rules, no balanced-entry checks, no idempotency logic, no `BEGIN/COMMIT`. Service composes primitives.
3. **Append-only ledger.** Never `DELETE` entries/transactions, never mutate `effective_at`/`amount`/`direction` after insert.
4. **`posted_at` set by DB.** `UpdateTransactionStatus` sets `posted_at = NOW()` via CASE clause when status flips to `POSTED`. Service does not pass timestamps for posting.
5. **Bare enum strings forbidden at call sites.** Use `models.StatusPosted`, `models.DirectionDebit`, etc. — not `"POSTED"`, `"DEBIT"`.
6. **Nullable columns are pointers.** `*string`, `*time.Time`. Pass `nil` for NULL. Do not deref at call site — `database/sql` handles nil internally.
7. **Idempotency**: precheck via `GetTransactionByIdempotencyKey`. DB `UNIQUE` is the backstop. Losing-race caller catches `pq` error code `23505` and re-fetches.
8. **Lock order**: when locking multiple accounts, sort their UUIDs lexicographically and lock smaller-first to prevent deadlock. Go string `<`/`>` is byte-by-byte and total — sufficient as canonical order. Pattern is in `transfer.go`.
9. **Idempotent migrations.** New schema = new numbered pair under `migrations/`, never edit applied ones. Enum types via `DO $$ BEGIN ... EXCEPTION WHEN duplicate_object ... $$`.

---

## Sign convention recap

`repository.GetAccountBalance` returns `SUM(DEBIT) - SUM(CREDIT)` (raw net-debit, minor units). Repo is convention-neutral — service layer translates.

- **Asset** account types (`TREASURY`, `CARD_SETTLEMENT`, `ACH_CLEARING`): DEBIT-normal. Funded → raw is positive. No flip.
- **Liability/revenue** types (`USER_CASH`, `FEE_REVENUE`): CREDIT-normal. Funded → raw is *negative* (this is correct double-entry, not a bug). Flip sign before comparing to a positive `Amount`.

Helpers in `transfer.go` (move to a shared `service/balance.go` per goal #9):
```go
func needsFundsCheck(t models.AccountType) bool { return t == models.AccountUserCash }
func availableBalance(t models.AccountType, raw int64) int64 {
    if t == models.AccountUserCash { return -raw }
    return raw
}
```

Account-type roles (also documented in `internal/models/README.md`):

| Type | Role | Source of transfer? | Funds check? |
|---|---|---|---|
| `USER_CASH` | User wallet (liability) | Yes — every user-initiated send | **Yes** |
| `EXTERNAL` | Outside-world counterparty | Yes (deposits) | No (unbounded) |
| `CARD_SETTLEMENT` | Card-network rail | Internal only | No |
| `ACH_CLEARING` | ACH rail | Internal only | No |
| `FEE_REVENUE` | Earned fees | Never (destination only) | N/A |
| `TREASURY` | Platform reserves | Internal only | No (audited separately) |

---

## Goals (priority order)

### P1 — Tighten existing `transfer.go`

1. **Account-type guard at top of `Transfer`.** Restrict to `USER_CASH ↔ USER_CASH`. Return a new sentinel `ErrInvalidAccountType` if either side is anything else. Add the sentinel to `internal/errors/errors.go`. Reason: the "DEBIT from, CREDIT to" pattern is only correct when both accounts share the same normal-balance side. Cross-type transfers (asset↔asset, asset↔liability) need different DEBIT/CREDIT direction — those will be separate service funcs (deposit, withdrawal, etc.).

2. **Idempotency race fallback.** Current code prechecks via `GetTransactionByIdempotencyKey`. A concurrent caller losing the race hits `pq` `23505` (unique violation on `idempotency_key`) when calling `repository.CreateTransaction`. Catch it, re-fetch, return existing — turns the race into idempotent success.

   Skeleton:
   ```go
   import "github.com/lib/pq"

   t, err := repository.CreateTransaction(ctx, tx, nil, p.IdempotencyKey, p.TransactionType, p.Description)
   if err != nil {
       var pqErr *pq.Error
       if errors.As(err, &pqErr) && pqErr.Code == "23505" {
           // Lost the race — another caller committed first. Re-fetch and return their txn.
           if existing, ferr := repository.GetTransactionByIdempotencyKey(ctx, database, p.IdempotencyKey); ferr == nil {
               return existing, nil
           } else {
               return models.Transaction{}, fmt.Errorf("transfer idempotency refetch: %w", ferr)
           }
       }
       return models.Transaction{}, fmt.Errorf("create transaction: %w", err)
   }
   ```
   Note: re-fetch uses `database` (the `*sql.DB`), not `tx`. The losing tx is being rolled back anyway via `defer tx.Rollback()` — read the committed winner from a fresh connection.

3. **Domain error preservation through wrapping.** `LockAccount` returns `errx.ErrAccountNotFound` when the account does not exist. Service currently wraps with `fmt.Errorf("lock account: %w", err)` — `%w` preserves the chain so `errors.Is(err, errx.ErrAccountNotFound)` still works at the boundary. Verify the call site behaves and consider stripping the wrap at the service edge if downstream callers want bare sentinels for HTTP-status mapping. Decision: leave `%w` (gains stack context), require HTTP layer to use `errors.Is`.

### P2 — Extract shared helpers

4. **`internal/service/balance.go`** — move `needsFundsCheck` and `availableBalance` out of `transfer.go`. Will be reused by deposit/withdrawal/fee. Keep helpers tiny; do not promote to a struct.

### P3 — New service flows

For each flow: same skeleton as `Transfer` (BeginTx → lock in canonical order → validate → CreateTransaction PENDING → CreateEntry x2 → UpdateTransactionStatus POSTED → Commit). The only thing that varies is the DEBIT/CREDIT direction per account type and whether a funds check applies.

5. **`deposit.go` — `EXTERNAL → USER_CASH`.** Money entering the platform.
   - Direction: DEBIT on `EXTERNAL`, CREDIT on `USER_CASH`. (Asset side gains; liability to user grows.)
   - No funds check (EXTERNAL unbounded).
   - Lock both accounts canonically.
   - Type guard: source must be `EXTERNAL`, dest must be `USER_CASH`.
   - Same idempotency pattern.

6. **`withdrawal.go` — `USER_CASH → EXTERNAL`.** Money leaving the platform.
   - Direction: DEBIT on `USER_CASH` (decrease debt to user), CREDIT on `EXTERNAL`.
   - **Funds check required.** Use the helpers from `balance.go`.
   - Type guard: source `USER_CASH`, dest `EXTERNAL`.
   - For now, post immediately. Real systems have a PENDING-while-rail-settles phase with webhook-driven POSTED — out of scope until async settlement is needed.

7. **`fee.go` — `USER_CASH → FEE_REVENUE`.** Assess platform fee.
   - Direction: DEBIT on `USER_CASH`, CREDIT on `FEE_REVENUE`.
   - Funds check on user side.
   - Often paired with another flow (withdrawal + fee). For now expose as standalone; bundling to come later.

8. **`reversal.go` — refund/chargeback.** Pick one of two patterns and document the choice in CLAUDE.md:
   - **Option A (recommended): new opposing transaction.** Insert a fresh transaction with `external_id` referencing the original (now legitimately useful — `external_id` is non-unique by design, multiple txns can share it). Entries reverse direction (DEBIT becomes CREDIT, accounts swap). Original stays POSTED for audit. Funds check on whichever account is now the source.
   - **Option B: status flip.** `repository.UpdateTransactionStatus(..., StatusReversed)`. Cleaner audit trail but no new entries — balances don't actually move. Only useful if downstream readers know to skip REVERSED transactions when computing balances. Probably wrong for ledger correctness.
   - Decide before coding. Default to A.

9. **`account_open.go` — `OpenAccount(userID, type, currency)`.** Currently no service wraps account creation; handlers (future) would call `repository.CreateAccount` directly. Wrap so invariants live in one place — e.g., "one USER_CASH per (user, currency)", "default-create a USER_CASH on user signup", "EXTERNAL accounts can only be created by admin path."

### P4 — Cross-cutting

10. **`Clock` interface for `time.Now()`.** Inject a clock into service constructors so tests can run deterministically. Skeleton:
    ```go
    type Clock interface{ Now() time.Time }
    type realClock struct{}
    func (realClock) Now() time.Time { return time.Now() }
    ```
    Replace direct `time.Now()` calls with `s.clock.Now()`.

11. **Tests.** Pick one of:
    - `testcontainers-go` — spins up real Postgres per test. Slower but tests real SQL behavior including FOR UPDATE locking.
    - Mock `DBTX` — fast, but mocks don't validate SQL or lock semantics. Misses the entire reason DBTX exists.
    
    Recommend testcontainers. Tests to write first:
    - `Transfer` happy path (USER_CASH → USER_CASH).
    - Currency mismatch.
    - Insufficient funds.
    - Idempotent retry (call twice with same `idempotency_key`, second returns same txn).
    - Concurrent transfers crossing same pair (verify no deadlock).
    - Account-not-found (after P1 #1).

---

## Out of scope (do not add yet)

- HTTP handlers / routing — separate `internal/http` layer when service is stable.
- Auth, permissions, rate limiting — middleware concern.
- Logging/metrics — thread `ctx` through, attach observability layer above service later.
- Caching — premature.
- `sqlx` / `pgx` migration — current `database/sql` + `lib/pq` is fine.
- Bulk insert helpers (`CreateEntries`) — only 2 entries per transaction, premature.

### Deferred until HTTP layer lands

These come from `.todo` — record so they aren't forgotten when API work starts.

- **`CreateUser` duplicate email → HTTP 409.** `repository.CreateUser` returns the wrapped pq error from the `users.email UNIQUE` constraint when an email is reused. Detect `pq` error code `23505` at the HTTP boundary and map to 409 Conflict. Same `errors.As(err, &pqErr)` pattern as the idempotency race fallback in P1 #2. Could also surface a domain sentinel (`errx.ErrEmailAlreadyExists`) from the service layer wrapping `CreateUser` if a `RegisterUser` service ever lands — preferable to teaching the HTTP layer to read pq codes.
- **Currency stays 3-letter ISO 4217 string in DB.** Schema column is `VARCHAR(3) NOT NULL`. Validate the 3-letter format at the HTTP/request layer (regex `^[A-Z]{3}$` plus optional ISO 4217 allowlist). Service layer assumes input is already valid format — currency mismatch in `Transfer` is a different check (compares stored values, not format).

---

## Recommended execution order

1. P1 #1 (type guard) → P1 #2 (race fallback) → P2 #4 (extract helpers).
2. P3 #5 (deposit) → P3 #6 (withdrawal) — both reuse helpers.
3. P3 #7 (fee) → P3 #9 (account open).
4. P3 #8 (reversal) — design discussion first.
5. P4 #10 (clock) before tests.
6. P4 #11 (tests) — close the loop.

When in doubt, re-read this file's "Hard constraints" section before coding.

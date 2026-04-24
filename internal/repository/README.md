# internal/repository

Data-access layer. Per-entity primitives against `*sql.DB`. **Primitives only** — no validation beyond types, no multi-row orchestration, no business rules. Idempotency checks, account locking, cross-row invariants, and `BEGIN/COMMIT` boundaries live in [`internal/service`](../service).

## File layout

| File | Role |
|---|---|
| `users.go` | `CreateUser`, `GetUser`, `UpdateUserEmail`, `DeleteUser`. |
| `accounts.go` | Create / Get / Delete for `accounts`. No `Update` — accounts are immutable. |
| `transactions.go` | `CreateTransaction` (inserts PENDING, `posted_at` NULL), `UpdateTransactionStatus` (flips status + sets `posted_at = NOW()` on `POSTED`), `GetTransaction`, `GetTransactionByIdempotencyKey`, `GetTransactionByExternalID` (returns `[]Transaction` — `external_id` is non-unique). |
| `entries.go` | `CreateEntry` (caller supplies `effective_at`), `GetEntry`, `GetEntriesByTransactionID`, `GetEntriesByAccountID` (paginated), `GetAccountBalance` (net-debit aggregate, optional as-of). |

One file per entity. New entity = new file, same shape.

## Design decisions

### Free functions, not methods on a store struct

Every func takes `ctx context.Context` and `database *sql.DB` as explicit args:

```go
func CreateUser(ctx context.Context, database *sql.DB, email string) (models.User, error)
```

Rationale:
- Early project — no dependency graph to wire through constructors.
- Easy to pass a `*sql.Tx` later by switching the param type to a `DBTX` interface (see below).
- No hidden state.

Refactor to `type UserStore struct { db DBTX }` once tests/HTTP handlers need mocking.

### `*sql.DB` today, `DBTX` interface later

Repo funcs currently accept `*sql.DB`. When the service layer needs to compose several repo calls inside one `BEGIN/COMMIT` (e.g. `transfer.go` inserting txn header + 2 entries atomically), swap the param type to an interface both `*sql.DB` and `*sql.Tx` satisfy:

```go
type DBTX interface {
    QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
    QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
    ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
}
```

One-line change per function. Don't pre-refactor until a service call actually needs it.

### `RETURNING` on every write

`INSERT`/`UPDATE` statements use `RETURNING col, col, ...` and `Scan` directly into the struct. Single round trip. Caller always gets authoritative DB state (generated UUIDs, `DEFAULT NOW()` timestamps, `posted_at` set by the `CASE WHEN 'POSTED' THEN NOW()` branch) — no local guessing.

### `sql.ErrNoRows` as the "not found" signal

- `Get*`: `QueryRow.Scan` already returns `sql.ErrNoRows` when no row — we wrap with context but preserve via `%w`.
- `Delete*`: `Exec` succeeds on zero-row match, so we check `RowsAffected() == 0` and return `sql.ErrNoRows` manually.

Caller distinguishes with `errors.Is(err, sql.ErrNoRows)`. Uniform contract across read + delete.

### Typed enums at the boundary

Functions accept `models.AccountType`, `models.EntryDirection`, `models.TransactionStatus` — not plain `string`. Bad values fail at compile, not `INSERT`. Underlying type is `string`, so `lib/pq` scan/bind works without custom `sql.Scanner` impls.

### Nullable columns = pointer params

`*string` for nullable text/UUID, `*time.Time` for nullable timestamp. Pass `nil` for NULL. `database/sql` + `lib/pq` handle the nil-check internally — do **not** dereference at the call site. Example: `externalID *string` in `CreateTransaction`, `asOf *time.Time` in `GetAccountBalance`.

### Append-only for ledger rows

No `DeleteEntry`, no `DeleteTransaction`, no `UpdateEntry`. Entries are immutable once written — reversing a transaction means inserting a new transaction with opposing entries, or flipping status to `REVERSED`. `effective_at` is set at insert and never mutated. If "settlement" is a later event, model it as a separate timestamp/status rather than editing history.

Users and accounts can be hard-deleted via `Delete*`; the schema's `ON DELETE RESTRICT` blocks removal when dependent ledger rows exist — caller gets pq `23503`. Soft delete (`deleted_at` column + filter in `Get*`) is the likely next step when a "close account" / "deactivate user" feature lands.

### No `UpdateAccount`

Accounts are immutable by design. Changing `account_type`, `owner_id`, or `currency` after creation corrupts history. If "close this account" is needed, add a `closed_at` column + dedicated `CloseAccount` func — don't reach for a generic `Update`.

### Balance sign convention

`GetAccountBalance` returns `SUM(DEBIT) - SUM(CREDIT)` in minor units (net-debit). Positive = net inflow on debit side, negative = net inflow on credit side. Per double-entry rules, asset account types (`TREASURY`) have DEBIT as their normal side; liability/revenue types (`USER_CASH`, `FEE_REVENUE`) have CREDIT. The service/display layer flips sign per `AccountType` when rendering for users. Repo stays convention-neutral — raw ledger math.

Optional `asOf *time.Time`: nil = current balance, non-nil filters `effective_at <= asOf` for historical balance queries.

### Pagination

`GetEntriesByAccountID` takes `limit int, offset int` and orders `effective_at DESC, entry_id DESC` (stable tiebreaker). Keyset pagination is preferred long-term for deep lists — swap in once account histories get large.

## Planned additions

- **Keyset pagination** for `List*` reads that replace current offset-based.
- **`LockAccount` primitive** — `SELECT ... FOR UPDATE` wrapper for callers that orchestrate multi-row flows. Currently inlined in `service/transfer.go`.
- **Soft delete** — `deleted_at` columns on `users` / `accounts` once real product flows need closure/deactivation.
- **Repository refactor** — promote free functions to methods on a store struct when tests/mocks/multi-package wiring make the implicit `*sql.DB` arg painful.

## Conventions

- Queries are `const q = \`...\`` at the top of each function. SQL stays close to the Go that uses it.
- Error messages prefix the operation: `fmt.Errorf("create account: %w", err)`. Makes stack-trace-free logs legible.
- All queries use parameterized placeholders (`$1`, `$2`) — never string concat. Non-negotiable.
- Rows-iterating reads: always `defer rows.Close()` + check `rows.Err()` after the loop.
- Column order in `INSERT (...) VALUES (...)` and `RETURNING ...` / `SELECT ...` lists is kept consistent across a file — drift between query and `Scan` target list is the most common bug source.

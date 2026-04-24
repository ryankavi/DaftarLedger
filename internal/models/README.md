# internal/models

Plain Go structs mirroring the DB schema. No ORM, no tags — just types.

## Rules

- One struct per table: `User`, `Account`, `Entry`, `Transaction`.
- Nullable SQL columns → pointer (`*string`, `*time.Time`). Non-null → value type. Example: `Transaction.PostedAt *time.Time` because a PENDING row has NULL `posted_at`.
- DB enum columns → named string type + exported consts: `EntryDirection`, `AccountType`, `TransactionStatus`. Use the consts (`DirectionDebit`, `StatusPosted`, ...) at call sites — never bare string literals.
- `Amount` is `int64` minor units (cents). Never floats.

## Adding a column

1. New migration pair in `migrations/`.
2. Add the field to the matching struct here.
3. Update any `SELECT`/`INSERT` column lists in `internal/repository/*.go`.

Order matters — structs and queries drift silently otherwise.

## Tags

No `db` or `json` tags yet. Add them the same commit they're first used:
- `db:"..."` when switching to `sqlx`/`pgx` struct scan.
- `json:"..."` when a handler marshals the struct to a response body.

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

## Accounts

┌─────────────────┬─────────────────────────────────────────────────┬─────────────────────────────────┬───────────────────────────────┐
│   AccountType   │               Role in money flow                │      Source of a transfer?      │      Needs funds check?       │
├─────────────────┼─────────────────────────────────────────────────┼─────────────────────────────────┼───────────────────────────────┤
│ USER_CASH       │ User wallet (liability)                         │ Yes — every user-initiated send │ Yes                           │
├─────────────────┼─────────────────────────────────────────────────┼─────────────────────────────────┼───────────────────────────────┤
│ EXTERNAL        │ Outside-world counterparty (bank, card network) │ Yes, in deposits                │ No — by design unbounded      │
├─────────────────┼─────────────────────────────────────────────────┼─────────────────────────────────┼───────────────────────────────┤
│ CARD_SETTLEMENT │ Card-network clearing rail                      │ Internal only                   │ No — bookkeeping intermediary │
├─────────────────┼─────────────────────────────────────────────────┼─────────────────────────────────┼───────────────────────────────┤
│ ACH_CLEARING    │ ACH clearing rail                               │ Internal only                   │ No — bookkeeping intermediary │
├─────────────────┼─────────────────────────────────────────────────┼─────────────────────────────────┼───────────────────────────────┤
│ FEE_REVENUE     │ Platform's earned fees                          │ Never (it only receives)        │ N/A — never a source          │
├─────────────────┼─────────────────────────────────────────────────┼─────────────────────────────────┼───────────────────────────────┤
│ TREASURY        │ Platform's own cash reserves                    │ Internal only                   │ No — audited, not gated       │
└─────────────────┴─────────────────────────────────────────────────┴─────────────────────────────────┴───────────────────────────────┘

Reasons each skips the check:

  - EXTERNAL is a synthetic counterparty representing "the rest of the world." It's the source side of every deposit. Treating it as having a finite balance would make deposits fail
  randomly — outside money is, by definition, unlimited from the ledger's POV.
  - CARD_SETTLEMENT / ACH_CLEARING are pass-through rails. Money flows in and out as networks settle. Their balances oscillate around zero on normal operation. A funds-check here
  would block legitimate flows mid-settlement.
  - FEE_REVENUE can only ever appear as a destination (CREDIT side) of a transaction. It never acts as a source, so the question "does it have enough" never comes up.
  - TREASURY is the platform's own cash. A real system audits it via separate controls (alerts, daily reconciliation) rather than gating live transfers — the platform shouldn't have
  transfers fail because the operations team forgot to top up reserves.
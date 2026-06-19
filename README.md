# PayClone

## Running the app

The app connects to PostgreSQL on **port 5433** to avoid conflicting with a native Postgres on 5432.

**Start the database (first time or after removing the container):**

```bash
docker run --name pg-container -e POSTGRES_PASSWORD=secret -p 5433:5432 -v payclone_pgdata:/var/lib/postgresql/data -d postgres:16
```

Wait a few seconds, then create the database:

```bash
docker exec pg-container createdb -U postgres gopgtest
```

**Run the app:**

```bash
go run ./main.go
```

**Connect to the database:**
***Exec into the container***
```bash
docker exec -it pg-container psql -U postgres
```
***Access the database***
```bash
\c gopgtest     # database name
\dt             # display tables
\d <accounts>   # display accounts table
```

**If the container already exists:** `docker start pg-container`

**Reset database (remove container and volume):** `docker rm -f pg-container` then `docker volume rm payclone_pgdata`

## Start from scratch

**Remove the postgres container (and volume):**

```bash
docker rm -f pg-container && docker volume rm payclone_pgdata
```

## Seeded platform accounts (simulated ledger)

PayClone is a **simulated** double-entry ledger — there's no real bank, card network, or ACH rail behind it. But a real ledger application still needs the *platform-side* accounts those rails settle against: an `EXTERNAL` account standing in for "the outside world," a `TREASURY`, a `FEE_REVENUE` book, plus card/ACH clearing accounts. Money never appears from nowhere — a deposit is `EXTERNAL → USER_CASH`, a withdrawal is `USER_CASH → EXTERNAL`, a fee is `USER_CASH → FEE_REVENUE`. Every one of those flows needs a platform account on the other side of the entry.

End users can't create those accounts (a `USER` may only open a `USER_CASH` wallet; the platform types are admin-only), and a fresh deploy shouldn't require an operator to hand-insert rows before the demo works. So migration `000007_seed_platform_accounts` seeds them automatically on startup, with **fixed UUIDs**:

| Account type      | Seeded id                              |
| ----------------- | -------------------------------------- |
| `EXTERNAL`        | `a0000000-0000-0000-0000-000000000001` |
| `TREASURY`        | `a0000000-0000-0000-0000-000000000002` |
| `FEE_REVENUE`     | `a0000000-0000-0000-0000-000000000003` |
| `CARD_SETTLEMENT` | `a0000000-0000-0000-0000-000000000004` |
| `ACH_CLEARING`    | `a0000000-0000-0000-0000-000000000005` |

They're owned by a deterministic "system" user that can never log in (its bcrypt hash is unusable). Because the ids are fixed and stable across deploys, the frontend hardcodes them (`frontend/src/platformAccounts.ts`) and prefills the relevant fields — so you can deposit into your wallet with nothing but an amount. This is the cleanest way to prop up a believable demo of a real-world ledger: the platform's own books exist from the first boot, exactly as they would in production, with no manual seeding.

## OpenAPI

View at [Swagger](https://editor.swagger.io/)

## Service features

1) Canonical ordering: PayClone removes deadlocks in db by removing the possibility of a cycle, via simple lexigraphical order. Both goroutines lock same account first.

2) Idempotency: Every transaction uses a UNIQUE idempotency key to prevent the same transaction being posted to the db.

3) Atomic commits: Use a Tx connection pool for transactions to allow rollback on failed db operations.
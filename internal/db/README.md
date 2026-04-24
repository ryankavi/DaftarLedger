# internal/db

Bootstrap only. Owns the DB handle's lifecycle: ping + run migrations on startup. No queries.

## File layout

| File | Role |
|---|---|
| `db.go` | `SetUpDB(database *sql.DB)` — pings, then runs `golang-migrate` against `file://migrations` using the same live `*sql.DB` wrapped via `postgres.WithInstance`. `migrate.ErrNoChange` is swallowed. |

Per-entity CRUD lives in [`internal/repository`](../repository). Business logic lives in [`internal/service`](../service).

## Why this exists as its own package

Keeping `SetUpDB` separate from `repository` avoids a cycle risk once `repository` grows test helpers that need a live DB — the test helper can import `db` without pulling in every query function.

## Migration rules (unchanged)

- Files in `migrations/` as numbered pairs: `NNNNNN_name.up.sql` / `.down.sql`.
- Path is relative (`file://migrations`) — app must run from repo root.
- Schema changes = new numbered pair. Never edit applied migrations (`schema_migrations` tracks version).
- Enum types created via `DO $$ BEGIN ... EXCEPTION WHEN duplicate_object ... $$` so migrations stay idempotent.

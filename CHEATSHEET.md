# PayClone Backend Cheat Sheet

A slightly deeper look at the features summarized in the "Under the Hood" dropdown,
followed by why each piece of the tech stack matters to a ledger service. The
frontend is only UI — everything below describes the Go service.

---

## Backend features

### Double-entry ledger
**What:** Every transaction posts two balanced entries — a DEBIT and a CREDIT of
equal magnitude — so the sum of debits always equals the sum of credits.
**Impact:** Money can never appear or vanish; it only moves between accounts. The
books are self-checking, and any imbalance is a detectable bug rather than silent
corruption.

### ACID transactions
**What:** Each post runs inside a single SQL transaction (`BEGIN … COMMIT`) that
writes the transaction header plus both entries together.
**Impact:** A crash or error mid-write rolls everything back — you never end up
with a half-posted transaction (a header with one entry, or a debit with no
matching credit). The ledger is always in a consistent state.

### Idempotency
**What:** Each request carries a unique idempotency key; the engine prechecks it
and a `UNIQUE` constraint on the column backstops races, with the losing caller
catching the `23505` violation and re-fetching the original result.
**Impact:** Safe retries. A dropped response or double-click can't double-charge a
user — the second attempt returns the first attempt's outcome instead of posting
a duplicate.

### Authorization layer
**What:** Stateless HS256 JWTs are minted at login/signup and verified in
middleware on every protected route, which injects the caller's id + role; route
handlers then enforce ownership and role (e.g. admin-only, owns-the-source).
**Impact:** Identity and permissions are checked on every request, not trusted
from the client. A user can only move their own money; privileged flows (fees,
reversals) are gated to admins.

### Deadlock-safe locking
**What:** When a transfer locks both accounts (`SELECT … FOR UPDATE`), it always
acquires them in a canonical order — UUIDs sorted lexicographically, smallest
first.
**Impact:** Concurrent transfers touching the same pair of accounts can't deadlock
by grabbing locks in opposite orders. Throughput stays predictable under load.

### Append-only & reversible
**What:** Ledger rows are never updated or deleted; a mistake is corrected by
posting a new transaction with opposing entries (a reversal), and an original
can only be reversed once.
**Impact:** A complete, immutable audit trail. History is always reconstructable,
corrections are themselves recorded, and you can prove what happened and when.

### Integer money
**What:** Amounts are stored as minor units (cents) in a `BIGINT`, never as
floating-point.
**Impact:** No binary-float rounding drift — `0.10 + 0.20` is exactly `0.30`.
Totals reconcile to the penny, which is non-negotiable for money.

### DB-authoritative time
**What:** The `posted_at` timestamp is stamped by Postgres (`NOW()` in the UPDATE
that flips status to POSTED), not by the application clock.
**Impact:** One trusted clock for posting time, immune to drift or skew across
multiple app instances. Ordering and reporting stay coherent regardless of which
server handled the request.

### Auto-migrations
**What:** `golang-migrate` runs the versioned, idempotent migration files against
the database on every startup, swallowing "no change."
**Impact:** Schema and code deploy together — a fresh database or a new instance
self-provisions to the correct schema (including seeded platform accounts) with no
manual migration step.

### Typed domain errors
**What:** The service returns sentinel error values (insufficient funds, currency
mismatch, account closed, forbidden, …) that a single mapping layer translates to
precise HTTP status codes (400/403/404/409/422), masking 5xx response bodies.
**Impact:** Callers get accurate, machine-readable failures instead of a generic
500, while internal details stay hidden. Error handling is centralized and
consistent across every endpoint.

---

## Tech stack — what each brings to a ledger service

### Go
A compiled, statically typed language with first-class concurrency. For a ledger
it gives predictable performance, a single self-contained binary to deploy, and a
type system that catches whole classes of mistakes before they touch money.

### PostgreSQL 16
A mature relational database with strong ACID guarantees, real transactions, row
locking (`FOR UPDATE`), and constraints. It is the actual enforcer of correctness
— the balancing, uniqueness, and isolation a ledger depends on live here.

### database/sql
Go's standard database abstraction. It gives explicit control over transactions
and connection pooling without hiding the SQL — exactly the transparency you want
when every statement is part of an auditable money movement.

### lib/pq
The PostgreSQL driver behind `database/sql`. It surfaces Postgres-specific
details — notably the `23505` unique-violation code the idempotency path relies on
to detect and absorb duplicate posts.

### golang-migrate
Versioned schema migrations applied programmatically. It makes the database schema
reproducible and auditable, so the ledger's structure evolves in tracked,
reversible steps rather than ad-hoc edits.

### golang-jwt/jwt v5
A vetted JWT implementation that pins the signing algorithm (HS256) on verify,
defending against alg-confusion attacks. It lets the service authenticate callers
statelessly — no session store — while keeping the security-critical verify path
trustworthy.

### bcrypt
An adaptive password-hashing function with a tunable work factor. It ensures user
credentials are never stored in a reversible form, so a database leak doesn't hand
over account access to the money inside.

### net/http
Go's standard HTTP server and router. No heavyweight framework means a small,
well-understood surface area — fewer dependencies to audit around the endpoints
that move funds.

### log/slog
Structured (key/value) logging in the standard library. Structured logs make a
financial service's activity queryable and machine-parseable — essential for
tracing a transaction, debugging a failed post, or feeding an audit pipeline.

### Docker
Containerized, reproducible runtime for the database (and the service). It pins the
exact Postgres version and config across dev, test, and deploy, so behavior the
ledger relies on doesn't shift between environments.

### testcontainers-go
Spins up a real Postgres in a container for each test package. The ledger's
correctness lives in actual SQL — transactions, locks, constraints — so tests run
against a real database instead of a mock that can't reproduce that behavior.

### godotenv
Loads configuration (database URL, JWT secret, admin bootstrap) from the
environment, with real env vars taking precedence. It keeps secrets out of the
codebase and lets the same binary run safely across environments.

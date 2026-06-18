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

## OpenAPI

View at [Swagger](https://editor.swagger.io/)

## Service features

1) Canonical ordering: PayClone removes deadlocks in db by removing the possibility of a cycle, via simple lexigraphical order. Both goroutines lock same account first.

2) Idempotency: Every transaction uses a UNIQUE idempotency key to prevent the same transaction being posted to the db.

3) Atomic commits: Use a Tx connection pool for transactions to allow rollback on failed db operations.
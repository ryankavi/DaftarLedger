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

**If the container already exists:** `docker start pg-container`

**Reset database (remove container and volume):** `docker rm -f pg-container` then `docker volume rm payclone_pgdata`

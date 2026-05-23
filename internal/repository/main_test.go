package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// testDB is the package-wide *sql.DB pointed at a Postgres container that
// is started once in TestMain and torn down when the package's tests finish.
// Tests share schema; per-test isolation is provided by truncateAll.
var testDB *sql.DB

func TestMain(m *testing.M) {
	code, err := runTests(m)
	if err != nil {
		log.Fatalf("test setup: %v", err)
	}
	os.Exit(code)
}

func runTests(m *testing.M) (int, error) {
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:16",
		postgres.WithDatabase("payclone_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("secret"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return 0, fmt.Errorf("start postgres container: %w", err)
	}
	defer func() {
		if err := ctr.Terminate(ctx); err != nil {
			log.Printf("terminate container: %v", err)
		}
	}()

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return 0, fmt.Errorf("connection string: %w", err)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return 0, fmt.Errorf("sql open: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return 0, fmt.Errorf("ping: %w", err)
	}

	if err := runMigrationsForTests(db); err != nil {
		return 0, fmt.Errorf("migrations: %w", err)
	}

	testDB = db
	return m.Run(), nil
}

// runMigrationsForTests applies migrations against the test container.
// We chdir to the repo root so the existing `file://migrations` relative
// URL resolves the same way it does in production startup. The original
// working directory is restored before tests run so individual tests are
// not affected.
func runMigrationsForTests(db *sql.DB) error {
	oldWd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}
	repoRoot := filepath.Join(oldWd, "..", "..")
	if err := os.Chdir(repoRoot); err != nil {
		return fmt.Errorf("chdir to repo root: %w", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	driver, err := migratepg.WithInstance(db, &migratepg.Config{})
	if err != nil {
		return fmt.Errorf("migrate driver: %w", err)
	}

	mig, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate instance: %w", err)
	}
	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// truncateAll wipes all ledger data between tests while preserving schema
// and the schema_migrations bookkeeping table. CASCADE handles FK chains
// (entries → transactions, accounts; accounts → users).
func truncateAll(t *testing.T) {
	t.Helper()
	_, err := testDB.ExecContext(context.Background(),
		`TRUNCATE TABLE entries, transactions, accounts, users RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

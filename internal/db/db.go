package db

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/ryankavi/payclone/migrations"
)

func SetUpDB(database *sql.DB) error {
	if err := database.Ping(); err != nil {
		return fmt.Errorf("db ping failed: %w", err)
	}
	fmt.Println("Database connection OK")

	if err := runMigrations(database); err != nil {
		return fmt.Errorf("migrations failed: %w", err)
	}

	fmt.Println("Migrations complete")
	return nil
}

func runMigrations(database *sql.DB) error {
	driver, err := postgres.WithInstance(database, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}

	// Source the migrations from the embedded FS rather than file://migrations,
	// so they travel inside the binary and don't depend on the working directory.
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("creating migrate instance: %w", err)
	}

	// m.Up() executes every up-migration embedded in the binary.
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}

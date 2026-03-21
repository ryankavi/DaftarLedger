package db

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		return fmt.Errorf("creating migrate instance: %w", err)
	}

	// m.Up() tells golang-migrate to execute the sql questions in /internal/migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}

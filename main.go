package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	fmt.Println("Hello world!")

	// Use port 5433 to avoid conflict with native Postgres on 5432; Docker: -p 5433:5432
	// connStr := "host=127.0.0.1 port=5433 user=postgres password=secret dbname=gopgtest sslmode=disable"
	connStr := "host=localhost port=5433 user=postgres password=secret dbname=gopgtest sslmode=disable"

	db, err := sql.Open("postgres", connStr)

	if err != nil {
		log.Fatalf("Fail on db open: %s", err)
	}

	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatalf("Fail on db ping: %s", err)
	}
	fmt.Println("Database connection OK")

	if err = enableUUIDExtension(db); err != nil {
		log.Fatal("Fail on enable UUID extension")
	}

	if err = createUsersTable(db); err != nil {
		log.Fatal("Fail on users table creation")
	}

	if err = createAccountsTable(db); err != nil {
		log.Fatal("Fail on accounts table creation")
	}

	fmt.Println("Tables created")
}

func enableUUIDExtension(db *sql.DB) error {
	query := `CREATE EXTENSION IF NOT EXISTS pgcrypto;`
	_, err := db.Exec(query)
	return err
}

func createUsersTable(db *sql.DB) error {
	/* Users Table
	- User ID
	- Email
	- Created At
	*/
	query := `CREATE TABLE IF NOT EXISTS users (
		user_id UUID PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW()
	)`

	_, err := db.Exec(query)
	if err != nil {
		return err
	}

	return nil
}

func createAccountsTable(db *sql.DB) error {
	/* Accounts Table
	- Account ID
	- Account Type
	- Owner ID
	- Currency
	- Status
	- Created At
	- Metadata
	*/
	typeQuery := `CREATE TYPE account_type AS ENUM (
		'USER_CASH',
		'CARD_SETTLEMENT',
		'ACH_CLEARING',
		'FEE_REVENUE',
		'SYSTEM'
	);`

	_, err := db.Exec(typeQuery)
	if err != nil {
		return err
	}

	query := `CREATE TABLE IF NOT EXISTS accounts (
		account_id UUID PRIMARY KEY,
		account_type account_type NOT NULL,
		owner_id UUID REFERENCES users(user_id) NOT NULL,
		currency CHAR(3),
		created_at TIMESTAMPTZ DEFAULT NOW()
	)`

	_, err = db.Exec(query)
	if err != nil {
		return err
	}

	return nil
}

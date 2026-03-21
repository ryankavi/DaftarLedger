package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"github.com/ryankavi/payclone/internal/db"
)

func main() {
	fmt.Println("Hello world!")

	// Use port 5433 to avoid conflict with native Postgres on 5432; Docker: -p 5433:5432
	// connStr := "host=127.0.0.1 port=5433 user=postgres password=secret dbname=gopgtest sslmode=disable"
	// Move to env var
	connStr := "host=localhost port=5433 user=postgres password=secret dbname=gopgtest sslmode=disable"

	database, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Fail on db open: %s", err)
	}
	defer database.Close()

	if err = db.SetUpDB(database); err != nil {
		log.Fatalf("Database setup failed: %s", err)
	}
}

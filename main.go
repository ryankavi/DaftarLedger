package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/ryankavi/payclone/internal/db"
	httpapi "github.com/ryankavi/payclone/internal/http"
)

const SHUTDOWN_TIMEOUT = 10 * time.Second

func main() {
	// Use port 5433 to avoid conflict with native Postgres on 5432; Docker: -p 5433:5432
	// Move to env var
	// connStr := "host=127.0.0.1 port=5433 user=postgres password=secret dbname=gopgtest sslmode=disable"
	connStr := "host=localhost port=5433 user=postgres password=secret dbname=gopgtest sslmode=disable"

	database, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Fail on db open: %s", err)
	}
	defer database.Close()

	if err = db.SetUpDB(database); err != nil {
		log.Fatalf("Database setup failed: %s", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := httpapi.NewServer(database, logger)
	if err := srv.Run(ctx, ":8080", SHUTDOWN_TIMEOUT); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("http run: %s", err)
	}
}

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

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/ryankavi/payclone/internal/auth"
	"github.com/ryankavi/payclone/internal/db"
	httpapi "github.com/ryankavi/payclone/internal/http"
	"github.com/ryankavi/payclone/internal/service"
)

const SHUTDOWN_TIMEOUT = 10 * time.Second

// TOKEN_TTL is the access-token lifetime. Kept short on purpose: tokens are
// stateless with no revocation, so a stolen token is valid until it expires.
const TOKEN_TTL = 15 * time.Minute

// defaultDatabaseURL is the dev fallback used when DATABASE_URL (or .env) is
// absent. JWT_SECRET has no fallback on purpose — a missing signing key is fatal.
const defaultDatabaseURL = "postgres://postgres:secret@localhost:5433/gopgtest?sslmode=disable"

// getenv returns the env var or fallback when it is unset/empty.
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	// Load .env if present. Real environment variables take precedence —
	// godotenv only sets keys not already in the environment — so the EC2
	// host's exported vars win over this file.
	_ = godotenv.Load()

	connStr := getenv("DATABASE_URL", defaultDatabaseURL)

	database, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Fail on db open: %s", err)
	}
	defer database.Close()

	if err = db.SetUpDB(database); err != nil {
		log.Fatalf("Database setup failed: %s", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Fail fast: the server must not boot able to mint/verify tokens with an
	// empty signing key. auth.New returns ErrEmptySecret when JWT_SECRET is unset.
	authenticator, err := auth.New(os.Getenv("JWT_SECRET"), TOKEN_TTL)
	if err != nil {
		log.Fatalf("auth init: %s", err)
	}

	// Bootstrap an admin when configured. signup only ever creates RoleUser, so
	// this is the only path that mints an admin. Idempotent — safe on every boot.
	// Requires both env vars; if only one is set it's a misconfig, so fail loud.
	adminEmail, adminPass := os.Getenv("BOOTSTRAP_ADMIN_EMAIL"), os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	switch {
	case adminEmail != "" && adminPass != "":
		if err := service.EnsureAdmin(context.Background(), database, adminEmail, adminPass); err != nil {
			log.Fatalf("bootstrap admin: %s", err)
		}
		logger.Info("bootstrap admin ensured", "email", adminEmail)
	case adminEmail != "" || adminPass != "":
		log.Fatalf("bootstrap admin: set both BOOTSTRAP_ADMIN_EMAIL and BOOTSTRAP_ADMIN_PASSWORD, or neither")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := httpapi.NewServer(database, logger, authenticator)
	if err := srv.Run(ctx, ":8080", SHUTDOWN_TIMEOUT); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("http run: %s", err)
	}
}

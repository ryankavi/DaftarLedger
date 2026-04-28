package repository

import (
	"context"
	"database/sql"
)

// DBTX is the subset of database/sql needed by repository functions.
// Both *sql.DB and *sql.Tx satisfy it, so callers can run a repo
// function standalone (pass *sql.DB) or inside a transaction
// (pass *sql.Tx) without duplicating the function.
//
// BeginTx / Commit / Rollback are intentionally excluded — only the
// service layer owns transaction lifecycle.
type DBTX interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Compile-time assertion: drift in either stdlib type fails the build here.
var (
	_ DBTX = (*sql.DB)(nil)
	_ DBTX = (*sql.Tx)(nil)
)

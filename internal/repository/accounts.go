package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	errx "github.com/ryankavi/payclone/internal/errors"
	"github.com/ryankavi/payclone/internal/models"
)

func CreateAccount(ctx context.Context, db DBTX, ownerID string, accountType models.AccountType, currency string) (models.Account, error) {
	const q = `
		INSERT INTO accounts (account_type, owner_id, currency)
		VALUES ($1, $2, $3)
		RETURNING account_id, account_type, owner_id, currency, created_at
	`

	var a models.Account
	err := db.QueryRowContext(ctx, q, accountType, ownerID, currency).Scan(
		&a.AccountID,
		&a.AccountType,
		&a.OwnerID,
		&a.Currency,
		&a.CreatedAt,
	)
	if err != nil {
		return models.Account{}, fmt.Errorf("create account: %w", err)
	}
	return a, nil
}

func GetAccount(ctx context.Context, db DBTX, accountID string) (models.Account, error) {
	const q = `
		SELECT account_id, account_type, owner_id, currency, created_at
		FROM accounts
		WHERE account_id = $1
	`

	var a models.Account
	err := db.QueryRowContext(ctx, q, accountID).Scan(
		&a.AccountID,
		&a.AccountType,
		&a.OwnerID,
		&a.Currency,
		&a.CreatedAt,
	)
	if err != nil {
		return models.Account{}, fmt.Errorf("get account: %w", err)
	}
	return a, nil
}

// LockAccount acquires a row-level lock on the account for the duration
// of the caller's transaction. Returns sql.ErrNoRows if the account does
// not exist. MUST be called with *sql.Tx — passing *sql.DB acquires the
// lock on a pooled connection that is released as soon as the query
// returns, defeating the lock's purpose.
func LockAccount(ctx context.Context, db DBTX, accountID string) error {
	const q = `SELECT 1 FROM accounts WHERE account_id = $1 FOR UPDATE`

	var n int
	if err := db.QueryRowContext(ctx, q, accountID).Scan(&n); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errx.ErrAccountNotFound
		}
		return fmt.Errorf("lock account %s: %w", accountID, err)
	}
	return nil
}

func DeleteAccount(ctx context.Context, db DBTX, accountID string) error {
	const q = `
		DELETE FROM accounts
		WHERE account_id = $1
	`

	res, err := db.ExecContext(ctx, q, accountID)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete account rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

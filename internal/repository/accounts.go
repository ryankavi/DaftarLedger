package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ryankavi/payclone/internal/models"
)

func CreateAccount(ctx context.Context, database *sql.DB, ownerID string, accountType models.AccountType, currency string) (models.Account, error) {
	const q = `
		INSERT INTO accounts (account_type, owner_id, currency)
		VALUES ($1, $2, $3)
		RETURNING account_id, account_type, owner_id, currency, created_at
	`

	var a models.Account
	err := database.QueryRowContext(ctx, q, accountType, ownerID, currency).Scan(
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

func GetAccount(ctx context.Context, database *sql.DB, accountID string) (models.Account, error) {
	const q = `
		SELECT account_id, account_type, owner_id, currency, created_at
		FROM accounts
		WHERE account_id = $1
	`

	var a models.Account
	err := database.QueryRowContext(ctx, q, accountID).Scan(
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

func DeleteAccount(ctx context.Context, database *sql.DB, accountID string) error {
	const q = `
		DELETE FROM accounts
		WHERE account_id = $1
	`

	res, err := database.ExecContext(ctx, q, accountID)
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

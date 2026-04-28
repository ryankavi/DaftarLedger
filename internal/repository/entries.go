package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ryankavi/payclone/internal/models"
)

func CreateEntry(ctx context.Context, db DBTX, transactionID string, accountID string, amount int64, currency string, direction models.EntryDirection, memo *string, effectiveAt time.Time) (models.Entry, error) {
	const q = `
		INSERT INTO entries (transaction_id, account_id, amount, currency, direction, memo, effective_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING entry_id, transaction_id, account_id, amount, currency, direction, memo, created_at, effective_at
	`

	var e models.Entry
	err := db.QueryRowContext(ctx, q, transactionID, accountID, amount, currency, direction, memo, effectiveAt).Scan(
		&e.EntryID,
		&e.TransactionID,
		&e.AccountID,
		&e.Amount,
		&e.Currency,
		&e.Direction,
		&e.Memo,
		&e.CreatedAt,
		&e.EffectiveAt,
	)
	if err != nil {
		return models.Entry{}, fmt.Errorf("create entry: %w", err)
	}
	return e, nil
}

func GetEntry(ctx context.Context, db DBTX, entryID string) (models.Entry, error) {
	const q = `
		SELECT entry_id, transaction_id, account_id, amount, currency, direction, memo, created_at, effective_at
		FROM entries
		WHERE entry_id = $1
	`

	var e models.Entry
	err := db.QueryRowContext(ctx, q, entryID).Scan(
		&e.EntryID,
		&e.TransactionID,
		&e.AccountID,
		&e.Amount,
		&e.Currency,
		&e.Direction,
		&e.Memo,
		&e.CreatedAt,
		&e.EffectiveAt,
	)
	if err != nil {
		return models.Entry{}, fmt.Errorf("get entry: %w", err)
	}
	return e, nil
}

func GetEntriesByTransactionID(ctx context.Context, db DBTX, transactionID string) ([]models.Entry, error) {
	const q = `
		SELECT entry_id, transaction_id, account_id, amount, currency, direction, memo, created_at, effective_at
		FROM entries
		WHERE transaction_id = $1
		ORDER BY direction DESC
	`

	rows, err := db.QueryContext(ctx, q, transactionID)
	if err != nil {
		return nil, fmt.Errorf("get transaction entries: %w", err)
	}
	defer rows.Close()

	var entries []models.Entry
	for rows.Next() {
		var e models.Entry
		if err := rows.Scan(
			&e.EntryID,
			&e.TransactionID,
			&e.AccountID,
			&e.Amount,
			&e.Currency,
			&e.Direction,
			&e.Memo,
			&e.CreatedAt,
			&e.EffectiveAt,
		); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entries from transaction id: %w", err)
	}
	return entries, nil
}

func GetEntriesByAccountID(ctx context.Context, db DBTX, accountID string, limit int, offset int) ([]models.Entry, error) {
	const q = `
		SELECT entry_id, transaction_id, account_id, amount, currency, direction, memo, created_at, effective_at
		FROM entries
		WHERE account_id = $1
		ORDER BY effective_at DESC, entry_id DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := db.QueryContext(ctx, q, accountID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get entries by account id: %w", err)
	}
	defer rows.Close()

	var entries []models.Entry
	for rows.Next() {
		var e models.Entry
		if err := rows.Scan(
			&e.EntryID,
			&e.TransactionID,
			&e.AccountID,
			&e.Amount,
			&e.Currency,
			&e.Direction,
			&e.Memo,
			&e.CreatedAt,
			&e.EffectiveAt,
		); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entries from account id: %w", err)
	}
	return entries, nil
}

// GetAccountBalance returns net-debit balance (SUM(DEBIT) - SUM(CREDIT)) in minor units.
// Positive = net inflow on debit side; negative = net inflow on credit side.
// Caller interprets sign per account type's normal balance.
// If asOf is non-nil, only entries with effective_at <= asOf are included.
func GetAccountBalance(ctx context.Context, db DBTX, accountID string, asOf *time.Time) (int64, error) {
	const q = `
		SELECT COALESCE(SUM(CASE direction WHEN 'DEBIT' THEN amount ELSE -amount END), 0)
		FROM entries
		WHERE account_id = $1
		  AND ($2::timestamptz IS NULL OR effective_at <= $2)
	`

	var balance int64
	if err := db.QueryRowContext(ctx, q, accountID, asOf).Scan(&balance); err != nil {
		return 0, fmt.Errorf("get account balance: %w", err)
	}
	return balance, nil
}

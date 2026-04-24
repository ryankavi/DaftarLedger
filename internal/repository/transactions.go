package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ryankavi/payclone/internal/models"
)

func CreateTransaction(ctx context.Context, database *sql.DB, externalID *string, idempotencyKey string, transactionType string, description *string) (models.Transaction, error) {
	const q = `
		INSERT INTO transactions (external_id, idempotency_key, transaction_type, transaction_description, transaction_status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING transaction_id, external_id, idempotency_key, transaction_type, transaction_description, transaction_status, posted_at, created_at
	`

	var t models.Transaction
	err := database.QueryRowContext(ctx, q, externalID, idempotencyKey, transactionType, description, models.StatusPending).Scan(
		&t.TransactionID,
		&t.ExternalID,
		&t.IdempotencyKey,
		&t.TransactionType,
		&t.TransactionDescription,
		&t.TransactionStatus,
		&t.PostedAt,
		&t.CreatedAt,
	)
	if err != nil {
		return models.Transaction{}, fmt.Errorf("create transaction: %w", err)
	}
	return t, nil
}

func UpdateTransactionStatus(ctx context.Context, database *sql.DB, transactionID string, newStatus models.TransactionStatus) (models.Transaction, error) {
	const q = `
		UPDATE transactions
		SET transaction_status = $2,
			posted_at = CASE WHEN $2 = 'POSTED' THEN NOW() ELSE posted_at END
		WHERE transaction_id = $1
		RETURNING transaction_id, external_id, idempotency_key, transaction_type, transaction_description, transaction_status, posted_at, created_at
	`

	var t models.Transaction
	err := database.QueryRowContext(ctx, q, transactionID, newStatus).Scan(
		&t.TransactionID,
		&t.ExternalID,
		&t.IdempotencyKey,
		&t.TransactionType,
		&t.TransactionDescription,
		&t.TransactionStatus,
		&t.PostedAt,
		&t.CreatedAt,
	)
	if err != nil {
		return models.Transaction{}, fmt.Errorf("update transaction status: %w", err)
	}
	return t, nil
}

func GetTransaction(ctx context.Context, database *sql.DB, transactionID string) (models.Transaction, error) {
	const q = `
		SELECT transaction_id, external_id, idempotency_key, transaction_type, transaction_description, transaction_status, posted_at, created_at
		FROM transactions
		WHERE transaction_id = $1
	`

	var t models.Transaction
	err := database.QueryRowContext(ctx, q, transactionID).Scan(
		&t.TransactionID,
		&t.ExternalID,
		&t.IdempotencyKey,
		&t.TransactionType,
		&t.TransactionDescription,
		&t.TransactionStatus,
		&t.PostedAt,
		&t.CreatedAt,
	)
	if err != nil {
		return models.Transaction{}, fmt.Errorf("get transaction by id: %w", err)
	}
	return t, nil
}

func GetTransactionByIdempotencyKey(ctx context.Context, database *sql.DB, key string) (models.Transaction, error) {
	const q = `
		SELECT transaction_id, external_id, idempotency_key, transaction_type, transaction_description, transaction_status, posted_at, created_at
		FROM transactions
		WHERE idempotency_key = $1
	`

	var t models.Transaction
	err := database.QueryRowContext(ctx, q, key).Scan(
		&t.TransactionID,
		&t.ExternalID,
		&t.IdempotencyKey,
		&t.TransactionType,
		&t.TransactionDescription,
		&t.TransactionStatus,
		&t.PostedAt,
		&t.CreatedAt,
	)
	if err != nil {
		return models.Transaction{}, fmt.Errorf("get transaction by idempotency key: %w", err)
	}
	return t, nil
}

func GetTransactionByExternalID(ctx context.Context, database *sql.DB, externalID string) ([]models.Transaction, error) {
	const q = `
		SELECT transaction_id, external_id, idempotency_key, transaction_type, transaction_description, transaction_status, posted_at, created_at
		FROM transactions
		WHERE external_id = $1
	`

	rows, err := database.QueryContext(ctx, q, externalID)
	if err != nil {
		return nil, fmt.Errorf("get transactions for external id: %w", err)
	}
	defer rows.Close()

	var transactions []models.Transaction
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(
			&t.TransactionID,
			&t.ExternalID,
			&t.IdempotencyKey,
			&t.TransactionType,
			&t.TransactionDescription,
			&t.TransactionStatus,
			&t.PostedAt,
			&t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		transactions = append(transactions, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transactions: %w", err)
	}
	return transactions, nil
}

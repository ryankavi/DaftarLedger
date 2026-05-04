package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	errx "github.com/ryankavi/payclone/internal/errors"
	"github.com/ryankavi/payclone/internal/models"
	"github.com/ryankavi/payclone/internal/repository"
)

type TransferParams struct {
	FromAccountID   string
	ToAccountID     string
	Amount          int64
	Currency        string
	IdempotencyKey  string
	TransactionType string
	Memo            *string
	Description     *string
}

func Transfer(ctx context.Context, database *sql.DB, p TransferParams) (models.Transaction, error) {
	if p.Amount <= 0 {
		return models.Transaction{}, errx.ErrInvalidAmount
	}
	if p.FromAccountID == p.ToAccountID {
		return models.Transaction{}, errx.ErrSameAccount
	}

	// If a transaction with this idempotency_key already committed, return it
	if existing, err := repository.GetTransactionByIdempotencyKey(ctx, database, p.IdempotencyKey); err == nil {
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return models.Transaction{}, fmt.Errorf("transfer idempotency check: %w", err)
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return models.Transaction{}, fmt.Errorf("transfer begin tx: %w", err)
	}
	defer tx.Rollback()

	// Order accounts in canonical (ascending) order to prevent deadlock
	// when two transfers cross the same pair in opposite directions.
	firstID, secondID := p.FromAccountID, p.ToAccountID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}

	// Lock both accounts, then read full account rows.
	accounts := make(map[string]models.Account, 2)
	for _, id := range []string{firstID, secondID} {
		// Lock is undone after Rollback() or Commit() on tx
		if err := repository.LockAccount(ctx, tx, id); err != nil {
			return models.Transaction{}, fmt.Errorf("lock account: %w", err)
		}
		a, err := repository.GetAccount(ctx, tx, id)
		if err != nil {
			return models.Transaction{}, fmt.Errorf("get account: %w", err)
		}
		accounts[id] = a
	}

	from := accounts[p.FromAccountID]
	to := accounts[p.ToAccountID]

	// Only USER<->USER cash transfers. Cross-type flows live in their own service funcs.
	if from.AccountType != models.AccountUserCash || to.AccountType != models.AccountUserCash {
		return models.Transaction{}, errx.ErrInvalidAccountType
	}

	// Confirm matching currency
	if from.Currency != p.Currency || to.Currency != p.Currency {
		return models.Transaction{}, errx.ErrCurrencyMismatch
	}

	// Funds check — skip rail/platform sources (EXTERNAL, TREASURY).
	if needsFundsCheck(from.AccountType) {
		raw, err := repository.GetAccountBalance(ctx, tx, p.FromAccountID, nil)
		if err != nil {
			return models.Transaction{}, fmt.Errorf("get source balance: %w", err)
		}
		if availableBalance(from.AccountType, raw) < p.Amount {
			return models.Transaction{}, errx.ErrInsufficientFunds
		}
	}

	t, err := repository.CreateTransaction(ctx, tx, nil, p.IdempotencyKey, p.TransactionType, p.Description)
	if err != nil {
		// Catch concurrent caller with the same idempotency_key, 23505 is result of UNIQUE violation rejection
		// Re-fetch via the live *sql.DB (this tx is going to roll back) to see new transaction
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			existing, ferr := repository.GetTransactionByIdempotencyKey(ctx, database, p.IdempotencyKey)
			if ferr != nil {
				return models.Transaction{}, fmt.Errorf("transfer idempotency refetch: %w", ferr)
			}
			return existing, nil
		}
		return models.Transaction{}, fmt.Errorf("create transaction: %w", err)
	}

	now := time.Now()
	if _, err := repository.CreateEntry(ctx, tx, t.TransactionID, p.FromAccountID, p.Amount, p.Currency, models.DirectionDebit, p.Memo, now); err != nil {
		return models.Transaction{}, fmt.Errorf("create debit entry: %w", err)
	}
	if _, err := repository.CreateEntry(ctx, tx, t.TransactionID, p.ToAccountID, p.Amount, p.Currency, models.DirectionCredit, p.Memo, now); err != nil {
		return models.Transaction{}, fmt.Errorf("create credit entry: %w", err)
	}

	t, err = repository.UpdateTransactionStatus(ctx, tx, t.TransactionID, models.StatusPosted)
	if err != nil {
		return models.Transaction{}, fmt.Errorf("post transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return models.Transaction{}, fmt.Errorf("transfer commit: %w", err)
	}

	return t, nil
}

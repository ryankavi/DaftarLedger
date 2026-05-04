package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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

// needsFundsCheck reports whether the source account type requires a
// balance check. Rail/platform accounts (EXTERNAL, TREASURY) are unbounded
// sources by design — checking them would block legitimate flows.
func needsFundsCheck(t models.AccountType) bool {
	return t == models.AccountUserCash
}

// availableBalance translates raw net-debit balance into spendable units
// by flipping sign for CREDIT-normal account types. USER_CASH is a
// liability from the platform's POV, so a funded wallet has a negative
// raw balance.
func availableBalance(t models.AccountType, raw int64) int64 {
	if t == models.AccountUserCash {
		return -raw
	}
	return raw
}

func Transfer(ctx context.Context, database *sql.DB, p TransferParams) (models.Transaction, error) {
	if p.Amount <= 0 {
		return models.Transaction{}, errx.ErrInvalidAmount
	}
	if p.FromAccountID == p.ToAccountID {
		return models.Transaction{}, errx.ErrSameAccount
	}

	// Pre-check: if a prior call with this idempotency_key already committed, return it.
	// Race window between this read and the INSERT is closed by the UNIQUE constraint on idempotency_key;
	// a losing concurrent caller sees pq error 23505 and can retry — the retry hits this branch.
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

	// Restrict to peer USER_CASH transfers. Cross-type flows (deposit,
	// withdrawal, fee, internal rail moves) live in their own service funcs
	// with their own auth, funds-check, and audit semantics.
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

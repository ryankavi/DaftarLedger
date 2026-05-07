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

type PostParams struct {
	FromAccountID  string
	ToAccountID    string
	Amount         int64
	Currency       string
	IdempotencyKey string
	Memo           *string
	Description    *string
}

type PostType string

const (
	PostCashTransfer PostType = "CASH_TRANSFER"
	PostDeposit      PostType = "DEPOSIT"
	PostWithdraw     PostType = "WITHDRAW"
	PostAssessFee    PostType = "FEE_ASSESSMENT"
	PostRefundFee    PostType = "FEE_REFUND"
	// PostBookFee disabled: FEE_REVENUE -> TREASURY is not a balanced
	// double-entry pair (both sides decrease). Needs an equity account
	// or a redefined model. See switch case below.
	// PostBookFee      PostType = "FEE_REVENUE"
)

func Transfer(ctx context.Context, database *sql.DB, p PostParams) (models.Transaction, error) {
	return post(ctx, database, p, PostCashTransfer)
}
func Deposit(ctx context.Context, database *sql.DB, p PostParams) (models.Transaction, error) {
	return post(ctx, database, p, PostDeposit)
}
func Withdrawal(ctx context.Context, database *sql.DB, p PostParams) (models.Transaction, error) {
	return post(ctx, database, p, PostWithdraw)
}
func AssessFee(ctx context.Context, database *sql.DB, p PostParams) (models.Transaction, error) {
	return post(ctx, database, p, PostAssessFee)
}
func RefundFee(ctx context.Context, database *sql.DB, p PostParams) (models.Transaction, error) {
	return post(ctx, database, p, PostRefundFee)
}

func post(ctx context.Context, database *sql.DB, p PostParams, postType PostType) (models.Transaction, error) {
	if p.Amount <= 0 {
		return models.Transaction{}, errx.ErrInvalidAmount
	}
	if p.FromAccountID == p.ToAccountID {
		return models.Transaction{}, errx.ErrSameAccount
	}

	// If a transaction with this idempotency_key already committed, return it.
	// If the existing transaction is a different post type, the caller reused
	// the key across flows — reject with ErrIdempotencyConflict.
	if existing, err := repository.GetTransactionByIdempotencyKey(ctx, database, p.IdempotencyKey); err == nil {
		if existing.TransactionType != string(postType) {
			return models.Transaction{}, fmt.Errorf("idempotency_key %q already used for type %q, requested %q: %w", p.IdempotencyKey, existing.TransactionType, postType, errx.ErrIdempotencyConflict)
		}
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return models.Transaction{}, fmt.Errorf("post idempotency check: %w", err)
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return models.Transaction{}, fmt.Errorf("post begin tx: %w", err)
	}
	defer tx.Rollback()

	// Order accounts in canonical (ascending) order to prevent deadlock
	// when two posts cross the same pair in opposite directions.
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

	var fromDirection, toDirection models.EntryDirection

	// Restrict post and set entry directions based on account type.
	// Sign rule: source loses on its normal side; dest gains on its normal side.
	switch postType {
	case PostCashTransfer:
		if from.AccountType != models.AccountUserCash || to.AccountType != models.AccountUserCash {
			return models.Transaction{}, fmt.Errorf("cash transfer requires USER_CASH -> USER_CASH: %w", errx.ErrInvalidAccountType)
		}
		fromDirection, toDirection = models.DirectionDebit, models.DirectionCredit
	case PostDeposit:
		if from.AccountType != models.AccountExternal || to.AccountType != models.AccountUserCash {
			return models.Transaction{}, fmt.Errorf("deposit requires EXTERNAL -> USER_CASH: %w", errx.ErrInvalidAccountType)
		}
		fromDirection, toDirection = models.DirectionDebit, models.DirectionCredit
	case PostWithdraw:
		if from.AccountType != models.AccountUserCash || to.AccountType != models.AccountExternal {
			return models.Transaction{}, fmt.Errorf("withdraw requires USER_CASH -> EXTERNAL: %w", errx.ErrInvalidAccountType)
		}
		fromDirection, toDirection = models.DirectionDebit, models.DirectionCredit
	case PostAssessFee:
		if from.AccountType != models.AccountUserCash || to.AccountType != models.AccountFeeRevenue {
			return models.Transaction{}, fmt.Errorf("fee assessment requires USER_CASH -> FEE_REVENUE: %w", errx.ErrInvalidAccountType)
		}
		fromDirection, toDirection = models.DirectionDebit, models.DirectionCredit
	case PostRefundFee:
		if from.AccountType != models.AccountFeeRevenue || to.AccountType != models.AccountUserCash {
			return models.Transaction{}, fmt.Errorf("fee refund requires FEE_REVENUE -> USER_CASH: %w", errx.ErrInvalidAccountType)
		}
		fromDirection, toDirection = models.DirectionDebit, models.DirectionCredit
	// case PostBookFee:
	// 	// Disabled: FEE_REVENUE (CREDIT-normal) -> TREASURY (DEBIT-normal) is
	// 	// not a balanced double-entry pair. Closing revenue into asset needs
	// 	// an equity account (retained earnings) or a redefined model.
	// 	if from.AccountType != models.AccountFeeRevenue || to.AccountType != models.AccountTreasury {
	// 		return models.Transaction{}, fmt.Errorf("fee booking requires FEE_REVENUE -> TREASURY: %w", errx.ErrInvalidAccountType)
	// 	}
	// 	fromDirection, toDirection = models.DirectionDebit, models.DirectionDebit
	default:
		return models.Transaction{}, fmt.Errorf("unknown post type %q: %w", postType, errx.ErrInvalidAccountType)
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

	t, err := repository.CreateTransaction(ctx, tx, nil, p.IdempotencyKey, string(postType), p.Description)
	if err != nil {
		// Catch concurrent caller with the same idempotency_key, 23505 is result of UNIQUE violation rejection
		// Re-fetch via the live *sql.DB (this tx is going to roll back) to see new transaction
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			existing, ferr := repository.GetTransactionByIdempotencyKey(ctx, database, p.IdempotencyKey)
			if ferr != nil {
				return models.Transaction{}, fmt.Errorf("post idempotency refetch: %w", ferr)
			}
			if existing.TransactionType != string(postType) {
				return models.Transaction{}, fmt.Errorf("idempotency_key %q already used for type %q, requested %q: %w", p.IdempotencyKey, existing.TransactionType, postType, errx.ErrIdempotencyConflict)
			}
			return existing, nil
		}
		return models.Transaction{}, fmt.Errorf("create transaction: %w", err)
	}

	now := time.Now()

	if _, err := repository.CreateEntry(ctx, tx, t.TransactionID, p.FromAccountID, p.Amount, p.Currency, fromDirection, p.Memo, now); err != nil {
		return models.Transaction{}, fmt.Errorf("create from entry: %w", err)
	}
	if _, err := repository.CreateEntry(ctx, tx, t.TransactionID, p.ToAccountID, p.Amount, p.Currency, toDirection, p.Memo, now); err != nil {
		return models.Transaction{}, fmt.Errorf("create to entry: %w", err)
	}

	t, err = repository.UpdateTransactionStatus(ctx, tx, t.TransactionID, models.StatusPosted)
	if err != nil {
		return models.Transaction{}, fmt.Errorf("post transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return models.Transaction{}, fmt.Errorf("post commit: %w", err)
	}

	return t, nil
}

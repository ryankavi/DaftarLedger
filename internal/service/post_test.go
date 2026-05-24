package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	errx "github.com/ryankavi/payclone/internal/errors"
	"github.com/ryankavi/payclone/internal/models"
	"github.com/ryankavi/payclone/internal/repository"
)

func TestPost_CashTransfer_Happy(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	from := seedAccount(t, ctx, "ct-from@example.com", models.AccountUserCash, "USD")
	to := seedAccount(t, ctx, "ct-to@example.com", models.AccountUserCash, "USD")
	fundUserCash(t, ctx, from, "ct-ext@example.com", "ct-fund", 1000)

	txn, err := Post(ctx, testDB, PostParams{
		FromAccountID:  from.AccountID,
		ToAccountID:    to.AccountID,
		Amount:         400,
		Currency:       "USD",
		IdempotencyKey: "ct-1",
	}, PostCashTransfer)
	require.NoError(t, err)
	assert.Equal(t, models.StatusPosted, txn.TransactionStatus)
	assert.NotNil(t, txn.PostedAt)

	rawFrom, err := repository.GetAccountBalance(ctx, testDB, from.AccountID, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(600), availableBalance(models.AccountUserCash, rawFrom))

	rawTo, err := repository.GetAccountBalance(ctx, testDB, to.AccountID, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(400), availableBalance(models.AccountUserCash, rawTo))
}

func TestPost_Deposit_Happy(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	target := seedAccount(t, ctx, "dep@example.com", models.AccountUserCash, "USD")
	ext := seedAccount(t, ctx, "dep-ext@example.com", models.AccountExternal, "USD")

	_, err := Post(ctx, testDB, PostParams{
		FromAccountID:  ext.AccountID,
		ToAccountID:    target.AccountID,
		Amount:         500,
		Currency:       "USD",
		IdempotencyKey: "dep-1",
	}, PostDeposit)
	require.NoError(t, err)

	raw, _ := repository.GetAccountBalance(ctx, testDB, target.AccountID, nil)
	assert.Equal(t, int64(500), availableBalance(models.AccountUserCash, raw))
}

func TestPost_Withdraw_Happy(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	user := seedAccount(t, ctx, "wd@example.com", models.AccountUserCash, "USD")
	ext := fundUserCash(t, ctx, user, "wd-ext@example.com", "wd-fund", 1000)

	_, err := Post(ctx, testDB, PostParams{
		FromAccountID:  user.AccountID,
		ToAccountID:    ext.AccountID,
		Amount:         300,
		Currency:       "USD",
		IdempotencyKey: "wd-1",
	}, PostWithdraw)
	require.NoError(t, err)

	raw, _ := repository.GetAccountBalance(ctx, testDB, user.AccountID, nil)
	assert.Equal(t, int64(700), availableBalance(models.AccountUserCash, raw))
}

func TestPost_AssessFee_Happy(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	user := seedAccount(t, ctx, "fee@example.com", models.AccountUserCash, "USD")
	fee := seedAccount(t, ctx, "fee-rev@example.com", models.AccountFeeRevenue, "USD")
	fundUserCash(t, ctx, user, "fee-ext@example.com", "fee-fund", 1000)

	_, err := Post(ctx, testDB, PostParams{
		FromAccountID:  user.AccountID,
		ToAccountID:    fee.AccountID,
		Amount:         50,
		Currency:       "USD",
		IdempotencyKey: "fee-1",
	}, PostAssessFee)
	require.NoError(t, err)

	raw, _ := repository.GetAccountBalance(ctx, testDB, user.AccountID, nil)
	assert.Equal(t, int64(950), availableBalance(models.AccountUserCash, raw))
}

func TestPost_RefundFee_Happy(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	user := seedAccount(t, ctx, "rf@example.com", models.AccountUserCash, "USD")
	fee := seedAccount(t, ctx, "rf-rev@example.com", models.AccountFeeRevenue, "USD")
	fundUserCash(t, ctx, user, "rf-ext@example.com", "rf-fund", 1000)

	// First assess to give FEE_REVENUE balance, then refund.
	_, err := Post(ctx, testDB, PostParams{
		FromAccountID:  user.AccountID,
		ToAccountID:    fee.AccountID,
		Amount:         100,
		Currency:       "USD",
		IdempotencyKey: "rf-assess",
	}, PostAssessFee)
	require.NoError(t, err)

	_, err = Post(ctx, testDB, PostParams{
		FromAccountID:  fee.AccountID,
		ToAccountID:    user.AccountID,
		Amount:         100,
		Currency:       "USD",
		IdempotencyKey: "rf-refund",
	}, PostRefundFee)
	require.NoError(t, err)

	raw, _ := repository.GetAccountBalance(ctx, testDB, user.AccountID, nil)
	assert.Equal(t, int64(1000), availableBalance(models.AccountUserCash, raw))
}

func TestPost_InvalidAmount(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "amt-a@example.com", models.AccountUserCash, "USD")
	b := seedAccount(t, ctx, "amt-b@example.com", models.AccountUserCash, "USD")

	_, err := Post(ctx, testDB, PostParams{
		FromAccountID: a.AccountID, ToAccountID: b.AccountID,
		Amount: 0, Currency: "USD", IdempotencyKey: "amt-0",
	}, PostCashTransfer)
	require.ErrorIs(t, err, errx.ErrInvalidAmount)

	_, err = Post(ctx, testDB, PostParams{
		FromAccountID: a.AccountID, ToAccountID: b.AccountID,
		Amount: -5, Currency: "USD", IdempotencyKey: "amt-neg",
	}, PostCashTransfer)
	require.ErrorIs(t, err, errx.ErrInvalidAmount)
}

func TestPost_SameAccount(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "same@example.com", models.AccountUserCash, "USD")

	_, err := Post(ctx, testDB, PostParams{
		FromAccountID: a.AccountID, ToAccountID: a.AccountID,
		Amount: 10, Currency: "USD", IdempotencyKey: "same-1",
	}, PostCashTransfer)
	require.ErrorIs(t, err, errx.ErrSameAccount)
}

func TestPost_CurrencyMismatch(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	from := seedAccount(t, ctx, "cm-from@example.com", models.AccountUserCash, "USD")
	to := seedAccount(t, ctx, "cm-to@example.com", models.AccountUserCash, "EUR")
	fundUserCash(t, ctx, from, "cm-ext@example.com", "cm-fund", 1000)

	_, err := Post(ctx, testDB, PostParams{
		FromAccountID: from.AccountID, ToAccountID: to.AccountID,
		Amount: 100, Currency: "USD", IdempotencyKey: "cm-1",
	}, PostCashTransfer)
	require.ErrorIs(t, err, errx.ErrCurrencyMismatch)
}

func TestPost_InsufficientFunds(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	from := seedAccount(t, ctx, "if-from@example.com", models.AccountUserCash, "USD")
	to := seedAccount(t, ctx, "if-to@example.com", models.AccountUserCash, "USD")
	fundUserCash(t, ctx, from, "if-ext@example.com", "if-fund", 100)

	_, err := Post(ctx, testDB, PostParams{
		FromAccountID: from.AccountID, ToAccountID: to.AccountID,
		Amount: 500, Currency: "USD", IdempotencyKey: "if-1",
	}, PostCashTransfer)
	require.ErrorIs(t, err, errx.ErrInsufficientFunds)
}

func TestPost_AccountNotOpen(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	from := seedAccount(t, ctx, "ao-from@example.com", models.AccountUserCash, "USD")
	to := seedAccount(t, ctx, "ao-to@example.com", models.AccountUserCash, "USD")
	require.NoError(t, FreezeAccount(ctx, testDB, to.AccountID))
	fundUserCash(t, ctx, from, "ao-ext@example.com", "ao-fund", 1000)

	_, err := Post(ctx, testDB, PostParams{
		FromAccountID: from.AccountID, ToAccountID: to.AccountID,
		Amount: 100, Currency: "USD", IdempotencyKey: "ao-1",
	}, PostCashTransfer)
	require.ErrorIs(t, err, errx.ErrAccountNotOpen)
}

func TestPost_AccountNotFound(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "nf@example.com", models.AccountUserCash, "USD")

	_, err := Post(ctx, testDB, PostParams{
		FromAccountID: a.AccountID, ToAccountID: missingAccountID,
		Amount: 10, Currency: "USD", IdempotencyKey: "nf-1",
	}, PostCashTransfer)
	require.ErrorIs(t, err, errx.ErrAccountNotFound)
}

func TestPost_WrongAccountTypeGuard(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	// PostCashTransfer requires USER_CASH -> USER_CASH; supply EXTERNAL as source.
	ext := seedAccount(t, ctx, "wt-ext@example.com", models.AccountExternal, "USD")
	user := seedAccount(t, ctx, "wt-user@example.com", models.AccountUserCash, "USD")

	_, err := Post(ctx, testDB, PostParams{
		FromAccountID: ext.AccountID, ToAccountID: user.AccountID,
		Amount: 10, Currency: "USD", IdempotencyKey: "wt-1",
	}, PostCashTransfer)
	require.ErrorIs(t, err, errx.ErrInvalidAccountType)
}

func TestPost_IdempotentRetry(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	from := seedAccount(t, ctx, "id-from@example.com", models.AccountUserCash, "USD")
	to := seedAccount(t, ctx, "id-to@example.com", models.AccountUserCash, "USD")
	fundUserCash(t, ctx, from, "id-ext@example.com", "id-fund", 1000)

	params := PostParams{
		FromAccountID: from.AccountID, ToAccountID: to.AccountID,
		Amount: 200, Currency: "USD", IdempotencyKey: "retry-1",
	}
	first, err := Post(ctx, testDB, params, PostCashTransfer)
	require.NoError(t, err)

	second, err := Post(ctx, testDB, params, PostCashTransfer)
	require.NoError(t, err)
	assert.Equal(t, first.TransactionID, second.TransactionID, "same key must return same txn")

	// Balance moved exactly once.
	raw, _ := repository.GetAccountBalance(ctx, testDB, from.AccountID, nil)
	assert.Equal(t, int64(800), availableBalance(models.AccountUserCash, raw))
}

func TestPost_IdempotencyTypeConflict(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	from := seedAccount(t, ctx, "tc-from@example.com", models.AccountUserCash, "USD")
	to := seedAccount(t, ctx, "tc-to@example.com", models.AccountUserCash, "USD")
	fundUserCash(t, ctx, from, "tc-ext@example.com", "tc-fund", 1000)

	_, err := Post(ctx, testDB, PostParams{
		FromAccountID: from.AccountID, ToAccountID: to.AccountID,
		Amount: 100, Currency: "USD", IdempotencyKey: "shared-key",
	}, PostCashTransfer)
	require.NoError(t, err)

	// Same key, different post type.
	ext := seedAccount(t, ctx, "tc-ext2@example.com", models.AccountExternal, "USD")
	_, err = Post(ctx, testDB, PostParams{
		FromAccountID: ext.AccountID, ToAccountID: to.AccountID,
		Amount: 100, Currency: "USD", IdempotencyKey: "shared-key",
	}, PostDeposit)
	require.ErrorIs(t, err, errx.ErrIdempotencyConflict)
}

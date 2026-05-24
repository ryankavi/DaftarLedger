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

// postAndReverse posts a forward txn then reverses it, returning both.
func postAndReverse(t *testing.T, ctx context.Context, p PostParams, pt PostType, revKey string) (forward, reversal models.Transaction) {
	t.Helper()
	forward, err := Post(ctx, testDB, p, pt)
	require.NoError(t, err)
	reversal, err = Reverse(ctx, testDB, ReverseParams{
		TransactionID:  forward.TransactionID,
		IdempotencyKey: revKey,
	})
	require.NoError(t, err)
	return forward, reversal
}

func TestReverse_CashTransfer(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	from := seedAccount(t, ctx, "rct-from@example.com", models.AccountUserCash, "USD")
	to := seedAccount(t, ctx, "rct-to@example.com", models.AccountUserCash, "USD")
	fundUserCash(t, ctx, from, "rct-ext@example.com", "rct-fund", 1000)

	_, rev := postAndReverse(t, ctx, PostParams{
		FromAccountID: from.AccountID, ToAccountID: to.AccountID,
		Amount: 300, Currency: "USD", IdempotencyKey: "rct-fwd",
	}, PostCashTransfer, "rct-rev")
	assert.Equal(t, string(PostCashTransferReversal), rev.TransactionType)

	rawFrom, _ := repository.GetAccountBalance(ctx, testDB, from.AccountID, nil)
	assert.Equal(t, int64(1000), availableBalance(models.AccountUserCash, rawFrom))
	rawTo, _ := repository.GetAccountBalance(ctx, testDB, to.AccountID, nil)
	assert.Equal(t, int64(0), availableBalance(models.AccountUserCash, rawTo))
}

func TestReverse_Deposit(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	user := seedAccount(t, ctx, "rd-user@example.com", models.AccountUserCash, "USD")
	ext := seedAccount(t, ctx, "rd-ext@example.com", models.AccountExternal, "USD")

	_, rev := postAndReverse(t, ctx, PostParams{
		FromAccountID: ext.AccountID, ToAccountID: user.AccountID,
		Amount: 500, Currency: "USD", IdempotencyKey: "rd-fwd",
	}, PostDeposit, "rd-rev")
	assert.Equal(t, string(PostDepositReversal), rev.TransactionType)

	raw, _ := repository.GetAccountBalance(ctx, testDB, user.AccountID, nil)
	assert.Equal(t, int64(0), availableBalance(models.AccountUserCash, raw))
}

func TestReverse_Withdraw(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	user := seedAccount(t, ctx, "rw-user@example.com", models.AccountUserCash, "USD")
	ext := fundUserCash(t, ctx, user, "rw-ext@example.com", "rw-fund", 1000)

	_, rev := postAndReverse(t, ctx, PostParams{
		FromAccountID: user.AccountID, ToAccountID: ext.AccountID,
		Amount: 400, Currency: "USD", IdempotencyKey: "rw-fwd",
	}, PostWithdraw, "rw-rev")
	assert.Equal(t, string(PostWithdrawReversal), rev.TransactionType)

	raw, _ := repository.GetAccountBalance(ctx, testDB, user.AccountID, nil)
	assert.Equal(t, int64(1000), availableBalance(models.AccountUserCash, raw))
}

func TestReverse_AssessFee(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	user := seedAccount(t, ctx, "raf-user@example.com", models.AccountUserCash, "USD")
	fee := seedAccount(t, ctx, "raf-fee@example.com", models.AccountFeeRevenue, "USD")
	fundUserCash(t, ctx, user, "raf-ext@example.com", "raf-fund", 1000)

	_, rev := postAndReverse(t, ctx, PostParams{
		FromAccountID: user.AccountID, ToAccountID: fee.AccountID,
		Amount: 100, Currency: "USD", IdempotencyKey: "raf-fwd",
	}, PostAssessFee, "raf-rev")
	assert.Equal(t, string(PostAssessFeeReversal), rev.TransactionType)

	raw, _ := repository.GetAccountBalance(ctx, testDB, user.AccountID, nil)
	assert.Equal(t, int64(1000), availableBalance(models.AccountUserCash, raw))
}

func TestReverse_RefundFee(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	user := seedAccount(t, ctx, "rrf-user@example.com", models.AccountUserCash, "USD")
	fee := seedAccount(t, ctx, "rrf-fee@example.com", models.AccountFeeRevenue, "USD")
	fundUserCash(t, ctx, user, "rrf-ext@example.com", "rrf-fund", 1000)

	// Assess to give FEE_REVENUE a balance, refund it back, then reverse the refund.
	_, err := Post(ctx, testDB, PostParams{
		FromAccountID: user.AccountID, ToAccountID: fee.AccountID,
		Amount: 200, Currency: "USD", IdempotencyKey: "rrf-assess",
	}, PostAssessFee)
	require.NoError(t, err)

	_, rev := postAndReverse(t, ctx, PostParams{
		FromAccountID: fee.AccountID, ToAccountID: user.AccountID,
		Amount: 200, Currency: "USD", IdempotencyKey: "rrf-refund",
	}, PostRefundFee, "rrf-rev")
	assert.Equal(t, string(PostRefundFeeReversal), rev.TransactionType)

	raw, _ := repository.GetAccountBalance(ctx, testDB, user.AccountID, nil)
	assert.Equal(t, int64(800), availableBalance(models.AccountUserCash, raw))
}

// TestReverse_OfReversal posts a deposit, reverses it, then reverses the
// reversal. The chained call must re-enter Post via the forward inverse from
// reversalOf, succeeding and restoring the user's balance.
func TestReverse_OfReversal(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	user := seedAccount(t, ctx, "ror-user@example.com", models.AccountUserCash, "USD")
	ext := seedAccount(t, ctx, "ror-ext@example.com", models.AccountExternal, "USD")

	fwd, err := Post(ctx, testDB, PostParams{
		FromAccountID: ext.AccountID, ToAccountID: user.AccountID,
		Amount: 600, Currency: "USD", IdempotencyKey: "ror-fwd",
	}, PostDeposit)
	require.NoError(t, err)

	rev, err := Reverse(ctx, testDB, ReverseParams{TransactionID: fwd.TransactionID, IdempotencyKey: "ror-rev"})
	require.NoError(t, err)

	// Reverse the reversal — back to a deposit equivalent.
	revRev, err := Reverse(ctx, testDB, ReverseParams{TransactionID: rev.TransactionID, IdempotencyKey: "ror-rev-rev"})
	require.NoError(t, err)
	assert.Equal(t, string(PostDeposit), revRev.TransactionType, "reversal of reversal flips back to forward type")

	raw, _ := repository.GetAccountBalance(ctx, testDB, user.AccountID, nil)
	assert.Equal(t, int64(600), availableBalance(models.AccountUserCash, raw))
}

func TestReverse_NotPosted(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	// Insert a PENDING txn directly via repo so it never flips to POSTED.
	txn, err := repository.CreateTransaction(ctx, testDB, nil, "pending-key", string(PostCashTransfer), nil)
	require.NoError(t, err)

	_, err = Reverse(ctx, testDB, ReverseParams{TransactionID: txn.TransactionID, IdempotencyKey: "np-rev"})
	require.ErrorIs(t, err, errx.ErrNotReversible)
}

func TestReverse_AlreadyReversed(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	user := seedAccount(t, ctx, "ar-user@example.com", models.AccountUserCash, "USD")
	ext := seedAccount(t, ctx, "ar-ext@example.com", models.AccountExternal, "USD")

	fwd, err := Post(ctx, testDB, PostParams{
		FromAccountID: ext.AccountID, ToAccountID: user.AccountID,
		Amount: 100, Currency: "USD", IdempotencyKey: "ar-fwd",
	}, PostDeposit)
	require.NoError(t, err)

	_, err = Reverse(ctx, testDB, ReverseParams{TransactionID: fwd.TransactionID, IdempotencyKey: "ar-rev-1"})
	require.NoError(t, err)

	_, err = Reverse(ctx, testDB, ReverseParams{TransactionID: fwd.TransactionID, IdempotencyKey: "ar-rev-2"})
	require.ErrorIs(t, err, errx.ErrAlreadyReversed)
}

// TestReverse_BypassesFrozenGuard confirms reversal posts succeed even when
// one side has been frozen after the original posted. The reversalPostTypes
// map should skip the OPEN check.
func TestReverse_BypassesFrozenGuard(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	user := seedAccount(t, ctx, "rf-user@example.com", models.AccountUserCash, "USD")
	ext := seedAccount(t, ctx, "rf-ext2@example.com", models.AccountExternal, "USD")

	fwd, err := Post(ctx, testDB, PostParams{
		FromAccountID: ext.AccountID, ToAccountID: user.AccountID,
		Amount: 100, Currency: "USD", IdempotencyKey: "frz-fwd",
	}, PostDeposit)
	require.NoError(t, err)

	require.NoError(t, FreezeAccount(ctx, testDB, user.AccountID))

	_, err = Reverse(ctx, testDB, ReverseParams{TransactionID: fwd.TransactionID, IdempotencyKey: "frz-rev"})
	require.NoError(t, err, "reversal must bypass OPEN guard")
}

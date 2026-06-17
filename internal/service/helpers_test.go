package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ryankavi/payclone/internal/models"
	"github.com/ryankavi/payclone/internal/repository"
)

// seedUser creates a user with the given email. Email uniqueness is the
// caller's responsibility.
func seedUser(t *testing.T, ctx context.Context, email string) models.User {
	t.Helper()
	u, err := repository.CreateUser(ctx, testDB, email, "hashed-pw")
	require.NoError(t, err)
	return u
}

// seedAccount creates a user + account pair owned by that user.
func seedAccount(t *testing.T, ctx context.Context, email string, at models.AccountType, currency string) models.Account {
	t.Helper()
	u := seedUser(t, ctx, email)
	a, err := repository.CreateAccount(ctx, testDB, u.UserID, at, currency)
	require.NoError(t, err)
	return a
}

// fundUserCash deposits `amount` minor units into target via a PostDeposit
// from a freshly-seeded EXTERNAL account. Returns the EXTERNAL account so the
// caller can reuse it (e.g. as a withdraw destination).
func fundUserCash(t *testing.T, ctx context.Context, target models.Account, externalEmail, idemKey string, amount int64) models.Account {
	t.Helper()
	ext := seedAccount(t, ctx, externalEmail, models.AccountExternal, target.Currency)
	_, err := Post(ctx, testDB, PostParams{
		FromAccountID:  ext.AccountID,
		ToAccountID:    target.AccountID,
		Amount:         amount,
		Currency:       target.Currency,
		IdempotencyKey: idemKey,
	}, PostDeposit)
	require.NoError(t, err)
	return ext
}

package repository

import (
	"context"
	"testing"

	"github.com/ryankavi/payclone/internal/models"
	"github.com/stretchr/testify/require"
)

// seedAccount creates a user + account pair for tests that need a target
// account but don't care about ownership specifics. Email uniqueness is the
// caller's responsibility — pass a distinct value per call within a test.
func seedAccount(t *testing.T, ctx context.Context, email string, accountType models.AccountType) models.Account {
	t.Helper()
	u, err := CreateUser(ctx, testDB, email, "hashed-pw")
	require.NoError(t, err)
	a, err := CreateAccount(ctx, testDB, u.UserID, accountType, "USD")
	require.NoError(t, err)
	return a
}

// seedTransaction creates a PENDING transaction with the given idempotency
// key. Caller passes a key unique within the test.
func seedTransaction(t *testing.T, ctx context.Context, idemKey string) models.Transaction {
	t.Helper()
	txn, err := CreateTransaction(ctx, testDB, nil, idemKey, "TEST", nil)
	require.NoError(t, err)
	return txn
}

// ptr returns a pointer to v. Useful for inline construction of pointer
// arguments to repo functions (e.g. *time.Time for asOf, *string for memo)
// inside table-driven test cases.
func ptr[T any](v T) *T { return &v }

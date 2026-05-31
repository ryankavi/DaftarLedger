package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	errx "github.com/ryankavi/payclone/internal/errors"
	"github.com/ryankavi/payclone/internal/models"
)

// missingAccountID is a syntactically valid UUID guaranteed not to exist
// after truncateAll. Used to drive the not-found branches.
const missingAccountID = "00000000-0000-0000-0000-000000000000"

func TestCreateAccount(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	u, err := CreateUser(ctx, testDB, "owner@example.com")
	require.NoError(t, err)

	got, err := CreateAccount(ctx, testDB, u.UserID, models.AccountUserCash, "USD")
	require.NoError(t, err)

	assert.NotEmpty(t, got.AccountID, "AccountID should be DB-generated UUID")
	assert.Equal(t, models.AccountUserCash, got.AccountType)
	assert.Equal(t, models.AccountStatusOpen, got.AccountStatus, "schema default")
	assert.Equal(t, u.UserID, got.OwnerID)
	assert.Equal(t, "USD", got.Currency)
	assert.False(t, got.CreatedAt.IsZero(), "CreatedAt should be DB-populated")

	roundTrip, err := GetAccount(ctx, testDB, got.AccountID)
	require.NoError(t, err)
	assert.Equal(t, got.AccountID, roundTrip.AccountID)
	assert.Equal(t, got.OwnerID, roundTrip.OwnerID)
}

func TestGetAccount_NotFound(t *testing.T) {
	truncateAll(t)

	_, err := GetAccount(context.Background(), testDB, missingAccountID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestGetAccountsFromOwnerID(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	owner, err := CreateUser(ctx, testDB, "multi@example.com")
	require.NoError(t, err)

	// Two accounts for the same owner; distinct types to satisfy
	// UNIQUE(owner_id, account_type).
	a1, err := CreateAccount(ctx, testDB, owner.UserID, models.AccountUserCash, "USD")
	require.NoError(t, err)
	a2, err := CreateAccount(ctx, testDB, owner.UserID, models.AccountExternal, "USD")
	require.NoError(t, err)

	// A different owner's account must not leak into the result.
	other, err := CreateUser(ctx, testDB, "other@example.com")
	require.NoError(t, err)
	_, err = CreateAccount(ctx, testDB, other.UserID, models.AccountUserCash, "USD")
	require.NoError(t, err)

	got, err := GetAccountsFromOwnerID(ctx, testDB, owner.UserID)
	require.NoError(t, err)
	require.Len(t, got, 2, "should return exactly the owner's two accounts")

	ids := []string{got[0].AccountID, got[1].AccountID}
	assert.ElementsMatch(t, []string{a1.AccountID, a2.AccountID}, ids)
	for _, a := range got {
		assert.Equal(t, owner.UserID, a.OwnerID, "no other owner's account should leak")
	}
}

func TestGetAccountsFromOwnerID_Empty(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	owner, err := CreateUser(ctx, testDB, "noaccounts@example.com")
	require.NoError(t, err)

	// No rows is not an error for a list query — expect an empty result,
	// not sql.ErrNoRows.
	got, err := GetAccountsFromOwnerID(ctx, testDB, owner.UserID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestUpdateAccountStatus(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	u, err := CreateUser(ctx, testDB, "update@example.com")
	require.NoError(t, err)
	a, err := CreateAccount(ctx, testDB, u.UserID, models.AccountUserCash, "USD")
	require.NoError(t, err)
	require.Equal(t, models.AccountStatusOpen, a.AccountStatus, "precondition: new account is OPEN")

	updated, err := UpdateAccountStatus(ctx, testDB, a.AccountID, models.AccountStatusFrozen)
	require.NoError(t, err)
	assert.Equal(t, models.AccountStatusFrozen, updated.AccountStatus)
	assert.Equal(t, a.AccountID, updated.AccountID)

	// Confirm the change actually persisted, not just echoed by RETURNING.
	reread, err := GetAccount(ctx, testDB, a.AccountID)
	require.NoError(t, err)
	assert.Equal(t, models.AccountStatusFrozen, reread.AccountStatus)
}

func TestUpdateAccountStatus_NotFound(t *testing.T) {
	truncateAll(t)

	_, err := UpdateAccountStatus(context.Background(), testDB, missingAccountID, models.AccountStatusFrozen)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestLockAccount_Clean(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	u, err := CreateUser(ctx, testDB, "lock-ok@example.com")
	require.NoError(t, err)
	a, err := CreateAccount(ctx, testDB, u.UserID, models.AccountUserCash, "USD")
	require.NoError(t, err)

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	require.NoError(t, LockAccount(ctx, tx, a.AccountID))
	require.NoError(t, tx.Commit())
}

func TestLockAccount_AccountNotFound(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = LockAccount(ctx, tx, missingAccountID)
	require.ErrorIs(t, err, errx.ErrAccountNotFound)
}

// TestLockAccount_BlocksConcurrentLock proves SELECT ... FOR UPDATE actually
// blocks a second locker on the same row until the first transaction
// finishes. The kind of property a mocked DBTX could never exercise.
func TestLockAccount_BlocksConcurrentLock(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	u, err := CreateUser(ctx, testDB, "lock-contend@example.com")
	require.NoError(t, err)
	a, err := CreateAccount(ctx, testDB, u.UserID, models.AccountUserCash, "USD")
	require.NoError(t, err)

	tx1, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx1.Rollback() }()
	require.NoError(t, LockAccount(ctx, tx1, a.AccountID))

	tx2, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx2.Rollback() }()

	const blockWindow = 400 * time.Millisecond
	tx2Ctx, cancel := context.WithTimeout(ctx, blockWindow)
	defer cancel()

	start := time.Now()
	err = LockAccount(tx2Ctx, tx2, a.AccountID)
	elapsed := time.Since(start)

	require.Error(t, err, "second lock should fail when context cancels")
	assert.GreaterOrEqual(t, elapsed, blockWindow-50*time.Millisecond,
		"second lock returned in %v; expected to block ~%v (lock not held?)", elapsed, blockWindow)
}

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	errx "github.com/ryankavi/payclone/internal/errors"
	"github.com/ryankavi/payclone/internal/models"
	"github.com/ryankavi/payclone/internal/repository"
)

const missingAccountID = "00000000-0000-0000-0000-000000000000"

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		ok    bool
	}{
		{"simple", "user@example.com", true},
		{"plus tag", "user+tag@example.com", true},
		{"empty", "", false},
		{"whitespace only", "   ", false},
		{"no @", "userexample.com", false},
		{"display name form rejected", "User <user@example.com>", false},
		{"too long", strings.Repeat("a", 250) + "@x.com", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.ok, validateEmail(tc.email))
		})
	}
}

func TestCreateUserWithAccount_UnauthenticatedHappy(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a, err := CreateUserWithAccount(ctx, testDB, "new@example.com", "password123", string(models.AccountUserCash), "USD")
	require.NoError(t, err)
	assert.NotEmpty(t, a.AccountID)
	assert.Equal(t, models.AccountUserCash, a.AccountType)
	assert.Equal(t, models.AccountStatusOpen, a.AccountStatus)
	assert.Equal(t, "USD", a.Currency)
}

func TestCreateUserWithAccount_InvalidEmail(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	_, err := CreateUserWithAccount(ctx, testDB, "not-an-email", "password123", string(models.AccountUserCash), "USD")
	require.ErrorIs(t, err, errx.ErrInvalidEmail)
}

// Signup is always a USER, so a platform account type is forbidden.
func TestCreateUserWithAccount_PlatformTypeForbidden(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	_, err := CreateUserWithAccount(ctx, testDB, "platform-signup@example.com", "password123", string(models.AccountTreasury), "USD")
	require.ErrorIs(t, err, errx.ErrForbidden)
}

func TestCreateUserWithAccount_EmailTaken(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	_, err := CreateUserWithAccount(ctx, testDB, "dup@example.com", "password123", string(models.AccountUserCash), "USD")
	require.NoError(t, err)

	_, err = CreateUserWithAccount(ctx, testDB, "dup@example.com", "password123", string(models.AccountUserCash), "USD")
	require.ErrorIs(t, err, errx.ErrEmailTaken)
}

func TestCreateAccount_AuthenticatedHappy(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	u := seedUser(t, ctx, "owner@example.com")
	a, err := CreateAccount(ctx, testDB, u.UserID, models.RoleUser, string(models.AccountUserCash), "USD")
	require.NoError(t, err)
	assert.Equal(t, u.UserID, a.OwnerID)
	assert.Equal(t, models.AccountUserCash, a.AccountType)
}

func TestCreateAccount_TypeExists(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	u := seedUser(t, ctx, "owner2@example.com")
	_, err := CreateAccount(ctx, testDB, u.UserID, models.RoleUser, string(models.AccountUserCash), "USD")
	require.NoError(t, err)

	_, err = CreateAccount(ctx, testDB, u.UserID, models.RoleUser, string(models.AccountUserCash), "USD")
	require.ErrorIs(t, err, errx.ErrAccountTypeExists)
}

// A USER may only own USER_CASH; platform types are forbidden to them.
func TestCreateAccount_UserCannotCreatePlatformType(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	u := seedUser(t, ctx, "user-platform@example.com")
	_, err := CreateAccount(ctx, testDB, u.UserID, models.RoleUser, string(models.AccountExternal), "USD")
	require.ErrorIs(t, err, errx.ErrForbidden)
}

// An ADMIN provisions platform accounts and must not own a consumer wallet.
func TestCreateAccount_AdminCannotCreateUserCash(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	u := seedUser(t, ctx, "admin-wallet@example.com")
	_, err := CreateAccount(ctx, testDB, u.UserID, models.RoleAdmin, string(models.AccountUserCash), "USD")
	require.ErrorIs(t, err, errx.ErrForbidden)
}

func TestCreateAccount_AdminCanCreatePlatformType(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	u := seedUser(t, ctx, "admin-platform@example.com")
	a, err := CreateAccount(ctx, testDB, u.UserID, models.RoleAdmin, string(models.AccountExternal), "USD")
	require.NoError(t, err)
	assert.Equal(t, models.AccountExternal, a.AccountType)
}

func TestCreateAccount_UnknownTypeRejected(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	u := seedUser(t, ctx, "bad-type@example.com")
	_, err := CreateAccount(ctx, testDB, u.UserID, models.RoleAdmin, "NOT_A_TYPE", "USD")
	require.ErrorIs(t, err, errx.ErrInvalidAccountType)
}

func TestCloseAccount_Happy(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "close@example.com", models.AccountUserCash, "USD")
	require.NoError(t, CloseAccount(ctx, testDB, a.AccountID))

	got, err := repository.GetAccount(ctx, testDB, a.AccountID)
	require.NoError(t, err)
	assert.Equal(t, models.AccountStatusClosed, got.AccountStatus)
}

func TestCloseAccount_AlreadyClosed(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "close2@example.com", models.AccountUserCash, "USD")
	require.NoError(t, CloseAccount(ctx, testDB, a.AccountID))

	err := CloseAccount(ctx, testDB, a.AccountID)
	require.ErrorIs(t, err, errx.ErrAccountClosed)
}

func TestCloseAccount_NonZeroBalance(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	target := seedAccount(t, ctx, "fund@example.com", models.AccountUserCash, "USD")
	fundUserCash(t, ctx, target, "ext@example.com", "deposit-1", 1000)

	err := CloseAccount(ctx, testDB, target.AccountID)
	require.ErrorIs(t, err, errx.ErrBalanceNotZero)
}

func TestCloseAccount_NotFound(t *testing.T) {
	truncateAll(t)
	err := CloseAccount(context.Background(), testDB, missingAccountID)
	require.ErrorIs(t, err, errx.ErrAccountNotFound)
}

func TestFreezeAccount_Happy(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "freeze@example.com", models.AccountUserCash, "USD")
	require.NoError(t, FreezeAccount(ctx, testDB, a.AccountID))

	got, err := repository.GetAccount(ctx, testDB, a.AccountID)
	require.NoError(t, err)
	assert.Equal(t, models.AccountStatusFrozen, got.AccountStatus)
}

func TestFreezeAccount_NotOpen(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "freeze2@example.com", models.AccountUserCash, "USD")
	require.NoError(t, FreezeAccount(ctx, testDB, a.AccountID))

	err := FreezeAccount(ctx, testDB, a.AccountID)
	require.ErrorIs(t, err, errx.ErrAccountNotOpen)
}

func TestReopenAccount_Happy(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "reopen@example.com", models.AccountUserCash, "USD")
	require.NoError(t, FreezeAccount(ctx, testDB, a.AccountID))
	require.NoError(t, ReopenAccount(ctx, testDB, a.AccountID))

	got, err := repository.GetAccount(ctx, testDB, a.AccountID)
	require.NoError(t, err)
	assert.Equal(t, models.AccountStatusOpen, got.AccountStatus)
}

func TestReopenAccount_NotFrozen(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "reopen2@example.com", models.AccountUserCash, "USD")
	err := ReopenAccount(ctx, testDB, a.AccountID)
	require.ErrorIs(t, err, errx.ErrAccountNotFrozen)
}

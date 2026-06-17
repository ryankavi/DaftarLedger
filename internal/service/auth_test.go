package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryankavi/payclone/internal/models"
	"github.com/ryankavi/payclone/internal/repository"
)

func TestEnsureAdmin_CreatesAndAuthenticates(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	require.NoError(t, EnsureAdmin(ctx, testDB, "admin@example.com", "admin-password"))

	u, err := repository.GetUserByEmail(ctx, testDB, "admin@example.com")
	require.NoError(t, err)
	assert.Equal(t, models.RoleAdmin, u.Role)

	// The bootstrapped admin logs in through the normal credential flow.
	got, err := Login(ctx, testDB, "admin@example.com", "admin-password")
	require.NoError(t, err)
	assert.Equal(t, models.RoleAdmin, got.Role)
	assert.Equal(t, u.UserID, got.UserID)
}

func TestEnsureAdmin_Idempotent(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	require.NoError(t, EnsureAdmin(ctx, testDB, "admin@example.com", "pw-one"))
	first, err := repository.GetUserByEmail(ctx, testDB, "admin@example.com")
	require.NoError(t, err)

	// Re-run with a new password: no error, same row, latest password wins.
	require.NoError(t, EnsureAdmin(ctx, testDB, "admin@example.com", "pw-two"))
	second, err := repository.GetUserByEmail(ctx, testDB, "admin@example.com")
	require.NoError(t, err)

	assert.Equal(t, first.UserID, second.UserID, "must not create a duplicate user")
	_, err = Login(ctx, testDB, "admin@example.com", "pw-two")
	require.NoError(t, err, "latest password should authenticate")
}

func TestEnsureAdmin_PromotesExistingUser(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	// A normal user exists first (role defaults to USER).
	_, err := repository.CreateUser(ctx, testDB, "promote@example.com", "hashed-pw")
	require.NoError(t, err)

	require.NoError(t, EnsureAdmin(ctx, testDB, "promote@example.com", "admin-pw"))

	u, err := repository.GetUserByEmail(ctx, testDB, "promote@example.com")
	require.NoError(t, err)
	assert.Equal(t, models.RoleAdmin, u.Role)
}

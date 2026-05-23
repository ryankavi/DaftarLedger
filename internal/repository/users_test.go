package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryankavi/payclone/internal/models"
)

const missingUserID = "00000000-0000-0000-0000-000000000000"

func TestCreateUser(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	const userEmail = "user@example.com"

	u, err := CreateUser(ctx, testDB, userEmail)
	require.NoError(t, err)

	assert.NotEmpty(t, u.UserID, "UserID should be DB-generated UUID")
	assert.Equal(t, userEmail, u.Email)
	assert.False(t, u.CreatedAt.IsZero(), "CreatedAt is DB-populated")
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	const userEmail = "dup@example.com"

	_, err := CreateUser(ctx, testDB, userEmail)
	require.NoError(t, err)

	_, err = CreateUser(ctx, testDB, userEmail)
	require.Error(t, err, "duplicate email insert should fail UNIQUE constraint")

	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr), "expected pq.Error, got %T: %v", err, err)
	assert.Equal(t, pq.ErrorCode("23505"), pqErr.Code, "expected unique_violation (23505)")
}

func TestGetUser(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	const userEmail = "user@example.com"

	u, err := CreateUser(ctx, testDB, userEmail)
	require.NoError(t, err)

	got, err := GetUser(ctx, testDB, u.UserID)
	require.NoError(t, err)
	assert.Equal(t, u.UserID, got.UserID)
	assert.Equal(t, userEmail, got.Email)
	assert.True(t, u.CreatedAt.Equal(got.CreatedAt), "CreatedAt should round-trip")
}

func TestGetUser_NotFound(t *testing.T) {
	truncateAll(t)

	_, err := GetUser(context.Background(), testDB, missingUserID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestUpdateUserEmail(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	const userEmail = "user@example.com"
	const newUserEmail = "new-user@example.com"

	u, err := CreateUser(ctx, testDB, userEmail)
	require.NoError(t, err)

	updated, err := UpdateUserEmail(ctx, testDB, u.UserID, newUserEmail)
	require.NoError(t, err)
	assert.Equal(t, newUserEmail, updated.Email)
	assert.Equal(t, u.UserID, updated.UserID, "UPDATE must not change the PK")
	assert.True(t, u.CreatedAt.Equal(updated.CreatedAt), "UPDATE must not change CreatedAt")

	reread, err := GetUser(ctx, testDB, u.UserID)
	require.NoError(t, err)
	assert.Equal(t, newUserEmail, reread.Email)
}

func TestUpdateUserEmail_NotFound(t *testing.T) {
	truncateAll(t)

	_, err := UpdateUserEmail(context.Background(), testDB, missingUserID, "ghost@example.com")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestDeleteUser(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	u, err := CreateUser(ctx, testDB, "delete-me@example.com")
	require.NoError(t, err)

	err = DeleteUser(ctx, testDB, u.UserID)
	require.NoError(t, err)

	_, err = GetUser(ctx, testDB, u.UserID)
	require.ErrorIs(t, err, sql.ErrNoRows, "user should not be retrievable after delete")
}

func TestDeleteUser_NotFound(t *testing.T) {
	truncateAll(t)

	err := DeleteUser(context.Background(), testDB, missingUserID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestDeleteUser_BlockedByAccountFK(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	// accounts.owner_id REFERENCES users(user_id) with no ON DELETE clause,
	// which defaults to NO ACTION. Deleting a user that still owns an
	// account must fail with a foreign-key violation (pq error code 23503),
	// not silently orphan the account.
	a := seedAccount(t, ctx, "owner-with-account@example.com", models.AccountUserCash)

	err := DeleteUser(ctx, testDB, a.OwnerID)
	require.Error(t, err, "delete should fail while user still owns an account")

	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr), "expected pq.Error, got %T: %v", err, err)
	assert.Equal(t, pq.ErrorCode("23503"), pqErr.Code, "expected foreign_key_violation (23503)")
}

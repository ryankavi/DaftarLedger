package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryankavi/payclone/internal/models"
)

const missingTransactionID = "00000000-0000-0000-0000-000000000000"

func TestCreateTransaction(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	extID := "11111111-1111-1111-1111-111111111111"
	desc := "test transaction"

	txn, err := CreateTransaction(ctx, testDB, &extID, "ck-create-txn", "CASH_TRANSFER", &desc)
	require.NoError(t, err)

	assert.NotEmpty(t, txn.TransactionID, "TransactionID should be DB-generated UUID")
	require.NotNil(t, txn.ExternalID)
	assert.Equal(t, extID, *txn.ExternalID)
	assert.Equal(t, "ck-create-txn", txn.IdempotencyKey)
	assert.Equal(t, "CASH_TRANSFER", txn.TransactionType)
	require.NotNil(t, txn.TransactionDescription)
	assert.Equal(t, desc, *txn.TransactionDescription)
	assert.Equal(t, models.StatusPending, txn.TransactionStatus, "CreateTransaction always inserts as PENDING")
	assert.Nil(t, txn.PostedAt, "PostedAt should be NULL on a new PENDING row")
	assert.False(t, txn.CreatedAt.IsZero(), "CreatedAt is DB-populated")
}

func TestCreateTransaction_NullableFieldsRoundTrip(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	// external_id and transaction_description are both nullable. Passing nil
	// must round-trip as nil — not an empty string, not an error.
	txn, err := CreateTransaction(ctx, testDB, nil, "ck-nullable", "DEPOSIT", nil)
	require.NoError(t, err)

	assert.Nil(t, txn.ExternalID, "nil external_id should round-trip as NULL → nil")
	assert.Nil(t, txn.TransactionDescription, "nil description should round-trip as NULL → nil")
}

func TestCreateTransaction_DuplicateIdempotencyKey(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	// idempotency_key has UNIQUE NOT NULL. The 23505 race fallback in the
	// service layer's Post engine depends on this constraint actually firing.
	_, err := CreateTransaction(ctx, testDB, nil, "ck-dup", "DEPOSIT", nil)
	require.NoError(t, err)

	_, err = CreateTransaction(ctx, testDB, nil, "ck-dup", "DEPOSIT", nil)
	require.Error(t, err, "second insert with duplicate idempotency_key should fail")

	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr), "expected pq.Error, got %T: %v", err, err)
	assert.Equal(t, pq.ErrorCode("23505"), pqErr.Code, "expected unique_violation (23505)")
}

func TestUpdateTransactionStatus_PendingToPostedSetsPostedAt(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	txn := seedTransaction(t, ctx, "ck-pending-to-posted")
	require.Nil(t, txn.PostedAt, "precondition: fresh PENDING txn has NULL posted_at")

	before := time.Now().UTC()
	updated, err := UpdateTransactionStatus(ctx, testDB, txn.TransactionID, models.StatusPosted)
	require.NoError(t, err)

	assert.Equal(t, models.StatusPosted, updated.TransactionStatus)
	require.NotNil(t, updated.PostedAt, "POSTED transition must populate posted_at via NOW()")
	// Clock source is the DB. Allow a generous bracket — only verifying the
	// timestamp landed in the right ballpark, not the wall clock precision.
	assert.WithinDuration(t, before, *updated.PostedAt, 5*time.Second,
		"posted_at should be set near NOW()")
}

func TestUpdateTransactionStatus_PendingToReversedLeavesPostedAtNull(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	txn := seedTransaction(t, ctx, "ck-pending-to-reversed")

	updated, err := UpdateTransactionStatus(ctx, testDB, txn.TransactionID, models.StatusReversed)
	require.NoError(t, err)

	assert.Equal(t, models.StatusReversed, updated.TransactionStatus)
	assert.Nil(t, updated.PostedAt,
		"posted_at CASE clause only fires on POSTED — REVERSED must not touch it")
}

func TestUpdateTransactionStatus_PostedToReversedPreservesPostedAt(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	txn := seedTransaction(t, ctx, "ck-posted-to-reversed")

	// First flip to POSTED so posted_at is set by the DB.
	posted, err := UpdateTransactionStatus(ctx, testDB, txn.TransactionID, models.StatusPosted)
	require.NoError(t, err)
	require.NotNil(t, posted.PostedAt)
	originalPostedAt := *posted.PostedAt

	// Then flip to REVERSED — posted_at should be preserved, not nulled.
	reversed, err := UpdateTransactionStatus(ctx, testDB, txn.TransactionID, models.StatusReversed)
	require.NoError(t, err)

	assert.Equal(t, models.StatusReversed, reversed.TransactionStatus)
	require.NotNil(t, reversed.PostedAt, "posted_at must survive the REVERSED transition")
	assert.True(t, originalPostedAt.Equal(*reversed.PostedAt),
		"posted_at should be unchanged: was %v, now %v", originalPostedAt, *reversed.PostedAt)
}

func TestUpdateTransactionStatus_NotFound(t *testing.T) {
	truncateAll(t)

	_, err := UpdateTransactionStatus(context.Background(), testDB, missingTransactionID, models.StatusPosted)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestGetTransaction(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	created := seedTransaction(t, ctx, "ck-get")

	got, err := GetTransaction(ctx, testDB, created.TransactionID)
	require.NoError(t, err)
	assert.Equal(t, created.TransactionID, got.TransactionID)
	assert.Equal(t, created.IdempotencyKey, got.IdempotencyKey)
	assert.Equal(t, created.TransactionType, got.TransactionType)
	assert.Equal(t, models.StatusPending, got.TransactionStatus)
}

func TestGetTransaction_NotFound(t *testing.T) {
	truncateAll(t)

	_, err := GetTransaction(context.Background(), testDB, missingTransactionID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestGetTransactionByIdempotencyKey(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	created := seedTransaction(t, ctx, "ck-by-key")

	got, err := GetTransactionByIdempotencyKey(ctx, testDB, "ck-by-key")
	require.NoError(t, err)
	assert.Equal(t, created.TransactionID, got.TransactionID, "lookup by key should return the same row")
}

func TestGetTransactionByIdempotencyKey_NotFound(t *testing.T) {
	truncateAll(t)

	_, err := GetTransactionByIdempotencyKey(context.Background(), testDB, "ck-never-existed")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestGetTransactionByExternalID_MultipleMatches(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	// external_id is intentionally NON-UNIQUE — it's an informational tag
	// that may appear on multiple transactions (e.g. a card-network ref ID
	// shared by an original txn and its later reversal). The function
	// returning []Transaction reflects this.
	extID := "22222222-2222-2222-2222-222222222222"

	a, err := CreateTransaction(ctx, testDB, &extID, "ck-ext-a", "DEPOSIT", nil)
	require.NoError(t, err)
	b, err := CreateTransaction(ctx, testDB, &extID, "ck-ext-b", "DEPOSIT_REVERSAL", nil)
	require.NoError(t, err)

	got, err := GetTransactionByExternalID(ctx, testDB, extID)
	require.NoError(t, err)
	require.Len(t, got, 2, "both transactions sharing the external_id should be returned")

	// Don't pin the slice order — the query has no ORDER BY. Collect IDs
	// into a set and assert membership.
	ids := map[string]bool{got[0].TransactionID: true, got[1].TransactionID: true}
	assert.True(t, ids[a.TransactionID], "result missing txn a")
	assert.True(t, ids[b.TransactionID], "result missing txn b")
}

func TestGetTransactionByExternalID_NoMatch(t *testing.T) {
	truncateAll(t)

	// Empty slice + nil error is the contract — not sql.ErrNoRows. Service
	// layer uses len(result) == 0 to branch, not error checks.
	got, err := GetTransactionByExternalID(context.Background(), testDB, missingTransactionID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestTransactionHasReversal_False(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	// "Has reversal" really asks: does any other transaction carry this
	// transaction_id as its external_id? With only the original present,
	// the answer is false.
	original := seedTransaction(t, ctx, "ck-no-reversal")

	hasRev, err := TransactionHasReversal(ctx, testDB, original.TransactionID)
	require.NoError(t, err)
	assert.False(t, hasRev)
}

func TestTransactionHasReversal_True(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	// Reversal convention: a reversal transaction stores the ORIGINAL's
	// transaction_id in its external_id column. So once a reversal exists,
	// TransactionHasReversal(original) is true.
	original := seedTransaction(t, ctx, "ck-original")

	_, err := CreateTransaction(ctx, testDB, &original.TransactionID, "ck-reversal", "CASH_TRANSFER_REVERSAL", nil)
	require.NoError(t, err)

	hasRev, err := TransactionHasReversal(ctx, testDB, original.TransactionID)
	require.NoError(t, err)
	assert.True(t, hasRev)
}

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryankavi/payclone/internal/models"
)

const missingEntryID = "00000000-0000-0000-0000-000000000000"

func TestCreateEntry(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "entry-create@example.com", models.AccountUserCash)
	txn := seedTransaction(t, ctx, "ck-create-entry")

	effective := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	memo := "test memo"

	e, err := CreateEntry(ctx, testDB, txn.TransactionID, a.AccountID, 12345, "USD", models.DirectionDebit, &memo, effective)
	require.NoError(t, err)

	assert.NotEmpty(t, e.EntryID, "EntryID should be DB-generated UUID")
	assert.Equal(t, txn.TransactionID, e.TransactionID)
	assert.Equal(t, a.AccountID, e.AccountID)
	assert.Equal(t, int64(12345), e.Amount)
	assert.Equal(t, "USD", e.Currency)
	assert.Equal(t, models.DirectionDebit, e.Direction)
	require.NotNil(t, e.Memo)
	assert.Equal(t, "test memo", *e.Memo)
	assert.False(t, e.CreatedAt.IsZero(), "CreatedAt is DB-populated")
	assert.True(t, e.EffectiveAt.Equal(effective),
		"EffectiveAt should equal caller-supplied %v, got %v", effective, e.EffectiveAt)
}

func TestCreateEntry_NilMemo(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "entry-nilmemo@example.com", models.AccountUserCash)
	txn := seedTransaction(t, ctx, "ck-nilmemo")

	e, err := CreateEntry(ctx, testDB, txn.TransactionID, a.AccountID, 100, "USD", models.DirectionCredit, nil, time.Now())
	require.NoError(t, err)
	assert.Nil(t, e.Memo, "nil memo should round-trip as NULL → nil")
}

func TestGetEntry(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "entry-get@example.com", models.AccountUserCash)
	txn := seedTransaction(t, ctx, "ck-get-entry")

	created, err := CreateEntry(ctx, testDB, txn.TransactionID, a.AccountID, 500, "USD", models.DirectionDebit, nil, time.Now())
	require.NoError(t, err)

	got, err := GetEntry(ctx, testDB, created.EntryID)
	require.NoError(t, err)
	assert.Equal(t, created.EntryID, got.EntryID)
	assert.Equal(t, int64(500), got.Amount)
	assert.Equal(t, models.DirectionDebit, got.Direction)
}

func TestGetEntry_NotFound(t *testing.T) {
	truncateAll(t)

	_, err := GetEntry(context.Background(), testDB, missingEntryID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestGetEntriesByTransactionID(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	src := seedAccount(t, ctx, "src@example.com", models.AccountUserCash)
	dst := seedAccount(t, ctx, "dst@example.com", models.AccountUserCash)
	txn := seedTransaction(t, ctx, "ck-bytxn")

	now := time.Now().UTC()
	debitEntry, err := CreateEntry(ctx, testDB, txn.TransactionID, src.AccountID, 200, "USD", models.DirectionDebit, nil, now)
	require.NoError(t, err)
	creditEntry, err := CreateEntry(ctx, testDB, txn.TransactionID, dst.AccountID, 200, "USD", models.DirectionCredit, nil, now)
	require.NoError(t, err)

	entries, err := GetEntriesByTransactionID(ctx, testDB, txn.TransactionID)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// Postgres sorts enums by CREATE TYPE declaration order, not alphabetically.
	// `entry_direction` is declared as ('DEBIT', 'CREDIT'), so DEBIT < CREDIT
	// in enum order → ORDER BY direction DESC returns CREDIT first.
	assert.Equal(t, creditEntry.EntryID, entries[0].EntryID)
	assert.Equal(t, models.DirectionCredit, entries[0].Direction)
	assert.Equal(t, debitEntry.EntryID, entries[1].EntryID)
	assert.Equal(t, models.DirectionDebit, entries[1].Direction)
}

func TestGetEntriesByTransactionID_Empty(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	// Real txn that simply has no entries yet — more realistic than a
	// non-existent transaction_id.
	txn := seedTransaction(t, ctx, "ck-empty-txn")

	entries, err := GetEntriesByTransactionID(ctx, testDB, txn.TransactionID)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestGetEntriesByAccountID_Pagination(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "page@example.com", models.AccountUserCash)
	txn := seedTransaction(t, ctx, "ck-page")

	// Three entries with strictly increasing effective_at. ORDER BY
	// effective_at DESC means newest comes back first.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e1, err := CreateEntry(ctx, testDB, txn.TransactionID, a.AccountID, 100, "USD", models.DirectionDebit, nil, base)
	require.NoError(t, err)
	e2, err := CreateEntry(ctx, testDB, txn.TransactionID, a.AccountID, 200, "USD", models.DirectionDebit, nil, base.Add(time.Hour))
	require.NoError(t, err)
	e3, err := CreateEntry(ctx, testDB, txn.TransactionID, a.AccountID, 300, "USD", models.DirectionDebit, nil, base.Add(2*time.Hour))
	require.NoError(t, err)

	// LIMIT 2 OFFSET 0 → newest two (e3, e2).
	page1, err := GetEntriesByAccountID(ctx, testDB, a.AccountID, 2, 0)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	assert.Equal(t, e3.EntryID, page1[0].EntryID)
	assert.Equal(t, e2.EntryID, page1[1].EntryID)

	// LIMIT 2 OFFSET 2 → oldest only (e1).
	page2, err := GetEntriesByAccountID(ctx, testDB, a.AccountID, 2, 2)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	assert.Equal(t, e1.EntryID, page2[0].EntryID)
}

func TestGetEntriesByAccountID_Empty(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()

	a := seedAccount(t, ctx, "empty-acct@example.com", models.AccountUserCash)
	entries, err := GetEntriesByAccountID(ctx, testDB, a.AccountID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestGetAccountBalance(t *testing.T) {
	// All cases share this base time. Per-entry offsets are expressed as
	// time.Duration relative to base, which lets the asOf cases reference
	// the same reference point.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	type entryInput struct {
		amount    int64
		direction models.EntryDirection
		offset    time.Duration // entry's effective_at = base + offset
	}

	tests := []struct {
		name    string
		entries []entryInput
		asOf    *time.Time
		want    int64
		wantMsg string
	}{
		{
			name:    "empty account returns zero",
			entries: nil,
			asOf:    nil,
			want:    0,
			wantMsg: "no entries → balance 0 via COALESCE",
		},
		{
			name: "net debit is SUM(DEBIT) minus SUM(CREDIT)",
			entries: []entryInput{
				{amount: 1000, direction: models.DirectionDebit, offset: 0},
				{amount: 300, direction: models.DirectionCredit, offset: 0},
			},
			asOf: nil,
			want: 700,
		},
		{
			name: "asOf cutoff excludes entries after the cutoff",
			entries: []entryInput{
				{amount: 500, direction: models.DirectionDebit, offset: 0},
				{amount: 200, direction: models.DirectionDebit, offset: time.Hour},
			},
			asOf:    ptr(base.Add(30 * time.Minute)),
			want:    500,
			wantMsg: "only entries with effective_at <= cutoff included",
		},
		{
			name: "asOf past all entries includes everything",
			entries: []entryInput{
				{amount: 500, direction: models.DirectionDebit, offset: 0},
				{amount: 200, direction: models.DirectionDebit, offset: time.Hour},
			},
			asOf: ptr(base.Add(2 * time.Hour)),
			want: 700,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			truncateAll(t)
			ctx := context.Background()

			a := seedAccount(t, ctx, "bal@example.com", models.AccountUserCash)

			if len(tc.entries) > 0 {
				txn := seedTransaction(t, ctx, "ck-"+tc.name)
				for _, e := range tc.entries {
					_, err := CreateEntry(ctx, testDB, txn.TransactionID, a.AccountID,
						e.amount, "USD", e.direction, nil, base.Add(e.offset))
					require.NoError(t, err)
				}
			}

			bal, err := GetAccountBalance(ctx, testDB, a.AccountID, tc.asOf)
			require.NoError(t, err)
			assert.Equal(t, tc.want, bal, tc.wantMsg)
		})
	}
}

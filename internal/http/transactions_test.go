package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ryankavi/payclone/internal/models"
	"github.com/ryankavi/payclone/internal/service"
)

func transactionBody(from, to string, amount int64, currency, idem string) map[string]any {
	return map[string]any{
		"from_account_id": from,
		"to_account_id":   to,
		"amount":          amount,
		"currency":        currency,
		"idempotency_key": idem,
	}
}

func TestTransfer_Happy(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	sender, token := seedUser(t, srv, "sender@example.com", models.RoleUser)
	from := seedAccount(t, sender, models.AccountUserCash, "USD")
	fund(t, from, "fund-transfer", 1000)
	receiver := createUser(t, "receiver@example.com", models.RoleUser)
	to := seedAccount(t, receiver, models.AccountUserCash, "USD")

	rec := doJSON(t, srv, "POST", "/api/transactions/transfer", token,
		transactionBody(from.AccountID, to.AccountID, 500, "USD", "idem-transfer"))

	require.Equal(t, http.StatusCreated, rec.Code)
}

// Transferring out of an account the caller doesn't own is forbidden.
func TestTransfer_NotOwnerForbidden(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	owner := createUser(t, "owner@example.com", models.RoleUser)
	from := seedAccount(t, owner, models.AccountUserCash, "USD")
	receiver := createUser(t, "receiver@example.com", models.RoleUser)
	to := seedAccount(t, receiver, models.AccountUserCash, "USD")
	_, attackerToken := seedUser(t, srv, "attacker@example.com", models.RoleUser)

	rec := doJSON(t, srv, "POST", "/api/transactions/transfer", attackerToken,
		transactionBody(from.AccountID, to.AccountID, 500, "USD", "idem-steal"))

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestWithdraw_Happy(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	owner, token := seedUser(t, srv, "owner@example.com", models.RoleUser)
	from := seedAccount(t, owner, models.AccountUserCash, "USD")
	fund(t, from, "fund-withdraw", 1000)
	extOwner := createUser(t, "ext@example.com", models.RoleUser)
	ext := seedAccount(t, extOwner, models.AccountExternal, "USD")

	rec := doJSON(t, srv, "POST", "/api/transactions/withdraw", token,
		transactionBody(from.AccountID, ext.AccountID, 500, "USD", "idem-withdraw"))

	require.Equal(t, http.StatusCreated, rec.Code)
}

// Withdrawing more than the (zero) balance is a 422 (ErrInsufficientFunds).
func TestWithdraw_InsufficientFunds(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	owner, token := seedUser(t, srv, "owner@example.com", models.RoleUser)
	from := seedAccount(t, owner, models.AccountUserCash, "USD") // unfunded
	extOwner := createUser(t, "ext@example.com", models.RoleUser)
	ext := seedAccount(t, extOwner, models.AccountExternal, "USD")

	rec := doJSON(t, srv, "POST", "/api/transactions/withdraw", token,
		transactionBody(from.AccountID, ext.AccountID, 500, "USD", "idem-withdraw-broke"))

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestDeposit_Admin(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	_, adminToken := seedUser(t, srv, "admin@example.com", models.RoleAdmin)
	user := createUser(t, "u@example.com", models.RoleUser)
	to := seedAccount(t, user, models.AccountUserCash, "USD")
	extOwner := createUser(t, "ext@example.com", models.RoleUser)
	ext := seedAccount(t, extOwner, models.AccountExternal, "USD")

	rec := doJSON(t, srv, "POST", "/api/transactions/deposit", adminToken,
		transactionBody(ext.AccountID, to.AccountID, 1000, "USD", "idem-deposit"))

	require.Equal(t, http.StatusCreated, rec.Code)
}

// A user-role token cannot call deposit — authz rejects before any DB work, so
// the account ids don't even need to exist.
func TestDeposit_NonAdminForbidden(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	_, userToken := seedUser(t, srv, "u@example.com", models.RoleUser)

	rec := doJSON(t, srv, "POST", "/api/transactions/deposit", userToken,
		transactionBody("from-id", "to-id", 1000, "USD", "idem-deposit-user"))

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAssessFee_Admin(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	_, adminToken := seedUser(t, srv, "admin@example.com", models.RoleAdmin)
	user := createUser(t, "u@example.com", models.RoleUser)
	from := seedAccount(t, user, models.AccountUserCash, "USD")
	fund(t, from, "fund-fee", 1000)
	feeOwner := createUser(t, "fee@example.com", models.RoleUser)
	fee := seedAccount(t, feeOwner, models.AccountFeeRevenue, "USD")

	rec := doJSON(t, srv, "POST", "/api/transactions/assess-fee", adminToken,
		transactionBody(from.AccountID, fee.AccountID, 100, "USD", "idem-fee"))

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestAssessFee_NonAdminForbidden(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	_, userToken := seedUser(t, srv, "u@example.com", models.RoleUser)

	rec := doJSON(t, srv, "POST", "/api/transactions/assess-fee", userToken,
		transactionBody("from-id", "to-id", 100, "USD", "idem-fee-user"))

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRefundFee_Admin(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	_, adminToken := seedUser(t, srv, "admin@example.com", models.RoleAdmin)
	feeOwner := createUser(t, "fee@example.com", models.RoleUser)
	fee := seedAccount(t, feeOwner, models.AccountFeeRevenue, "USD")
	user := createUser(t, "u@example.com", models.RoleUser)
	to := seedAccount(t, user, models.AccountUserCash, "USD")

	rec := doJSON(t, srv, "POST", "/api/transactions/refund-fee", adminToken,
		transactionBody(fee.AccountID, to.AccountID, 100, "USD", "idem-refund"))

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestRefundFee_NonAdminForbidden(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	_, userToken := seedUser(t, srv, "u@example.com", models.RoleUser)

	rec := doJSON(t, srv, "POST", "/api/transactions/refund-fee", userToken,
		transactionBody("from-id", "to-id", 100, "USD", "idem-refund-user"))

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestReverse_Admin(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	_, adminToken := seedUser(t, srv, "admin@example.com", models.RoleAdmin)
	user := createUser(t, "u@example.com", models.RoleUser)
	to := seedAccount(t, user, models.AccountUserCash, "USD")
	extOwner := createUser(t, "ext@example.com", models.RoleUser)
	ext := seedAccount(t, extOwner, models.AccountExternal, "USD")

	// A posted deposit to reverse.
	txn, err := service.Post(context.Background(), testDB, service.PostParams{
		FromAccountID:  ext.AccountID,
		ToAccountID:    to.AccountID,
		Amount:         1000,
		Currency:       "USD",
		IdempotencyKey: "dep-to-reverse",
	}, service.PostDeposit)
	require.NoError(t, err)

	rec := doJSON(t, srv, "POST", "/api/transactions/"+txn.TransactionID+"/reverse", adminToken,
		map[string]any{"idempotency_key": "rev-1"})

	require.Equal(t, http.StatusCreated, rec.Code)
}

// reverse checks admin before touching the body or the transaction id.
func TestReverse_NonAdminForbidden(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	_, userToken := seedUser(t, srv, "u@example.com", models.RoleUser)

	rec := doJSON(t, srv, "POST", "/api/transactions/some-id/reverse", userToken,
		map[string]any{"idempotency_key": "rev-user"})

	require.Equal(t, http.StatusForbidden, rec.Code)
}

package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ryankavi/payclone/internal/models"
)

func TestCreateAccount_Happy(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	_, token := seedUser(t, srv, "u@example.com", models.RoleUser)

	rec := doJSON(t, srv, "POST", "/api/accounts", token, map[string]any{
		"account_type": string(models.AccountUserCash),
		"currency":     "USD",
	})

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestCreateAccount_NoToken(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)

	rec := doJSON(t, srv, "POST", "/api/accounts", "", map[string]any{
		"account_type": string(models.AccountUserCash),
		"currency":     "USD",
	})

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// An empty account list must serialize as [] (not null) so clients can iterate.
func TestGetAccounts_EmptyIsArray(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	_, token := seedUser(t, srv, "u@example.com", models.RoleUser)

	rec := doJSON(t, srv, "GET", "/api/accounts", token, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"accounts":[]`)
}

func TestGetAccounts_OnlyOwn(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	owner, token := seedUser(t, srv, "owner@example.com", models.RoleUser)
	seedAccount(t, owner, models.AccountUserCash, "USD")
	// A second user's account must not appear in owner's list.
	other := createUser(t, "other@example.com", models.RoleUser)
	seedAccount(t, other, models.AccountUserCash, "USD")

	rec := doJSON(t, srv, "GET", "/api/accounts", token, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp getAccountsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Accounts, 1)
}

func TestGetAccountBalance_Owner(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	owner, token := seedUser(t, srv, "owner@example.com", models.RoleUser)
	acct := seedAccount(t, owner, models.AccountUserCash, "USD")

	rec := doJSON(t, srv, "GET", "/api/accounts/"+acct.AccountID+"/balance", token, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp accountBalanceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, int64(0), resp.Balance)
}

func TestGetAccountBalance_NonOwnerForbidden(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	owner, _ := seedUser(t, srv, "owner@example.com", models.RoleUser)
	acct := seedAccount(t, owner, models.AccountUserCash, "USD")
	_, otherToken := seedUser(t, srv, "other@example.com", models.RoleUser)

	rec := doJSON(t, srv, "GET", "/api/accounts/"+acct.AccountID+"/balance", otherToken, nil)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCloseAccount_Owner(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	owner, token := seedUser(t, srv, "owner@example.com", models.RoleUser)
	acct := seedAccount(t, owner, models.AccountUserCash, "USD") // zero balance

	rec := doJSON(t, srv, "POST", "/api/accounts/"+acct.AccountID+"/close", token, nil)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Body.String(), "204 carries no body")
}

func TestCloseAccount_NonOwnerForbidden(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	owner, _ := seedUser(t, srv, "owner@example.com", models.RoleUser)
	acct := seedAccount(t, owner, models.AccountUserCash, "USD")
	_, otherToken := seedUser(t, srv, "other@example.com", models.RoleUser)

	rec := doJSON(t, srv, "POST", "/api/accounts/"+acct.AccountID+"/close", otherToken, nil)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestFreezeAccount_Owner(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	owner, token := seedUser(t, srv, "owner@example.com", models.RoleUser)
	acct := seedAccount(t, owner, models.AccountUserCash, "USD")

	rec := doJSON(t, srv, "POST", "/api/accounts/"+acct.AccountID+"/freeze", token, nil)

	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestReopenAccount_Owner(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	owner, token := seedUser(t, srv, "owner@example.com", models.RoleUser)
	acct := seedAccount(t, owner, models.AccountUserCash, "USD")

	// Must be FROZEN before it can be reopened.
	require.Equal(t, http.StatusNoContent,
		doJSON(t, srv, "POST", "/api/accounts/"+acct.AccountID+"/freeze", token, nil).Code)

	rec := doJSON(t, srv, "POST", "/api/accounts/"+acct.AccountID+"/reopen", token, nil)

	require.Equal(t, http.StatusNoContent, rec.Code)
}

// Reopening an account that isn't frozen is a 422 (ErrAccountNotFrozen).
func TestReopenAccount_NotFrozen(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)
	owner, token := seedUser(t, srv, "owner@example.com", models.RoleUser)
	acct := seedAccount(t, owner, models.AccountUserCash, "USD") // OPEN, not frozen

	rec := doJSON(t, srv, "POST", "/api/accounts/"+acct.AccountID+"/reopen", token, nil)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

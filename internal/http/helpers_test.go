package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ryankavi/payclone/internal/auth"
	"github.com/ryankavi/payclone/internal/models"
	"github.com/ryankavi/payclone/internal/repository"
	"github.com/ryankavi/payclone/internal/service"
)

const testSecret = "test-signing-secret"

// newTestServer builds a Server backed by testDB with a fixed-secret
// Authenticator, so tests can mint their own valid tokens via srv.auth.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	a, err := auth.New(testSecret, time.Hour)
	require.NoError(t, err)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(testDB, logger, a)
}

// doJSON sends an optional JSON body to the server via httptest and returns the
// recorder. token, when non-empty, is sent as a Bearer credential.
func doJSON(t *testing.T, srv *Server, method, target, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, target, rdr)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// createUser inserts a user with the given role and returns its id. Used where
// only the id matters (e.g. an EXTERNAL account's owner). Role is set with a
// direct UPDATE because signup only ever creates RoleUser.
func createUser(t *testing.T, email string, role models.UserRole) string {
	t.Helper()
	ctx := context.Background()
	u, err := repository.CreateUser(ctx, testDB, email, "hashed-pw")
	require.NoError(t, err)
	if role != models.RoleUser {
		_, err = testDB.ExecContext(ctx, `UPDATE users SET role = $1 WHERE user_id = $2`, role, u.UserID)
		require.NoError(t, err)
	}
	return u.UserID
}

// seedUser creates a user and returns its id plus a signed token for it. The
// token's role matches the seeded role, so authz tests get a real credential.
func seedUser(t *testing.T, srv *Server, email string, role models.UserRole) (id, token string) {
	t.Helper()
	id = createUser(t, email, role)
	token, err := srv.auth.Sign(id, role)
	require.NoError(t, err)
	return id, token
}

// seedAccount creates an account of the given type owned by ownerID.
func seedAccount(t *testing.T, ownerID string, at models.AccountType, currency string) models.Account {
	t.Helper()
	a, err := repository.CreateAccount(context.Background(), testDB, ownerID, at, currency)
	require.NoError(t, err)
	return a
}

// fund deposits amount minor units into target from a fresh EXTERNAL account
// (the deposit path has no funds check), giving withdraw/transfer happy paths a
// funded source. idemKey must be unique within the test.
func fund(t *testing.T, target models.Account, idemKey string, amount int64) {
	t.Helper()
	extOwner := createUser(t, "ext-"+idemKey+"@example.com", models.RoleUser)
	ext := seedAccount(t, extOwner, models.AccountExternal, target.Currency)
	_, err := service.Post(context.Background(), testDB, service.PostParams{
		FromAccountID:  ext.AccountID,
		ToAccountID:    target.AccountID,
		Amount:         amount,
		Currency:       target.Currency,
		IdempotencyKey: idemKey,
	}, service.PostDeposit)
	require.NoError(t, err)
}

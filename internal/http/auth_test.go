package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ryankavi/payclone/internal/models"
)

func signupBody(email, password string) map[string]any {
	return map[string]any{
		"email":        email,
		"password":     password,
		"account_type": string(models.AccountUserCash),
		"currency":     "USD",
	}
}

func TestSignup_Happy(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)

	rec := doJSON(t, srv, "POST", "/signup", "", signupBody("new@example.com", "password123"))

	require.Equal(t, http.StatusCreated, rec.Code)
	var resp signupResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.AccountID)
	require.NotEmpty(t, resp.Token, "signup should return a token")
	require.Equal(t, "USD", resp.Currency)
}

func TestSignup_PasswordTooShort(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)

	rec := doJSON(t, srv, "POST", "/signup", "", signupBody("short@example.com", "short"))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestLogin_Happy(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)

	// Sign up so the user exists with a real bcrypt hash.
	require.Equal(t, http.StatusCreated,
		doJSON(t, srv, "POST", "/signup", "", signupBody("log@example.com", "password123")).Code)

	rec := doJSON(t, srv, "POST", "/login", "", map[string]any{
		"email":    "log@example.com",
		"password": "password123",
	})

	require.Equal(t, http.StatusOK, rec.Code)
	var resp loginResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Token)
}

func TestLogin_WrongPassword(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)

	require.Equal(t, http.StatusCreated,
		doJSON(t, srv, "POST", "/signup", "", signupBody("log@example.com", "password123")).Code)

	rec := doJSON(t, srv, "POST", "/login", "", map[string]any{
		"email":    "log@example.com",
		"password": "wrong-password",
	})

	// Same generic 401 as an unknown email — no enumeration.
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestLogin_UnknownEmail(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)

	rec := doJSON(t, srv, "POST", "/login", "", map[string]any{
		"email":    "nobody@example.com",
		"password": "password123",
	})

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// A protected route rejects a malformed/garbage bearer token with 401.
func TestProtected_InvalidToken(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t)

	rec := doJSON(t, srv, "GET", "/accounts", "not-a-real-jwt", nil)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

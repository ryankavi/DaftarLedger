package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryankavi/payclone/internal/models"
)

const testSecret = "test-signing-secret"

func TestNew_EmptySecret(t *testing.T) {
	_, err := New("", time.Hour)
	require.ErrorIs(t, err, ErrEmptySecret)
}

func TestSignParse_RoundTrip(t *testing.T) {
	a, err := New(testSecret, time.Hour)
	require.NoError(t, err)

	tok, err := a.Sign("user-123", models.RoleAdmin)
	require.NoError(t, err)

	claims, err := a.Parse(tok)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.Subject)
	assert.Equal(t, models.RoleAdmin, claims.Role)
	assert.True(t, claims.ExpiresAt.After(claims.IssuedAt.Time), "exp must be after iat")
}

func TestParse_WrongSecret(t *testing.T) {
	signer, err := New(testSecret, time.Hour)
	require.NoError(t, err)
	tok, err := signer.Sign("user-123", models.RoleUser)
	require.NoError(t, err)

	verifier, err := New("a-different-secret", time.Hour)
	require.NoError(t, err)

	_, err = verifier.Parse(tok)
	require.Error(t, err, "signature must not verify under a different secret")
}

func TestParse_Expired(t *testing.T) {
	a, err := New(testSecret, -time.Minute) // already expired
	require.NoError(t, err)
	tok, err := a.Sign("user-123", models.RoleUser)
	require.NoError(t, err)

	_, err = a.Parse(tok)
	require.ErrorIs(t, err, jwt.ErrTokenExpired)
}

// A token signed with "alg":"none" must be rejected — WithValidMethods pins
// HS256, which is the defense against the alg-confusion attack class.
func TestParse_RejectsNoneAlg(t *testing.T) {
	a, err := New(testSecret, time.Hour)
	require.NoError(t, err)

	claims := Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "attacker"}}
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	unsigned, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = a.Parse(unsigned)
	require.Error(t, err, "none-alg token must be rejected")
}

// Package auth signs and verifies HS256 access tokens. It is deliberately
// DB-free and stdlib-plus-golang-jwt only, so it unit-tests without a
// container and the HTTP layer can stay free of crypto details.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/ryankavi/payclone/internal/models"
)

// ErrEmptySecret is returned by New when the signing secret is blank, so
// startup can fail fast rather than silently signing with an empty key.
var ErrEmptySecret = errors.New("auth: empty signing secret")

// Authenticator signs and verifies HS256 tokens against a single symmetric
// secret. Stateless: a token is valid until its exp regardless of server
// state (no refresh, no revocation — see PLAN).
type Authenticator struct {
	secret []byte
	ttl    time.Duration
}

// New builds an Authenticator, which holds secrets and knows how to sign/verify
func New(secret string, ttl time.Duration) (*Authenticator, error) {
	if secret == "" {
		return nil, ErrEmptySecret
	}
	return &Authenticator{secret: []byte(secret), ttl: ttl}, nil
}

// Claims is the token payload: the user's role plus the registered claims
// (sub = user id, iat, exp).
type Claims struct {
	Role models.UserRole `json:"role"`
	jwt.RegisteredClaims
}

// Sign mints a signed token for the given user and role, expiring ttl from now.
func (a *Authenticator) Sign(userID string, role models.UserRole) (string, error) {
	now := time.Now()
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.ttl)),
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(a.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Parse verifies a token's signature and expiry and returns its claims.
func (a *Authenticator) Parse(tokenStr string) (Claims, error) {
	var claims Claims
	// WithValidMethods pins the algorithm to HS256 — this is the control that
	// defeats the alg-confusion / "alg":"none" class of attacks. DO NOT DROP.
	_, err := jwt.ParseWithClaims(
		tokenStr,
		&claims,
		func(*jwt.Token) (any, error) { return a.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("parse token: %w", err)
	}
	return claims, nil
}

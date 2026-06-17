package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	errx "github.com/ryankavi/payclone/internal/errors"
	"github.com/ryankavi/payclone/internal/models"
	"github.com/ryankavi/payclone/internal/repository"
)

// Login verifies an email + password pair. On success it returns the user
// (including role) so the caller can mint a token. Both "no such email" and
// "wrong password" collapse to ErrInvalidCredentials so the API can't be used
// to enumerate which emails are registered. The token itself is minted by the
// HTTP layer, not here — this layer stays DB-only and crypto-token-free.
func Login(ctx context.Context, database *sql.DB, email, password string) (models.User, error) {
	user, err := repository.GetUserByEmail(ctx, database, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, errx.ErrInvalidCredentials
		}
		return models.User{}, fmt.Errorf("login lookup: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return models.User{}, errx.ErrInvalidCredentials
	}

	return user, nil
}

// EnsureAdmin idempotently ensures an admin account with the given credentials
// exists. Called at startup from main when BOOTSTRAP_ADMIN_* are set — the
// system has no other way to mint an admin (signup only creates RoleUser). The
// password is bcrypt-hashed here, the same as signup, so the admin can log in
// through the normal /login flow.
func EnsureAdmin(ctx context.Context, database *sql.DB, email, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	return repository.UpsertAdmin(ctx, database, email, string(hash))
}

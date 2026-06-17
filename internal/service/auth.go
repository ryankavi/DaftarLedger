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

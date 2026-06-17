package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ryankavi/payclone/internal/models"
)

func CreateUser(ctx context.Context, db DBTX, email string, passwordHash string) (models.User, error) {
	const q = `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING user_id, email, password_hash, role, created_at
	`

	var u models.User
	err := db.QueryRowContext(ctx, q, email, passwordHash).Scan(&u.UserID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return models.User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func GetUser(ctx context.Context, db DBTX, userID string) (models.User, error) {
	const q = `
		SELECT user_id, email, password_hash, role, created_at
		FROM users
		WHERE user_id = $1
	`

	var u models.User
	err := db.QueryRowContext(ctx, q, userID).Scan(&u.UserID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return models.User{}, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

// GetUserByEmail looks up a user by email for the login flow. Returns the
// password hash and role so the service can verify credentials and issue a
// token. sql.ErrNoRows on no match — callers must map it to the same generic
// 401 as a bad password to avoid leaking which emails exist.
func GetUserByEmail(ctx context.Context, db DBTX, email string) (models.User, error) {
	const q = `
		SELECT user_id, email, password_hash, role, created_at
		FROM users
		WHERE email = $1
	`

	var u models.User
	err := db.QueryRowContext(ctx, q, email).Scan(&u.UserID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return models.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// UpsertAdmin ensures a user with the given email exists as an ADMIN with the
// given password hash. Idempotent: on a re-run (or if the email already exists)
// it promotes the row to ADMIN and resets the hash, so the email is treated as
// operator-controlled — the bootstrap env vars are authoritative for it.
func UpsertAdmin(ctx context.Context, db DBTX, email, passwordHash string) error {
	const q = `
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, $2, 'ADMIN')
		ON CONFLICT (email)
		DO UPDATE SET password_hash = EXCLUDED.password_hash, role = 'ADMIN'
	`

	if _, err := db.ExecContext(ctx, q, email, passwordHash); err != nil {
		return fmt.Errorf("upsert admin: %w", err)
	}
	return nil
}

func UpdateUserEmail(ctx context.Context, db DBTX, userID string, newEmail string) (models.User, error) {
	const q = `
		UPDATE users
		SET email = $2
		WHERE user_id = $1
		RETURNING user_id, email, password_hash, role, created_at
	`

	var u models.User
	err := db.QueryRowContext(ctx, q, userID, newEmail).Scan(&u.UserID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return models.User{}, fmt.Errorf("update user email: %w", err)
	}
	return u, nil
}

func DeleteUser(ctx context.Context, db DBTX, userID string) error {
	const q = `
		DELETE FROM users
		WHERE user_id = $1
	`

	res, err := db.ExecContext(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete user rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

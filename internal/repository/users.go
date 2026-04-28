package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ryankavi/payclone/internal/models"
)

func CreateUser(ctx context.Context, db DBTX, email string) (models.User, error) {
	const q = `
		INSERT INTO users (email)
		VALUES ($1)
		RETURNING user_id, email, created_at
	`

	var u models.User
	err := db.QueryRowContext(ctx, q, email).Scan(&u.UserID, &u.Email, &u.CreatedAt)
	if err != nil {
		return models.User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func GetUser(ctx context.Context, db DBTX, userID string) (models.User, error) {
	const q = `
		SELECT user_id, email, created_at
		FROM users
		WHERE user_id = $1
	`

	var u models.User
	err := db.QueryRowContext(ctx, q, userID).Scan(&u.UserID, &u.Email, &u.CreatedAt)
	if err != nil {
		return models.User{}, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func UpdateUserEmail(ctx context.Context, db DBTX, userID string, newEmail string) (models.User, error) {
	const q = `
		UPDATE users
		SET email = $2
		WHERE user_id = $1
		RETURNING user_id, email, created_at
	`

	var u models.User
	err := db.QueryRowContext(ctx, q, userID, newEmail).Scan(&u.UserID, &u.Email, &u.CreatedAt)
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

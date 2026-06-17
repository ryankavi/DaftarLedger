package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/lib/pq"
	errx "github.com/ryankavi/payclone/internal/errors"
	"github.com/ryankavi/payclone/internal/models"
	"github.com/ryankavi/payclone/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// Signup, no auth required. password is plaintext; it is bcrypt-hashed here
// before the row is written — the plaintext never leaves this function.
func CreateUserWithAccount(ctx context.Context, database *sql.DB, email string, password string, accountType string, currency string) (models.Account, error) {

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return models.Account{}, fmt.Errorf("create account begin tx: %w", err)
	}
	defer tx.Rollback()

	var ownerID string
	if !validateEmail(email) {
		return models.Account{}, errx.ErrInvalidEmail
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.Account{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := repository.CreateUser(ctx, tx, email, string(hash))
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return models.Account{}, errx.ErrEmailTaken
		}
		return models.Account{}, err
	}
	ownerID = user.UserID

	account, err := repository.CreateAccount(ctx, tx, ownerID, models.AccountType(accountType), currency)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return models.Account{}, errx.ErrAccountTypeExists
		}
		return models.Account{}, err
	}

	if err := tx.Commit(); err != nil {
		return models.Account{}, fmt.Errorf("create account commit: %w", err)
	}

	return account, nil
}

func validateEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" || len(email) > 254 {
		return false
	}
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	// ParseAddress accepts "Name <addr@x>"; require bare addr only.
	return addr.Address == email
}

// Authenticated users only, ownerID comes from auth context
func CreateAccount(ctx context.Context, database *sql.DB, ownerID string, accountType string, currency string) (models.Account, error) {

	account, err := repository.CreateAccount(ctx, database, ownerID, models.AccountType(accountType), currency)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return models.Account{}, errx.ErrAccountTypeExists
		}
		return models.Account{}, err
	}

	return account, nil
}

func GetAccounts(ctx context.Context, database *sql.DB, ownerID string) ([]models.Account, error) {
	accounts, err := repository.GetAccountsFromOwnerID(ctx, database, ownerID)
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

func GetAccountBalance(ctx context.Context, db *sql.DB, accountID string, asOf *time.Time) (int64, error) {
	account, err := repository.GetAccount(ctx, db, accountID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, errx.ErrAccountNotFound
		}
		return 0, err
	}
	raw, err := repository.GetAccountBalance(ctx, db, accountID, asOf)
	if err != nil {
		return 0, err
	}
	return availableBalance(account.AccountType, raw), nil
}

func CloseAccount(ctx context.Context, database *sql.DB, accountID string) error {

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("close account begin tx: %w", err)
	}
	defer tx.Rollback()

	err = repository.LockAccount(ctx, tx, accountID)
	if err != nil {
		return fmt.Errorf("lock account for close: %w", err)
	}

	account, err := repository.GetAccount(ctx, tx, accountID)
	if err != nil {
		return fmt.Errorf("get account to close: %w", err)
	}

	if account.AccountStatus == models.AccountStatusClosed {
		return fmt.Errorf("cannot close already closed account: %w", errx.ErrAccountClosed)
	}
	balance, err := repository.GetAccountBalance(ctx, tx, accountID, nil)
	if err != nil {
		return fmt.Errorf("get account balance before close: %w", err)
	}
	if balance != 0 {
		return errx.ErrBalanceNotZero
	}

	// update account to close if it's frozen/open
	_, err = repository.UpdateAccountStatus(ctx, tx, accountID, models.AccountStatusClosed)
	if err != nil {
		return fmt.Errorf("close account: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("close account commit: %w", err)
	}

	return nil
}

func FreezeAccount(ctx context.Context, database *sql.DB, accountID string) error {

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("freeze account begin tx: %w", err)
	}
	defer tx.Rollback()

	err = repository.LockAccount(ctx, tx, accountID)
	if err != nil {
		return fmt.Errorf("lock account for freeze: %w", err)
	}

	account, err := repository.GetAccount(ctx, tx, accountID)
	if err != nil {
		return fmt.Errorf("get account to freeze: %w", err)
	}
	if account.AccountStatus != models.AccountStatusOpen {
		return fmt.Errorf("cannot freeze non-open account: %w", errx.ErrAccountNotOpen)
	}

	// update account to freeze if it's open
	_, err = repository.UpdateAccountStatus(ctx, tx, accountID, models.AccountStatusFrozen)
	if err != nil {
		return fmt.Errorf("cannot freeze account: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("freeze account commit: %w", err)
	}

	return nil
}

func ReopenAccount(ctx context.Context, database *sql.DB, accountID string) error {

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("reopen account begin tx: %w", err)
	}
	defer tx.Rollback()

	err = repository.LockAccount(ctx, tx, accountID)
	if err != nil {
		return fmt.Errorf("lock account for reopen: %w", err)
	}

	account, err := repository.GetAccount(ctx, tx, accountID)
	if err != nil {
		return fmt.Errorf("get account to reopen: %w", err)
	}
	if account.AccountStatus != models.AccountStatusFrozen {
		return fmt.Errorf("can only reopen frozen account: %w", errx.ErrAccountNotFrozen)
	}

	// update account to reopen if it's frozen
	_, err = repository.UpdateAccountStatus(ctx, tx, accountID, models.AccountStatusOpen)
	if err != nil {
		return fmt.Errorf("reopen account: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("reopen account commit: %w", err)
	}

	return nil
}

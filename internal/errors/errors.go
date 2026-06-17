package errors

import "errors"

var (
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrCurrencyMismatch    = errors.New("currency mismatch")
	ErrAccountNotFound     = errors.New("account not found")
	ErrSameAccount         = errors.New("from and to accounts must differ")
	ErrAccountClosed       = errors.New("account is closed")
	ErrAccountNotOpen      = errors.New("account not open")
	ErrAccountNotFrozen    = errors.New("account not frozen")
	ErrBalanceNotZero      = errors.New("account balance not zero")
	ErrInvalidAmount       = errors.New("amount must be positive")
	ErrInvalidAccountType  = errors.New("invalid account type for this operation")
	ErrIdempotencyConflict = errors.New("idempotency_key reused for a different transaction type")
	ErrNotReversible       = errors.New("transaction cannot be reversed")
	ErrAlreadyReversed     = errors.New("transaction already has a reversal")
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrInvalidEmail        = errors.New("invalid email")
	ErrEmailTaken          = errors.New("email already registered")
	ErrAccountTypeExists   = errors.New("user already has an account of this type")
	ErrForbidden           = errors.New("user operation forbidden")
	// ErrInvalidCredentials is the single error returned for both an unknown
	// email and a wrong password, so the login API can't be used to enumerate
	// which emails are registered. Its message is safe to show the client.
	ErrInvalidCredentials = errors.New("invalid email or password")
)

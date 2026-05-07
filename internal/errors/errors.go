package errors

import "errors"

var (
	ErrInsufficientFunds  = errors.New("insufficient funds")
	ErrCurrencyMismatch   = errors.New("currency mismatch")
	ErrAccountNotFound    = errors.New("account not found")
	ErrSameAccount        = errors.New("from and to accounts must differ")
	ErrInvalidAmount      = errors.New("amount must be positive")
	ErrInvalidAccountType = errors.New("invalid account type for this operation")
	ErrIdempotencyConflict = errors.New("idempotency_key reused for a different transaction type")
)

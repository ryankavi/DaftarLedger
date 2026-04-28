package errors

import "errors"

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrCurrencyMismatch  = errors.New("currency mismatch")
	ErrAccountNotFound   = errors.New("account not found")
	ErrSameAccount       = errors.New("from and to accounts must differ")
	ErrInvalidAmount     = errors.New("amount must be positive")
)

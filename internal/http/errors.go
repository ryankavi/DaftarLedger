package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	errx "github.com/ryankavi/payclone/internal/errors"
)

// toHTTPStatus maps a domain error to its HTTP status code. Anything
// unrecognized — including infra failures — maps to 500. The default is the
// safety net: a new domain error that nobody mapped falls through to 500
// rather than silently producing a 200.
func toHTTPStatus(err error) int {
	switch {
	case errors.Is(err, errx.ErrInvalidAmount),
		errors.Is(err, errx.ErrSameAccount),
		errors.Is(err, errx.ErrInvalidAccountType),
		errors.Is(err, errx.ErrCurrencyMismatch),
		errors.Is(err, errx.ErrInvalidEmail):
		return http.StatusBadRequest // 400
	case errors.Is(err, errx.ErrForbidden):
		return http.StatusForbidden // 403
	case errors.Is(err, errx.ErrAccountNotFound),
		errors.Is(err, errx.ErrTransactionNotFound):
		return http.StatusNotFound // 404
	case errors.Is(err, errx.ErrIdempotencyConflict),
		errors.Is(err, errx.ErrAlreadyReversed),
		errors.Is(err, errx.ErrEmailTaken),
		errors.Is(err, errx.ErrAccountTypeExists),
		errors.Is(err, errx.ErrBalanceNotZero):
		return http.StatusConflict // 409
	case errors.Is(err, errx.ErrInsufficientFunds),
		errors.Is(err, errx.ErrNotReversible),
		errors.Is(err, errx.ErrAccountClosed),
		errors.Is(err, errx.ErrAccountNotOpen),
		errors.Is(err, errx.ErrAccountNotFrozen):
		return http.StatusUnprocessableEntity // 422
	default:
		return http.StatusInternalServerError // 500
	}
}

// writeError resolves err to a status, logs it with that status, and writes
// the response. Domain errors echo their message to the client; 5xx errors
// get a generic message so driver/SQL internals never leak to callers.
func writeError(w http.ResponseWriter, logger *slog.Logger, logMsg string, err error) {
	status := toHTTPStatus(err)
	logger.Error(logMsg, "err", err, "status", status)
	clientMsg := err.Error()
	if status >= http.StatusInternalServerError {
		clientMsg = "internal error"
	}
	http.Error(w, clientMsg, status)
}

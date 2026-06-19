package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	errx "github.com/ryankavi/payclone/internal/errors"
	"github.com/ryankavi/payclone/internal/models"
	"github.com/ryankavi/payclone/internal/repository"
	"github.com/ryankavi/payclone/internal/service"
)

type signupRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	AccountType string `json:"account_type"`
	Currency    string `json:"currency"`
}

// minPasswordLen is the edge-validated floor on signup passwords. bcrypt
// silently truncates beyond 72 bytes; we don't cap here, that's a separate
// concern.
const minPasswordLen = 8

type signupResponse struct {
	AccountID   string `json:"account_id"`
	AccountType string `json:"account_type"`
	Currency    string `json:"currency"`
	Token       string `json:"token"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

func (s *Server) signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Password) < minPasswordLen {
		http.Error(w, "password too short", http.StatusBadRequest)
		return
	}

	account, err := service.CreateUserWithAccount(r.Context(), s.db, req.Email, req.Password, req.AccountType, req.Currency)
	if err != nil {
		writeError(w, s.logger, "signup failed", err)
		return
	}

	// A fresh signup is always RoleUser (the migration's default; signup never
	// creates admins). account.OwnerID is the new user's id. Issue a token so
	// the client is authenticated immediately without a second /login round-trip.
	token, err := s.auth.Sign(account.OwnerID, models.RoleUser)
	if err != nil {
		writeError(w, s.logger, "signup token", err)
		return
	}

	resp := signupResponse{
		AccountID:   account.AccountID,
		AccountType: string(account.AccountType),
		Currency:    account.Currency,
		Token:       token,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("encode signup response", "err", err)
	}
}

// login authenticates an existing user and returns a bearer token. Public
// route (registered outside authMiddleware) — you can't have a token yet.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Login returns the same ErrInvalidCredentials for unknown-email and
	// wrong-password → toHTTPStatus maps it to a generic 401.
	user, err := service.Login(r.Context(), s.db, req.Email, req.Password)
	if err != nil {
		writeError(w, s.logger, "login failed", err)
		return
	}

	token, err := s.auth.Sign(user.UserID, user.Role)
	if err != nil {
		writeError(w, s.logger, "login token", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(loginResponse{Token: token}); err != nil {
		s.logger.Error("encode login response", "err", err)
	}
}

type createAccountRequest struct {
	AccountType string `json:"account_type"`
	Currency    string `json:"currency"`
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userId, ok := UserIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Role gates which account types the caller may create — enforced in the
	// service (USER ↔ USER_CASH, ADMIN ↔ platform accounts).
	role, _ := RoleFromCtx(r.Context())

	account, err := service.CreateAccount(r.Context(), s.db, userId, role, req.AccountType, req.Currency)
	if err != nil {
		writeError(w, s.logger, "create account failed", err)
		return
	}

	resp := signupResponse{
		AccountID:   account.AccountID,
		AccountType: string(account.AccountType),
		Currency:    account.Currency,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("encode create account response", "err", err)
	}
}

type accountResponse struct {
	AccountID   string `json:"account_id"`
	AccountType string `json:"account_type"`
	Status      string `json:"account_status"`
	Currency    string `json:"currency"`
}

type getAccountsResponse struct {
	Accounts []accountResponse `json:"accounts"`
}

func (s *Server) getAccounts(w http.ResponseWriter, r *http.Request) {
	userId, ok := UserIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	accounts, err := service.GetAccounts(r.Context(), s.db, userId)
	if err != nil {
		writeError(w, s.logger, "get accounts failed", err)
		return
	}

	output := make([]accountResponse, 0, len(accounts))
	for _, a := range accounts {
		output = append(output, accountResponse{
			AccountID:   a.AccountID,
			AccountType: string(a.AccountType),
			Status:      string(a.AccountStatus),
			Currency:    a.Currency,
		})
	}

	resp := getAccountsResponse{
		Accounts: output,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("encode get accounts response", "err", err)
	}
}

type accountBalanceResponse struct {
	Balance int64 `json:"balance"`
}

func (s *Server) getAccountBalance(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("id")

	// as_of is optional; absent means "current balance". When present it
	// must be RFC3339 — reject anything else rather than silently ignore.
	var asOf *time.Time
	if raw := r.URL.Query().Get("as_of"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			http.Error(w, "as_of must be RFC3339", http.StatusBadRequest)
			return
		}
		asOf = &t
	}

	if err := s.assertOwns(r.Context(), accountID); err != nil {
		writeError(w, s.logger, "get account balance: ownership check failed", err)
		return
	}

	balance, err := service.GetAccountBalance(r.Context(), s.db, accountID, asOf)
	if err != nil {
		writeError(w, s.logger, "get account balance failed", err)
		return
	}

	resp := accountBalanceResponse{
		Balance: balance,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("encode get account balance response", "err", err)
	}
}

func (s *Server) closeAccount(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("id")

	if err := s.assertOwns(r.Context(), accountID); err != nil {
		writeError(w, s.logger, "close account: ownership check failed", err)
		return
	}

	if err := service.CloseAccount(r.Context(), s.db, accountID); err != nil {
		writeError(w, s.logger, "close account failed", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TODO: if frozen by admin, must reopen with admin. add column freeze_by in db
func (s *Server) freezeAccount(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("id")

	if err := s.assertOwns(r.Context(), accountID); err != nil {
		writeError(w, s.logger, "freeze account: ownership check failed", err)
		return
	}

	if err := service.FreezeAccount(r.Context(), s.db, accountID); err != nil {
		writeError(w, s.logger, "freeze account failed", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) reopenAccount(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("id")

	if err := s.assertOwns(r.Context(), accountID); err != nil {
		writeError(w, s.logger, "reopen account: ownership check failed", err)
		return
	}

	if err := service.ReopenAccount(r.Context(), s.db, accountID); err != nil {
		writeError(w, s.logger, "reopen account failed", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Service only checks that both accounts share the request currency. This regex
// rejects malformed currency codes here before touching the DB.
var currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)

type transactionRequest struct {
	FromAccountID  string  `json:"from_account_id"`
	ToAccountID    string  `json:"to_account_id"`
	ExternalID     *string `json:"external_id"`
	Amount         int64   `json:"amount"`
	Currency       string  `json:"currency"`
	IdempotencyKey string  `json:"idempotency_key"`
	Memo           *string `json:"memo"`
	Description    *string `json:"description"`
}

type transactionResponse struct {
	TransactionID     string     `json:"transaction_id"`
	ExternalID        *string    `json:"external_id,omitempty"`
	IdempotencyKey    string     `json:"idempotency_key"`
	TransactionType   string     `json:"transaction_type"`
	Description       *string    `json:"description,omitempty"`
	TransactionStatus string     `json:"transaction_status"`
	PostedAt          *time.Time `json:"posted_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// postTransaction is the shared body for the five forward transaction flows.
// They differ only in PostType and authz, so both are injected: postType is
// hardcoded per route, authz runs against the decoded request (ownership for
// user flows, role for platform flows). Decode → currency format → authz →
// service.Post → 201.
func (s *Server) postTransaction(w http.ResponseWriter, r *http.Request, postType service.PostType, authz func(ctx context.Context, req transactionRequest) error) {
	var req transactionRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !currencyRe.MatchString(req.Currency) {
		http.Error(w, "currency must match ^[A-Z]{3}$", http.StatusBadRequest)
		return
	}

	if err := authz(r.Context(), req); err != nil {
		writeError(w, s.logger, string(postType)+" authz failed", err)
		return
	}

	txn, err := service.Post(r.Context(), s.db, service.PostParams{
		FromAccountID:  req.FromAccountID,
		ToAccountID:    req.ToAccountID,
		ExternalID:     req.ExternalID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		IdempotencyKey: req.IdempotencyKey,
		Memo:           req.Memo,
		Description:    req.Description,
	}, postType)
	if err != nil {
		writeError(w, s.logger, string(postType)+" failed", err)
		return
	}

	writeTransaction(w, s.logger, txn)
}

// writeTransaction writes the posted transaction and returns 201 Created
func writeTransaction(w http.ResponseWriter, logger *slog.Logger, txn models.Transaction) {
	resp := transactionResponse{
		TransactionID:     txn.TransactionID,
		ExternalID:        txn.ExternalID,
		IdempotencyKey:    txn.IdempotencyKey,
		TransactionType:   txn.TransactionType,
		Description:       txn.TransactionDescription,
		TransactionStatus: string(txn.TransactionStatus),
		PostedAt:          txn.PostedAt,
		CreatedAt:         txn.CreatedAt,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Error("encode transaction response", "err", err)
	}
}

// ownsFrom authorizes a user flow: the caller must own the source account.
func (s *Server) ownsFrom(ctx context.Context, req transactionRequest) error {
	return s.assertOwns(ctx, req.FromAccountID)
}

// adminOnly authorizes a platform flow: caller must be admin/system.
func (s *Server) adminOnly(ctx context.Context, _ transactionRequest) error {
	if !s.isAdmin(ctx) {
		return errx.ErrForbidden
	}
	return nil
}

func (s *Server) cashTransfer(w http.ResponseWriter, r *http.Request) {
	s.postTransaction(w, r, service.PostCashTransfer, s.ownsFrom)
}

func (s *Server) deposit(w http.ResponseWriter, r *http.Request) {
	s.postTransaction(w, r, service.PostDeposit, s.adminOnly)
}

func (s *Server) withdraw(w http.ResponseWriter, r *http.Request) {
	s.postTransaction(w, r, service.PostWithdraw, s.ownsFrom)
}

func (s *Server) feeAssess(w http.ResponseWriter, r *http.Request) {
	s.postTransaction(w, r, service.PostAssessFee, s.adminOnly)
}

func (s *Server) feeRefund(w http.ResponseWriter, r *http.Request) {
	s.postTransaction(w, r, service.PostRefundFee, s.adminOnly)
}

type reverseRequest struct {
	IdempotencyKey string  `json:"idempotency_key"`
	Memo           *string `json:"memo"`
}

// reverse is admin-only (user self-reversal is an abuse risk). The service
// derives the inverse PostType internally — callers never name it.
func (s *Server) reverse(w http.ResponseWriter, r *http.Request) {
	transactionID := r.PathValue("id")

	if !s.isAdmin(r.Context()) {
		writeError(w, s.logger, "reverse authz failed", errx.ErrForbidden)
		return
	}

	var req reverseRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	txn, err := service.Reverse(r.Context(), s.db, service.ReverseParams{
		TransactionID:  transactionID,
		IdempotencyKey: req.IdempotencyKey,
		Memo:           req.Memo,
	})
	if err != nil {
		writeError(w, s.logger, "reverse failed", err)
		return
	}

	writeTransaction(w, s.logger, txn)
}

func (s *Server) assertOwns(ctx context.Context, accountID string) error {
	userId, ok := UserIDFromCtx(ctx)
	if !ok {
		// no user id in context
		return errx.ErrForbidden
	}
	account, err := repository.GetAccount(ctx, s.db, accountID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errx.ErrAccountNotFound
		}
		return err
	}
	if account.OwnerID != userId {
		return errx.ErrForbidden
	}
	return nil
}

// isAdmin reports whether the caller holds the admin role. The role comes from
// the verified token (placed in ctx by authMiddleware).
func (s *Server) isAdmin(ctx context.Context) bool {
	role, ok := RoleFromCtx(ctx)
	if !ok {
		return false
	}
	return role == models.RoleAdmin
}

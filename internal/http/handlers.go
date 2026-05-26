package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	errx "github.com/ryankavi/payclone/internal/errors"
	"github.com/ryankavi/payclone/internal/service"
)

type signupRequest struct {
	Email       string `json:"email"`
	AccountType string `json:"account_type"`
	Currency    string `json:"currency"`
}

type signupResponse struct {
	AccountID   string `json:"account_id"`
	AccountType string `json:"account_type"`
	Currency    string `json:"currency"`
}

func (s *Server) signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	account, err := service.CreateUserWithAccount(r.Context(), s.db, req.Email, req.AccountType, req.Currency)
	if err != nil {
		s.logger.Error("signup failed", "err", err)
		switch {
		case errors.Is(err, errx.ErrInvalidEmail),
			errors.Is(err, errx.ErrInvalidAccountType):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, errx.ErrEmailTaken),
			errors.Is(err, errx.ErrAccountTypeExists):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
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
		s.logger.Error("encode signup response", "err", err)
	}
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) getAccount(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) getAccountBalance(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) closeAccount(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) freezeAccount(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) reopenAccount(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) cashTransfer(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) deposit(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) withdraw(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) feeAssess(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) feeRefund(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) reverse(w http.ResponseWriter, r *http.Request) {
}

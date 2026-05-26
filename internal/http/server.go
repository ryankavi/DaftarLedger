package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Server struct {
	db     *sql.DB
	logger *slog.Logger
	mux    *http.ServeMux
}

func NewServer(db *sql.DB, logger *slog.Logger) *Server {
	s := &Server{
		db:     db,
		logger: logger,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s
}

// routes wires every handler onto s.mux. Add registrations here as handlers land.
func (s *Server) routes() {
	s.mux.HandleFunc("POST /signup", s.signup)
	s.mux.HandleFunc("POST /accounts", s.createAccount)
	s.mux.HandleFunc("GET /accounts/{id}", s.getAccount)
	s.mux.HandleFunc("GET /accounts/{id}/balance", s.getAccountBalance)
	s.mux.HandleFunc("POST /accounts/{id}/close", s.closeAccount)
	s.mux.HandleFunc("POST /accounts/{id}/freeze", s.freezeAccount)
	s.mux.HandleFunc("POST /accounts/{id}/reopen", s.reopenAccount)

	s.mux.HandleFunc("POST /transactions/transfer", s.cashTransfer)
	s.mux.HandleFunc("POST /transactions/deposit", s.deposit)
	s.mux.HandleFunc("POST /transactions/withdraw", s.withdraw)
	s.mux.HandleFunc("POST /transactions/assess-fee", s.feeAssess)
	s.mux.HandleFunc("POST /transactions/refund-fee", s.feeRefund)
	s.mux.HandleFunc("POST /transactions/{id}/reverse", s.reverse)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Run starts the HTTP server and blocks until ctx is cancelled, then performs
// a graceful shutdown bounded by shutdownTimeout.
func (s *Server) Run(ctx context.Context, addr string, shutdownTimeout time.Duration) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: s,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		s.logger.Info("http server shutting down")
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

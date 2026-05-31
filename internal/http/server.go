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

	protected := http.NewServeMux()
	s.mux.Handle("/", s.authMiddleware(protected))

	protected.HandleFunc("POST /accounts", s.createAccount)
	protected.HandleFunc("GET /accounts", s.getAccounts)
	protected.HandleFunc("GET /accounts/{id}/balance", s.getAccountBalance)
	protected.HandleFunc("POST /accounts/{id}/close", s.closeAccount)
	protected.HandleFunc("POST /accounts/{id}/freeze", s.freezeAccount)
	protected.HandleFunc("POST /accounts/{id}/reopen", s.reopenAccount)

	protected.HandleFunc("POST /transactions/transfer", s.cashTransfer)
	protected.HandleFunc("POST /transactions/deposit", s.deposit)
	protected.HandleFunc("POST /transactions/withdraw", s.withdraw)
	protected.HandleFunc("POST /transactions/assess-fee", s.feeAssess)
	protected.HandleFunc("POST /transactions/refund-fee", s.feeRefund)
	protected.HandleFunc("POST /transactions/{id}/reverse", s.reverse)

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

package httpapi

import (
	"net/http"
	"strings"
)

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Stub: accept "Bearer <uuid>" or raw X-User-ID.
		var id string
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			id = strings.TrimPrefix(h, "Bearer ")
		} else {
			id = r.Header.Get("X-User-ID")
		}
		if id == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := WithUserID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

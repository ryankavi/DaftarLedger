package httpapi

import (
	"net/http"
	"strings"
)

// authMiddleware authenticates (identity only, not permission). It requires a
// valid HS256 bearer token: signature + expiry are verified by Authenticator.
// Parse, and the verified sub + role are placed in the request context for
// handlers and the authz layer to read. Any missing/malformed/invalid/expired
// token is a 401. There is deliberately no header fallback — a token is the
// only way to establish identity.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || tokenStr == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		claims, err := s.auth.Parse(tokenStr)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := WithUserID(r.Context(), claims.Subject)
		ctx = WithRole(ctx, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

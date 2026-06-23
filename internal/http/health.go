package httpapi

import "net/http"

// health is an unauthenticated liveness probe: it writes 200 with no body so a
// load balancer, monitor, or end-to-end check (through CloudFront at
// /api/health) can confirm the process is up and serving. It touches no
// dependencies — a 200 here means "the HTTP server is alive", not "the DB is
// reachable"; add a separate readiness check if you need the latter.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

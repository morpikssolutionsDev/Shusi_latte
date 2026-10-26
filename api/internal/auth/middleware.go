package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/actor"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crew"
)

type contextKey string

const crewContextKey contextKey = "crew"

// CrewFromContext returns the authenticated crew record, if any.
func CrewFromContext(r *http.Request) (crew.Crew, bool) {
	c, ok := r.Context().Value(crewContextKey).(crew.Crew)
	return c, ok
}

func writeProblem(w http.ResponseWriter, status int, code, title string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"type":"about:blank","title":"` + title + `","status":` + itoa(status) + `,"code":"` + code + `"}`))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// allowedRoles returns the roles allowed for a staff path and method,
// following the BRD §5.1 permission matrix, or nil if the path is
// not a staff endpoint. An empty slice means any authenticated role.
func allowedRoles(path, method string) ([]string, bool) {
	// Public endpoints.
	if path == "/healthz" || path == "/api/v1/health" ||
		strings.HasPrefix(path, "/public/") ||
		path == "/auth/login" || path == "/auth/override" || path == "/auth/logout" || path == "/auth/screen-login" {
		return nil, false
	}
	if !strings.HasPrefix(path, "/api/v1/") && path != "/auth/me" {
		return nil, false
	}
	// Sold Out toggle: Management and Outlet Admin (BR-ADM-07).
	if strings.HasPrefix(path, "/api/v1/variants/") && strings.HasSuffix(path, "/sold-out") {
		return []string{crew.RoleManagement, crew.RoleOutletAdmin}, true
	}
	// Tables and QR codes: Management and Outlet Admin (BR-ADM-08).
	if strings.HasPrefix(path, "/api/v1/tables") {
		return []string{crew.RoleManagement, crew.RoleOutletAdmin}, true
	}
	// Management-only areas (BRD §5.1).
	if strings.HasPrefix(path, "/api/v1/crew") ||
		strings.HasPrefix(path, "/api/v1/display/slides") ||
		strings.HasPrefix(path, "/api/v1/reports/") ||
		strings.HasPrefix(path, "/api/v1/settings/") ||
		strings.HasPrefix(path, "/api/v1/discounts") ||
		path == "/api/v1/audit-log" {
		return []string{crew.RoleManagement}, true
	}
	// Catalog and pricing calculation: reads for any staff role,
	// mutations for Management only (BR-ADM-01 to BR-ADM-06).
	if strings.HasPrefix(path, "/api/v1/catalog") ||
		strings.HasPrefix(path, "/api/v1/prep-areas") ||
		strings.HasPrefix(path, "/api/v1/categories") ||
		strings.HasPrefix(path, "/api/v1/products") ||
		strings.HasPrefix(path, "/api/v1/modifier-groups") ||
		strings.HasPrefix(path, "/api/v1/modifiers") ||
		strings.HasPrefix(path, "/api/v1/dietary-tags") ||
		path == "/api/v1/pricing/calculate" {
		if method == http.MethodGet || path == "/api/v1/pricing/calculate" {
			return []string{}, true
		}
		return []string{crew.RoleManagement}, true
	}
	// Anything else under /api/v1/ requires a session.
	return []string{}, true
}

// Middleware enforces server-side authorisation (TR-AUTH-05).
// It wraps the whole mux in main, so every handler benefits without
// each handler needing its own role check.
func (s *Store) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roles, staff := allowedRoles(r.URL.Path, r.Method)
		if !staff {
			next.ServeHTTP(w, r)
			return
		}
		// First-user bootstrap: with no crew records at all, the
		// first crew creation is allowed without a session.
		if r.URL.Path == "/api/v1/crew" && r.Method == http.MethodPost && s.crew.CrewCount() == 0 {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(CookieName)
		if err != nil || cookie.Value == "" {
			writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
			return
		}
		sess, err := s.ValidateSession(cookie.Value)
		if err != nil {
			writeProblem(w, http.StatusUnauthorized, "unauthenticated", "session is invalid or expired")
			return
		}
		if len(roles) > 0 && !roleAllowed(sess.Crew.Role, roles) {
			writeProblem(w, http.StatusForbidden, "forbidden", "role does not permit this action")
			return
		}
		ctx := context.WithValue(r.Context(), crewContextKey, sess.Crew)
		ctx = actor.WithActor(ctx, sess.Crew.CrewID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

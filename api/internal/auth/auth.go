// Package auth implements sign-in, sessions and PIN Overrides from
// TR-AUTH-01 to TR-AUTH-09 and BR-R-24, BR-R-25 and BR-R-38.
//
// Sign-in is Crew ID plus a 6-digit PIN for every role (TR-AUTH-01).
// PIN verification, including the 15-minute lockout after 5
// consecutive failures, lives in the crew store (TR-AUTH-02) and is
// reused here, so login and override share one lockout counter.
// Sessions are opaque random tokens in HttpOnly, Secure,
// SameSite=Strict cookies, stored server-side in the session table
// (TR-AUTH-03). Management sessions expire after 30 minutes of
// inactivity (TR-AUTH-04, BR-R-25); other sessions end after 12
// hours (TR-AUTH-09, BR-R-38), or at shift end once shifts exist
// in Phase 7. Authorisation is checked server-side per endpoint
// against role, following the BRD §5.1 permission matrix
// (TR-AUTH-05); the UI hiding a button is never the enforcement
// point.
//
// PIN Override (TR-AUTH-06): the authorising person enters their
// PIN on the requesting device. The server verifies their role
// permits the action and issues a short-lived, single-use override
// token, valid for one action and 60 seconds (TR-API-03). Future
// phases consume that token from the X-Override-Token header when
// performing Void, Remake, Unpaid Bill, unverified QRIS, Refund
// and manual discount actions. Override tokens live in memory:
// they are 60-second tokens in a single-binary server, so losing
// them on restart only means the authoriser re-enters their PIN.
//
// Bootstrap: on a fresh database with no crew records at all, the
// first POST /api/v1/crew is allowed without a session, so the
// first Management user can be created. Once any crew record
// exists, crew creation requires Management, like every other
// staff-record change. There is no seeded default PIN.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crew"
)

const (
	CookieName = "sushi_session"

	managementInactivity = 30 * time.Minute
	otherSessionTTL      = 12 * time.Hour
	overrideTTL          = 60 * time.Second

	// Per-IP login rate limit (TR-AUTH-02). The spec requires
	// per-IP rate limiting but gives no number; 10 failed attempts
	// per IP in 15 minutes is this implementation's choice.
	ipLimitWindow = 15 * time.Minute
	ipMaxFailures = 10
)

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Session is a row from the session table, with its crew record.
type Session struct {
	Token     string
	Crew      crew.Crew
	CreatedAt int64
	LastSeen  int64
	ExpiresAt int64
}

// SessionExpiry returns the expiry for a new session for this role.
func SessionExpiry(role string, now time.Time) time.Time {
	if role == crew.RoleManagement {
		return now.Add(managementInactivity)
	}
	return now.Add(otherSessionTTL)
}

// OverrideAction roles, from TR-AUTH-06.
func OverrideRoles(action string) ([]string, bool) {
	switch action {
	case "manual_discount":
		return []string{crew.RoleManagement}, true
	case "void", "remake", "unpaid_bill", "unverified_qris", "refund_le_cap":
		return []string{crew.RoleOutletAdmin, crew.RoleManagement}, true
	case "refund_gt_cap":
		return []string{crew.RoleManagement}, true
	default:
		return nil, false
	}
}

func roleAllowed(role string, allowed []string) bool {
	for _, a := range allowed {
		if role == a {
			return true
		}
	}
	return false
}

// OverrideToken is a single-use, 60-second authorisation.
type OverrideToken struct {
	Token     string
	Action    string
	Approver  crew.Crew
	ExpiresAt time.Time
}

// IPLimiter tracks failed logins per client IP.
type IPLimiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
}

func NewIPLimiter() *IPLimiter {
	return &IPLimiter{failures: map[string][]time.Time{}}
}

// Blocked reports whether this IP has exceeded the failure limit.
func (l *IPLimiter) Blocked(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	recent := []time.Time{}
	for _, t := range l.failures[ip] {
		if now.Sub(t) < ipLimitWindow {
			recent = append(recent, t)
		}
	}
	l.failures[ip] = recent
	return len(recent) >= ipMaxFailures
}

// RecordFailure records a failed login from this IP.
func (l *IPLimiter) RecordFailure(ip string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[ip] = append(l.failures[ip], now)
}

// Clear removes the failure record for an IP, after a successful login.
func (l *IPLimiter) Clear(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, ip)
}

// Package auth implements Crew ID + PIN sign-in, lockout, sessions, screen mode and
// PIN Overrides (TRD §8).
package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/store"
)

const (
	lockThreshold     = 5
	lockDuration      = 15 * time.Minute
	managementIdle    = 30 * time.Minute
	crewSessionLength = 12 * time.Hour
	overrideLifetime  = 60 * time.Second
	touchEvery        = time.Minute
	// Screen-mode sessions last until the end of the business day, 04:00 WIB.
	businessDayEndsHour = 4
)

// Principal is the authenticated identity of a request.
type Principal struct {
	SessionID string   `json:"-"`
	ID        string   `json:"-"` // crew.id
	CrewID    string   `json:"crewId"`
	Name      string   `json:"name"`
	Role      string   `json:"role"`
	Stations  []string `json:"stations"`
	ImageKey  string   `json:"imageKey,omitempty"`
	Kind      string   `json:"kind"`             // crew | screen
	Screen    string   `json:"screen,omitempty"` // set when Kind is screen
	ExpiresAt int64    `json:"expiresAt"`
}

func (p *Principal) HasRole(roles ...string) bool {
	for _, r := range roles {
		if p.Role == r {
			return true
		}
	}
	return false
}

func (p *Principal) HasStation(station string) bool {
	for _, s := range p.Stations {
		if s == station {
			return true
		}
	}
	return false
}

type Service struct {
	db    *dbx.DB
	clock platform.Clock
}

func NewService(db *dbx.DB, clock platform.Clock) *Service {
	return &Service{db: db, clock: clock}
}

var (
	errBadCredentials = httpx.NewProblem(http.StatusUnauthorized, "invalid_credentials", "Crew ID atau PIN salah")
	errSessionExpired = httpx.NewProblem(http.StatusUnauthorized, "session_expired", "Sesi berakhir, silakan masuk lagi")
)

func lockedProblem(until int64) *httpx.Problem {
	return httpx.NewProblem(http.StatusLocked, "account_locked", "Akun terkunci sementara").With("lockedUntil", until)
}

// Login signs a Crew member in and returns the session token (TR-AUTH-01..04, TR-AUTH-09).
func (s *Service) Login(ctx context.Context, crewID, pin, deviceLabel string) (string, *Principal, error) {
	crew, err := s.CheckPIN(ctx, crewID, pin)
	if err != nil {
		return "", nil, err
	}
	now := s.clock.Now()
	expires := now.Add(crewSessionLength)
	var idle sql.NullInt64
	if crew.Role == RoleManagement {
		idle = sql.NullInt64{Int64: managementIdle.Milliseconds(), Valid: true}
	}
	return s.createSession(ctx, crew, "crew", "", deviceLabel, now, expires, idle)
}

// CheckPIN verifies a Crew member's PIN and applies the lockout rules (TR-AUTH-02); the HRIS
// clock-in pad uses it too, so a wrong PIN there counts towards the same lockout.
// A failed attempt must be committed even though the request fails, so the transaction
// returns nil and the outcome is reported after the commit.
func (s *Service) CheckPIN(ctx context.Context, crewID, pin string) (store.Crew, error) {
	var crew store.Crew
	var outcome error
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		now := s.clock.Now()
		c, err := tx.GetCrewByCrewID(ctx, crewID)
		if errors.Is(err, sql.ErrNoRows) {
			outcome = errBadCredentials
			return nil
		}
		if err != nil {
			return err
		}
		if c.LockedUntil.Valid && c.LockedUntil.Int64 > platform.Millis(now) {
			outcome = lockedProblem(c.LockedUntil.Int64)
			return nil
		}
		if !ValidPIN(pin) || !VerifyPIN(pin, c.PinHash) {
			row, err := tx.RecordFailedLogin(ctx, store.RecordFailedLoginParams{
				Threshold: lockThreshold,
				LockUntil: sql.NullInt64{Int64: platform.Millis(now.Add(lockDuration)), Valid: true},
				Now:       platform.Millis(now),
				ID:        c.ID,
			})
			if err != nil {
				return err
			}
			if row.LockedUntil.Valid && row.LockedUntil.Int64 > platform.Millis(now) {
				outcome = lockedProblem(row.LockedUntil.Int64)
			} else {
				outcome = httpx.NewProblem(http.StatusUnauthorized, "invalid_credentials", "Crew ID atau PIN salah").
					With("attemptsRemaining", lockThreshold-row.FailedAttempts)
			}
			return nil
		}
		if c.FailedAttempts > 0 || c.LockedUntil.Valid {
			if err := tx.ClearLockout(ctx, store.ClearLockoutParams{UpdatedAt: platform.Millis(now), ID: c.ID}); err != nil {
				return err
			}
		}
		crew = c
		return nil
	})
	if err != nil {
		return store.Crew{}, err
	}
	return crew, outcome
}

func (s *Service) createSession(ctx context.Context, crew store.Crew, kind, screen, deviceLabel string, now, expires time.Time, idle sql.NullInt64) (string, *Principal, error) {
	token, digest := platform.NewToken()
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		return tx.CreateSession(ctx, store.CreateSessionParams{
			ID: digest, CrewID: crew.ID, Kind: kind,
			Screen:        sql.NullString{String: screen, Valid: screen != ""},
			DeviceLabel:   sql.NullString{String: deviceLabel, Valid: deviceLabel != ""},
			CreatedAt:     platform.Millis(now),
			LastSeenAt:    platform.Millis(now),
			ExpiresAt:     platform.Millis(expires),
			IdleTimeoutMs: idle,
		})
	})
	if err != nil {
		return "", nil, err
	}
	p, err := s.principal(ctx, crew, digest, kind, screen, platform.Millis(expires))
	return token, p, err
}

func (s *Service) principal(ctx context.Context, crew store.Crew, sessionID, kind, screen string, expiresAt int64) (*Principal, error) {
	stations, err := s.db.Read().ListCrewStations(ctx, crew.ID)
	if err != nil {
		return nil, err
	}
	return &Principal{
		SessionID: sessionID, ID: crew.ID, CrewID: crew.CrewID, Name: crew.FullName, Role: crew.Role,
		Stations: stations, ImageKey: crew.ImageKey.String, Kind: kind, Screen: screen, ExpiresAt: expiresAt,
	}, nil
}

// Authenticate resolves a session token, enforcing absolute and idle expiry.
func (s *Service) Authenticate(ctx context.Context, token string) (*Principal, error) {
	digest := platform.Digest(token)
	sess, err := s.db.Read().GetSession(ctx, digest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, httpx.ErrUnauthorised
	}
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	nowMs := platform.Millis(now)
	idleExpired := sess.IdleTimeoutMs.Valid && nowMs-sess.LastSeenAt > sess.IdleTimeoutMs.Int64
	if sess.ExpiresAt <= nowMs || idleExpired {
		_ = s.db.Write(ctx, func(tx *dbx.Tx) error { return tx.DeleteSession(ctx, digest) })
		return nil, errSessionExpired
	}
	if nowMs-sess.LastSeenAt > touchEvery.Milliseconds() {
		_ = s.db.Write(ctx, func(tx *dbx.Tx) error {
			return tx.TouchSession(ctx, store.TouchSessionParams{LastSeenAt: nowMs, ID: digest})
		})
	}
	crew, err := s.db.Read().GetCrew(ctx, sess.CrewID)
	if err != nil {
		return nil, err
	}
	if crew.Active == 0 {
		return nil, httpx.ErrUnauthorised
	}
	return s.principal(ctx, crew, digest, sess.Kind, sess.Screen.String, sess.ExpiresAt)
}

func (s *Service) Logout(ctx context.Context, p *Principal) error {
	return s.db.Write(ctx, func(tx *dbx.Tx) error { return tx.DeleteSession(ctx, p.SessionID) })
}

// EnterScreenMode turns an Outlet Admin or Management session on a shared device into a
// screen-mode session limited to one screen, valid until the end of the business day.
func (s *Service) EnterScreenMode(ctx context.Context, p *Principal, screen, deviceLabel string) (string, *Principal, error) {
	if !p.HasRole(RoleOutletAdmin, RoleManagement) {
		return "", nil, httpx.ErrForbidden
	}
	switch screen {
	case ScreenKitchen, ScreenBar, ScreenDisplay, ScreenHRIS:
	default:
		return "", nil, httpx.NewProblem(http.StatusBadRequest, "unknown_screen", "Unknown screen")
	}
	crew, err := s.db.Read().GetCrew(ctx, p.ID)
	if err != nil {
		return "", nil, err
	}
	if err := s.Logout(ctx, p); err != nil {
		return "", nil, err
	}
	now := s.clock.Now()
	return s.createSession(ctx, crew, "screen", screen, deviceLabel, now, endOfBusinessDay(now), sql.NullInt64{})
}

func endOfBusinessDay(now time.Time) time.Time {
	local := now.In(platform.Jakarta)
	end := time.Date(local.Year(), local.Month(), local.Day(), businessDayEndsHour, 0, 0, 0, platform.Jakarta)
	if !end.After(local) {
		end = end.AddDate(0, 0, 1)
	}
	return end
}

// IssueOverride verifies an approver's PIN for one action and returns a one-time token
// valid for 60 seconds (TR-AUTH-06, TR-API-03).
func (s *Service) IssueOverride(ctx context.Context, requester *Principal, approverCrewID, pin, action string) (string, int64, error) {
	if !KnownAction(action) {
		return "", 0, httpx.NewProblem(http.StatusBadRequest, "unknown_action", "Unknown override action")
	}
	approver, err := s.CheckPIN(ctx, approverCrewID, pin)
	if err != nil {
		return "", 0, err
	}
	if !CanApprove(approver.Role, action) {
		return "", 0, httpx.NewProblem(http.StatusForbidden, "approver_not_allowed", "Peran ini tidak bisa menyetujui tindakan ini").
			With("action", action)
	}
	now := s.clock.Now()
	expires := now.Add(overrideLifetime)
	token, digest := platform.NewToken()
	err = s.db.Write(ctx, func(tx *dbx.Tx) error {
		return tx.CreateOverrideToken(ctx, store.CreateOverrideTokenParams{
			ID: digest, ApproverID: approver.ID, RequesterID: requester.ID, Action: action,
			CreatedAt: platform.Millis(now), ExpiresAt: platform.Millis(expires),
		})
	})
	return token, platform.Millis(expires), err
}

var errOverrideInvalid = httpx.NewProblem(http.StatusForbidden, "override_invalid", "Persetujuan PIN tidak valid atau kedaluwarsa")

// ConsumeOverride spends an override token inside the caller's transaction and records
// the audit entry for the approved action. It returns the approver's crew.id.
func ConsumeOverride(ctx context.Context, tx *dbx.Tx, now time.Time, requester *Principal, token, action string, entry audit.Entry) (string, error) {
	if token == "" {
		return "", httpx.NewProblem(http.StatusForbidden, "override_required", "Tindakan ini butuh persetujuan PIN").With("action", action)
	}
	approver, err := tx.ConsumeOverrideToken(ctx, store.ConsumeOverrideTokenParams{
		Now: sql.NullInt64{Int64: platform.Millis(now), Valid: true}, ID: platform.Digest(token),
		Action: action, RequesterID: requester.ID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", errOverrideInvalid
	}
	if err != nil {
		return "", err
	}
	entry.Actor = requester.ID
	entry.Approver = approver
	if err := audit.Write(ctx, tx, now, entry); err != nil {
		return "", err
	}
	return approver, nil
}

// NewCrew is the input for creating a Crew member (Admin Panel and the demo seed).
type NewCrew struct {
	CrewID   string
	FullName string
	Role     string
	Stations []string
	PIN      string
	ImageKey string
}

// CreateCrew inserts a Crew member with a hashed PIN inside the caller's transaction.
func CreateCrew(ctx context.Context, tx *dbx.Tx, now time.Time, in NewCrew) (string, error) {
	hash, err := HashPIN(in.PIN)
	if err != nil {
		return "", err
	}
	id := platform.NewID()
	ms := platform.Millis(now)
	if err := tx.CreateCrew(ctx, store.CreateCrewParams{
		ID: id, CrewID: in.CrewID, FullName: in.FullName, Role: in.Role,
		ImageKey: sql.NullString{String: in.ImageKey, Valid: in.ImageKey != ""},
		PinHash:  hash, CreatedAt: ms, UpdatedAt: ms,
	}); err != nil {
		return "", err
	}
	for _, st := range in.Stations {
		if err := tx.AddCrewStation(ctx, store.AddCrewStationParams{CrewID: id, Station: st}); err != nil {
			return "", err
		}
	}
	return id, nil
}

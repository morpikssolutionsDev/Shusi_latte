// Package crew is Crew administration for Management (BR-ADM-09): records, roles,
// Stations, portraits, activation and PIN resets.
package crew

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/store"
)

type Member struct {
	ID          string   `json:"id"`
	CrewID      string   `json:"crewId"`
	FullName    string   `json:"fullName"`
	Role        string   `json:"role"`
	Stations    []string `json:"stations"`
	ImageKey    string   `json:"imageKey,omitempty"`
	Active      bool     `json:"active"`
	LockedUntil *int64   `json:"lockedUntil,omitempty"`
}

type Input struct {
	CrewID   string   `json:"crewId"`
	FullName string   `json:"fullName"`
	Role     string   `json:"role"`
	Stations []string `json:"stations"`
	ImageKey string   `json:"imageKey,omitempty"`
	Active   bool     `json:"active"`
	PIN      string   `json:"pin,omitempty"` // required on create only
}

type Service struct {
	db    *dbx.DB
	clock platform.Clock
}

func NewService(db *dbx.DB, clock platform.Clock) *Service { return &Service{db: db, clock: clock} }

var crewIDPattern = regexp.MustCompile(`^[A-Z][0-9]{3}$`)

func invalid(detail string) error {
	p := httpx.NewProblem(http.StatusUnprocessableEntity, "invalid_crew", "Data crew tidak valid")
	p.Detail = detail
	return p
}

func (s *Service) List(ctx context.Context) ([]Member, error) {
	q := s.db.Read()
	rows, err := q.ListAllCrew(ctx)
	if err != nil {
		return nil, err
	}
	stations, err := q.ListAllCrewStations(ctx)
	if err != nil {
		return nil, err
	}
	now := platform.Millis(s.clock.Now())
	out := make([]Member, 0, len(rows))
	for _, r := range rows {
		m := Member{ID: r.ID, CrewID: r.CrewID, FullName: r.FullName, Role: r.Role, ImageKey: r.ImageKey.String, Active: r.Active == 1, Stations: []string{}}
		if r.LockedUntil.Valid && r.LockedUntil.Int64 > now {
			v := r.LockedUntil.Int64
			m.LockedUntil = &v
		}
		for _, st := range stations {
			if st.CrewID == r.ID {
				m.Stations = append(m.Stations, st.Station)
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func validate(in *Input) error {
	in.CrewID = strings.ToUpper(strings.TrimSpace(in.CrewID))
	in.FullName = strings.TrimSpace(in.FullName)
	if in.FullName == "" {
		return invalid("Nama wajib diisi")
	}
	switch in.Role {
	case auth.RoleManagement, auth.RoleOutletAdmin:
		in.Stations = nil
	case auth.RoleCrew:
		if len(in.Stations) == 0 {
			return invalid("Crew wajib punya minimal satu Station")
		}
		for _, st := range in.Stations {
			if !slices.Contains([]string{auth.StationWaiter, auth.StationCashier, auth.StationKitchen}, st) {
				return invalid("Station tidak dikenal")
			}
		}
	default:
		return invalid("Peran tidak dikenal")
	}
	return nil
}

func (s *Service) Create(ctx context.Context, actor string, in Input) (Member, error) {
	if err := validate(&in); err != nil {
		return Member{}, err
	}
	if !crewIDPattern.MatchString(in.CrewID) {
		return Member{}, invalid("Crew ID: satu huruf peran + tiga angka, mis. W006")
	}
	if !auth.ValidPIN(in.PIN) {
		return Member{}, invalid("PIN harus 6 angka")
	}
	var id string
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		var err error
		id, err = auth.CreateCrew(ctx, tx, s.clock.Now(), auth.NewCrew{CrewID: in.CrewID, FullName: in.FullName, Role: in.Role,
			Stations: in.Stations, PIN: in.PIN, ImageKey: in.ImageKey})
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				return invalid("Crew ID sudah dipakai")
			}
			return err
		}
		in.PIN = ""
		return audit.Write(ctx, tx, s.clock.Now(), audit.Entry{Actor: actor, Action: "crew.created", Entity: "crew", EntityID: id, After: in})
	})
	if err != nil {
		return Member{}, err
	}
	return s.get(ctx, id)
}

func (s *Service) Update(ctx context.Context, actor, id string, in Input) (Member, error) {
	if err := validate(&in); err != nil {
		return Member{}, err
	}
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		before, err := tx.GetCrew(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return httpx.NewProblem(http.StatusNotFound, "not_found", "Crew tidak ditemukan")
		}
		if err != nil {
			return err
		}
		losingManagement := before.Role == auth.RoleManagement && before.Active == 1 && (in.Role != auth.RoleManagement || !in.Active)
		if losingManagement {
			n, err := tx.CountActiveManagement(ctx)
			if err != nil {
				return err
			}
			if n <= 1 {
				return invalid("Harus ada minimal satu Management aktif")
			}
		}
		now := platform.Millis(s.clock.Now())
		if err := tx.UpdateCrew(ctx, store.UpdateCrewParams{FullName: in.FullName, Role: in.Role,
			ImageKey: sql.NullString{String: in.ImageKey, Valid: in.ImageKey != ""}, Active: boolInt(in.Active), UpdatedAt: now, ID: id}); err != nil {
			return err
		}
		if err := tx.ClearCrewStations(ctx, id); err != nil {
			return err
		}
		for _, st := range in.Stations {
			if err := tx.AddCrewStation(ctx, store.AddCrewStationParams{CrewID: id, Station: st}); err != nil {
				return err
			}
		}
		// Role, Station or activation changes take effect immediately: end existing sessions.
		if !in.Active || before.Role != in.Role {
			if err := tx.DeleteCrewSessions(ctx, id); err != nil {
				return err
			}
		}
		return audit.Write(ctx, tx, s.clock.Now(), audit.Entry{Actor: actor, Action: "crew.updated", Entity: "crew", EntityID: id,
			Before: map[string]any{"fullName": before.FullName, "role": before.Role, "active": before.Active == 1}, After: in})
	})
	if err != nil {
		return Member{}, err
	}
	return s.get(ctx, id)
}

// ResetPIN sets a new PIN and clears any lockout (BR-ADM-09). Sessions are ended.
func (s *Service) ResetPIN(ctx context.Context, actor, id, pin string) error {
	if !auth.ValidPIN(pin) {
		return invalid("PIN harus 6 angka")
	}
	hash, err := auth.HashPIN(pin)
	if err != nil {
		return err
	}
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		if _, err := tx.GetCrew(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return httpx.NewProblem(http.StatusNotFound, "not_found", "Crew tidak ditemukan")
		} else if err != nil {
			return err
		}
		if err := tx.SetCrewPIN(ctx, store.SetCrewPINParams{PinHash: hash, UpdatedAt: platform.Millis(s.clock.Now()), ID: id}); err != nil {
			return err
		}
		if err := tx.DeleteCrewSessions(ctx, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, s.clock.Now(), audit.Entry{Actor: actor, Action: "crew.pin_reset", Entity: "crew", EntityID: id})
	})
}

func (s *Service) get(ctx context.Context, id string) (Member, error) {
	all, err := s.List(ctx)
	if err != nil {
		return Member{}, err
	}
	for _, m := range all {
		if m.ID == id {
			return m, nil
		}
	}
	return Member{}, sql.ErrNoRows
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

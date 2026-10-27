// Package hris is the HRIS (OP-15): employee records, shift scheduling, clock-in and clock-out on
// the shared HRIS tablet with its blockers, auto clock-out and corrections, leave requests and the
// attendance export for external payroll (BR-HR-01..07).
package hris

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

// Topic carries attendance, shift and leave changes to the HRIS tablet and the Admin.
const Topic = "hris"

const (
	earlyClockIn  = 2 * time.Hour  // a clock-in up to 2 h before a shift belongs to it
	noShiftCap    = 12 * time.Hour // without a shift, auto clock-out counts from clock-in
	maxShift      = 16 * time.Hour
	lateAfter     = 5 * time.Minute
	maxLeaveDays  = 30
	dateLayout    = time.DateOnly
	clockInAction = "in"
)

type Service struct {
	db    *dbx.DB
	clock platform.Clock
	auth  *auth.Service
}

func NewService(db *dbx.DB, clock platform.Clock, authSvc *auth.Service) *Service {
	return &Service{db: db, clock: clock, auth: authSvc}
}

// WithClock returns a copy of the service running on another clock (seed, tests).
func (s *Service) WithClock(c platform.Clock) *Service {
	cp := *s
	cp.clock = c
	return &cp
}

func invalid(detail string) error {
	p := httpx.NewProblem(http.StatusUnprocessableEntity, "invalid_input", "Data belum lengkap")
	p.Detail = detail
	return p
}

func conflict(code, detail string) *httpx.Problem {
	p := httpx.NewProblem(http.StatusConflict, code, detail)
	p.Detail = detail
	return p
}

func nullStr(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }
func nullMs(ms int64) sql.NullInt64   { return sql.NullInt64{Int64: ms, Valid: true} }

func ptr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}

// Rupiah formats an amount the way staff screens do: "Rp 200.000".
func Rupiah(n int64) string {
	sign := ""
	if n < 0 {
		sign, n = "−", -n
	}
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return sign + "Rp " + s
}

func publish(tx *dbx.Tx, kind, id string) {
	tx.Publish(realtime.Event{Topic: Topic, Type: kind, ID: id})
}

// crewNames maps crew.id to its Crew ID and name for the views.
type person struct{ Code, Name string }

func (s *Service) people(ctx context.Context) (map[string]person, error) {
	rows, err := s.db.Read().ListAllCrew(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]person, len(rows))
	for _, c := range rows {
		out[c.ID] = person{c.CrewID, c.FullName}
	}
	return out, nil
}

// ---------------------------------------------------------------- clock in / out (BR-HR-03, BR-HR-07)

type ClockInput struct {
	CrewID string `json:"crewId"` // the Crew ID typed on the pad (W001)
	PIN    string `json:"pin"`
	// AcknowledgeClaims clocks out although Claims are still open; they stay with the Outlet
	// Admin to reassign (BR-WT-11, G-06).
	AcknowledgeClaims bool `json:"acknowledgeClaims"`
}

type ShiftTime struct {
	StartsAt int64 `json:"startsAt"`
	EndsAt   int64 `json:"endsAt"`
}

type ClockResult struct {
	Action        string     `json:"action"` // in | out
	CrewID        string     `json:"crewId"`
	Name          string     `json:"name"`
	At            int64      `json:"at"`
	Shift         *ShiftTime `json:"shift,omitempty"`
	LateMinutes   int64      `json:"lateMinutes"`
	WorkedMinutes int64      `json:"workedMinutes"`
}

type OpenClaim struct {
	OrderID string `json:"orderId"`
	Seq     int64  `json:"seq"`
	Status  string `json:"status"`
	Table   string `json:"table"`
}

// Clock clocks a Crew member in, or out when already in. Only an HRIS screen-mode session may
// call it (TR-AUTH-08); the PIN goes through the sign-in lockout. Clock-out is refused while the
// Crew member holds cash (BR-HR-07) and asks for acknowledgement of open Claims (G-06).
func (s *Service) Clock(ctx context.Context, in ClockInput) (ClockResult, error) {
	code := strings.ToUpper(strings.TrimSpace(in.CrewID))
	crew, err := s.auth.CheckPIN(ctx, code, in.PIN)
	if err != nil {
		return ClockResult{}, err
	}
	return s.ClockCrew(ctx, crew, in.AcknowledgeClaims)
}

// ClockCrew applies every clock-in and clock-out rule to a Crew member already identified: by
// the pad's PIN check above, or by the demo seed playing the week's attendance.
func (s *Service) ClockCrew(ctx context.Context, crew store.Crew, acknowledgeClaims bool) (ClockResult, error) {
	in := ClockInput{AcknowledgeClaims: acknowledgeClaims}
	if crew.Active == 0 {
		return ClockResult{}, httpx.NewProblem(http.StatusForbidden, "crew_inactive", "Akun crew tidak aktif")
	}
	now := s.clock.Now()
	nowMs := platform.Millis(now)
	out := ClockResult{CrewID: crew.CrewID, Name: crew.FullName, At: nowMs}
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		open, err := tx.GetOpenAttendance(ctx, crew.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return s.clockIn(ctx, tx, crew, nowMs, &out)
		}
		if err != nil {
			return err
		}
		held, err := tx.ListHeldPayments(ctx, crew.ID)
		if err != nil {
			return err
		}
		var amount int64
		for _, p := range held {
			amount += p.Amount
		}
		if amount > 0 {
			return conflict("cash_held", fmt.Sprintf("Setor %s ke kasir dulu.", Rupiah(amount))).With("amount", amount)
		}
		claims, err := tx.ListOpenClaims(ctx, nullStr(crew.ID))
		if err != nil {
			return err
		}
		if len(claims) > 0 && !in.AcknowledgeClaims {
			list := make([]OpenClaim, 0, len(claims))
			for _, c := range claims {
				oc := OpenClaim{OrderID: c.ID, Seq: c.Seq, Status: c.Status, Table: "Bawa pulang"}
				if tables, err := tx.ListBillTables(ctx, c.BillID); err == nil && len(tables) > 0 {
					labels := make([]string, len(tables))
					for i, t := range tables {
						labels[i] = t.Label
					}
					oc.Table = strings.Join(labels, " + ")
				}
				list = append(list, oc)
			}
			return conflict("open_claims", "Serahkan klaim ini ke pelayan lain atau Outlet Admin.").With("claims", list)
		}
		if n, err := tx.ClockOut(ctx, store.ClockOutParams{ClockOutAt: nullMs(nowMs), AutoClockedOut: 0, ID: open.ID}); err != nil {
			return err
		} else if n == 0 {
			return conflict("not_clocked_in", "Belum absen masuk")
		}
		out.Action = "out"
		out.WorkedMinutes = (nowMs - open.ClockInAt) / 60000
		if open.ShiftID.Valid {
			if sh, err := tx.GetShift(ctx, open.ShiftID.String); err == nil {
				out.Shift = &ShiftTime{sh.StartsAt, sh.EndsAt}
			}
		}
		if len(claims) > 0 {
			if err := audit.Write(ctx, tx, now, audit.Entry{Actor: crew.ID, Action: "attendance.clock_out_with_claims", Entity: "attendance",
				EntityID: open.ID, After: map[string]int{"openClaims": len(claims)}}); err != nil {
				return err
			}
		}
		publish(tx, "attendance.clock_out", open.ID)
		return nil
	})
	return out, err
}

func (s *Service) clockIn(ctx context.Context, tx *dbx.Tx, crew store.Crew, nowMs int64, out *ClockResult) error {
	id := platform.NewID()
	var shiftID sql.NullString
	sh, err := tx.FindShiftForClockIn(ctx, store.FindShiftForClockInParams{CrewID: crew.ID, EarlyStart: nowMs + earlyClockIn.Milliseconds(), At: nowMs})
	switch {
	case err == nil:
		shiftID = nullStr(sh.ID)
		out.Shift = &ShiftTime{sh.StartsAt, sh.EndsAt}
		if late := nowMs - sh.StartsAt; late > lateAfter.Milliseconds() {
			out.LateMinutes = late / 60000
		}
	case !errors.Is(err, sql.ErrNoRows):
		return err
	}
	if err := tx.ClockIn(ctx, store.ClockInParams{ID: id, CrewID: crew.ID, ShiftID: shiftID, ClockInAt: nowMs}); err != nil {
		return err
	}
	out.Action = clockInAction
	publish(tx, "attendance.clock_in", id)
	return nil
}

// AutoClockOut closes attendance still open 2 hours (ops setting) after its shift ended, or 12
// hours after clock-in without a shift; the record is flagged "auto" for the Outlet Admin to
// correct (BR-HR-06, G-22). It runs every 5 minutes.
func (s *Service) AutoClockOut(ctx context.Context) (int, error) {
	now := s.clock.Now()
	n := 0
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		ops, err := tx.GetOpsSetting(ctx)
		if err != nil {
			return err
		}
		grace := ops.AutoClockOutGraceSeconds * 1000
		rows, err := tx.ListOverdueAttendance(ctx, store.ListOverdueAttendanceParams{GraceMs: grace, Now: platform.Millis(now)})
		if err != nil {
			return err
		}
		for _, a := range rows {
			end := a.ClockInAt + noShiftCap.Milliseconds()
			if a.ShiftEndsAt.Valid {
				end = a.ShiftEndsAt.Int64
			}
			at := end + grace
			if _, err := tx.ClockOut(ctx, store.ClockOutParams{ClockOutAt: nullMs(at), AutoClockedOut: 1, ID: a.ID}); err != nil {
				return err
			}
			if err := audit.Write(ctx, tx, now, audit.Entry{Action: "attendance.auto_clock_out", Entity: "attendance", EntityID: a.ID,
				After: map[string]any{"crewId": a.CrewID, "clockOutAt": at}}); err != nil {
				return err
			}
			publish(tx, "attendance.auto_clock_out", a.ID)
			n++
		}
		return nil
	})
	return n, err
}

type CorrectInput struct {
	ClockInAt  int64  `json:"clockInAt"`
	ClockOutAt int64  `json:"clockOutAt"`
	Reason     string `json:"reason"`
}

// Correct fixes an attendance record's times with a reason; audit-logged (BR-HR-06).
func (s *Service) Correct(ctx context.Context, p *auth.Principal, id string, in CorrectInput) error {
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Reason == "" {
		return invalid("Tulis alasan koreksi.")
	}
	if in.ClockInAt <= 0 || in.ClockOutAt <= in.ClockInAt {
		return invalid("Jam keluar harus setelah jam masuk.")
	}
	now := s.clock.Now()
	if in.ClockOutAt > platform.Millis(now) {
		return invalid("Jam keluar belum lewat.")
	}
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		a, err := tx.GetAttendance(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return httpx.NewProblem(http.StatusNotFound, "not_found", "Data absensi tidak ditemukan")
		}
		if err != nil {
			return err
		}
		if err := tx.CorrectAttendance(ctx, store.CorrectAttendanceParams{ClockInAt: in.ClockInAt, ClockOutAt: nullMs(in.ClockOutAt),
			CorrectedBy: nullStr(p.ID), CorrectedAt: nullMs(platform.Millis(now)), CorrectionReason: nullStr(in.Reason), ID: id}); err != nil {
			return err
		}
		publish(tx, "attendance.corrected", id)
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "attendance.corrected", Entity: "attendance", EntityID: id,
			Before: map[string]any{"clockInAt": a.ClockInAt, "clockOutAt": ptr(a.ClockOutAt), "auto": a.AutoClockedOut == 1},
			After:  map[string]any{"clockInAt": in.ClockInAt, "clockOutAt": in.ClockOutAt}, Reason: in.Reason})
	})
}

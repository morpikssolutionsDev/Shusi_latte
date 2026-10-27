package hris_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"testing"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/hris"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/store"
	"sushilatte/api/internal/testutil"
)

var t0 = time.Date(2026, 10, 27, 8, 0, 0, 0, platform.Jakarta)

type fixture struct {
	db          *dbx.DB
	clock       *testutil.Clock
	svc         *hris.Service
	admin, mgmt *auth.Principal
	w1, w2, k1  *auth.Principal
	ctx         context.Context
}

func setup(t *testing.T) fixture {
	t.Helper()
	db, _ := testutil.NewDB(t)
	clock := testutil.NewClock(t0)
	f := fixture{db: db, clock: clock, ctx: context.Background()}
	mk := func(tx *dbx.Tx, code, role string, stations []string, pin string) *auth.Principal {
		id, err := auth.CreateCrew(f.ctx, tx, t0, auth.NewCrew{CrewID: code, FullName: code + " Crew", Role: role, Stations: stations, PIN: pin})
		if err != nil {
			t.Fatal(err)
		}
		return &auth.Principal{ID: id, CrewID: code, Name: code + " Crew", Role: role, Stations: stations, Kind: "crew"}
	}
	if err := db.Write(f.ctx, func(tx *dbx.Tx) error {
		f.mgmt = mk(tx, "M001", auth.RoleManagement, []string{}, "111111")
		f.admin = mk(tx, "A001", auth.RoleOutletAdmin, []string{}, "222222")
		f.w1 = mk(tx, "W001", auth.RoleCrew, []string{auth.StationWaiter}, "333333")
		f.w2 = mk(tx, "W002", auth.RoleCrew, []string{auth.StationWaiter}, "333333")
		f.k1 = mk(tx, "K001", auth.RoleCrew, []string{auth.StationKitchen}, "555555")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.svc = hris.NewService(db, clock, auth.NewService(db, clock))
	return f
}

func code(err error) string {
	var p *httpx.Problem
	if errors.As(err, &p) {
		return p.Code
	}
	return ""
}

func (f fixture) shift(t *testing.T, who *auth.Principal, start time.Time, hours int) string {
	t.Helper()
	id, err := f.svc.SaveShift(f.ctx, f.admin, "", hris.ShiftInput{CrewID: who.ID, StartsAt: platform.Millis(start), EndsAt: platform.Millis(start.Add(time.Duration(hours) * time.Hour))})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// BR-HR-03, TR-HR-01: clock in links the shift and reports lateness; the second clock is the clock-out.
func TestClockInAndOut(t *testing.T) {
	f := setup(t)
	f.shift(t, f.w1, t0, 8)
	f.clock.Advance(12 * time.Minute)
	in, err := f.svc.Clock(f.ctx, hris.ClockInput{CrewID: "w001", PIN: "333333"})
	if err != nil || in.Action != "in" || in.Shift == nil || in.LateMinutes != 12 {
		t.Fatalf("clock in: %+v %v", in, err)
	}
	f.clock.Advance(8 * time.Hour)
	out, err := f.svc.Clock(f.ctx, hris.ClockInput{CrewID: "W001", PIN: "333333"})
	if err != nil || out.Action != "out" || out.WorkedMinutes != 480 {
		t.Fatalf("clock out: %+v %v", out, err)
	}
	rows, _ := f.svc.Attendance(f.ctx, "2026-10-27", "2026-10-27")
	if len(rows) != 1 || rows[0].Status != "done" || rows[0].LateMinutes != 12 {
		t.Fatalf("attendance: %+v", rows)
	}
}

// TR-HR-02, TR-AUTH-02: a wrong PIN on the pad counts towards the sign-in lockout.
func TestClockWrongPINLocks(t *testing.T) {
	f := setup(t)
	for i := 0; i < 5; i++ {
		_, err := f.svc.Clock(f.ctx, hris.ClockInput{CrewID: "W001", PIN: "000000"})
		if err == nil {
			t.Fatal("wrong PIN accepted")
		}
	}
	if _, err := f.svc.Clock(f.ctx, hris.ClockInput{CrewID: "W001", PIN: "333333"}); code(err) != "account_locked" {
		t.Fatalf("after 5 failures: %v", err)
	}
}

// BR-HR-06, TR-HR-04, G-22: still clocked in 2 h after the shift → auto clock-out, flagged; the Outlet
// Admin corrects it with a reason, audit-logged.
func TestAutoClockOutAndCorrection(t *testing.T) {
	f := setup(t)
	f.shift(t, f.k1, t0, 8)
	if _, err := f.svc.Clock(f.ctx, hris.ClockInput{CrewID: "K001", PIN: "555555"}); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(9*time.Hour + 59*time.Minute)
	if n, _ := f.svc.AutoClockOut(f.ctx); n != 0 {
		t.Fatal("clocked out before shift end + 2 h")
	}
	f.clock.Advance(2 * time.Minute)
	if n, err := f.svc.AutoClockOut(f.ctx); n != 1 || err != nil {
		t.Fatalf("auto clock-out: %d %v", n, err)
	}
	rows, _ := f.svc.Attendance(f.ctx, "2026-10-27", "2026-10-27")
	if rows[0].Status != "auto" || !rows[0].Auto || *rows[0].ClockOutAt != platform.Millis(t0.Add(10*time.Hour)) {
		t.Fatalf("auto row: %+v", rows[0])
	}
	if err := f.svc.Correct(f.ctx, f.admin, rows[0].AttendanceID, hris.CorrectInput{ClockInAt: *rows[0].ClockInAt,
		ClockOutAt: platform.Millis(t0.Add(8 * time.Hour)), Reason: ""}); code(err) != "invalid_input" {
		t.Fatalf("correction without reason: %v", err)
	}
	if err := f.svc.Correct(f.ctx, f.admin, rows[0].AttendanceID, hris.CorrectInput{ClockInAt: *rows[0].ClockInAt,
		ClockOutAt: platform.Millis(t0.Add(8 * time.Hour)), Reason: "Lupa absen pulang"}); err != nil {
		t.Fatal(err)
	}
	rows, _ = f.svc.Attendance(f.ctx, "2026-10-27", "2026-10-27")
	if rows[0].Status != "done" || rows[0].WorkedMinutes != 480 || rows[0].CorrectedBy != "A001 Crew" {
		t.Fatalf("corrected row: %+v", rows[0])
	}
	logged, _ := f.db.Read().SearchAudit(f.ctx, store.SearchAuditParams{ActionPrefix: "attendance.corrected", RowLimit: 5})
	if len(logged) != 1 || logged[0].Reason.String != "Lupa absen pulang" {
		t.Fatalf("correction audit entries: %+v", logged)
	}
}

// BR-HR-02: shifts cannot overlap for one Crew member; a worked shift cannot be deleted.
func TestShiftRules(t *testing.T) {
	f := setup(t)
	id := f.shift(t, f.w1, t0, 8)
	if _, err := f.svc.SaveShift(f.ctx, f.admin, "", hris.ShiftInput{CrewID: f.w1.ID, StartsAt: platform.Millis(t0.Add(7 * time.Hour)),
		EndsAt: platform.Millis(t0.Add(12 * time.Hour))}); code(err) != "shift_overlap" {
		t.Fatalf("overlap: %v", err)
	}
	if _, err := f.svc.SaveShift(f.ctx, f.admin, "", hris.ShiftInput{CrewID: f.w2.ID, StartsAt: platform.Millis(t0), EndsAt: platform.Millis(t0.Add(17 * time.Hour))}); code(err) != "invalid_input" {
		t.Fatalf("17-hour shift: %v", err)
	}
	_, _ = f.svc.Clock(f.ctx, hris.ClockInput{CrewID: "W001", PIN: "333333"})
	if err := f.svc.DeleteShift(f.ctx, f.admin, id); code(err) != "shift_worked" {
		t.Fatalf("deleting a worked shift: %v", err)
	}
}

// BR-HR-04: Crew request leave; others decide; a rejection needs a reason; approved leave shows
// on the schedule and the attendance board.
func TestLeave(t *testing.T) {
	f := setup(t)
	id, err := f.svc.RequestLeave(f.ctx, f.w1, hris.LeaveInput{FromDate: "2026-11-02", ToDate: "2026-11-03", Type: "annual", Reason: "Acara keluarga"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RequestLeave(f.ctx, f.w1, hris.LeaveInput{FromDate: "2026-10-20", ToDate: "2026-10-20", Type: "annual"}); code(err) != "invalid_input" {
		t.Fatalf("annual leave in the past: %v", err)
	}
	own, _ := f.svc.RequestLeave(f.ctx, f.admin, hris.LeaveInput{FromDate: "2026-11-05", ToDate: "2026-11-05", Type: "annual"})
	if err := f.svc.DecideLeave(f.ctx, f.admin, own, true, ""); code(err) != "own_leave" {
		t.Fatalf("deciding own leave: %v", err)
	}
	if err := f.svc.DecideLeave(f.ctx, f.admin, id, false, ""); code(err) != "invalid_input" {
		t.Fatalf("rejection without reason: %v", err)
	}
	if err := f.svc.DecideLeave(f.ctx, f.admin, id, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DecideLeave(f.ctx, f.mgmt, id, false, "x"); code(err) != "leave_decided" {
		t.Fatalf("deciding twice: %v", err)
	}
	mine, _ := f.svc.Leave(f.ctx, f.w1)
	all, _ := f.svc.Leave(f.ctx, f.admin)
	if len(mine) != 1 || mine[0].Status != "approved" || len(all) != 2 {
		t.Fatalf("leave lists: mine=%d all=%d", len(mine), len(all))
	}
	f.shift(t, f.w1, time.Date(2026, 11, 2, 8, 0, 0, 0, platform.Jakarta), 8)
	f.clock.Advance(7 * 24 * time.Hour)
	rows, _ := f.svc.Attendance(f.ctx, "2026-11-02", "2026-11-02")
	sch, _ := f.svc.Schedule(f.ctx, "2026-11-01", "2026-11-07")
	if len(rows) != 1 || rows[0].Status != "leave" || len(sch.Leave) != 1 {
		t.Fatalf("leave on the board: %+v, schedule leave %d", rows, len(sch.Leave))
	}
}

// BR-HR-05, Phase 7 done-when: the export has one row per seeded shift with its clock times.
func TestExportMatchesShifts(t *testing.T) {
	f := setup(t)
	f.shift(t, f.w1, t0, 8)
	f.shift(t, f.w2, t0.Add(6*time.Hour), 8)
	_, _ = f.svc.Clock(f.ctx, hris.ClockInput{CrewID: "W001", PIN: "333333"})
	f.clock.Advance(8 * time.Hour)
	_, _ = f.svc.Clock(f.ctx, hris.ClockInput{CrewID: "W001", PIN: "333333"})
	f.clock.Advance(24 * time.Hour)
	var buf bytes.Buffer
	if err := f.svc.ExportCSV(f.ctx, &buf, "2026-10-27", "2026-10-27"); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[1][0] != "W001" || rows[1][5] != "2026-10-27 08:00" || rows[1][6] != "2026-10-27 16:00" || rows[1][7] != "480" ||
		rows[2][0] != "W002" || rows[2][9] != "absent" {
		t.Fatalf("export: %v", rows)
	}
}

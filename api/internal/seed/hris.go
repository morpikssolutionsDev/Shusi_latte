package seed

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/hris"
	"sushilatte/api/internal/platform"
)

// HRIS at T0 (DEMO-AND-SEED §8): employee records; shifts for the past week, today and the next
// 7 days; attendance played through the clock-in pad; K002's auto clock-out yesterday; leave
// requests. Runs before any Bill exists, so past clock-outs meet no held cash or open Claims.

// Today's morning team carries the live story; W004 is on it but has not clocked in (late).
var (
	morningTeam = []string{"A001", "W001", "W002", "W003", "W004", "C001", "K001", "K003"}
	eveningTeam = []string{"W005", "C002", "K002"}
)

var employeeDetails = map[string]hris.EmployeeInput{
	"M001": {Phone: "0812-1100-2301", EmploymentType: "full_time", JoinedOn: "2023-02-01", EmergencyContact: "Lina (istri) 0812-1100-2302"},
	"A001": {Phone: "0813-2200-4410", EmploymentType: "full_time", JoinedOn: "2023-06-12", EmergencyContact: "Joni (ayah) 0813-2200-4411"},
	"W001": {Phone: "0857-3100-1201", EmploymentType: "full_time", JoinedOn: "2024-01-08"},
	"W002": {Phone: "0857-3100-1202", EmploymentType: "full_time", JoinedOn: "2024-03-18"},
	"W003": {Phone: "0857-3100-1203", EmploymentType: "full_time", JoinedOn: "2024-07-01"},
	"W004": {Phone: "0857-3100-1204", EmploymentType: "part_time", JoinedOn: "2025-02-10", Notes: "Kuliah sore Senin–Rabu"},
	"W005": {Phone: "0857-3100-1205", EmploymentType: "intern", JoinedOn: "2026-08-04", Notes: "Magang 6 bulan"},
	"C001": {Phone: "0821-4400-3301", EmploymentType: "full_time", JoinedOn: "2023-09-04"},
	"C002": {Phone: "0821-4400-3302", EmploymentType: "part_time", JoinedOn: "2025-05-19"},
	"K001": {Phone: "0838-5500-7701", EmploymentType: "full_time", JoinedOn: "2023-02-01", Notes: "Kepala dapur"},
	"K002": {Phone: "0838-5500-7702", EmploymentType: "full_time", JoinedOn: "2024-11-11"},
	"K003": {Phone: "0838-5500-7703", EmploymentType: "full_time", JoinedOn: "2024-04-22", Notes: "Barista"},
}

func seedHRIS(ctx context.Context, db *dbx.DB, opts Options, crewIDs map[string]string) error {
	t0 := opts.T0.In(platform.Jakarta)
	base := hris.NewService(db, fixedClock{t0}, auth.NewService(db, fixedClock{t0}))
	at := func(t time.Time) *hris.Service { return base.WithClock(fixedClock{t}) }
	principal := func(code string) *auth.Principal {
		for _, a := range DemoCrew {
			if a.CrewID == code {
				return &auth.Principal{ID: crewIDs[code], CrewID: code, Name: a.Name, Role: a.Role, Stations: a.Stations, Kind: "crew"}
			}
		}
		return nil
	}
	mgmt, admin := principal("M001"), principal("A001")
	rng := rand.New(rand.NewPCG(Rand, 8))

	for code, e := range employeeDetails {
		if err := base.SaveEmployee(ctx, mgmt, crewIDs[code], e); err != nil {
			return fmt.Errorf("employee %s: %w", code, err)
		}
	}

	// Leave (§8): K001 approved next Thursday, W004 pending next Monday–Tuesday, C002 rejected.
	midnight := time.Date(t0.Year(), t0.Month(), t0.Day(), 0, 0, 0, 0, platform.Jakarta)
	next := func(day time.Weekday) time.Time {
		d := midnight.AddDate(0, 0, 1)
		for d.Weekday() != day {
			d = d.AddDate(0, 0, 1)
		}
		return d
	}
	date := func(t time.Time) string { return t.Format(time.DateOnly) }
	asked := midnight.AddDate(0, 0, -3).Add(10 * time.Hour)
	for _, l := range []struct {
		code     string
		from, to time.Time
		reason   string
		approve  *bool
		decision string
	}{
		{"K001", next(time.Thursday), next(time.Thursday), "Urusan keluarga", new(true), ""},
		{"W004", next(time.Monday), next(time.Monday).AddDate(0, 0, 1), "Ujian kampus", nil, ""},
		{"C002", next(time.Saturday), next(time.Saturday), "Acara pernikahan saudara", new(false), "Jadwal penuh"},
	} {
		id, err := at(asked).RequestLeave(ctx, principal(l.code), hris.LeaveInput{FromDate: date(l.from), ToDate: date(l.to), Type: "annual", Reason: l.reason})
		if err != nil {
			return fmt.Errorf("leave %s: %w", l.code, err)
		}
		if l.approve != nil {
			if err := at(asked.Add(2*time.Hour)).DecideLeave(ctx, admin, id, *l.approve, l.decision); err != nil {
				return fmt.Errorf("decide leave %s: %w", l.code, err)
			}
		}
	}
	onLeave := map[string]string{"K001": date(next(time.Thursday))}

	// Shifts: today's around T0, other days Pagi 08.00–16.00 and Malam 15.00–23.00.
	type planned struct {
		code       string
		start, end time.Time
		today      bool // starts the evening before when T0 is after midnight
	}
	var plan []planned
	todayStart := t0.Truncate(time.Hour).Add(-7 * time.Hour)
	for d := -7; d <= 7; d++ {
		day := midnight.AddDate(0, 0, d)
		pagi, malam := day.Add(8*time.Hour), day.Add(15*time.Hour)
		if d == 0 {
			pagi, malam = todayStart, todayStart.Add(8*time.Hour)
		}
		for i, code := range append(slices.Clone(morningTeam), eveningTeam...) {
			// One day off a week each, but everyone works today and K002 worked last night.
			if d != 0 && !(d == -1 && code == "K002") && (day.YearDay()+i)%7 == 0 {
				continue
			}
			if onLeave[code] == date(day) {
				continue
			}
			start, hours := pagi, 8*time.Hour
			if d == 0 {
				hours = 9 * time.Hour // today's morning shift covers the whole live story (T0 −7h …)
			}
			if i >= len(morningTeam) {
				start, hours = malam, 8*time.Hour
			}
			plan = append(plan, planned{code, start, start.Add(hours), d == 0})
		}
	}
	for _, p := range plan {
		if _, err := at(midnight.AddDate(0, 0, -8)).SaveShift(ctx, admin, "", hris.ShiftInput{CrewID: crewIDs[p.code],
			StartsAt: platform.Millis(p.start), EndsAt: platform.Millis(p.end)}); err != nil {
			return fmt.Errorf("shift %s %s: %w", p.code, p.start, err)
		}
	}

	// Attendance for the past week, played on the pad: most arrive a few minutes early, ~10 % late.
	// The seed identifies the Crew member directly; the pad's PIN check is not what is shown here.
	clock := func(code string, t time.Time) error {
		c, err := db.Read().GetCrew(ctx, crewIDs[code])
		if err != nil {
			return err
		}
		_, err = at(t).ClockCrew(ctx, c, false)
		return err
	}
	for _, p := range plan {
		if p.today || !p.start.Before(midnight) {
			continue
		}
		in := p.start.Add(-time.Duration(1+rng.IntN(10)) * time.Minute)
		if rng.IntN(100) < 10 {
			in = p.start.Add(time.Duration(6+rng.IntN(15)) * time.Minute)
		}
		if err := clock(p.code, in); err != nil {
			return fmt.Errorf("clock in %s: %w", p.code, err)
		}
		// K002 forgot to clock out last night (G-22).
		if p.code == "K002" && !p.start.Before(midnight.AddDate(0, 0, -1)) {
			continue
		}
		if err := clock(p.code, p.end.Add(time.Duration(rng.IntN(15))*time.Minute)); err != nil {
			return fmt.Errorf("clock out %s: %w", p.code, err)
		}
	}
	if sweep := midnight.Add(time.Hour + time.Minute); sweep.Before(t0) { // last night's 23.00 + 2 h
		if _, err := at(sweep).AutoClockOut(ctx); err != nil {
			return err
		}
	}

	// Today: the morning team is in, W003 a little late, W004 not yet.
	for i, code := range morningTeam {
		if code == "W004" {
			continue
		}
		in := todayStart.Add(-time.Duration(2+i) * time.Minute)
		if code == "W003" {
			in = todayStart.Add(7 * time.Minute)
		}
		if err := clock(code, in); err != nil {
			return fmt.Errorf("clock in %s today: %w", code, err)
		}
	}
	return nil
}

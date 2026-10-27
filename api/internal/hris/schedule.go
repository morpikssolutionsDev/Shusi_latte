package hris

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/store"
)

// ---------------------------------------------------------------- shifts (BR-HR-02)

type ShiftView struct {
	ID       string `json:"id"`
	CrewID   string `json:"crewId"` // crew.id
	Code     string `json:"code"`
	Name     string `json:"name"`
	StartsAt int64  `json:"startsAt"`
	EndsAt   int64  `json:"endsAt"`
}

type ShiftInput struct {
	CrewID   string `json:"crewId"` // crew.id
	StartsAt int64  `json:"startsAt"`
	EndsAt   int64  `json:"endsAt"`
}

type Schedule struct {
	From   string        `json:"from"`
	To     string        `json:"to"`
	Crew   []RosterEntry `json:"crew"` // active Crew, the rows of the schedule
	Shifts []ShiftView   `json:"shifts"`
	Leave  []LeaveView   `json:"leave"` // approved leave in the range: the schedule shows the absence
}

type RosterEntry struct {
	ID   string `json:"id"` // crew.id
	Code string `json:"code"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// dayRange turns calendar dates (WIB) into [since, until) milliseconds, until exclusive.
func dayRange(from, to string) (int64, int64, error) {
	f, err := time.ParseInLocation(dateLayout, from, platform.Jakarta)
	if err != nil {
		return 0, 0, invalid("Tanggal tidak valid.")
	}
	t, err := time.ParseInLocation(dateLayout, to, platform.Jakarta)
	if err != nil || t.Before(f) || t.Sub(f) > 62*24*time.Hour {
		return 0, 0, invalid("Rentang tanggal tidak valid.")
	}
	return platform.Millis(f), platform.Millis(t.AddDate(0, 0, 1)), nil
}

func (s *Service) Schedule(ctx context.Context, from, to string) (Schedule, error) {
	since, until, err := dayRange(from, to)
	if err != nil {
		return Schedule{}, err
	}
	people, err := s.people(ctx)
	if err != nil {
		return Schedule{}, err
	}
	q := s.db.Read()
	shifts, err := q.ListShiftsBetween(ctx, store.ListShiftsBetweenParams{Since: since, Until: until})
	if err != nil {
		return Schedule{}, err
	}
	out := Schedule{From: from, To: to, Crew: []RosterEntry{}, Shifts: make([]ShiftView, 0, len(shifts)), Leave: []LeaveView{}}
	crew, err := q.ListAllCrew(ctx)
	if err != nil {
		return Schedule{}, err
	}
	for _, c := range crew {
		if c.Active == 1 {
			out.Crew = append(out.Crew, RosterEntry{ID: c.ID, Code: c.CrewID, Name: c.FullName, Role: c.Role})
		}
	}
	for _, sh := range shifts {
		out.Shifts = append(out.Shifts, shiftView(sh, people))
	}
	leave, err := q.ListApprovedLeaveBetween(ctx, store.ListApprovedLeaveBetweenParams{FromDate: from, ToDate: to})
	if err != nil {
		return Schedule{}, err
	}
	for _, l := range leave {
		out.Leave = append(out.Leave, leaveView(l, people))
	}
	return out, nil
}

func shiftView(sh store.Shift, people map[string]person) ShiftView {
	return ShiftView{ID: sh.ID, CrewID: sh.CrewID, Code: people[sh.CrewID].Code, Name: people[sh.CrewID].Name, StartsAt: sh.StartsAt, EndsAt: sh.EndsAt}
}

// SaveShift creates (id "") or updates a shift: at most 16 hours, no overlap with the same Crew
// member's other shifts. Audit-logged.
func (s *Service) SaveShift(ctx context.Context, p *auth.Principal, id string, in ShiftInput) (string, error) {
	if in.EndsAt <= in.StartsAt {
		return "", invalid("Jam selesai harus setelah jam mulai.")
	}
	if time.Duration(in.EndsAt-in.StartsAt)*time.Millisecond > maxShift {
		return "", invalid("Satu shift paling lama 16 jam.")
	}
	now := s.clock.Now()
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		c, err := tx.GetCrew(ctx, in.CrewID)
		if err != nil || c.Active == 0 {
			return invalid("Pilih crew yang aktif.")
		}
		n, err := tx.CountOverlappingShifts(ctx, store.CountOverlappingShiftsParams{CrewID: in.CrewID, ID: id, StartsAt: in.StartsAt, EndsAt: in.EndsAt})
		if err != nil {
			return err
		}
		if n > 0 {
			return conflict("shift_overlap", c.FullName+" sudah punya shift di jam itu")
		}
		action := "shift.updated"
		if id == "" {
			id, action = platform.NewID(), "shift.created"
			if err := tx.CreateShift(ctx, store.CreateShiftParams{ID: id, CrewID: in.CrewID, StartsAt: in.StartsAt, EndsAt: in.EndsAt,
				CreatedBy: p.ID, CreatedAt: platform.Millis(now)}); err != nil {
				return err
			}
		} else if n, err := tx.UpdateShift(ctx, store.UpdateShiftParams{CrewID: in.CrewID, StartsAt: in.StartsAt, EndsAt: in.EndsAt, ID: id}); err != nil {
			return err
		} else if n == 0 {
			return httpx.NewProblem(http.StatusNotFound, "not_found", "Shift tidak ditemukan")
		}
		publish(tx, action, id)
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: action, Entity: "shift", EntityID: id, After: in})
	})
	return id, err
}

// DeleteShift removes a shift nobody has clocked in against.
func (s *Service) DeleteShift(ctx context.Context, p *auth.Principal, id string) error {
	now := s.clock.Now()
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		sh, err := tx.GetShift(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return httpx.NewProblem(http.StatusNotFound, "not_found", "Shift tidak ditemukan")
		}
		if err != nil {
			return err
		}
		if n, err := tx.CountShiftAttendance(ctx, nullStr(id)); err != nil {
			return err
		} else if n > 0 {
			return conflict("shift_worked", "Shift ini sudah punya absensi; koreksi absensinya saja")
		}
		if _, err := tx.DeleteShift(ctx, id); err != nil {
			return err
		}
		publish(tx, "shift.deleted", id)
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "shift.deleted", Entity: "shift", EntityID: id,
			Before: ShiftInput{CrewID: sh.CrewID, StartsAt: sh.StartsAt, EndsAt: sh.EndsAt}})
	})
}

// ---------------------------------------------------------------- leave (BR-HR-04)

type LeaveView struct {
	ID             string `json:"id"`
	CrewID         string `json:"crewId"` // crew.id
	Code           string `json:"code"`
	Name           string `json:"name"`
	FromDate       string `json:"fromDate"`
	ToDate         string `json:"toDate"`
	Type           string `json:"type"` // annual | sick | unpaid
	Reason         string `json:"reason"`
	Status         string `json:"status"` // pending | approved | rejected
	DecidedBy      string `json:"decidedBy,omitempty"`
	DecidedAt      *int64 `json:"decidedAt,omitempty"`
	DecisionReason string `json:"decisionReason,omitempty"`
	CreatedAt      int64  `json:"createdAt"`
}

func leaveView(l store.LeaveRequest, people map[string]person) LeaveView {
	return LeaveView{ID: l.ID, CrewID: l.CrewID, Code: people[l.CrewID].Code, Name: people[l.CrewID].Name, FromDate: l.FromDate, ToDate: l.ToDate,
		Type: l.Type, Reason: l.Reason, Status: l.Status, DecidedBy: people[l.DecidedBy.String].Name, DecidedAt: ptr(l.DecidedAt),
		DecisionReason: l.DecisionReason.String, CreatedAt: l.CreatedAt}
}

type LeaveInput struct {
	FromDate string `json:"fromDate"`
	ToDate   string `json:"toDate"`
	Type     string `json:"type"`
	Reason   string `json:"reason"`
}

// RequestLeave records a Crew member's own leave request, pending a decision.
func (s *Service) RequestLeave(ctx context.Context, p *auth.Principal, in LeaveInput) (string, error) {
	f, err1 := time.Parse(dateLayout, in.FromDate)
	t, err2 := time.Parse(dateLayout, in.ToDate)
	if err1 != nil || err2 != nil || t.Before(f) {
		return "", invalid("Pilih tanggal mulai dan selesai.")
	}
	if t.Sub(f) >= maxLeaveDays*24*time.Hour {
		return "", invalid("Satu pengajuan cuti paling lama 30 hari.")
	}
	if !slices.Contains([]string{"annual", "sick", "unpaid"}, in.Type) {
		return "", invalid("Pilih jenis cuti.")
	}
	now := s.clock.Now()
	today := now.In(platform.Jakarta).Format(dateLayout)
	if in.Type != "sick" && in.FromDate < today {
		return "", invalid("Cuti tahunan dan tanpa gaji diajukan sebelum harinya.")
	}
	id := platform.NewID()
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		if err := tx.CreateLeaveRequest(ctx, store.CreateLeaveRequestParams{ID: id, CrewID: p.ID, FromDate: in.FromDate, ToDate: in.ToDate,
			Type: in.Type, Reason: strings.TrimSpace(in.Reason), CreatedAt: platform.Millis(now)}); err != nil {
			return err
		}
		publish(tx, "leave.requested", id)
		return nil
	})
	return id, err
}

// Leave lists every request for Outlet Admin and Management, and a Crew member's own otherwise.
func (s *Service) Leave(ctx context.Context, p *auth.Principal) ([]LeaveView, error) {
	people, err := s.people(ctx)
	if err != nil {
		return nil, err
	}
	var rows []store.LeaveRequest
	if p.HasRole(auth.RoleManagement, auth.RoleOutletAdmin) {
		rows, err = s.db.Read().ListLeaveRequests(ctx)
	} else {
		rows, err = s.db.Read().ListLeaveRequestsByCrew(ctx, p.ID)
	}
	if err != nil {
		return nil, err
	}
	out := make([]LeaveView, 0, len(rows))
	for _, l := range rows {
		out = append(out, leaveView(l, people))
	}
	return out, nil
}

// DecideLeave approves or rejects a pending request; a rejection needs a reason, and nobody
// decides their own request. Audit-logged.
func (s *Service) DecideLeave(ctx context.Context, p *auth.Principal, id string, approve bool, reason string) error {
	reason = strings.TrimSpace(reason)
	if !approve && reason == "" {
		return invalid("Tulis alasan penolakan.")
	}
	status := "rejected"
	if approve {
		status = "approved"
	}
	now := s.clock.Now()
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		l, err := tx.GetLeaveRequest(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return httpx.NewProblem(http.StatusNotFound, "not_found", "Pengajuan cuti tidak ditemukan")
		}
		if err != nil {
			return err
		}
		if l.CrewID == p.ID {
			return httpx.NewProblem(http.StatusForbidden, "own_leave", "Pengajuan cuti sendiri diputuskan orang lain")
		}
		if n, err := tx.DecideLeaveRequest(ctx, store.DecideLeaveRequestParams{Status: status, DecidedBy: nullStr(p.ID), DecidedAt: nullMs(platform.Millis(now)),
			DecisionReason: nullStr(reason), ID: id}); err != nil {
			return err
		} else if n == 0 {
			return conflict("leave_decided", "Pengajuan ini sudah diputuskan")
		}
		publish(tx, "leave."+status, id)
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "leave." + status, Entity: "leave_request", EntityID: id, Reason: reason})
	})
}

// ---------------------------------------------------------------- employees (BR-HR-01)

type Employee struct {
	ID               string `json:"id"` // crew.id
	CrewID           string `json:"crewId"`
	FullName         string `json:"fullName"`
	Role             string `json:"role"`
	ImageKey         string `json:"imageKey,omitempty"`
	Active           bool   `json:"active"`
	Phone            string `json:"phone"`
	EmploymentType   string `json:"employmentType"` // full_time | part_time | intern
	JoinedOn         string `json:"joinedOn,omitempty"`
	EmergencyContact string `json:"emergencyContact"`
	Notes            string `json:"notes"`
}

type EmployeeInput struct {
	Phone            string `json:"phone"`
	EmploymentType   string `json:"employmentType"`
	JoinedOn         string `json:"joinedOn"`
	EmergencyContact string `json:"emergencyContact"`
	Notes            string `json:"notes"`
}

func (s *Service) Employees(ctx context.Context) ([]Employee, error) {
	rows, err := s.db.Read().ListEmployees(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Employee, 0, len(rows))
	for _, r := range rows {
		out = append(out, Employee{ID: r.ID, CrewID: r.CrewID, FullName: r.FullName, Role: r.Role, ImageKey: r.ImageKey.String, Active: r.Active == 1,
			Phone: r.Phone, EmploymentType: r.EmploymentType, JoinedOn: r.JoinedOn.String, EmergencyContact: r.EmergencyContact, Notes: r.Notes})
	}
	return out, nil
}

// SaveEmployee updates the employee details of a Crew member; audit-logged.
func (s *Service) SaveEmployee(ctx context.Context, p *auth.Principal, crewID string, in EmployeeInput) error {
	if !slices.Contains([]string{"full_time", "part_time", "intern"}, in.EmploymentType) {
		return invalid("Pilih jenis kepegawaian.")
	}
	if in.JoinedOn != "" {
		if _, err := time.Parse(dateLayout, in.JoinedOn); err != nil {
			return invalid("Tanggal masuk tidak valid.")
		}
	}
	now := s.clock.Now()
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		if _, err := tx.GetCrew(ctx, crewID); err != nil {
			return httpx.NewProblem(http.StatusNotFound, "not_found", "Crew tidak ditemukan")
		}
		if err := tx.UpsertEmployee(ctx, store.UpsertEmployeeParams{CrewID: crewID, Phone: strings.TrimSpace(in.Phone), EmploymentType: in.EmploymentType,
			JoinedOn: nullStr(in.JoinedOn), EmergencyContact: strings.TrimSpace(in.EmergencyContact), Notes: strings.TrimSpace(in.Notes),
			UpdatedAt: platform.Millis(now)}); err != nil {
			return err
		}
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "employee.updated", Entity: "crew", EntityID: crewID, After: in})
	})
}

// ---------------------------------------------------------------- attendance board and export (BR-HR-05)

// AttendanceRow is one Crew member's day: their shift, their clock-in and clock-out, or leave.
type AttendanceRow struct {
	CrewID           string     `json:"crewId"` // crew.id
	Code             string     `json:"code"`
	Name             string     `json:"name"`
	Date             string     `json:"date"`
	Shift            *ShiftTime `json:"shift,omitempty"`
	AttendanceID     string     `json:"attendanceId,omitempty"`
	ClockInAt        *int64     `json:"clockInAt,omitempty"`
	ClockOutAt       *int64     `json:"clockOutAt,omitempty"`
	Auto             bool       `json:"auto"`
	CorrectedBy      string     `json:"correctedBy,omitempty"`
	CorrectionReason string     `json:"correctionReason,omitempty"`
	LateMinutes      int64      `json:"lateMinutes"`
	WorkedMinutes    int64      `json:"workedMinutes"`
	// Status: scheduled (not started) · late (started, not in) · absent (ended, never in) ·
	// working · done · auto (auto clock-out, not yet corrected) · leave
	Status string `json:"status"`
}

// Attendance lists every shift and attendance record in a date range (WIB calendar days),
// matched shift by shift, with approved leave.
func (s *Service) Attendance(ctx context.Context, from, to string) ([]AttendanceRow, error) {
	since, until, err := dayRange(from, to)
	if err != nil {
		return nil, err
	}
	people, err := s.people(ctx)
	if err != nil {
		return nil, err
	}
	q := s.db.Read()
	shifts, err := q.ListShiftsBetween(ctx, store.ListShiftsBetweenParams{Since: since, Until: until})
	if err != nil {
		return nil, err
	}
	records, err := q.ListAttendanceBetween(ctx, store.ListAttendanceBetweenParams{Since: since - earlyClockIn.Milliseconds(), Until: until})
	if err != nil {
		return nil, err
	}
	leave, err := q.ListApprovedLeaveBetween(ctx, store.ListApprovedLeaveBetweenParams{FromDate: from, ToDate: to})
	if err != nil {
		return nil, err
	}
	now := platform.Millis(s.clock.Now())
	onLeave := func(crewID, date string) bool {
		return slices.ContainsFunc(leave, func(l store.LeaveRequest) bool { return l.CrewID == crewID && l.FromDate <= date && date <= l.ToDate })
	}
	date := func(ms int64) string { return time.UnixMilli(ms).In(platform.Jakarta).Format(dateLayout) }
	used := map[string]bool{}
	var out []AttendanceRow
	add := func(r AttendanceRow, a *store.Attendance) {
		r.Code, r.Name = people[r.CrewID].Code, people[r.CrewID].Name
		if a != nil {
			used[a.ID] = true
			r.AttendanceID, r.ClockInAt, r.ClockOutAt = a.ID, &a.ClockInAt, ptr(a.ClockOutAt)
			r.Auto = a.AutoClockedOut == 1
			r.CorrectedBy, r.CorrectionReason = people[a.CorrectedBy.String].Name, a.CorrectionReason.String
			end := now
			if a.ClockOutAt.Valid {
				end = a.ClockOutAt.Int64
			}
			r.WorkedMinutes = (end - a.ClockInAt) / 60000
			if r.Shift != nil && a.ClockInAt-r.Shift.StartsAt > lateAfter.Milliseconds() {
				r.LateMinutes = (a.ClockInAt - r.Shift.StartsAt) / 60000
			}
			switch {
			case !a.ClockOutAt.Valid:
				r.Status = "working"
			case r.Auto && !a.CorrectedBy.Valid:
				r.Status = "auto"
			default:
				r.Status = "done"
			}
		} else {
			switch {
			case onLeave(r.CrewID, r.Date):
				r.Status = "leave"
			case now < r.Shift.StartsAt:
				r.Status = "scheduled"
			case now < r.Shift.EndsAt:
				r.Status = "late"
				r.LateMinutes = (now - r.Shift.StartsAt) / 60000
			default:
				r.Status = "absent"
			}
		}
		out = append(out, r)
	}
	for _, sh := range shifts {
		if sh.StartsAt < since {
			continue // started the day before the range
		}
		r := AttendanceRow{CrewID: sh.CrewID, Date: date(sh.StartsAt), Shift: &ShiftTime{sh.StartsAt, sh.EndsAt}}
		i := slices.IndexFunc(records, func(a store.Attendance) bool { return a.ShiftID.String == sh.ID })
		if i >= 0 {
			add(r, &records[i])
		} else {
			add(r, nil)
		}
	}
	for i, a := range records {
		if used[a.ID] || a.ClockInAt < since {
			continue
		}
		add(AttendanceRow{CrewID: a.CrewID, Date: date(a.ClockInAt)}, &records[i])
	}
	slices.SortStableFunc(out, func(a, b AttendanceRow) int {
		if c := strings.Compare(a.Date, b.Date); c != 0 {
			return c
		}
		return strings.Compare(a.Code, b.Code)
	})
	if out == nil {
		out = []AttendanceRow{}
	}
	return out, nil
}

// ExportCSV writes the attendance of a date range for external payroll (BR-HR-05): one row per
// shift or unscheduled clock-in, times in WIB.
func (s *Service) ExportCSV(ctx context.Context, w io.Writer, from, to string) error {
	rows, err := s.Attendance(ctx, from, to)
	if err != nil {
		return err
	}
	clock := func(ms *int64) string {
		if ms == nil {
			return ""
		}
		return time.UnixMilli(*ms).In(platform.Jakarta).Format("2006-01-02 15:04")
	}
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"crew_id", "name", "date", "shift_start", "shift_end", "clock_in", "clock_out", "worked_minutes", "late_minutes",
		"status", "auto_clock_out", "corrected_by", "correction_reason"})
	for _, r := range rows {
		var start, end string
		if r.Shift != nil {
			start, end = clock(&r.Shift.StartsAt), clock(&r.Shift.EndsAt)
		}
		_ = cw.Write([]string{r.Code, r.Name, r.Date, start, end, clock(r.ClockInAt), clock(r.ClockOutAt), strconv.FormatInt(r.WorkedMinutes, 10),
			strconv.FormatInt(r.LateMinutes, 10), r.Status, strconv.FormatBool(r.Auto), r.CorrectedBy, r.CorrectionReason})
	}
	cw.Flush()
	return cw.Error()
}

// Me is a Crew member's own HRIS view: open attendance, upcoming shifts and their leave.
type Me struct {
	ClockedInAt *int64      `json:"clockedInAt,omitempty"`
	Shifts      []ShiftView `json:"shifts"`
	Leave       []LeaveView `json:"leave"`
	HeldCash    int64       `json:"heldCash"`
}

func (s *Service) Me(ctx context.Context, p *auth.Principal) (Me, error) {
	now := s.clock.Now()
	q := s.db.Read()
	people, err := s.people(ctx)
	if err != nil {
		return Me{}, err
	}
	out := Me{Shifts: []ShiftView{}}
	if a, err := q.GetOpenAttendance(ctx, p.ID); err == nil {
		out.ClockedInAt = &a.ClockInAt
	}
	shifts, err := q.ListShiftsBetween(ctx, store.ListShiftsBetweenParams{Since: platform.Millis(now.Add(-12 * time.Hour)), Until: platform.Millis(now.AddDate(0, 0, 14))})
	if err != nil {
		return Me{}, err
	}
	for _, sh := range shifts {
		if sh.CrewID == p.ID {
			out.Shifts = append(out.Shifts, shiftView(sh, people))
		}
	}
	if out.Leave, err = s.Leave(ctx, &auth.Principal{ID: p.ID, Kind: "crew", Role: auth.RoleCrew}); err != nil {
		return Me{}, err
	}
	held, err := q.ListHeldPayments(ctx, p.ID)
	if err != nil {
		return Me{}, err
	}
	for _, h := range held {
		out.HeldCash += h.Amount
	}
	return out, nil
}

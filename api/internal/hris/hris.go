package hris

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crew"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/ops"
)

// Package hris implements Phase 7 HRIS (BR-HR-01 to BR-HR-07).
// Crew records from Phase 1 are the identity records; employee
// rows extend them. Shift, attendance and leave rows store the
// internal crew id; the API layer accepts and returns the human
// Crew ID and resolves it through the crew store.

func NewID() string {
	return crew.NewID()
}

type Store struct {
	Actor string
	db    *sql.DB
	crew  *crew.Store
	ops   *ops.Store
	// HeldCash reports cash a Waiter still holds, by human Crew ID.
	HeldCash func(crewID string) int64
}

func NewStore(db *sql.DB, crewStore *crew.Store, opsStore *ops.Store) *Store {
	db.SetMaxOpenConns(1)
	return &Store{db: db, crew: crewStore, ops: opsStore}
}

func (s *Store) actor() string {
	if s.Actor != "" {
		return s.Actor
	}
	return "system"
}

func (s *Store) audit(action, entity, entityID, reason string) {
	_, _ = s.db.Exec(
		`INSERT INTO audit_log (id, at, actor_crew_id, override_crew_id, action, entity, entity_id, before_json, after_json, reason) VALUES (?, ?, ?, NULL, ?, ?, ?, NULL, NULL, ?)`,
		NewID(), time.Now().UnixMilli(), s.actor(), action, entity, entityID, nullString(reason),
	)
}

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (s *Store) resolveCrewID(humanCrewID string) (crew.Crew, error) {
	return s.crew.GetByCrewID(humanCrewID)
}

// Employee

type Employee struct {
	ID               string  `json:"id"`
	CrewID           string  `json:"crew_id"`
	FullName         string  `json:"full_name"`
	HireDate         *string `json:"hire_date"`
	Phone            *string `json:"phone"`
	EmergencyContact *string `json:"emergency_contact"`
	EmploymentType   *string `json:"employment_type"`
	Notes            *string `json:"notes"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        int64   `json:"updated_at"`
}

type EmployeeInput struct {
	CrewID           string  `json:"crew_id"`
	HireDate         *string `json:"hire_date"`
	Phone            *string `json:"phone"`
	EmergencyContact *string `json:"emergency_contact"`
	EmploymentType   *string `json:"employment_type"`
	Notes            *string `json:"notes"`
}

func (s *Store) CreateEmployee(in EmployeeInput) (Employee, error) {
	c, err := s.resolveCrewID(in.CrewID)
	if err != nil {
		return Employee{}, errors.New("crew not found")
	}
	now := time.Now().UnixMilli()
	emp := Employee{ID: NewID(), CrewID: c.CrewID, FullName: c.FullName, HireDate: in.HireDate, Phone: in.Phone, EmergencyContact: in.EmergencyContact, EmploymentType: in.EmploymentType, Notes: in.Notes, CreatedAt: now, UpdatedAt: now}
	if _, err := s.db.Exec(`INSERT INTO employee (id, crew_id, hire_date, phone, emergency_contact, employment_type, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, emp.ID, c.ID, strOrNull(in.HireDate), strOrNull(in.Phone), strOrNull(in.EmergencyContact), strOrNull(in.EmploymentType), strOrNull(in.Notes), now, now); err != nil {
		return Employee{}, err
	}
	return emp, nil
}

func strOrNull(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func (s *Store) ListEmployees() []Employee {
	rows, err := s.db.Query(`SELECT e.id, e.crew_id, e.hire_date, e.phone, e.emergency_contact, e.employment_type, e.notes, e.created_at, e.updated_at FROM employee e ORDER BY e.created_at`)
	if err != nil {
		return []Employee{}
	}
	type raw struct {
		e                                      Employee
		internalID                             string
		hire, phone, emergency, empType, notes sql.NullString
	}
	var raws []raw
	for rows.Next() {
		var r raw
		if err := rows.Scan(&r.e.ID, &r.internalID, &r.hire, &r.phone, &r.emergency, &r.empType, &r.notes, &r.e.CreatedAt, &r.e.UpdatedAt); err == nil {
			raws = append(raws, r)
		}
	}
	rows.Close()
	out := []Employee{}
	for _, r := range raws {
		e := r.e
		if c, err := s.crew.GetCrew(r.internalID); err == nil {
			e.CrewID = c.CrewID
			e.FullName = c.FullName
		}
		e.HireDate = nullStrPtr(r.hire)
		e.Phone = nullStrPtr(r.phone)
		e.EmergencyContact = nullStrPtr(r.emergency)
		e.EmploymentType = nullStrPtr(r.empType)
		e.Notes = nullStrPtr(r.notes)
		out = append(out, e)
	}
	return out
}

func nullStrPtr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	v := n.String
	return &v
}

func (s *Store) UpdateEmployee(id string, in EmployeeInput) (Employee, error) {
	now := time.Now().UnixMilli()
	if _, err := s.db.Exec(`UPDATE employee SET hire_date = ?, phone = ?, emergency_contact = ?, employment_type = ?, notes = ?, updated_at = ? WHERE id = ?`, strOrNull(in.HireDate), strOrNull(in.Phone), strOrNull(in.EmergencyContact), strOrNull(in.EmploymentType), strOrNull(in.Notes), now, id); err != nil {
		return Employee{}, err
	}
	for _, e := range s.ListEmployees() {
		if e.ID == id {
			return e, nil
		}
	}
	return Employee{}, errors.New("employee not found")
}

// Shift

type Shift struct {
	ID       string `json:"id"`
	CrewID   string `json:"crew_id"`
	StartsAt int64  `json:"starts_at"`
	EndsAt   int64  `json:"ends_at"`
}

type ShiftInput struct {
	CrewID   string `json:"crew_id"`
	StartsAt int64  `json:"starts_at"`
	EndsAt   int64  `json:"ends_at"`
}

func (s *Store) CreateShift(in ShiftInput) (Shift, error) {
	if in.EndsAt <= in.StartsAt {
		return Shift{}, errors.New("shift end must be after shift start")
	}
	c, err := s.resolveCrewID(in.CrewID)
	if err != nil {
		return Shift{}, errors.New("crew not found")
	}
	// A shift must not overlap another shift for the same crew.
	var overlap int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM shift WHERE crew_id = ? AND starts_at < ? AND ends_at > ?`, c.ID, in.EndsAt, in.StartsAt).Scan(&overlap)
	if overlap > 0 {
		return Shift{}, errors.New("shift overlaps an existing shift for this crew")
	}
	sh := Shift{ID: NewID(), CrewID: c.CrewID, StartsAt: in.StartsAt, EndsAt: in.EndsAt}
	if _, err := s.db.Exec(`INSERT INTO shift (id, crew_id, starts_at, ends_at) VALUES (?, ?, ?, ?)`, sh.ID, c.ID, in.StartsAt, in.EndsAt); err != nil {
		return Shift{}, err
	}
	return sh, nil
}

func (s *Store) ListShifts(humanCrewID string, from, to int64) []Shift {
	var rows *sql.Rows
	var err error
	if humanCrewID != "" {
		c, err := s.resolveCrewID(humanCrewID)
		if err != nil {
			return []Shift{}
		}
		rows, err = s.db.Query(`SELECT id, crew_id, starts_at, ends_at FROM shift WHERE crew_id = ? AND starts_at >= ? AND starts_at <= ? ORDER BY starts_at`, c.ID, from, to)
	} else {
		rows, err = s.db.Query(`SELECT id, crew_id, starts_at, ends_at FROM shift WHERE starts_at >= ? AND starts_at <= ? ORDER BY starts_at`, from, to)
	}
	if err != nil {
		return []Shift{}
	}
	type raw struct {
		sh         Shift
		internalID string
	}
	var raws []raw
	for rows.Next() {
		var r raw
		if err := rows.Scan(&r.sh.ID, &r.internalID, &r.sh.StartsAt, &r.sh.EndsAt); err == nil {
			raws = append(raws, r)
		}
	}
	rows.Close()
	out := []Shift{}
	for _, r := range raws {
		sh := r.sh
		if c, err := s.crew.GetCrew(r.internalID); err == nil {
			sh.CrewID = c.CrewID
		}
		out = append(out, sh)
	}
	return out
}

func (s *Store) DeleteShift(id string) error {
	res, err := s.db.Exec(`DELETE FROM shift WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errors.New("shift not found")
	}
	return nil
}

// Attendance

type Attendance struct {
	ID               string  `json:"id"`
	CrewID           string  `json:"crew_id"`
	ClockInAt        int64   `json:"clock_in_at"`
	ClockOutAt       *int64  `json:"clock_out_at"`
	AutoClockedOut   bool    `json:"auto_clocked_out"`
	CorrectedBy      *string `json:"corrected_by"`
	CorrectionReason *string `json:"correction_reason"`
}

// ClockInOut handles a clock-in or clock-out from the HRIS
// screen. PIN verification is done by the caller (auth handler
// or screen check) before this is called, except that this
// function re-checks the held-cash rule on clock-out.
func (s *Store) ClockIn(humanCrewID string) (Attendance, error) {
	c, err := s.resolveCrewID(humanCrewID)
	if err != nil {
		return Attendance{}, errors.New("crew not found")
	}
	if !c.Active {
		return Attendance{}, errors.New("crew account is inactive")
	}
	// Approved leave covering today (Jakarta) blocks clock-in.
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	today := time.Now().In(jakarta).Format("2006-01-02")
	var onLeave int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM leave_request WHERE crew_id = ? AND status = 'approved' AND from_date <= ? AND to_date >= ?`, c.ID, today, today).Scan(&onLeave)
	if onLeave > 0 {
		return Attendance{}, errors.New("crew is on approved leave today")
	}
	var open int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM attendance WHERE crew_id = ? AND clock_out_at IS NULL`, c.ID).Scan(&open)
	if open > 0 {
		return Attendance{}, errors.New("crew is already clocked in")
	}
	now := time.Now().UnixMilli()
	att := Attendance{ID: NewID(), CrewID: c.CrewID, ClockInAt: now}
	if _, err := s.db.Exec(`INSERT INTO attendance (id, crew_id, clock_in_at) VALUES (?, ?, ?)`, att.ID, c.ID, now); err != nil {
		return Attendance{}, err
	}
	return att, nil
}

func (s *Store) ClockOut(humanCrewID string) (Attendance, error) {
	c, err := s.resolveCrewID(humanCrewID)
	if err != nil {
		return Attendance{}, errors.New("crew not found")
	}
	// BR-HR-07: a Waiter cannot clock out while holding cash.
	if s.HeldCash != nil {
		if held := s.HeldCash(c.CrewID); held > 0 {
			return Attendance{}, fmt.Errorf("hand over Rp %d to the Cashier first", held)
		}
	}
	var id string
	var clockIn int64
	err = s.db.QueryRow(`SELECT id, clock_in_at FROM attendance WHERE crew_id = ? AND clock_out_at IS NULL ORDER BY clock_in_at DESC LIMIT 1`, c.ID).Scan(&id, &clockIn)
	if err != nil {
		return Attendance{}, errors.New("crew is not clocked in")
	}
	now := time.Now().UnixMilli()
	if _, err := s.db.Exec(`UPDATE attendance SET clock_out_at = ? WHERE id = ?`, now, id); err != nil {
		return Attendance{}, err
	}
	return Attendance{ID: id, CrewID: c.CrewID, ClockInAt: clockIn, ClockOutAt: &now}, nil
}

func (s *Store) ListAttendance(humanCrewID string, from, to int64) []Attendance {
	var rows *sql.Rows
	var err error
	if humanCrewID != "" {
		c, err := s.resolveCrewID(humanCrewID)
		if err != nil {
			return []Attendance{}
		}
		rows, err = s.db.Query(`SELECT id, crew_id, clock_in_at, clock_out_at, auto_clocked_out, corrected_by, correction_reason FROM attendance WHERE crew_id = ? AND clock_in_at >= ? AND clock_in_at <= ? ORDER BY clock_in_at`, c.ID, from, to)
	} else {
		rows, err = s.db.Query(`SELECT id, crew_id, clock_in_at, clock_out_at, auto_clocked_out, corrected_by, correction_reason FROM attendance WHERE clock_in_at >= ? AND clock_in_at <= ? ORDER BY clock_in_at`, from, to)
	}
	if err != nil {
		return []Attendance{}
	}
	type raw struct {
		a                Attendance
		internalID       string
		outAt            sql.NullInt64
		correctedBy      sql.NullString
		correctionReason sql.NullString
		auto             int
	}
	var raws []raw
	for rows.Next() {
		var r raw
		if err := rows.Scan(&r.a.ID, &r.internalID, &r.a.ClockInAt, &r.outAt, &r.auto, &r.correctedBy, &r.correctionReason); err == nil {
			raws = append(raws, r)
		}
	}
	rows.Close()
	out := []Attendance{}
	for _, r := range raws {
		a := r.a
		if c, err := s.crew.GetCrew(r.internalID); err == nil {
			a.CrewID = c.CrewID
		}
		if r.outAt.Valid {
			v := r.outAt.Int64
			a.ClockOutAt = &v
		}
		a.AutoClockedOut = r.auto == 1
		a.CorrectedBy = nullStrPtr(r.correctedBy)
		a.CorrectionReason = nullStrPtr(r.correctionReason)
		out = append(out, a)
	}
	return out
}

// CorrectAttendance corrects an attendance row with a reason,
// audit-logged (BR-HR-06, TR-AUD-01).
func (s *Store) CorrectAttendance(actorHumanCrewID, attendanceID string, clockInAt, clockOutAt *int64, reason string) (Attendance, error) {
	if reason == "" {
		return Attendance{}, errors.New("correction reason is required")
	}
	var internalID string
	var oldIn int64
	var oldOut sql.NullInt64
	err := s.db.QueryRow(`SELECT crew_id, clock_in_at, clock_out_at FROM attendance WHERE id = ?`, attendanceID).Scan(&internalID, &oldIn, &oldOut)
	if err != nil {
		return Attendance{}, errors.New("attendance not found")
	}
	newIn := oldIn
	if clockInAt != nil {
		newIn = *clockInAt
	}
	var newOut any
	if clockOutAt != nil {
		newOut = *clockOutAt
	} else if oldOut.Valid {
		newOut = oldOut.Int64
	}
	if _, err := s.db.Exec(`UPDATE attendance SET clock_in_at = ?, clock_out_at = ?, corrected_by = ?, correction_reason = ? WHERE id = ?`, newIn, newOut, actorHumanCrewID, reason, attendanceID); err != nil {
		return Attendance{}, err
	}
	s.audit("attendance_correction", "attendance", attendanceID, reason)
	for _, a := range s.ListAttendance("", 0, time.Now().UnixMilli()+86400000) {
		if a.ID == attendanceID {
			return a, nil
		}
	}
	return Attendance{}, errors.New("attendance not found after correction")
}

// AutoClockOut closes open attendance rows past shift end +
// grace (BR-HR-06). Grace defaults to the ops setting
// (7200 seconds) when the ops store is available.
func (s *Store) AutoClockOut() []Attendance {
	graceSeconds := int64(7200)
	if s.ops != nil {
		if st, err := s.ops.GetSettings(); err == nil {
			graceSeconds = int64(st.AutoClockOutGraceSeconds)
		}
	}
	now := time.Now().UnixMilli()
	graceMs := graceSeconds * 1000
	rows, err := s.db.Query(`SELECT id, crew_id, clock_in_at FROM attendance WHERE clock_out_at IS NULL`)
	if err != nil {
		return []Attendance{}
	}
	type openRow struct {
		id, internalID string
		clockIn        int64
	}
	var opens []openRow
	for rows.Next() {
		var o openRow
		if err := rows.Scan(&o.id, &o.internalID, &o.clockIn); err == nil {
			opens = append(opens, o)
		}
	}
	rows.Close()
	out := []Attendance{}
	for _, o := range opens {
		// Latest shift ending before now - grace.
		var shiftEnd int64
		err := s.db.QueryRow(`SELECT ends_at FROM shift WHERE crew_id = ? AND ends_at <= ? ORDER BY ends_at DESC LIMIT 1`, o.internalID, now-graceMs).Scan(&shiftEnd)
		if err != nil {
			continue
		}
		clockOut := shiftEnd + graceMs
		if _, err := s.db.Exec(`UPDATE attendance SET clock_out_at = ?, auto_clocked_out = 1 WHERE id = ? AND clock_out_at IS NULL`, clockOut, o.id); err != nil {
			continue
		}
		s.audit("auto_clock_out", "attendance", o.id, "past shift end + grace")
		if c, err := s.crew.GetCrew(o.internalID); err == nil {
			out = append(out, Attendance{ID: o.id, CrewID: c.CrewID, ClockInAt: o.clockIn, ClockOutAt: &clockOut, AutoClockedOut: true})
		}
	}
	return out
}

// Leave

type LeaveRequest struct {
	ID        string  `json:"id"`
	CrewID    string  `json:"crew_id"`
	FromDate  string  `json:"from_date"`
	ToDate    string  `json:"to_date"`
	Type      string  `json:"type"`
	Reason    *string `json:"reason"`
	Status    string  `json:"status"`
	DecidedBy *string `json:"decided_by"`
	DecidedAt *int64  `json:"decided_at"`
	CreatedAt int64   `json:"created_at"`
}

type LeaveInput struct {
	CrewID   string  `json:"crew_id"`
	FromDate string  `json:"from_date"`
	ToDate   string  `json:"to_date"`
	Type     string  `json:"type"`
	Reason   *string `json:"reason"`
}

func (s *Store) CreateLeave(in LeaveInput) (LeaveRequest, error) {
	if in.FromDate == "" || in.ToDate == "" || in.ToDate < in.FromDate {
		return LeaveRequest{}, errors.New("leave dates are invalid")
	}
	c, err := s.resolveCrewID(in.CrewID)
	if err != nil {
		return LeaveRequest{}, errors.New("crew not found")
	}
	now := time.Now().UnixMilli()
	lr := LeaveRequest{ID: NewID(), CrewID: c.CrewID, FromDate: in.FromDate, ToDate: in.ToDate, Type: in.Type, Reason: in.Reason, Status: "pending", CreatedAt: now}
	if _, err := s.db.Exec(`INSERT INTO leave_request (id, crew_id, from_date, to_date, type, reason, status, created_at) VALUES (?, ?, ?, ?, ?, ?, 'pending', ?)`, lr.ID, c.ID, in.FromDate, in.ToDate, in.Type, strOrNull(in.Reason), now); err != nil {
		return LeaveRequest{}, err
	}
	return lr, nil
}

func (s *Store) ListLeave(humanCrewID, status string) []LeaveRequest {
	var rows *sql.Rows
	var err error
	if humanCrewID != "" {
		c, err := s.resolveCrewID(humanCrewID)
		if err != nil {
			return []LeaveRequest{}
		}
		rows, err = s.db.Query(`SELECT id, crew_id, from_date, to_date, type, reason, status, decided_by, decided_at, created_at FROM leave_request WHERE crew_id = ? ORDER BY created_at DESC`, c.ID)
	} else if status != "" {
		rows, err = s.db.Query(`SELECT id, crew_id, from_date, to_date, type, reason, status, decided_by, decided_at, created_at FROM leave_request WHERE status = ? ORDER BY created_at DESC`, status)
	} else {
		rows, err = s.db.Query(`SELECT id, crew_id, from_date, to_date, type, reason, status, decided_by, decided_at, created_at FROM leave_request ORDER BY created_at DESC`)
	}
	if err != nil {
		return []LeaveRequest{}
	}
	type raw struct {
		lr         LeaveRequest
		internalID string
		reason     sql.NullString
		decidedBy  sql.NullString
		decidedAt  sql.NullInt64
	}
	var raws []raw
	for rows.Next() {
		var r raw
		if err := rows.Scan(&r.lr.ID, &r.internalID, &r.lr.FromDate, &r.lr.ToDate, &r.lr.Type, &r.reason, &r.lr.Status, &r.decidedBy, &r.decidedAt, &r.lr.CreatedAt); err == nil {
			raws = append(raws, r)
		}
	}
	rows.Close()
	out := []LeaveRequest{}
	for _, r := range raws {
		lr := r.lr
		if c, err := s.crew.GetCrew(r.internalID); err == nil {
			lr.CrewID = c.CrewID
		}
		lr.Reason = nullStrPtr(r.reason)
		lr.DecidedBy = nullStrPtr(r.decidedBy)
		if r.decidedAt.Valid {
			v := r.decidedAt.Int64
			lr.DecidedAt = &v
		}
		out = append(out, lr)
	}
	return out
}

func (s *Store) DecideLeave(deciderHumanCrewID, leaveID, decision string) (LeaveRequest, error) {
	if decision != "approved" && decision != "rejected" {
		return LeaveRequest{}, errors.New("decision must be approved or rejected")
	}
	res, err := s.db.Exec(`UPDATE leave_request SET status = ?, decided_by = ?, decided_at = ? WHERE id = ? AND status = 'pending'`, decision, deciderHumanCrewID, time.Now().UnixMilli(), leaveID)
	if err != nil {
		return LeaveRequest{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return LeaveRequest{}, errors.New("leave request is not pending")
	}
	s.audit("leave_decision", "leave_request", leaveID, decision)
	for _, lr := range s.ListLeave("", "") {
		if lr.ID == leaveID {
			return lr, nil
		}
	}
	return LeaveRequest{}, errors.New("leave request not found after decision")
}

// ExportAttendance returns CSV for external payroll (BR-HR-05).
func (s *Store) ExportAttendance(from, to int64) string {
	rows := s.ListAttendance("", from, to)
	out := "crew_id,clock_in_at,clock_out_at,auto_clocked_out,corrected_by,correction_reason\n"
	for _, a := range rows {
		outAt := ""
		if a.ClockOutAt != nil {
			outAt = fmt.Sprintf("%d", *a.ClockOutAt)
		}
		correctedBy := ""
		if a.CorrectedBy != nil {
			correctedBy = *a.CorrectedBy
		}
		reason := ""
		if a.CorrectionReason != nil {
			reason = *a.CorrectionReason
		}
		auto := "0"
		if a.AutoClockedOut {
			auto = "1"
		}
		out += fmt.Sprintf("%s,%d,%s,%s,%s,%s\n", a.CrewID, a.ClockInAt, outAt, auto, correctedBy, reason)
	}
	return out
}

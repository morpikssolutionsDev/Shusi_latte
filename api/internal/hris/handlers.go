package hris

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/actor"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/auth"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crew"
)

// Handler wires HRIS endpoints. Clock-in/out is only accepted
// from an HRIS screen-mode session (BR-HR-03, TR-AUTH-08):
// the request must carry X-Screen-Token for a screen signed
// in as "hris", plus Crew ID and PIN for the person clocking.
type Handler struct {
	Store *Store
	Auth  *auth.Store
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeProblem(w http.ResponseWriter, status int, code, title string) {
	writeJSON(w, status, map[string]any{
		"type": "about:blank", "title": title, "status": status, "code": code,
	})
}

func (h *Handler) withActor(r *http.Request) *Handler {
	copyStore := *h.Store
	copyStore.Actor = actor.FromRequest(r.Context())
	return &Handler{Store: &copyStore, Auth: h.Auth}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/employees", h.listEmployees)
	mux.HandleFunc("POST /api/v1/employees", h.createEmployee)
	mux.HandleFunc("PUT /api/v1/employees/{id}", h.updateEmployee)
	mux.HandleFunc("GET /api/v1/shifts", h.listShifts)
	mux.HandleFunc("POST /api/v1/shifts", h.createShift)
	mux.HandleFunc("DELETE /api/v1/shifts/{id}", h.deleteShift)
	mux.HandleFunc("POST /api/v1/attendance/clock", h.clock)
	mux.HandleFunc("GET /api/v1/attendance", h.listAttendance)
	mux.HandleFunc("POST /api/v1/attendance/{id}/correct", h.correctAttendance)
	mux.HandleFunc("POST /api/v1/attendance/auto-clock-out", h.autoClockOut)
	mux.HandleFunc("GET /api/v1/attendance/export", h.exportAttendance)
	mux.HandleFunc("GET /api/v1/leave-requests", h.listLeave)
	mux.HandleFunc("POST /api/v1/leave-requests", h.createLeave)
	mux.HandleFunc("POST /api/v1/leave-requests/{id}/decide", h.decideLeave)
}

func (h *Handler) crewMember(r *http.Request) (crew.Crew, bool) {
	return auth.CrewFromContext(r)
}

func requireRole(w http.ResponseWriter, c crew.Crew, roles ...string) bool {
	for _, role := range roles {
		if c.Role == role {
			return true
		}
	}
	writeProblem(w, http.StatusForbidden, "forbidden", "role does not permit this action")
	return false
}

func parseRange(r *http.Request) (int64, int64) {
	var from, to int64
	if v := r.URL.Query().Get("from_ms"); v != "" {
		from, _ = strconv.ParseInt(v, 10, 64)
	}
	if v := r.URL.Query().Get("to_ms"); v != "" {
		to, _ = strconv.ParseInt(v, 10, 64)
	}
	if to == 0 {
		to = 9223372036854775807
	}
	return from, to
}

func (h *Handler) listEmployees(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Store.ListEmployees())
}

func (h *Handler) createEmployee(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	if !requireRole(w, c, crew.RoleManagement) {
		return
	}
	var in EmployeeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	emp, err := h.Store.CreateEmployee(in)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_employee", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, emp)
}

func (h *Handler) updateEmployee(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	if !requireRole(w, c, crew.RoleManagement) {
		return
	}
	var in EmployeeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	emp, err := h.Store.UpdateEmployee(r.PathValue("id"), in)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "employee_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, emp)
}

func (h *Handler) listShifts(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.crewMember(r); !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	from, to := parseRange(r)
	writeJSON(w, http.StatusOK, h.Store.ListShifts(r.URL.Query().Get("crew_id"), from, to))
}

func (h *Handler) createShift(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	if !requireRole(w, c, crew.RoleOutletAdmin, crew.RoleManagement) {
		return
	}
	var in ShiftInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	sh, err := h.Store.CreateShift(in)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_shift", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sh)
}

func (h *Handler) deleteShift(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	if !requireRole(w, c, crew.RoleOutletAdmin, crew.RoleManagement) {
		return
	}
	if err := h.Store.DeleteShift(r.PathValue("id")); err != nil {
		writeProblem(w, http.StatusNotFound, "shift_not_found", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) verifyPIN(crewID, pin string) (bool, error) {
	_, ok, err := h.Auth.CrewStore().VerifyPINByCrewID(crewID, pin)
	return ok, err
}

// clock accepts only HRIS screen sessions (BR-HR-03).
// It also verifies the person's PIN, so a screen token alone
// cannot clock someone else in or out.
func (h *Handler) clock(w http.ResponseWriter, r *http.Request) {
	if _, err := h.Auth.ValidateScreenSession(r.Header.Get("X-Screen-Token"), "hris"); err != nil {
		writeProblem(w, http.StatusForbidden, "screen_session_required", "clock-in and clock-out require an HRIS screen session")
		return
	}
	var req struct {
		CrewID string `json:"crew_id"`
		PIN    string `json:"pin"`
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	// PIN verification uses the crew store via auth's crew
	// store accessor; the auth Store exposes CrewStore().
	verified, err := h.verifyPIN(req.CrewID, req.PIN)
	if err != nil || !verified {
		writeProblem(w, http.StatusUnauthorized, "invalid_credentials", "invalid crew id or pin")
		return
	}
	var att Attendance
	switch req.Action {
	case "in":
		att, err = h.Store.ClockIn(req.CrewID)
	case "out":
		att, err = h.Store.ClockOut(req.CrewID)
	default:
		writeProblem(w, http.StatusBadRequest, "invalid_action", "action must be in or out")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "clock_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, att)
}

func (h *Handler) listAttendance(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.crewMember(r); !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	from, to := parseRange(r)
	writeJSON(w, http.StatusOK, h.Store.ListAttendance(r.URL.Query().Get("crew_id"), from, to))
}

func (h *Handler) correctAttendance(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	if !requireRole(w, c, crew.RoleOutletAdmin, crew.RoleManagement) {
		return
	}
	var req struct {
		ClockInAt  *int64 `json:"clock_in_at"`
		ClockOutAt *int64 `json:"clock_out_at"`
		Reason     string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	att, err := h.withActor(r).Store.CorrectAttendance(c.CrewID, r.PathValue("id"), req.ClockInAt, req.ClockOutAt, req.Reason)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "correction_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, att)
}

func (h *Handler) autoClockOut(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	if !requireRole(w, c, crew.RoleOutletAdmin, crew.RoleManagement) {
		return
	}
	writeJSON(w, http.StatusOK, h.withActor(r).Store.AutoClockOut())
}

func (h *Handler) exportAttendance(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.crewMember(r); !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	from, to := parseRange(r)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=attendance.csv")
	_, _ = w.Write([]byte(h.Store.ExportAttendance(from, to)))
}

func (h *Handler) listLeave(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.crewMember(r); !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	writeJSON(w, http.StatusOK, h.Store.ListLeave(r.URL.Query().Get("crew_id"), r.URL.Query().Get("status")))
}

func (h *Handler) createLeave(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.crewMember(r); !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var in LeaveInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	lr, err := h.Store.CreateLeave(in)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_leave", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, lr)
}

func (h *Handler) decideLeave(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	if !requireRole(w, c, crew.RoleOutletAdmin, crew.RoleManagement) {
		return
	}
	var req struct {
		Decision string `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	lr, err := h.withActor(r).Store.DecideLeave(c.CrewID, r.PathValue("id"), req.Decision)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "decision_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, lr)
}

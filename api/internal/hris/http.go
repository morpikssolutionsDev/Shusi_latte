package hris

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/httpx"
)

type Handlers struct{ Svc *Service }

// Routes mounts the HRIS endpoints (TRD §6.1 HRIS). Clock-in only from an HRIS screen-mode
// session (BR-HR-03, TR-AUTH-08); schedules and leave decisions by Outlet Admin and Management
// (BR-HR-02, BR-HR-04); employee records and the export by Management (BR-HR-01, BR-HR-05).
func (h Handlers) Routes(r chi.Router) {
	pad := auth.Require(func(p *auth.Principal) bool { return p.Kind == "screen" && p.Screen == auth.ScreenHRIS })
	crew := auth.Require(func(p *auth.Principal) bool { return p.Kind == "crew" })
	admin := auth.Require(auth.Roles(auth.RoleOutletAdmin, auth.RoleManagement))
	management := auth.Require(auth.Roles(auth.RoleManagement))

	r.With(pad).Post("/attendance/clock", httpx.Handler(h.clock))
	r.With(admin).Get("/attendance", httpx.Handler(h.attendance))
	r.With(admin).Post("/attendance/{id}/correct", h.staff(h.correct))
	r.With(management).Get("/attendance/export", httpx.Handler(h.export))
	r.With(crew).Get("/shifts", httpx.Handler(h.schedule))
	r.With(admin).Post("/shifts", h.staff(h.createShift))
	r.With(admin).Put("/shifts/{id}", h.staff(h.updateShift))
	r.With(admin).Delete("/shifts/{id}", h.staff(h.deleteShift))
	r.With(crew).Get("/leave-requests", h.staff(h.leave))
	r.With(crew).Post("/leave-requests", h.staff(h.requestLeave))
	r.With(admin).Post("/leave-requests/{id}/decision", h.staff(h.decideLeave))
	r.With(management).Get("/employees", httpx.Handler(h.employees))
	r.With(management).Put("/employees/{id}", h.staff(h.saveEmployee))
	r.With(crew).Get("/hris/me", h.staff(h.me))
}

func (h Handlers) staff(fn func(http.ResponseWriter, *http.Request, *auth.Principal) error) http.HandlerFunc {
	return httpx.Handler(func(w http.ResponseWriter, r *http.Request) error {
		return fn(w, r, auth.FromContext(r.Context()))
	})
}

func noContent(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h Handlers) clock(w http.ResponseWriter, r *http.Request) error {
	var in ClockInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	res, err := h.Svc.Clock(r.Context(), in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, res)
	return nil
}

func (h Handlers) attendance(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.Svc.Attendance(r.Context(), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, rows)
	return nil
}

func (h Handlers) correct(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in CorrectInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.Correct(r.Context(), p, chi.URLParam(r, "id"), in); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) export(w http.ResponseWriter, r *http.Request) error {
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if _, _, err := dayRange(from, to); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="absensi-%s-%s.csv"`, from, to))
	return h.Svc.ExportCSV(r.Context(), w, from, to)
}

func (h Handlers) schedule(w http.ResponseWriter, r *http.Request) error {
	sch, err := h.Svc.Schedule(r.Context(), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, sch)
	return nil
}

func (h Handlers) createShift(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in ShiftInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	id, err := h.Svc.SaveShift(r.Context(), p, "", in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
	return nil
}

func (h Handlers) updateShift(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in ShiftInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if _, err := h.Svc.SaveShift(r.Context(), p, chi.URLParam(r, "id"), in); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) deleteShift(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	if err := h.Svc.DeleteShift(r.Context(), p, chi.URLParam(r, "id")); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) leave(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	rows, err := h.Svc.Leave(r.Context(), p)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, rows)
	return nil
}

func (h Handlers) requestLeave(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in LeaveInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	id, err := h.Svc.RequestLeave(r.Context(), p, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
	return nil
}

func (h Handlers) decideLeave(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		Approve bool   `json:"approve"`
		Reason  string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.DecideLeave(r.Context(), p, chi.URLParam(r, "id"), in.Approve, in.Reason); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) employees(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.Svc.Employees(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, rows)
	return nil
}

func (h Handlers) saveEmployee(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in EmployeeInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.SaveEmployee(r.Context(), p, chi.URLParam(r, "id"), in); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) me(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	me, err := h.Svc.Me(r.Context(), p)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, me)
	return nil
}

package auth

import (
	"encoding/json"
	"net"
	"net/http"
	"time"
)

// Handler wires sign-in, sign-out, current user and PIN Override.
type Handler struct {
	Store   *Store
	Limiter *IPLimiter
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeProblemJSON(w http.ResponseWriter, status int, code, title string) {
	writeJSON(w, status, map[string]any{
		"type": "about:blank", "title": title, "status": status, "code": code,
	})
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, sess Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sess.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(time.Until(time.UnixMilli(sess.ExpiresAt)).Seconds()),
	})
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/login", h.login)
	mux.HandleFunc("POST /auth/logout", h.logout)
	mux.HandleFunc("GET /auth/me", h.me)
	mux.HandleFunc("POST /auth/override", h.override)
	mux.HandleFunc("POST /auth/screen-login", h.screenLogin)
}

// screenLogin signs a screen in once per day (TR-AUTH-08).
// Only Outlet Admin or Management may sign a screen in, and
// the session is limited to that screen and expires at the end
// of the Jakarta day. It returns a screen token for the
// X-Screen-Token header; it is not a staff session cookie.
func (h *Handler) screenLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CrewID string `json:"crew_id"`
		PIN    string `json:"pin"`
		Screen string `json:"screen"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblemJSON(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	token, err := h.Store.CreateScreenSession(req.CrewID, req.PIN, req.Screen)
	if err != nil {
		writeProblemJSON(w, http.StatusForbidden, "screen_login_denied", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"screen_token": token.Token, "screen": token.Screen, "expires_at": token.ExpiresAt,
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	now := time.Now()
	if h.Limiter.Blocked(ip, now) {
		writeProblemJSON(w, http.StatusTooManyRequests, "rate_limited", "too many failed logins from this address")
		return
	}
	var req struct {
		CrewID      string  `json:"crew_id"`
		PIN         string  `json:"pin"`
		DeviceLabel *string `json:"device_label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblemJSON(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	label := ""
	if req.DeviceLabel != nil {
		label = *req.DeviceLabel
	}
	sess, err := h.Store.Login(req.CrewID, req.PIN, label)
	if err != nil {
		h.Limiter.RecordFailure(ip, now)
		writeProblemJSON(w, http.StatusUnauthorized, "invalid_credentials", "invalid crew id or pin")
		return
	}
	h.Limiter.Clear(ip)
	h.setSessionCookie(w, sess)
	writeJSON(w, http.StatusOK, map[string]any{
		"crew": sess.Crew, "expires_at": sess.ExpiresAt,
	})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(CookieName); err == nil {
		h.Store.Logout(cookie.Value)
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	c, ok := CrewFromContext(r)
	if !ok {
		writeProblemJSON(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *Handler) override(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CrewID string `json:"crew_id"`
		PIN    string `json:"pin"`
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblemJSON(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	token, err := h.Store.CreateOverride(req.CrewID, req.PIN, req.Action)
	if err != nil {
		writeProblemJSON(w, http.StatusForbidden, "override_denied", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"override_token": token.Token, "action": token.Action,
		"expires_at": token.ExpiresAt.UnixMilli(), "approver_crew_id": token.Approver.CrewID,
	})
}

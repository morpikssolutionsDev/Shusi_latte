// Package httpx holds HTTP conventions shared by every handler: JSON bodies and
// RFC 9457 problem details with a stable code (TR-API-02).
package httpx

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxBody = 1 << 20

// Problem is an RFC 9457 problem detail. Code is the stable, machine-readable error id.
type Problem struct {
	Status int            `json:"status"`
	Code   string         `json:"code"`
	Title  string         `json:"title"`
	Detail string         `json:"detail,omitempty"`
	Extra  map[string]any `json:"extra,omitempty"`
}

func (p *Problem) Error() string { return p.Code + ": " + p.Title }

func NewProblem(status int, code, title string) *Problem {
	return &Problem{Status: status, Code: code, Title: title}
}

func (p *Problem) With(key string, value any) *Problem {
	if p.Extra == nil {
		p.Extra = map[string]any{}
	}
	p.Extra[key] = value
	return p
}

var (
	ErrUnauthorised = NewProblem(http.StatusUnauthorized, "unauthorised", "Sign in required")
	ErrForbidden    = NewProblem(http.StatusForbidden, "forbidden", "Not allowed for this role")
	ErrBadRequest   = NewProblem(http.StatusBadRequest, "bad_request", "Invalid request body")
)

// WriteJSON writes v with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteJSONWithETag writes v with a content ETag and answers 304 when the client already has
// it (TR-PC-05: menus refetch the catalog after every menu event).
func WriteJSONWithETag(w http.ResponseWriter, r *http.Request, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		WriteError(w, err)
		return
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

// WriteError renders a *Problem, or a generic 500 for anything else.
func WriteError(w http.ResponseWriter, err error) {
	var p *Problem
	if !errors.As(err, &p) {
		p = NewProblem(http.StatusInternalServerError, "internal", "Something went wrong")
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// DecodeJSON reads a size-limited JSON body into v, rejecting unknown fields.
func DecodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return ErrBadRequest
	}
	return nil
}

// Handler adapts a handler that returns an error into an http.HandlerFunc.
func Handler(fn func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			WriteError(w, err)
		}
	}
}

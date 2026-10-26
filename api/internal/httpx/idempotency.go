package httpx

import (
	"bytes"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"sushilatte/api/internal/store"
)

// IdempotencyStore is the persistence the middleware needs.
type IdempotencyStore interface {
	Get(r *http.Request, scope, key string) (*store.IdempotencyKey, error)
	Save(r *http.Request, row store.SaveIdempotentResponseParams) error
}

// Idempotency replays the stored response for a repeated Idempotency-Key on mutating
// requests (TR-API-01). scope separates keys per session or client.
func Idempotency(s IdempotencyStore, scope func(*http.Request) string, now func() time.Time) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if key == "" || r.Method == http.MethodGet || r.Method == http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}
			sc := scope(r)
			prev, err := s.Get(r, sc, key)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				WriteError(w, err)
				return
			}
			if prev != nil {
				if prev.Method != r.Method || prev.Path != r.URL.Path {
					WriteError(w, NewProblem(http.StatusUnprocessableEntity, "idempotency_key_reused", "Idempotency-Key was used for a different request"))
					return
				}
				if prev.ContentType.Valid {
					w.Header().Set("Content-Type", prev.ContentType.String)
				}
				w.Header().Set("Idempotent-Replay", "true")
				w.WriteHeader(int(prev.Status))
				_, _ = w.Write(prev.Body)
				return
			}
			rec := &recorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			if rec.status < 500 {
				_ = s.Save(r, store.SaveIdempotentResponseParams{
					Scope: sc, Key: key, Method: r.Method, Path: r.URL.Path,
					Status:      int64(rec.status),
					ContentType: sql.NullString{String: rec.Header().Get("Content-Type"), Valid: true},
					Body:        rec.buf.Bytes(),
					CreatedAt:   now().UnixMilli(),
				})
			}
		})
	}
}

type recorder struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
}

func (r *recorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(b []byte) (int, error) {
	r.buf.Write(b)
	return r.ResponseWriter.Write(b)
}

// Package actor carries the authenticated crew ID through a request
// context, so audit rows record the person who acted, not "system".
// It imports nothing from the other domain packages, so handlers
// can read it without creating an import cycle with auth, which
// sets it in its middleware.
package actor

import "context"

type contextKey string

const actorKey contextKey = "actor_crew_id"

// WithActor returns a context carrying the human-readable Crew ID.
func WithActor(ctx context.Context, crewID string) context.Context {
	return context.WithValue(ctx, actorKey, crewID)
}

// FromContext returns the actor Crew ID, if the request was
// authenticated through the auth middleware.
func FromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(actorKey).(string)
	return v, ok && v != ""
}

// FromRequest is a convenience wrapper for HTTP handlers.
func FromRequest(ctx context.Context) string {
	if v, ok := FromContext(ctx); ok {
		return v
	}
	return ""
}

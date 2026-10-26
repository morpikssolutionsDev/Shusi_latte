// Package audit writes the append-only audit log (TRD §13). Every restricted action records
// who did it, who approved it through a PIN Override, and a before/after snapshot.
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/store"
)

type Entry struct {
	Actor    string // crew.id of the person acting
	Approver string // crew.id of the PIN Override approver, if any
	Action   string // e.g. "pin_override.used", "variant.sold_out.reenabled"
	Entity   string // e.g. "variant", "bill"
	EntityID string
	Before   any
	After    any
	Reason   string
}

type Writer interface {
	InsertAudit(ctx context.Context, arg store.InsertAuditParams) error
}

// Write records e inside the caller's transaction, so the audit entry commits or rolls
// back together with the change it describes.
func Write(ctx context.Context, q Writer, now time.Time, e Entry) error {
	before, err := snapshot(e.Before)
	if err != nil {
		return err
	}
	after, err := snapshot(e.After)
	if err != nil {
		return err
	}
	return q.InsertAudit(ctx, store.InsertAuditParams{
		ID:             platform.NewID(),
		At:             platform.Millis(now),
		ActorCrewID:    nullable(e.Actor),
		OverrideCrewID: nullable(e.Approver),
		Action:         e.Action,
		Entity:         e.Entity,
		EntityID:       nullable(e.EntityID),
		BeforeJson:     before,
		AfterJson:      after,
		Reason:         nullable(e.Reason),
	})
}

func snapshot(v any) (sql.NullString, error) {
	if v == nil {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func nullable(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

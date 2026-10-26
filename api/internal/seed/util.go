package seed

import (
	"database/sql"
	"time"

	"sushilatte/api/internal/platform"
)

func nullMs(t time.Time) sql.NullInt64 { return sql.NullInt64{Int64: platform.Millis(t), Valid: true} }

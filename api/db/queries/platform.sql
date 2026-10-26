-- name: InsertAudit :exec
INSERT INTO audit_log (id, at, actor_crew_id, override_crew_id, action, entity, entity_id, before_json, after_json, reason)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListAudit :many
SELECT * FROM audit_log ORDER BY at DESC LIMIT ?;

-- name: GetOpsSetting :one
SELECT * FROM ops_setting WHERE id = 1;

-- name: GetIdempotentResponse :one
SELECT * FROM idempotency_key WHERE scope = ? AND key = ?;

-- name: SaveIdempotentResponse :exec
INSERT INTO idempotency_key (scope, key, method, path, status, content_type, body, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (scope, key) DO NOTHING;

-- name: DeleteExpiredIdempotencyKeys :execrows
DELETE FROM idempotency_key WHERE created_at <= ?;

-- name: InsertAudit :exec
INSERT INTO audit_log (id, at, actor_crew_id, override_crew_id, action, entity, entity_id, before_json, after_json, reason)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListAudit :many
SELECT * FROM audit_log ORDER BY at DESC LIMIT $1::bigint;

-- name: GetOpsSetting :one
SELECT * FROM ops_setting WHERE id = 1;

-- name: GetIdempotentResponse :one
SELECT * FROM idempotency_key WHERE scope = $1 AND key = $2;

-- name: SaveIdempotentResponse :exec
INSERT INTO idempotency_key (scope, key, method, path, status, content_type, body, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (scope, key) DO NOTHING;

-- name: DeleteExpiredIdempotencyKeys :execrows
DELETE FROM idempotency_key WHERE created_at <= $1;

-- name: SetDemoSeed :exec
INSERT INTO demo_seed (id, story, t0) VALUES (1, $1, $2)
ON CONFLICT (id) DO UPDATE SET story = excluded.story, t0 = excluded.t0;

-- name: GetDemoSeed :one
SELECT * FROM demo_seed WHERE id = 1;

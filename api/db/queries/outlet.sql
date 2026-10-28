-- Pricing settings -------------------------------------------------------------

-- name: GetPricingSetting :one
SELECT * FROM pricing_setting WHERE id = 1;

-- name: UpdatePricingSetting :exec
UPDATE pricing_setting SET service_rate_bp = $1, tax_rate_bp = $2, price_mode = $3, refund_cap = $4 WHERE id = 1;

-- name: UpdateOpsSetting :exec
UPDATE ops_setting
SET claim_renotify_seconds = $1, claim_escalate_seconds = $2, pickup_reminder_seconds = $3,
    empty_bill_close_seconds = $4, auto_clock_out_grace_seconds = $5
WHERE id = 1;

-- Discounts --------------------------------------------------------------------

-- name: CreateDiscount :exec
INSERT INTO discount (id, name, kind, value, scope, trigger, days_mask, start_minute, end_minute, valid_from, valid_to,
                      min_subtotal, active, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15);

-- name: UpdateDiscount :exec
UPDATE discount
SET name = $1, kind = $2, value = $3, scope = $4, trigger = $5, days_mask = $6, start_minute = $7, end_minute = $8,
    valid_from = $9, valid_to = $10, min_subtotal = $11, active = $12, updated_at = $13
WHERE id = $14;

-- name: GetDiscount :one
SELECT * FROM discount WHERE id = $1;

-- name: ListDiscounts :many
SELECT * FROM discount ORDER BY active DESC, name;

-- name: ListDiscountTargets :many
SELECT * FROM discount_target;

-- name: ClearDiscountTargets :exec
DELETE FROM discount_target WHERE discount_id = $1;

-- name: AddDiscountTarget :exec
INSERT INTO discount_target (discount_id, target_type, target_id) VALUES ($1, $2, $3);

-- Tables -----------------------------------------------------------------------

-- name: CreateTable :exec
INSERT INTO dining_table (id, number, label, code, active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpdateTable :exec
UPDATE dining_table SET number = $1, code = $2, label = $3, active = $4, updated_at = $5 WHERE id = $6;

-- name: GetTable :one
SELECT * FROM dining_table WHERE id = $1;

-- name: ListTables :many
SELECT * FROM dining_table ORDER BY number;

-- Crew administration ------------------------------------------------------------

-- name: ListAllCrew :many
SELECT * FROM crew ORDER BY crew_id;

-- name: ListAllCrewStations :many
SELECT * FROM crew_station;

-- name: UpdateCrew :exec
UPDATE crew SET full_name = $1, role = $2, image_key = $3, active = $4, updated_at = $5 WHERE id = $6;

-- name: ClearCrewStations :exec
DELETE FROM crew_station WHERE crew_id = $1;

-- name: SetCrewPIN :exec
UPDATE crew SET pin_hash = $1, failed_attempts = 0, locked_until = NULL, updated_at = $2 WHERE id = $3;

-- name: CountActiveManagement :one
SELECT COUNT(*) FROM crew WHERE role = 'management' AND active = 1;

-- name: DeleteCrewSessions :exec
DELETE FROM session WHERE crew_id = $1;

-- Audit -------------------------------------------------------------------------

-- name: SearchAudit :many
SELECT * FROM audit_log
WHERE (sqlc.narg(entity)::text IS NULL OR entity = sqlc.narg(entity)::text)
  AND (sqlc.narg(action_prefix)::text IS NULL OR action LIKE sqlc.narg(action_prefix)::text || '%')
  AND (sqlc.narg(actor)::text IS NULL OR actor_crew_id = sqlc.narg(actor)::text OR override_crew_id = sqlc.narg(actor)::text)
  AND (sqlc.narg(from_ms)::bigint IS NULL OR at >= sqlc.narg(from_ms)::bigint)
  AND (sqlc.narg(before_ms)::bigint IS NULL OR at < sqlc.narg(before_ms)::bigint)
ORDER BY at DESC
LIMIT sqlc.arg(row_limit)::bigint;

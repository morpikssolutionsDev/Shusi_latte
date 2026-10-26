-- Pricing settings -------------------------------------------------------------

-- name: GetPricingSetting :one
SELECT * FROM pricing_setting WHERE id = 1;

-- name: UpdatePricingSetting :exec
UPDATE pricing_setting SET service_rate_bp = ?, tax_rate_bp = ?, price_mode = ?, refund_cap = ? WHERE id = 1;

-- name: UpdateOpsSetting :exec
UPDATE ops_setting
SET claim_renotify_seconds = ?, claim_escalate_seconds = ?, pickup_reminder_seconds = ?,
    empty_bill_close_seconds = ?, auto_clock_out_grace_seconds = ?
WHERE id = 1;

-- Discounts --------------------------------------------------------------------

-- name: CreateDiscount :exec
INSERT INTO discount (id, name, kind, value, scope, trigger, days_mask, start_minute, end_minute, valid_from, valid_to,
                      min_subtotal, active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateDiscount :exec
UPDATE discount
SET name = ?, kind = ?, value = ?, scope = ?, trigger = ?, days_mask = ?, start_minute = ?, end_minute = ?,
    valid_from = ?, valid_to = ?, min_subtotal = ?, active = ?, updated_at = ?
WHERE id = ?;

-- name: GetDiscount :one
SELECT * FROM discount WHERE id = ?;

-- name: ListDiscounts :many
SELECT * FROM discount ORDER BY active DESC, name;

-- name: ListDiscountTargets :many
SELECT * FROM discount_target;

-- name: ClearDiscountTargets :exec
DELETE FROM discount_target WHERE discount_id = ?;

-- name: AddDiscountTarget :exec
INSERT INTO discount_target (discount_id, target_type, target_id) VALUES (?, ?, ?);

-- Tables -----------------------------------------------------------------------

-- name: CreateTable :exec
INSERT INTO dining_table (id, number, label, qr_token, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateTable :exec
UPDATE dining_table SET number = ?, label = ?, active = ?, updated_at = ? WHERE id = ?;

-- name: RegenerateTableQR :exec
UPDATE dining_table SET qr_token = ?, qr_regenerated_at = ?, updated_at = ? WHERE id = ?;

-- name: GetTable :one
SELECT * FROM dining_table WHERE id = ?;

-- name: ListTables :many
SELECT * FROM dining_table ORDER BY number;

-- Crew administration ------------------------------------------------------------

-- name: ListAllCrew :many
SELECT * FROM crew ORDER BY crew_id;

-- name: ListAllCrewStations :many
SELECT * FROM crew_station;

-- name: UpdateCrew :exec
UPDATE crew SET full_name = ?, role = ?, image_key = ?, active = ?, updated_at = ? WHERE id = ?;

-- name: ClearCrewStations :exec
DELETE FROM crew_station WHERE crew_id = ?;

-- name: SetCrewPIN :exec
UPDATE crew SET pin_hash = ?, failed_attempts = 0, locked_until = NULL, updated_at = ? WHERE id = ?;

-- name: CountActiveManagement :one
SELECT COUNT(*) FROM crew WHERE role = 'management' AND active = 1;

-- name: DeleteCrewSessions :exec
DELETE FROM session WHERE crew_id = ?;

-- Audit -------------------------------------------------------------------------

-- name: SearchAudit :many
SELECT * FROM audit_log
WHERE (sqlc.narg(entity) IS NULL OR entity = sqlc.narg(entity))
  AND (sqlc.narg(action_prefix) IS NULL OR action LIKE sqlc.narg(action_prefix) || '%')
  AND (sqlc.narg(actor) IS NULL OR actor_crew_id = sqlc.narg(actor) OR override_crew_id = sqlc.narg(actor))
  AND (sqlc.narg(from_ms) IS NULL OR at >= sqlc.narg(from_ms))
  AND (sqlc.narg(before_ms) IS NULL OR at < sqlc.narg(before_ms))
ORDER BY at DESC
LIMIT sqlc.arg(row_limit);

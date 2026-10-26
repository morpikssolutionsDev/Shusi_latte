-- name: CreateCrew :exec
INSERT INTO crew (id, crew_id, full_name, role, image_key, pin_hash, active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?);

-- name: AddCrewStation :exec
INSERT INTO crew_station (crew_id, station) VALUES (?, ?);

-- name: GetCrewByCrewID :one
SELECT * FROM crew WHERE crew_id = ? AND active = 1;

-- name: GetCrew :one
SELECT * FROM crew WHERE id = ?;

-- name: ListActiveCrew :many
SELECT * FROM crew WHERE active = 1 ORDER BY crew_id;

-- name: ListCrewStations :many
SELECT station FROM crew_station WHERE crew_id = ? ORDER BY station;

-- name: RecordFailedLogin :one
UPDATE crew
SET failed_attempts = failed_attempts + 1,
    locked_until    = CASE WHEN failed_attempts + 1 >= sqlc.arg(threshold) THEN sqlc.arg(lock_until) ELSE locked_until END,
    updated_at      = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING failed_attempts, locked_until;

-- name: ClearLockout :exec
UPDATE crew SET failed_attempts = 0, locked_until = NULL, updated_at = ? WHERE id = ?;

-- name: SetCrewLockout :exec
UPDATE crew SET failed_attempts = ?, locked_until = ?, updated_at = ? WHERE id = ?;

-- name: CreateSession :exec
INSERT INTO session (id, crew_id, kind, screen, device_label, created_at, last_seen_at, expires_at, idle_timeout_ms)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetSession :one
SELECT * FROM session WHERE id = ?;

-- name: TouchSession :exec
UPDATE session SET last_seen_at = ? WHERE id = ?;

-- name: DeleteSession :exec
DELETE FROM session WHERE id = ?;

-- name: DeleteExpiredSessions :execrows
DELETE FROM session WHERE expires_at <= ?;

-- name: CreateOverrideToken :exec
INSERT INTO override_token (id, approver_id, requester_id, action, created_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ConsumeOverrideToken :one
UPDATE override_token
SET used_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND action = sqlc.arg(action)
  AND requester_id = sqlc.arg(requester_id)
  AND used_at IS NULL
  AND expires_at > sqlc.arg(now)
RETURNING approver_id;

-- name: DeleteExpiredOverrideTokens :execrows
DELETE FROM override_token WHERE expires_at <= ?;

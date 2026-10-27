-- ---------------------------------------------------------------- employees (BR-HR-01)

-- name: ListEmployees :many
SELECT c.id, c.crew_id, c.full_name, c.role, c.image_key, c.active,
       COALESCE(e.phone, '') AS phone, COALESCE(e.employment_type, 'full_time') AS employment_type,
       e.joined_on, COALESCE(e.emergency_contact, '') AS emergency_contact, COALESCE(e.notes, '') AS notes
FROM crew c LEFT JOIN employee e ON e.crew_id = c.id
ORDER BY c.crew_id;

-- name: UpsertEmployee :exec
INSERT INTO employee (crew_id, phone, employment_type, joined_on, emergency_contact, notes, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (crew_id) DO UPDATE SET phone = excluded.phone, employment_type = excluded.employment_type,
    joined_on = excluded.joined_on, emergency_contact = excluded.emergency_contact, notes = excluded.notes,
    updated_at = excluded.updated_at;

-- ---------------------------------------------------------------- shifts (BR-HR-02)

-- name: CreateShift :exec
INSERT INTO shift (id, crew_id, starts_at, ends_at, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?);

-- name: UpdateShift :execrows
UPDATE shift SET crew_id = ?, starts_at = ?, ends_at = ? WHERE id = ?;

-- name: DeleteShift :execrows
DELETE FROM shift WHERE id = ?;

-- name: CountShiftAttendance :one
SELECT COUNT(*) FROM attendance WHERE shift_id = ?;

-- name: GetShift :one
SELECT * FROM shift WHERE id = ?;

-- name: ListShiftsBetween :many
SELECT * FROM shift WHERE starts_at < sqlc.arg(until) AND ends_at > sqlc.arg(since) ORDER BY starts_at, crew_id;

-- name: CountOverlappingShifts :one
SELECT COUNT(*) FROM shift
WHERE crew_id = sqlc.arg(crew_id) AND id != sqlc.arg(id) AND starts_at < sqlc.arg(ends_at) AND ends_at > sqlc.arg(starts_at);

-- The shift a clock-in belongs to: starting within 2 hours, or already running.
-- name: FindShiftForClockIn :one
SELECT * FROM shift WHERE crew_id = sqlc.arg(crew_id) AND starts_at <= sqlc.arg(early_start) AND ends_at > sqlc.arg(at)
ORDER BY starts_at LIMIT 1;

-- ---------------------------------------------------------------- attendance (BR-HR-03, BR-HR-06)

-- name: GetOpenAttendance :one
SELECT * FROM attendance WHERE crew_id = ? AND clock_out_at IS NULL;

-- name: GetAttendance :one
SELECT * FROM attendance WHERE id = ?;

-- name: ClockIn :exec
INSERT INTO attendance (id, crew_id, shift_id, clock_in_at) VALUES (?, ?, ?, ?);

-- name: ClockOut :execrows
UPDATE attendance SET clock_out_at = ?, auto_clocked_out = ? WHERE id = ? AND clock_out_at IS NULL;

-- name: CorrectAttendance :exec
UPDATE attendance SET clock_in_at = ?, clock_out_at = ?, corrected_by = ?, corrected_at = ?, correction_reason = ? WHERE id = ?;

-- name: ListAttendanceBetween :many
SELECT * FROM attendance WHERE clock_in_at >= sqlc.arg(since) AND clock_in_at < sqlc.arg(until) ORDER BY clock_in_at;

-- Open attendance whose shift (or, without one, the 12-hour cap) ended more than the grace ago.
-- name: ListOverdueAttendance :many
SELECT a.*, s.ends_at AS shift_ends_at FROM attendance a LEFT JOIN shift s ON s.id = a.shift_id
WHERE a.clock_out_at IS NULL AND COALESCE(s.ends_at, a.clock_in_at + 43200000) + sqlc.arg(grace_ms) <= sqlc.arg(now);

-- Orders a Waiter must hand over before leaving (BR-WT-11, G-06): still Claimed, or Confirmed
-- with items not yet Served, on a live Bill.
-- name: ListOpenClaims :many
SELECT o.id, o.bill_id, o.seq, o.status FROM orders o JOIN bill b ON b.id = o.bill_id
WHERE o.claimed_by = ? AND b.status IN ('open', 'settling') AND (o.status = 'claimed' OR (o.status = 'confirmed' AND EXISTS (
    SELECT 1 FROM order_item i WHERE i.order_id = o.id AND i.status IN ('pending', 'confirmed', 'preparing', 'ready'))))
ORDER BY o.created_at;

-- ---------------------------------------------------------------- leave (BR-HR-04)

-- name: CreateLeaveRequest :exec
INSERT INTO leave_request (id, crew_id, from_date, to_date, type, reason, created_at) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetLeaveRequest :one
SELECT * FROM leave_request WHERE id = ?;

-- name: DecideLeaveRequest :execrows
UPDATE leave_request SET status = ?, decided_by = ?, decided_at = ?, decision_reason = ? WHERE id = ? AND status = 'pending';

-- name: ListLeaveRequests :many
SELECT * FROM leave_request ORDER BY from_date DESC, created_at DESC;

-- name: ListLeaveRequestsByCrew :many
SELECT * FROM leave_request WHERE crew_id = ? ORDER BY from_date DESC, created_at DESC;

-- name: ListApprovedLeaveBetween :many
SELECT * FROM leave_request WHERE status = 'approved' AND from_date <= sqlc.arg(to_date) AND to_date >= sqlc.arg(from_date);

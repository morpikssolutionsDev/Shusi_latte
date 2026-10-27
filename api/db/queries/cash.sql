-- ---------------------------------------------------------------- drawer sessions (BR-CS-04, BR-CS-06)

-- name: GetOpenDrawerSession :one
SELECT * FROM drawer_session WHERE cash_drawer_id = 'main' AND closed_at IS NULL;

-- name: GetDrawerSession :one
SELECT * FROM drawer_session WHERE id = ?;

-- name: OpenDrawerSession :exec
INSERT INTO drawer_session (id, cash_drawer_id, cashier_crew_id, opening_float, opened_at) VALUES (?, 'main', ?, ?, ?);

-- name: CloseDrawerSession :exec
UPDATE drawer_session SET expected_cash = ?, counted_cash = ?, difference = ?, closed_at = ?, closed_by = ? WHERE id = ? AND closed_at IS NULL;

-- name: ListDrawerSessionsBetween :many
SELECT * FROM drawer_session WHERE opened_at >= ? AND opened_at < ? ORDER BY opened_at;

-- name: ListSessionCashPayments :many
SELECT * FROM payment WHERE drawer_session_id = ? AND method = 'cash' AND cash_handover_id IS NULL ORDER BY taken_at;

-- name: ListSessionHandovers :many
SELECT * FROM cash_handover WHERE drawer_session_id = ? ORDER BY received_at;

-- name: ListSessionRefunds :many
SELECT * FROM refund WHERE drawer_session_id = ? ORDER BY created_at;

-- name: SetPaymentSession :exec
UPDATE payment SET drawer_session_id = ? WHERE id = ?;

-- ---------------------------------------------------------------- Cash Handover (BR-CS-05, TR-PAY-02)

-- name: ListHeldPayments :many
SELECT * FROM payment WHERE taken_by = ? AND status = 'held_by_waiter' ORDER BY taken_at;

-- name: ListAllHeldPayments :many
SELECT * FROM payment WHERE status = 'held_by_waiter' ORDER BY taken_by, taken_at;

-- name: CreateCashHandover :exec
INSERT INTO cash_handover (id, drawer_session_id, waiter_crew_id, expected_amount, received_amount, difference, received_by, received_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: SettleHeldPayments :exec
UPDATE payment SET status = 'settled', cash_handover_id = ?, drawer_session_id = ?
WHERE taken_by = ? AND status = 'held_by_waiter';

-- ---------------------------------------------------------------- QRIS (BR-R-30, BR-CS-07, TR-PAY-03)

-- name: ListPendingPayments :many
SELECT * FROM payment WHERE status = 'pending_verification' ORDER BY taken_at;

-- name: GetPayment :one
SELECT * FROM payment WHERE id = ?;

-- name: VerifyPayment :execrows
UPDATE payment SET status = 'settled', verified_by = ?, verified_at = ? WHERE id = ? AND status = 'pending_verification';

-- name: WriteOffPayment :execrows
UPDATE payment SET status = 'written_off', written_off_reason = ?, written_off_by = ? WHERE id = ? AND status = 'pending_verification';

-- ---------------------------------------------------------------- Refunds (BR-R-20, BR-CS-03)

-- name: CreateRefund :exec
INSERT INTO refund (id, bill_id, amount, reason, refunded_by, approved_by, drawer_session_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListRefundsByBill :many
SELECT * FROM refund WHERE bill_id = ? ORDER BY created_at;

-- ---------------------------------------------------------------- Takeaway and post-outage (BR-CS-02, BR-CS-08)

-- name: ListOpenTakeawayBills :many
SELECT * FROM bill WHERE type = 'takeaway' AND status IN ('open', 'settling') ORDER BY opened_at;

-- name: ListTodayBills :many
SELECT * FROM bill WHERE opened_at >= ? OR (closed_at IS NOT NULL AND closed_at >= ?) ORDER BY opened_at DESC;

-- name: SetBillOpenedAt :exec
UPDATE bill SET opened_at = ?, updated_at = ? WHERE id = ?;

-- name: ServeItemAt :exec
UPDATE order_item SET status = 'served', preparing_at = ?, ready_at = ?, served_at = ?, served_by = ?, updated_at = ? WHERE id = ?;

-- name: SetOrderTimes :exec
UPDATE orders SET created_at = ?, claimed_at = ?, confirmed_at = ?, updated_at = ? WHERE id = ?;

-- ---------------------------------------------------------------- operations report (BR-ADM-11)

-- name: ListRemakesBetween :many
SELECT i.*, o.bill_id FROM order_item i JOIN orders o ON o.id = i.order_id
WHERE i.remake_of_id IS NOT NULL AND i.created_at >= ? AND i.created_at < ? ORDER BY i.created_at;

-- name: ListRefundsBetween :many
SELECT * FROM refund WHERE created_at >= ? AND created_at < ? ORDER BY created_at;

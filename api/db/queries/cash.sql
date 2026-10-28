-- ---------------------------------------------------------------- drawer sessions (BR-CS-04, BR-CS-06)

-- name: GetOpenDrawerSession :one
SELECT * FROM drawer_session WHERE cash_drawer_id = 'main' AND closed_at IS NULL;

-- name: GetDrawerSession :one
SELECT * FROM drawer_session WHERE id = $1;

-- name: OpenDrawerSession :exec
INSERT INTO drawer_session (id, cash_drawer_id, cashier_crew_id, opening_float, opened_at) VALUES ($1, 'main', $2, $3, $4);

-- name: CloseDrawerSession :exec
UPDATE drawer_session SET expected_cash = $1, counted_cash = $2, difference = $3, closed_at = $4, closed_by = $5 WHERE id = $6 AND closed_at IS NULL;

-- name: ListDrawerSessionsBetween :many
SELECT * FROM drawer_session WHERE opened_at >= $1 AND opened_at < $2 ORDER BY opened_at;

-- name: ListSessionCashPayments :many
SELECT * FROM payment WHERE drawer_session_id = $1 AND method = 'cash' AND cash_handover_id IS NULL ORDER BY taken_at;

-- name: ListSessionHandovers :many
SELECT * FROM cash_handover WHERE drawer_session_id = $1 ORDER BY received_at;

-- name: ListSessionRefunds :many
SELECT * FROM refund WHERE drawer_session_id = $1 ORDER BY created_at;

-- name: SetPaymentSession :exec
UPDATE payment SET drawer_session_id = $1 WHERE id = $2;

-- ---------------------------------------------------------------- Cash Handover (BR-CS-05, TR-PAY-02)

-- name: ListHeldPayments :many
SELECT * FROM payment WHERE taken_by = $1 AND status = 'held_by_waiter' ORDER BY taken_at;

-- name: ListAllHeldPayments :many
SELECT * FROM payment WHERE status = 'held_by_waiter' ORDER BY taken_by, taken_at;

-- name: CreateCashHandover :exec
INSERT INTO cash_handover (id, drawer_session_id, waiter_crew_id, expected_amount, received_amount, difference, received_by, received_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: SettleHeldPayments :exec
UPDATE payment SET status = 'settled', cash_handover_id = $1, drawer_session_id = $2
WHERE taken_by = $3 AND status = 'held_by_waiter';

-- ---------------------------------------------------------------- QRIS (BR-R-30, BR-CS-07, TR-PAY-03)

-- name: ListPendingPayments :many
SELECT * FROM payment WHERE status = 'pending_verification' ORDER BY taken_at;

-- name: GetPayment :one
SELECT * FROM payment WHERE id = $1;

-- name: VerifyPayment :execrows
UPDATE payment SET status = 'settled', verified_by = $1, verified_at = $2 WHERE id = $3 AND status = 'pending_verification';

-- name: WriteOffPayment :execrows
UPDATE payment SET status = 'written_off', written_off_reason = $1, written_off_by = $2 WHERE id = $3 AND status = 'pending_verification';

-- ---------------------------------------------------------------- Refunds (BR-R-20, BR-CS-03)

-- name: CreateRefund :exec
INSERT INTO refund (id, bill_id, amount, reason, refunded_by, approved_by, drawer_session_id, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListRefundsByBill :many
SELECT * FROM refund WHERE bill_id = $1 ORDER BY created_at;

-- ---------------------------------------------------------------- Takeaway and post-outage (BR-CS-02, BR-CS-08)

-- name: ListOpenTakeawayBills :many
SELECT * FROM bill WHERE type = 'takeaway' AND status IN ('open', 'settling') ORDER BY opened_at;

-- name: ListTodayBills :many
SELECT * FROM bill WHERE opened_at >= $1 OR (closed_at IS NOT NULL AND closed_at >= $2) ORDER BY opened_at DESC;

-- name: SetBillOpenedAt :exec
UPDATE bill SET opened_at = $1, updated_at = $2 WHERE id = $3;

-- name: ServeItemAt :exec
UPDATE order_item SET status = 'served', preparing_at = $1, ready_at = $2, served_at = $3, served_by = $4, updated_at = $5 WHERE id = $6;

-- name: SetOrderTimes :exec
UPDATE orders SET created_at = $1, claimed_at = $2, confirmed_at = $3, updated_at = $4 WHERE id = $5;

-- ---------------------------------------------------------------- operations report (BR-ADM-11)

-- name: ListRemakesBetween :many
SELECT i.*, o.bill_id FROM order_item i JOIN orders o ON o.id = i.order_id
WHERE i.remake_of_id IS NOT NULL AND i.created_at >= $1 AND i.created_at < $2 ORDER BY i.created_at;

-- name: ListRefundsBetween :many
SELECT * FROM refund WHERE created_at >= $1 AND created_at < $2 ORDER BY created_at;

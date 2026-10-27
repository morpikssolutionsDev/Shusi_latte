-- ---------------------------------------------------------------- Settling (BR-R-27)

-- name: SettleBill :execrows
UPDATE bill SET status = 'settling', priced_at = ?, updated_at = ? WHERE id = ? AND status = 'open';

-- name: UnlockBill :execrows
UPDATE bill SET status = 'open', priced_at = NULL, updated_at = ? WHERE id = ? AND status = 'settling';

-- name: SetManualDiscount :exec
UPDATE bill SET manual_discount_id = ?, manual_discount_by = ?, updated_at = ? WHERE id = ?;

-- name: FreezeBillTotals :exec
UPDATE bill SET subtotal = ?, item_discount = ?, bill_discount = ?, service = ?, tax = ?, total = ?, updated_at = ? WHERE id = ?;

-- name: SetPayNowServeLater :exec
UPDATE bill SET pay_now_serve_later_by = ? WHERE id = ?;

-- name: CloseBillUnpaid :exec
UPDATE bill SET status = 'unpaid', unpaid_reason = ?, unpaid_override_by = ?, closed_at = ?, updated_at = ? WHERE id = ?;

-- ---------------------------------------------------------------- Move and Merge (TR-DM-03, TR-DM-06)

-- name: MoveBillTable :exec
UPDATE bill_table SET table_id = sqlc.arg(to_table) WHERE bill_id = sqlc.arg(bill_id) AND table_id = sqlc.arg(from_table);

-- name: RepointBillTables :exec
UPDATE bill_table SET bill_id = sqlc.arg(into_bill) WHERE bill_id = sqlc.arg(from_bill);

-- name: RepointOrders :exec
UPDATE orders SET bill_id = sqlc.arg(into_bill) WHERE bill_id = sqlc.arg(from_bill);

-- name: RepointCustomers :exec
UPDATE customer_entry SET bill_id = sqlc.arg(into_bill) WHERE bill_id = sqlc.arg(from_bill);

-- name: RepointWaiterCalls :exec
UPDATE waiter_call SET bill_id = sqlc.arg(into_bill) WHERE bill_id = sqlc.arg(from_bill);

-- name: MarkBillMerged :exec
UPDATE bill SET status = 'merged', merged_into_id = ?, closed_at = ?, updated_at = ? WHERE id = ?;

-- name: RenumberOrders :exec
UPDATE orders SET seq = seq + ? WHERE bill_id = ?;

-- name: MaxOrderSeq :one
SELECT CAST(COALESCE(MAX(seq), 0) AS INTEGER) FROM orders WHERE bill_id = ?;

-- ---------------------------------------------------------------- Split (BR-R-04, BR-R-29)

-- name: CreateSubBill :exec
INSERT INTO sub_bill (id, bill_id, method, share_index, share_count, created_at) VALUES (?, ?, ?, ?, ?, ?);

-- name: ListSubBills :many
SELECT * FROM sub_bill WHERE bill_id = ? ORDER BY share_index;

-- name: GetSubBill :one
SELECT * FROM sub_bill WHERE id = ?;

-- name: ClearItemSubBills :exec
UPDATE order_item SET sub_bill_id = NULL WHERE order_id IN (SELECT id FROM orders WHERE bill_id = ?);

-- name: DeleteSubBills :exec
DELETE FROM sub_bill WHERE bill_id = ?;

-- name: AssignItemSubBill :exec
UPDATE order_item SET sub_bill_id = ? WHERE id = ?;

-- name: CloseSubBill :exec
UPDATE sub_bill SET status = 'closed', subtotal = ?, item_discount = ?, bill_discount = ?, service = ?, tax = ?, total = ?, closed_at = ?
WHERE id = ?;

-- ---------------------------------------------------------------- Payments (BR-R-18, BR-R-19, BR-R-30)

-- name: CreatePayment :exec
INSERT INTO payment (id, bill_id, sub_bill_id, method, status, amount, tendered, change_given, rounding, taken_by, taken_at, override_by,
                     verified_by, verified_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListPaymentsByBill :many
SELECT * FROM payment WHERE bill_id = ? ORDER BY taken_at;

-- name: CountPaymentsByBill :one
SELECT COUNT(*) FROM payment WHERE bill_id = ?;

-- ---------------------------------------------------------------- Receipts (TR-PRN-08)

-- name: CreateReceiptPrint :exec
INSERT INTO receipt_print (id, bill_id, sub_bill_id, copy, is_reprint, printed_by, printed_at) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: CountReceiptPrints :one
SELECT COUNT(*) FROM receipt_print WHERE bill_id = ? AND copy = ? AND (sub_bill_id IS ? OR sub_bill_id = ?);

-- ---------------------------------------------------------------- Reports (BR-ADM-11)

-- name: ListClosedBillsBetween :many
SELECT * FROM bill WHERE status IN ('closed', 'unpaid') AND closed_at >= ? AND closed_at < ? ORDER BY closed_at;

-- name: ListPaymentsBetween :many
SELECT p.* FROM payment p JOIN bill b ON b.id = p.bill_id
WHERE b.status IN ('closed', 'unpaid') AND b.closed_at >= ? AND b.closed_at < ?;

-- name: ListSoldItemsBetween :many
SELECT i.product_id, i.product_name_id, SUM(i.quantity) AS quantity, SUM(i.quantity * i.unit_price) AS gross
FROM order_item i JOIN orders o ON o.id = i.order_id JOIN bill b ON b.id = o.bill_id
WHERE b.status = 'closed' AND b.closed_at >= ? AND b.closed_at < ? AND i.status NOT IN ('cancelled', 'voided')
GROUP BY i.product_id, i.product_name_id ORDER BY quantity DESC;

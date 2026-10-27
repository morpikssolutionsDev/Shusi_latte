-- ---------------------------------------------------------------- Tables and Bills

-- name: GetTableByQR :one
SELECT * FROM dining_table WHERE qr_token = ?;

-- name: GetOpenBillForTable :one
SELECT b.* FROM bill b JOIN bill_table bt ON bt.bill_id = b.id
WHERE bt.table_id = ? AND bt.is_open = 1;

-- name: NextBillNumber :one
SELECT CAST(COALESCE(MAX(number), 0) + 1 AS INTEGER) FROM bill WHERE business_date = ?;

-- name: CreateBill :exec
INSERT INTO bill (id, business_date, number, type, source, status, key_id, takeaway_name_enc, opened_at, updated_at)
VALUES (?, ?, ?, ?, ?, 'open', ?, ?, ?, ?);

-- name: AddBillTable :exec
INSERT INTO bill_table (bill_id, table_id, is_open) VALUES (?, ?, 1);

-- name: GetBill :one
SELECT * FROM bill WHERE id = ?;

-- name: ListBillTables :many
SELECT t.id, t.number, t.label FROM bill_table bt JOIN dining_table t ON t.id = bt.table_id
WHERE bt.bill_id = ? ORDER BY t.number;

-- name: SetBillStatus :exec
UPDATE bill SET status = ?, closed_at = ?, updated_at = ? WHERE id = ?;

-- name: SetBillTablesOpen :exec
UPDATE bill_table SET is_open = ? WHERE bill_id = ?;

-- ---------------------------------------------------------------- Customers

-- name: CreateCustomerEntry :exec
INSERT INTO customer_entry (id, bill_id, key_id, name_enc, dietary_note_enc, restriction_tags_enc, consent_at, active_device_id, joined_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetCustomerEntry :one
SELECT * FROM customer_entry WHERE id = ?;

-- name: ListCustomerEntries :many
SELECT * FROM customer_entry WHERE bill_id = ? ORDER BY joined_at, id;

-- name: SetCustomerDevice :exec
UPDATE customer_entry SET active_device_id = ?, rejoined_at = ? WHERE id = ?;

-- name: CreateGuestToken :exec
INSERT INTO guest_token (id, customer_entry_id, token_hash, created_at) VALUES (?, ?, ?, ?);

-- name: RevokeGuestTokens :exec
UPDATE guest_token SET revoked_at = ? WHERE customer_entry_id = ? AND revoked_at IS NULL;

-- name: GetGuestByTokenHash :one
SELECT g.id AS token_id, g.revoked_at, c.id AS customer_entry_id, c.bill_id, b.status AS bill_status
FROM guest_token g
JOIN customer_entry c ON c.id = g.customer_entry_id
JOIN bill b ON b.id = c.bill_id
WHERE g.token_hash = ?;

-- ---------------------------------------------------------------- Orders

-- name: NextOrderSeq :one
SELECT CAST(COALESCE(MAX(seq), 0) + 1 AS INTEGER) FROM orders WHERE bill_id = ?;

-- name: CreateOrder :exec
INSERT INTO orders (id, bill_id, seq, customer_entry_id, source, status, placed_by, key_id, restriction_tags_enc, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetOrder :one
SELECT * FROM orders WHERE id = ?;

-- name: ListOrdersByBill :many
SELECT * FROM orders WHERE bill_id = ? ORDER BY seq;

-- name: SetOrderStatus :exec
UPDATE orders SET status = ?, updated_at = ? WHERE id = ?;

-- name: TouchOrder :exec
UPDATE orders SET updated_at = ? WHERE id = ?;

-- name: CreateOrderItem :exec
INSERT INTO order_item (id, order_id, sort, product_id, variant_id, category_id, prep_area_id, product_name_id, product_name_en,
                        variant_name_id, variant_name_en, unit_price, prep_time_seconds, quantity, note, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListOrderItemsByBill :many
SELECT i.* FROM order_item i JOIN orders o ON o.id = i.order_id WHERE o.bill_id = ? ORDER BY o.seq, i.sort;

-- name: ListOrderItemsByOrder :many
SELECT * FROM order_item WHERE order_id = ? ORDER BY sort;

-- name: UpdatePendingItem :execrows
UPDATE order_item SET quantity = ?, note = ?, updated_at = ? WHERE id = ? AND status = 'pending';

-- name: CancelItem :execrows
UPDATE order_item
SET status = 'cancelled', cancel_reason = ?, cancel_note = ?, cancelled_by = ?, cancelled_at = ?, updated_at = ?
WHERE id = ? AND status = ?;

-- name: CreateOrderItemModifier :exec
INSERT INTO order_item_modifier (order_item_id, modifier_id, name_id, name_en, price) VALUES (?, ?, ?, ?, ?);

-- name: ListOrderItemModifiersByBill :many
SELECT m.* FROM order_item_modifier m
JOIN order_item i ON i.id = m.order_item_id JOIN orders o ON o.id = i.order_id
WHERE o.bill_id = ? ORDER BY m.order_item_id, m.price DESC, m.name_id;

-- name: CreateDietaryWarning :exec
INSERT INTO order_item_dietary_warning (order_item_id, dietary_tag_id, acknowledged_at) VALUES (?, ?, ?);

-- name: ListDietaryWarningsByBill :many
SELECT w.* FROM order_item_dietary_warning w
JOIN order_item i ON i.id = w.order_item_id JOIN orders o ON o.id = i.order_id
WHERE o.bill_id = ?;

-- ---------------------------------------------------------------- Waiter calls

-- name: CreateWaiterCall :exec
INSERT INTO waiter_call (id, kind, table_id, bill_id, customer_entry_id, created_at) VALUES (?, ?, ?, ?, ?, ?);

-- name: GetOpenWaiterCall :one
SELECT * FROM waiter_call WHERE customer_entry_id = ? AND kind = 'call' AND answered_at IS NULL
ORDER BY created_at DESC LIMIT 1;

-- ---------------------------------------------------------------- Kitchen queue (Estimated Wait)

-- name: ListQueueItems :many
SELECT i.id, i.order_id, i.prep_area_id, i.prep_time_seconds, i.quantity, i.status, i.preparing_at, i.remake_of_id,
       o.confirmed_at
FROM order_item i JOIN orders o ON o.id = i.order_id
WHERE i.status IN ('confirmed', 'preparing');

-- ---------------------------------------------------------------- Jobs

-- An open Bill with no live Order and no Order activity since the cutoff (BR-R-26).
-- name: ListEmptyBillsBefore :many
SELECT b.id FROM bill b
WHERE b.status = 'open' AND b.type = 'dine_in' AND b.opened_at < sqlc.arg(cutoff)
  AND NOT EXISTS (SELECT 1 FROM orders o WHERE o.bill_id = b.id AND (o.status <> 'cancelled' OR o.updated_at >= sqlc.arg(cutoff)));

-- name: ListBillsToAnonymise :many
SELECT id FROM bill WHERE status IN ('closed', 'unpaid', 'merged') AND anonymised_at IS NULL AND closed_at < ?;

-- name: AnonymiseCustomers :exec
UPDATE customer_entry SET name_enc = NULL, dietary_note_enc = NULL, restriction_tags_enc = NULL WHERE bill_id = ?;

-- name: AnonymiseOrders :exec
UPDATE orders SET restriction_tags_enc = NULL WHERE bill_id = ?;

-- name: MarkBillAnonymised :exec
UPDATE bill SET takeaway_name_enc = NULL, anonymised_at = ?, updated_at = ? WHERE id = ?;

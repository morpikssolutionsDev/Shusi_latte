-- ---------------------------------------------------------------- Tables and Bills

-- name: GetTableByCode :one
SELECT * FROM dining_table WHERE code = $1;

-- name: GetOpenBillForTable :one
SELECT b.* FROM bill b JOIN bill_table bt ON bt.bill_id = b.id
WHERE bt.table_id = $1 AND bt.is_open = 1;

-- name: NextBillNumber :one
SELECT CAST(COALESCE(MAX(number), 0) + 1 AS BIGINT) FROM bill WHERE business_date = $1;

-- name: CreateBill :exec
INSERT INTO bill (id, business_date, number, type, source, status, key_id, takeaway_name_enc, opened_at, updated_at)
VALUES ($1, $2, $3, $4, $5, 'open', $6, $7, $8, $9);

-- name: AddBillTable :exec
INSERT INTO bill_table (bill_id, table_id, is_open) VALUES ($1, $2, 1);

-- name: GetBill :one
SELECT * FROM bill WHERE id = $1;

-- name: ListBillTables :many
SELECT t.id, t.number, t.label FROM bill_table bt JOIN dining_table t ON t.id = bt.table_id
WHERE bt.bill_id = $1 ORDER BY t.number;

-- name: SetBillStatus :exec
UPDATE bill SET status = $1, closed_at = $2, updated_at = $3 WHERE id = $4;

-- name: SetBillTablesOpen :exec
UPDATE bill_table SET is_open = $1 WHERE bill_id = $2;

-- ---------------------------------------------------------------- Customers

-- name: CreateCustomerEntry :exec
INSERT INTO customer_entry (id, bill_id, key_id, name_enc, dietary_note_enc, restriction_tags_enc, consent_at, active_device_id, joined_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: GetCustomerEntry :one
SELECT * FROM customer_entry WHERE id = $1;

-- name: ListCustomerEntries :many
SELECT * FROM customer_entry WHERE bill_id = $1 ORDER BY joined_at, id;

-- name: SetCustomerDevice :exec
UPDATE customer_entry SET active_device_id = $1, rejoined_at = $2 WHERE id = $3;

-- name: CreateGuestToken :exec
INSERT INTO guest_token (id, customer_entry_id, token_hash, created_at) VALUES ($1, $2, $3, $4);

-- name: RevokeGuestTokens :exec
UPDATE guest_token SET revoked_at = $1 WHERE customer_entry_id = $2 AND revoked_at IS NULL;

-- name: GetGuestByTokenHash :one
SELECT g.id AS token_id, g.revoked_at, c.id AS customer_entry_id, c.bill_id, b.status AS bill_status
FROM guest_token g
JOIN customer_entry c ON c.id = g.customer_entry_id
JOIN bill b ON b.id = c.bill_id
WHERE g.token_hash = $1;

-- ---------------------------------------------------------------- Orders

-- name: NextOrderSeq :one
SELECT CAST(COALESCE(MAX(seq), 0) + 1 AS BIGINT) FROM orders WHERE bill_id = $1;

-- name: CreateOrder :exec
INSERT INTO orders (id, bill_id, seq, customer_entry_id, source, status, placed_by, key_id, restriction_tags_enc, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: GetOrder :one
SELECT * FROM orders WHERE id = $1;

-- name: ListOrdersByBill :many
SELECT * FROM orders WHERE bill_id = $1 ORDER BY seq;

-- name: SetOrderStatus :exec
UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3;

-- name: TouchOrder :exec
UPDATE orders SET updated_at = $1 WHERE id = $2;

-- name: CreateOrderItem :exec
INSERT INTO order_item (id, order_id, sort, product_id, variant_id, category_id, prep_area_id, product_name_id, product_name_en,
                        variant_name_id, variant_name_en, unit_price, prep_time_seconds, quantity, note, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18);

-- name: ListOrderItemsByBill :many
SELECT i.* FROM order_item i JOIN orders o ON o.id = i.order_id WHERE o.bill_id = $1 ORDER BY o.seq, i.sort;

-- name: ListOrderItemsByOrder :many
SELECT * FROM order_item WHERE order_id = $1 ORDER BY sort;

-- name: UpdatePendingItem :execrows
UPDATE order_item SET quantity = $1, note = $2, updated_at = $3 WHERE id = $4 AND status = 'pending';

-- name: CancelItem :execrows
UPDATE order_item
SET status = 'cancelled', cancel_reason = $1, cancel_note = $2, cancelled_by = $3, cancelled_at = $4, updated_at = $5
WHERE id = $6 AND status = $7;

-- name: CreateOrderItemModifier :exec
INSERT INTO order_item_modifier (order_item_id, modifier_id, name_id, name_en, price) VALUES ($1, $2, $3, $4, $5);

-- name: ListOrderItemModifiersByBill :many
SELECT m.* FROM order_item_modifier m
JOIN order_item i ON i.id = m.order_item_id JOIN orders o ON o.id = i.order_id
WHERE o.bill_id = $1 ORDER BY m.order_item_id, m.price DESC, m.name_id;

-- name: CreateDietaryWarning :exec
INSERT INTO order_item_dietary_warning (order_item_id, dietary_tag_id, acknowledged_at) VALUES ($1, $2, $3);

-- name: ListDietaryWarningsByBill :many
SELECT w.* FROM order_item_dietary_warning w
JOIN order_item i ON i.id = w.order_item_id JOIN orders o ON o.id = i.order_id
WHERE o.bill_id = $1;

-- ---------------------------------------------------------------- Waiter calls

-- name: CreateWaiterCall :exec
INSERT INTO waiter_call (id, kind, table_id, bill_id, customer_entry_id, created_at) VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetOpenWaiterCall :one
SELECT * FROM waiter_call WHERE customer_entry_id = $1 AND kind = 'call' AND answered_at IS NULL
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
SELECT id FROM bill WHERE status IN ('closed', 'unpaid', 'merged') AND anonymised_at IS NULL AND closed_at < $1;

-- name: AnonymiseCustomers :exec
UPDATE customer_entry SET name_enc = NULL, dietary_note_enc = NULL, restriction_tags_enc = NULL WHERE bill_id = $1;

-- name: AnonymiseOrders :exec
UPDATE orders SET restriction_tags_enc = NULL WHERE bill_id = $1;

-- name: MarkBillAnonymised :exec
UPDATE bill SET takeaway_name_enc = NULL, anonymised_at = $1, updated_at = $2 WHERE id = $3;

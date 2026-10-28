-- ---------------------------------------------------------------- Claim (TR-SM-02, TR-SM-03)

-- First writer wins: the losing Waiter updates nothing.
-- name: ClaimOrder :execrows
UPDATE orders SET claimed_by = $1, claimed_at = $2, status = 'claimed', updated_at = $3
WHERE id = $4 AND claimed_by IS NULL AND status = 'pending';

-- name: SetOrderClaimant :exec
UPDATE orders SET claimed_by = $1, updated_at = $2 WHERE id = $3;

-- name: CreateClaimTransfer :exec
INSERT INTO claim_transfer (id, order_id, from_crew_id, to_crew_id, kind, by_crew_id, at) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ConfirmOrder :execrows
UPDATE orders SET status = 'confirmed', confirmed_by = $1, confirmed_at = $2, updated_at = $3
WHERE id = $4 AND status = 'claimed';

-- name: ConfirmOrderItems :exec
UPDATE order_item SET status = 'confirmed', updated_at = $1 WHERE order_id = $2 AND status = 'pending';

-- name: CreateConfirmedOrder :exec
INSERT INTO orders (id, bill_id, seq, customer_entry_id, source, status, placed_by, claimed_by, claimed_at, confirmed_by,
                    confirmed_at, key_id, restriction_tags_enc, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, 'confirmed', $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- ---------------------------------------------------------------- Order Item transitions (TR-SM-01)

-- name: GetOrderItem :one
SELECT * FROM order_item WHERE id = $1;

-- name: ServeItem :execrows
UPDATE order_item SET status = 'served', served_at = $1, served_by = $2, updated_at = $3 WHERE id = $4 AND status = 'ready';

-- name: VoidItem :execrows
UPDATE order_item SET status = 'voided', void_reason = $1, voided_by = $2, voided_override_by = $3, voided_at = $4, updated_at = $5
WHERE id = $6 AND status IN ('ready', 'served');

-- name: StartItem :execrows
UPDATE order_item SET status = 'preparing', preparing_at = $1, updated_at = $2 WHERE id = $3 AND status = 'confirmed';

-- name: ReadyItem :execrows
UPDATE order_item SET status = 'ready', ready_at = $1, updated_at = $2 WHERE id = $3 AND status = 'preparing';

-- A Remake is a no-charge copy that starts Confirmed (TR-DM-07).
-- name: CreateRemakeItem :exec
INSERT INTO order_item (id, order_id, sort, product_id, variant_id, category_id, prep_area_id, product_name_id, product_name_en,
                        variant_name_id, variant_name_en, unit_price, prep_time_seconds, quantity, note, status,
                        remake_of_id, remake_reason, remake_override_by, created_at, updated_at)
SELECT sqlc.arg(new_id), o.order_id, (SELECT MAX(sort) + 1 FROM order_item WHERE order_id = o.order_id), o.product_id, o.variant_id,
       o.category_id, o.prep_area_id, o.product_name_id, o.product_name_en, o.variant_name_id, o.variant_name_en, 0,
       o.prep_time_seconds, o.quantity, o.note, 'confirmed', o.id, sqlc.arg(reason), sqlc.arg(override_by), sqlc.arg(now), sqlc.arg(now)
FROM order_item o WHERE o.id = sqlc.arg(original_id);

-- name: CopyRemakeModifiers :exec
INSERT INTO order_item_modifier (order_item_id, modifier_id, name_id, name_en, price)
SELECT sqlc.arg(new_id), m.modifier_id, m.name_id, m.name_en, 0 FROM order_item_modifier m WHERE m.order_item_id = sqlc.arg(original_id);

-- name: CopyRemakeWarnings :exec
INSERT INTO order_item_dietary_warning (order_item_id, dietary_tag_id, acknowledged_at)
SELECT sqlc.arg(new_id), w.dietary_tag_id, w.acknowledged_at FROM order_item_dietary_warning w WHERE w.order_item_id = sqlc.arg(original_id);

-- ---------------------------------------------------------------- flags (TR-SM-04, TR-SM-06)

-- name: ListOpenItemOrdersByVariant :many
SELECT DISTINCT order_id FROM order_item WHERE variant_id = $1 AND status IN ('pending', 'confirmed');

-- name: FlagSoldOutItems :exec
UPDATE order_item SET sold_out_flag = 1, updated_at = $1 WHERE variant_id = $2 AND status IN ('pending', 'confirmed');

-- name: ListOpenItemOrdersByProduct :many
SELECT DISTINCT order_id FROM order_item WHERE product_id = $1 AND status IN ('pending', 'confirmed');

-- name: FlagUnavailableItems :exec
UPDATE order_item SET unavailable_flag = 1, updated_at = $1 WHERE product_id = $2 AND status IN ('pending', 'confirmed');

-- ---------------------------------------------------------------- escalation and reminders (BR-WT-09, BR-WT-10)

-- name: ListOrdersToRenotify :many
SELECT * FROM orders WHERE status = 'pending' AND claimed_by IS NULL AND renotified_at IS NULL AND created_at < $1;

-- name: SetOrderRenotified :exec
UPDATE orders SET renotified_at = $1 WHERE id = $2;

-- name: ListOrdersToEscalate :many
SELECT * FROM orders WHERE status = 'pending' AND claimed_by IS NULL AND escalated_at IS NULL AND created_at < $1;

-- name: SetOrderEscalated :exec
UPDATE orders SET escalated_at = $1 WHERE id = $2;

-- name: ListCallsToRenotify :many
SELECT * FROM waiter_call WHERE answered_at IS NULL AND renotified_at IS NULL AND created_at < $1;

-- name: SetCallRenotified :exec
UPDATE waiter_call SET renotified_at = $1 WHERE id = $2;

-- name: ListCallsToEscalate :many
SELECT * FROM waiter_call WHERE answered_at IS NULL AND escalated_at IS NULL AND created_at < $1;

-- name: SetCallEscalated :exec
UPDATE waiter_call SET escalated_at = $1 WHERE id = $2;

-- name: AnswerWaiterCall :execrows
UPDATE waiter_call SET answered_by = $1, answered_at = $2 WHERE id = $3 AND answered_at IS NULL;

-- name: ListOpenWaiterCalls :many
SELECT * FROM waiter_call WHERE answered_at IS NULL ORDER BY created_at;

-- name: ListItemsToRemind :many
SELECT i.id, i.order_id, o.bill_id, o.claimed_by FROM order_item i JOIN orders o ON o.id = i.order_id
WHERE i.status = 'ready' AND i.pickup_reminded_at IS NULL AND i.ready_at < $1;

-- name: SetPickupReminded :exec
UPDATE order_item SET pickup_reminded_at = $1 WHERE id = $2;

-- ---------------------------------------------------------------- Waiter views

-- name: ListOpenBills :many
SELECT * FROM bill WHERE status IN ('open', 'settling') ORDER BY opened_at;

-- name: ListOpenBillTables :many
SELECT bt.bill_id, t.id AS table_id, t.number, t.label FROM bill_table bt JOIN dining_table t ON t.id = bt.table_id
WHERE bt.is_open = 1;

-- name: ListClaimTransfers :many
SELECT * FROM claim_transfer WHERE order_id = $1 ORDER BY at;

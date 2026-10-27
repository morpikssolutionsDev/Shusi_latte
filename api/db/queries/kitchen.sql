-- Kitchen queue (TR-KT-01): Confirmed and Preparing items, plus Ready items waiting for pickup.
-- name: ListKitchenItems :many
SELECT i.*, o.bill_id, o.seq AS order_seq, o.confirmed_at AS order_confirmed_at, o.claimed_by, o.customer_entry_id
FROM order_item i JOIN orders o ON o.id = i.order_id
WHERE i.prep_area_id = ? AND i.status IN ('confirmed', 'preparing', 'ready')
ORDER BY o.confirmed_at, o.seq, i.sort;

-- Bills whose Estimated Wait depends on a Prep Area's queue (TR-KT-04).
-- name: ListQueueBills :many
SELECT DISTINCT o.bill_id FROM order_item i JOIN orders o ON o.id = i.order_id
WHERE i.prep_area_id = ? AND i.status IN ('confirmed', 'preparing');

-- name: GetPrepArea :one
SELECT * FROM prep_area WHERE id = ?;

-- Prep Time performance (TR-KT-05): items made Ready in a window, with their target.
-- name: ListReadyItemsBetween :many
SELECT i.product_id, i.product_name_id, i.category_id, i.prep_area_id, i.prep_time_seconds, i.ready_at, o.confirmed_at
FROM order_item i JOIN orders o ON o.id = i.order_id
WHERE i.ready_at >= ? AND i.ready_at < ? AND o.confirmed_at IS NOT NULL;

-- name: GetPrepAreaByCode :one
SELECT * FROM prep_area WHERE code = ?;

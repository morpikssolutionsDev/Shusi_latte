-- name: CreateSlide :exec
INSERT INTO display_slide (id, template, sort, active, headline_id, headline_en, subline_id, subline_en, image_key, video_key, image_focus_x,
                           image_focus_y, product_id, discount_id, category_ids, steam, duration_seconds, active_from, active_to,
                           created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateSlide :exec
UPDATE display_slide
SET template = ?, sort = ?, active = ?, headline_id = ?, headline_en = ?, subline_id = ?, subline_en = ?, image_key = ?, video_key = ?,
    image_focus_x = ?, image_focus_y = ?, product_id = ?, discount_id = ?, category_ids = ?, steam = ?, duration_seconds = ?,
    active_from = ?, active_to = ?, updated_at = ?
WHERE id = ?;

-- name: GetSlide :one
SELECT * FROM display_slide WHERE id = ?;

-- name: ListSlides :many
SELECT * FROM display_slide ORDER BY sort, created_at;

-- name: DeleteSlide :exec
DELETE FROM display_slide WHERE id = ?;

-- name: SetSlideSort :exec
UPDATE display_slide SET sort = ?, updated_at = ? WHERE id = ?;

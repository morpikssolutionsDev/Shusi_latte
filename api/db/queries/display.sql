-- name: CreateSlide :exec
INSERT INTO display_slide (id, template, sort, active, headline_id, headline_en, subline_id, subline_en, image_key, video_key, image_focus_x,
                           image_focus_y, product_id, discount_id, category_ids, steam, duration_seconds, active_from, active_to,
                           created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21);

-- name: UpdateSlide :exec
UPDATE display_slide
SET template = $1, sort = $2, active = $3, headline_id = $4, headline_en = $5, subline_id = $6, subline_en = $7, image_key = $8, video_key = $9,
    image_focus_x = $10, image_focus_y = $11, product_id = $12, discount_id = $13, category_ids = $14, steam = $15, duration_seconds = $16,
    active_from = $17, active_to = $18, updated_at = $19
WHERE id = $20;

-- name: GetSlide :one
SELECT * FROM display_slide WHERE id = $1;

-- name: ListSlides :many
SELECT * FROM display_slide ORDER BY sort, created_at;

-- name: DeleteSlide :exec
DELETE FROM display_slide WHERE id = $1;

-- name: SetSlideSort :exec
UPDATE display_slide SET sort = $1, updated_at = $2 WHERE id = $3;

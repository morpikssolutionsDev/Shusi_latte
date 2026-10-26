-- Prep Areas ----------------------------------------------------------------

-- name: CreatePrepArea :exec
INSERT INTO prep_area (id, code, name_id, name_en, parallel_capacity, sort) VALUES (?, ?, ?, ?, ?, ?);

-- name: ListPrepAreas :many
SELECT * FROM prep_area ORDER BY sort, code;

-- name: UpdatePrepAreaCapacity :exec
UPDATE prep_area SET parallel_capacity = ? WHERE id = ?;

-- Categories ----------------------------------------------------------------

-- name: CreateCategory :exec
INSERT INTO category (id, name_id, name_en, prep_area_id, sort, active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateCategory :exec
UPDATE category SET name_id = ?, name_en = ?, prep_area_id = ?, sort = ?, active = ?, updated_at = ? WHERE id = ?;

-- name: GetCategory :one
SELECT * FROM category WHERE id = ?;

-- name: ListCategories :many
SELECT * FROM category ORDER BY sort, name_en;

-- name: CountProductsInCategory :one
SELECT COUNT(*) FROM product WHERE category_id = ? AND status <> 'archived';

-- Products -------------------------------------------------------------------

-- name: CreateProduct :exec
INSERT INTO product (id, category_id, name_id, name_en, description_id, description_en, image_key, image_focus_x,
                     image_focus_y, prep_time_seconds, status, sort, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateProduct :exec
UPDATE product
SET category_id = ?, name_id = ?, name_en = ?, description_id = ?, description_en = ?, image_key = ?,
    image_focus_x = ?, image_focus_y = ?, prep_time_seconds = ?, status = ?, sort = ?, updated_at = ?
WHERE id = ?;

-- name: GetProduct :one
SELECT * FROM product WHERE id = ?;

-- name: ListProducts :many
SELECT * FROM product ORDER BY sort, name_en;

-- name: MaxProductSort :one
SELECT CAST(COALESCE(MAX(sort), 0) AS INTEGER) FROM product WHERE category_id = ?;

-- Variants -------------------------------------------------------------------

-- name: CreateVariant :exec
INSERT INTO variant (id, product_id, name_id, name_en, price, is_default, sort) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateVariant :exec
UPDATE variant SET name_id = ?, name_en = ?, price = ?, is_default = ?, sort = ? WHERE id = ?;

-- name: ArchiveVariant :exec
UPDATE variant SET archived_at = ?, is_default = 0 WHERE id = ?;

-- name: GetVariant :one
SELECT * FROM variant WHERE id = ?;

-- name: ListVariants :many
SELECT * FROM variant WHERE archived_at IS NULL ORDER BY product_id, sort;

-- name: ListProductVariants :many
SELECT * FROM variant WHERE product_id = ? AND archived_at IS NULL ORDER BY sort;

-- name: SetVariantSoldOut :exec
UPDATE variant SET sold_out = 1, sold_out_at = ?, sold_out_by = ?, sold_out_source = ? WHERE id = ?;

-- name: ClearVariantSoldOut :exec
UPDATE variant SET sold_out = 0, sold_out_at = NULL, sold_out_by = NULL, sold_out_source = NULL WHERE id = ?;

-- Modifiers ------------------------------------------------------------------

-- name: CreateModifierGroup :exec
INSERT INTO modifier_group (id, product_id, name_id, name_en, min_select, max_select, sort) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateModifierGroup :exec
UPDATE modifier_group SET name_id = ?, name_en = ?, min_select = ?, max_select = ?, sort = ? WHERE id = ?;

-- name: ArchiveModifierGroup :exec
UPDATE modifier_group SET archived_at = ? WHERE id = ?;

-- name: ListModifierGroups :many
SELECT * FROM modifier_group WHERE archived_at IS NULL ORDER BY product_id, sort;

-- name: ListProductModifierGroups :many
SELECT * FROM modifier_group WHERE product_id = ? AND archived_at IS NULL ORDER BY sort;

-- name: CreateModifier :exec
INSERT INTO modifier (id, modifier_group_id, name_id, name_en, price, sort) VALUES (?, ?, ?, ?, ?, ?);

-- name: UpdateModifier :exec
UPDATE modifier SET name_id = ?, name_en = ?, price = ?, sort = ? WHERE id = ?;

-- name: ArchiveModifier :exec
UPDATE modifier SET archived_at = ? WHERE id = ?;

-- name: ListModifiers :many
SELECT * FROM modifier WHERE archived_at IS NULL ORDER BY modifier_group_id, sort;

-- name: ListGroupModifiers :many
SELECT * FROM modifier WHERE modifier_group_id = ? AND archived_at IS NULL ORDER BY sort;

-- Dietary Tags ---------------------------------------------------------------

-- name: CreateDietaryTag :exec
INSERT INTO dietary_tag (id, code, name_id, name_en, icon, sort, active) VALUES (?, ?, ?, ?, ?, ?, 1);

-- name: UpdateDietaryTag :exec
UPDATE dietary_tag SET name_id = ?, name_en = ?, icon = ?, sort = ?, active = ? WHERE id = ?;

-- name: ListDietaryTags :many
SELECT * FROM dietary_tag ORDER BY sort, code;

-- name: ListProductDietaryTags :many
SELECT * FROM product_dietary_tag;

-- name: ClearProductDietaryTags :exec
DELETE FROM product_dietary_tag WHERE product_id = ?;

-- name: AddProductDietaryTag :exec
INSERT INTO product_dietary_tag (product_id, dietary_tag_id) VALUES (?, ?);

-- name: ListModifierDietaryTags :many
SELECT * FROM modifier_dietary_tag;

-- name: ClearModifierDietaryTags :exec
DELETE FROM modifier_dietary_tag WHERE modifier_id = ?;

-- name: AddModifierDietaryTag :exec
INSERT INTO modifier_dietary_tag (modifier_id, dietary_tag_id) VALUES (?, ?);

-- Prep Areas ----------------------------------------------------------------

-- name: CreatePrepArea :exec
INSERT INTO prep_area (id, code, name_id, name_en, parallel_capacity, sort) VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListPrepAreas :many
SELECT * FROM prep_area ORDER BY sort, code;

-- name: UpdatePrepAreaCapacity :exec
UPDATE prep_area SET parallel_capacity = $1 WHERE id = $2;

-- Categories ----------------------------------------------------------------

-- name: CreateCategory :exec
INSERT INTO category (id, name_id, name_en, prep_area_id, sort, active, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: UpdateCategory :exec
UPDATE category SET name_id = $1, name_en = $2, prep_area_id = $3, sort = $4, active = $5, updated_at = $6 WHERE id = $7;

-- name: GetCategory :one
SELECT * FROM category WHERE id = $1;

-- name: ListCategories :many
SELECT * FROM category ORDER BY sort, name_en;

-- name: CountProductsInCategory :one
SELECT COUNT(*) FROM product WHERE category_id = $1 AND status <> 'archived';

-- Products -------------------------------------------------------------------

-- name: CreateProduct :exec
INSERT INTO product (id, category_id, name_id, name_en, description_id, description_en, image_key, image_focus_x,
                     image_focus_y, prep_time_seconds, status, sort, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: UpdateProduct :exec
UPDATE product
SET category_id = $1, name_id = $2, name_en = $3, description_id = $4, description_en = $5, image_key = $6,
    image_focus_x = $7, image_focus_y = $8, prep_time_seconds = $9, status = $10, sort = $11, updated_at = $12
WHERE id = $13;

-- name: GetProduct :one
SELECT * FROM product WHERE id = $1;

-- name: ListProducts :many
SELECT * FROM product ORDER BY sort, name_en;

-- name: MaxProductSort :one
SELECT CAST(COALESCE(MAX(sort), 0) AS BIGINT) FROM product WHERE category_id = $1;

-- Variants -------------------------------------------------------------------

-- name: CreateVariant :exec
INSERT INTO variant (id, product_id, name_id, name_en, price, is_default, sort) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpdateVariant :exec
UPDATE variant SET name_id = $1, name_en = $2, price = $3, is_default = $4, sort = $5 WHERE id = $6;

-- name: ArchiveVariant :exec
UPDATE variant SET archived_at = $1, is_default = 0 WHERE id = $2;

-- name: GetVariant :one
SELECT * FROM variant WHERE id = $1;

-- name: ListVariants :many
SELECT * FROM variant WHERE archived_at IS NULL ORDER BY product_id, sort;

-- name: ListProductVariants :many
SELECT * FROM variant WHERE product_id = $1 AND archived_at IS NULL ORDER BY sort;

-- name: SetVariantSoldOut :exec
UPDATE variant SET sold_out = 1, sold_out_at = $1, sold_out_by = $2, sold_out_source = $3 WHERE id = $4;

-- name: ClearVariantSoldOut :exec
UPDATE variant SET sold_out = 0, sold_out_at = NULL, sold_out_by = NULL, sold_out_source = NULL WHERE id = $1;

-- Modifiers ------------------------------------------------------------------

-- name: CreateModifierGroup :exec
INSERT INTO modifier_group (id, product_id, name_id, name_en, min_select, max_select, sort) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpdateModifierGroup :exec
UPDATE modifier_group SET name_id = $1, name_en = $2, min_select = $3, max_select = $4, sort = $5 WHERE id = $6;

-- name: ArchiveModifierGroup :exec
UPDATE modifier_group SET archived_at = $1 WHERE id = $2;

-- name: ListModifierGroups :many
SELECT * FROM modifier_group WHERE archived_at IS NULL ORDER BY product_id, sort;

-- name: ListProductModifierGroups :many
SELECT * FROM modifier_group WHERE product_id = $1 AND archived_at IS NULL ORDER BY sort;

-- name: CreateModifier :exec
INSERT INTO modifier (id, modifier_group_id, name_id, name_en, price, sort) VALUES ($1, $2, $3, $4, $5, $6);

-- name: UpdateModifier :exec
UPDATE modifier SET name_id = $1, name_en = $2, price = $3, sort = $4 WHERE id = $5;

-- name: ArchiveModifier :exec
UPDATE modifier SET archived_at = $1 WHERE id = $2;

-- name: ListModifiers :many
SELECT * FROM modifier WHERE archived_at IS NULL ORDER BY modifier_group_id, sort;

-- name: ListGroupModifiers :many
SELECT * FROM modifier WHERE modifier_group_id = $1 AND archived_at IS NULL ORDER BY sort;

-- Dietary Tags ---------------------------------------------------------------

-- name: CreateDietaryTag :exec
INSERT INTO dietary_tag (id, code, name_id, name_en, icon, sort, active) VALUES ($1, $2, $3, $4, $5, $6, 1);

-- name: UpdateDietaryTag :exec
UPDATE dietary_tag SET name_id = $1, name_en = $2, icon = $3, sort = $4, active = $5 WHERE id = $6;

-- name: ListDietaryTags :many
SELECT * FROM dietary_tag ORDER BY sort, code;

-- name: ListProductDietaryTags :many
SELECT * FROM product_dietary_tag;

-- name: ClearProductDietaryTags :exec
DELETE FROM product_dietary_tag WHERE product_id = $1;

-- name: AddProductDietaryTag :exec
INSERT INTO product_dietary_tag (product_id, dietary_tag_id) VALUES ($1, $2);

-- name: ListModifierDietaryTags :many
SELECT * FROM modifier_dietary_tag;

-- name: ClearModifierDietaryTags :exec
DELETE FROM modifier_dietary_tag WHERE modifier_id = $1;

-- name: AddModifierDietaryTag :exec
INSERT INTO modifier_dietary_tag (modifier_id, dietary_tag_id) VALUES ($1, $2);

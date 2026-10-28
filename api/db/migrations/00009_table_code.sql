-- Static Table QR codes (BR-ADM-08, decided 2026-10-28): every Table is reached at /table-NN, a
-- code derived from its number, so a printed QR never changes and cannot be regenerated.

-- +goose Up
ALTER TABLE dining_table RENAME COLUMN qr_token TO code;
ALTER TABLE dining_table DROP COLUMN qr_regenerated_at;
UPDATE dining_table SET code = 'table-' || lpad(number::text, 2, '0');

-- +goose Down
ALTER TABLE dining_table ADD COLUMN qr_regenerated_at BIGINT;
ALTER TABLE dining_table RENAME COLUMN code TO qr_token;

-- What the demo seed built (DEMO-AND-SEED §9): the clean start, or the full story with busy Tables
-- and 14 days of history. The launcher shows scenario labels and tours only for the story.

-- +goose Up
CREATE TABLE demo_seed (
    id    BIGINT PRIMARY KEY CHECK (id = 1),
    story BIGINT NOT NULL CHECK (story IN (0, 1)),
    t0    BIGINT NOT NULL
);

-- +goose Down
DROP TABLE demo_seed;

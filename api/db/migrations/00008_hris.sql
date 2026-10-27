-- HRIS: employee records, shifts, attendance and leave (OP-15, TRD §4.5, BR-HR-01..07).

-- +goose Up
-- Employee details beyond sign-in, maintained by Management (BR-HR-01). One row per Crew member.
CREATE TABLE employee (
    crew_id           TEXT PRIMARY KEY REFERENCES crew (id),
    phone             TEXT NOT NULL DEFAULT '',
    employment_type   TEXT NOT NULL DEFAULT 'full_time' CHECK (employment_type IN ('full_time', 'part_time', 'intern')),
    joined_on         TEXT,          -- YYYY-MM-DD
    emergency_contact TEXT NOT NULL DEFAULT '',
    notes             TEXT NOT NULL DEFAULT '',
    updated_at        INTEGER NOT NULL
);

CREATE TABLE shift (
    id         TEXT PRIMARY KEY,
    crew_id    TEXT NOT NULL REFERENCES crew (id),
    starts_at  INTEGER NOT NULL,
    ends_at    INTEGER NOT NULL CHECK (ends_at > starts_at),
    created_by TEXT NOT NULL REFERENCES crew (id),
    created_at INTEGER NOT NULL
);
CREATE INDEX shift_crew_start ON shift (crew_id, starts_at);
CREATE INDEX shift_start ON shift (starts_at);

-- Clock-in and clock-out on the shared HRIS tablet (BR-HR-03). At most one open per Crew member.
CREATE TABLE attendance (
    id                TEXT PRIMARY KEY,
    crew_id           TEXT NOT NULL REFERENCES crew (id),
    shift_id          TEXT REFERENCES shift (id),
    clock_in_at       INTEGER NOT NULL,
    clock_out_at      INTEGER,
    auto_clocked_out  INTEGER NOT NULL DEFAULT 0, -- BR-HR-06
    corrected_by      TEXT REFERENCES crew (id),
    corrected_at      INTEGER,
    correction_reason TEXT
);
CREATE UNIQUE INDEX attendance_one_open ON attendance (crew_id) WHERE clock_out_at IS NULL;
CREATE INDEX attendance_clock_in ON attendance (clock_in_at);

CREATE TABLE leave_request (
    id              TEXT PRIMARY KEY,
    crew_id         TEXT NOT NULL REFERENCES crew (id),
    from_date       TEXT NOT NULL, -- YYYY-MM-DD, inclusive
    to_date         TEXT NOT NULL CHECK (to_date >= from_date),
    type            TEXT NOT NULL CHECK (type IN ('annual', 'sick', 'unpaid')),
    reason          TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    decided_by      TEXT REFERENCES crew (id),
    decided_at      INTEGER,
    decision_reason TEXT,
    created_at      INTEGER NOT NULL
);
CREATE INDEX leave_request_dates ON leave_request (from_date, to_date);

-- +goose Down
DROP TABLE leave_request;
DROP TABLE attendance;
DROP TABLE shift;
DROP TABLE employee;

-- +goose Up
-- HRIS, Phase 7: employees, shifts, attendance, leave.
-- BR-HR-01 to BR-HR-07, TRD §4.5. Crew records from Phase 1
-- (crew, crew_station, session) are the identity records;
-- employee extends them with HR details, linked by the
-- internal crew id. Shift, attendance and leave rows also
-- store the internal crew id, so they join to crew; the API
-- accepts and returns the human Crew ID and resolves it.
-- Attendance correction and auto clock-out are audit-logged
-- (TR-AUD-01).

CREATE TABLE IF NOT EXISTS employee (
    id TEXT PRIMARY KEY,
    crew_id TEXT NOT NULL UNIQUE REFERENCES crew(id),
    hire_date TEXT,
    phone TEXT,
    emergency_contact TEXT,
    employment_type TEXT,
    notes TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_employee_crew ON employee(crew_id);

CREATE TABLE IF NOT EXISTS shift (
    id TEXT PRIMARY KEY,
    crew_id TEXT NOT NULL REFERENCES crew(id),
    starts_at INTEGER NOT NULL,
    ends_at INTEGER NOT NULL,
    CHECK (ends_at > starts_at)
);
CREATE INDEX IF NOT EXISTS idx_shift_crew ON shift(crew_id, starts_at);

CREATE TABLE IF NOT EXISTS attendance (
    id TEXT PRIMARY KEY,
    crew_id TEXT NOT NULL REFERENCES crew(id),
    clock_in_at INTEGER NOT NULL,
    clock_out_at INTEGER,
    auto_clocked_out INTEGER NOT NULL DEFAULT 0 CHECK (auto_clocked_out IN (0, 1)),
    corrected_by TEXT,
    correction_reason TEXT
);
CREATE INDEX IF NOT EXISTS idx_attendance_crew ON attendance(crew_id, clock_in_at);

CREATE TABLE IF NOT EXISTS leave_request (
    id TEXT PRIMARY KEY,
    crew_id TEXT NOT NULL REFERENCES crew(id),
    from_date TEXT NOT NULL,
    to_date TEXT NOT NULL,
    type TEXT NOT NULL,
    reason TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    decided_by TEXT,
    decided_at INTEGER,
    created_at INTEGER NOT NULL,
    CHECK (to_date >= from_date)
);
CREATE INDEX IF NOT EXISTS idx_leave_crew ON leave_request(crew_id, from_date);

-- +goose Down
DROP TABLE IF EXISTS leave_request;
DROP TABLE IF EXISTS attendance;
DROP TABLE IF EXISTS shift;
DROP TABLE IF EXISTS employee;

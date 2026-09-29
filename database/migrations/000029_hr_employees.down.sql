-- Membatalkan 000029.
DROP TRIGGER IF EXISTS trg_employees_sync_version ON employees;
DROP TRIGGER IF EXISTS trg_employees_tombstone ON employees;
DROP TABLE IF EXISTS holidays;
DROP TABLE IF EXISTS work_schedules;
DROP TABLE IF EXISTS employees;

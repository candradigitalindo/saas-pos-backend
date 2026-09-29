-- Membatalkan 000030.
DROP TRIGGER IF EXISTS trg_attendances_sync_version ON attendances;
DROP TRIGGER IF EXISTS trg_attendances_tombstone ON attendances;
DROP TRIGGER IF EXISTS trg_leave_requests_sync_version ON leave_requests;
DROP TRIGGER IF EXISTS trg_leave_requests_tombstone ON leave_requests;

DROP TABLE IF EXISTS leave_balances;
DROP TABLE IF EXISTS leave_requests;
DROP TABLE IF EXISTS attendance_days;
DROP TABLE IF EXISTS attendance_corrections;
DROP TABLE IF EXISTS attendances;

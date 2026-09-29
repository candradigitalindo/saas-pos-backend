-- Membatalkan 000020.
DROP TRIGGER IF EXISTS trg_visits_sync_version ON visits;
DROP TRIGGER IF EXISTS trg_visits_tombstone ON visits;
DROP TABLE IF EXISTS visits;
DROP TABLE IF EXISTS visit_plans;

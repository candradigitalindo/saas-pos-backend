-- Membatalkan 000017.
DROP TRIGGER IF EXISTS trg_deals_sync_version ON deals;
DROP TRIGGER IF EXISTS trg_deals_tombstone ON deals;
DROP TRIGGER IF EXISTS trg_activities_sync_version ON activities;
DROP TRIGGER IF EXISTS trg_activities_tombstone ON activities;

DROP INDEX IF EXISTS idx_customers_tenant_owner;

DROP TABLE IF EXISTS crm_document_counters;
DROP TABLE IF EXISTS activities;
DROP TABLE IF EXISTS deals;
DROP TABLE IF EXISTS pipeline_stages;
DROP TABLE IF EXISTS pipelines;
DROP TABLE IF EXISTS lead_sources;

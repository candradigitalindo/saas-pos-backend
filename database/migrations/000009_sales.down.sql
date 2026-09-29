-- Membatalkan 000009. Runner membungkus dalam satu transaksi.

DROP TABLE IF EXISTS receipt_counters;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS sale_payments;
DROP TABLE IF EXISTS sale_items;
DROP TABLE IF EXISTS sales;
DROP TABLE IF EXISTS cash_movements;
DROP TABLE IF EXISTS shifts;

DROP INDEX IF EXISTS uq_outlets_tenant_code;
ALTER TABLE outlets DROP COLUMN IF EXISTS code;

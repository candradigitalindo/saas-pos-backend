-- Membatalkan 000040. Tagihan ganti paket yang masih terbuka kehilangan paket
-- tujuannya — lunasi atau batalkan dulu sebelum menurunkan migrasi ini.
DROP INDEX IF EXISTS idx_sub_invoices_credit_from;
DROP INDEX IF EXISTS idx_sub_invoices_plan;
ALTER TABLE subscription_invoices
    DROP COLUMN IF EXISTS credit_from_invoice_id,
    DROP COLUMN IF EXISTS credit_amount,
    DROP COLUMN IF EXISTS kind,
    DROP COLUMN IF EXISTS plan_id;

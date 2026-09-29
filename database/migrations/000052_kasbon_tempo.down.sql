-- Membatalkan 000052. Jatuh tempo kasbon yang sudah terisi tetap ada di
-- receivables.due_date (kolom itu milik 000007).
DROP INDEX IF EXISTS idx_receivable_payments_paid_at;
ALTER TABLE receivable_payments
    DROP CONSTRAINT IF EXISTS receivable_payments_drawer_cash,
    DROP CONSTRAINT IF EXISTS receivable_payments_cash_movement_fk,
    DROP COLUMN IF EXISTS note,
    DROP COLUMN IF EXISTS cash_movement_id;
ALTER TABLE customers DROP COLUMN IF EXISTS credit_term_days;

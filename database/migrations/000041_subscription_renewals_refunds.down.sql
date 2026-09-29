-- Membatalkan 000041. Status & rekening tujuan pengembalian hilang; jumlahnya
-- tetap tercatat.
DROP INDEX IF EXISTS idx_sub_refunds_paid_by;
DROP INDEX IF EXISTS idx_sub_refunds_status;
ALTER TABLE subscription_refunds
    DROP COLUMN IF EXISTS payout_reference,
    DROP COLUMN IF EXISTS paid_by,
    DROP COLUMN IF EXISTS paid_at,
    DROP COLUMN IF EXISTS destination_holder,
    DROP COLUMN IF EXISTS destination_account,
    DROP COLUMN IF EXISTS destination_bank,
    DROP COLUMN IF EXISTS status;
DROP TABLE IF EXISTS subscription_notices;

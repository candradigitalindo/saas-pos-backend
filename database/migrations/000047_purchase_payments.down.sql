-- Membatalkan 000047. Catatan pembayaran hilang; paid_amount yang sudah
-- diisi (termasuk "lunas" untuk pembelian lama) TIDAK dikembalikan ke 0.
DROP INDEX IF EXISTS idx_purchases_unpaid;
DROP TABLE IF EXISTS purchase_payments;
ALTER TABLE purchases DROP CONSTRAINT IF EXISTS purchases_paid_amount_range;

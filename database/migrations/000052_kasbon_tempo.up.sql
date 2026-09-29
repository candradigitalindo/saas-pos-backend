-- Migrasi 000052 — kasbon: tempo per pelanggan & setoran tunai ke laci.
--
-- 1. customers.credit_term_days: tempo kasbon pelanggan (hari). Kasbon baru
--    dari kasir mendapat jatuh tempo = tanggal usaha + tempo; 0 = tanpa
--    jatuh tempo (seperti sebelumnya). receivables.due_date sudah ada sejak
--    000007 tetapi tidak pernah diisi — tanpa itu tidak ada kasbon yang bisa
--    "lewat jatuh tempo".
--
-- 2. receivable_payments.cash_movement_id: setoran TUNAI yang diterima di
--    kasir masuk ke laci. Dulu setoran tidak tercatat di laci sama sekali,
--    sehingga saat tutup shift uang fisiknya tampak "lebih" sebesar setoran
--    itu. Kini penerima memilih: masuk laci kasir (dicatat sebagai uang masuk
--    shift yang sedang buka — satu gerakan kas untuk satu setoran, walau
--    setoran itu melunasi beberapa kasbon) atau uang lain (laci tidak
--    berubah). Setoran lama: NULL = tidak lewat laci.

ALTER TABLE customers ADD COLUMN credit_term_days INT NOT NULL DEFAULT 0
    CONSTRAINT customers_credit_term_days_range CHECK (credit_term_days BETWEEN 0 AND 365);

ALTER TABLE receivable_payments
    ADD COLUMN cash_movement_id CHAR(26),
    ADD COLUMN note TEXT,
    ADD CONSTRAINT receivable_payments_cash_movement_fk
        FOREIGN KEY (tenant_id, cash_movement_id) REFERENCES cash_movements (tenant_id, id) ON DELETE RESTRICT,
    -- Hanya uang tunai yang bisa masuk laci.
    ADD CONSTRAINT receivable_payments_drawer_cash CHECK (cash_movement_id IS NULL OR method = 'cash');

-- Riwayat setoran seorang pelanggan & "terakhir bayar" di ringkasan kasbon.
CREATE INDEX idx_receivable_payments_paid_at ON receivable_payments (tenant_id, receivable_id, paid_at DESC);

-- Migrasi 000047 — utang pemasok: pembayaran barang masuk.
--
-- purchases.paid_amount & due_date sudah ada sejak 000010, tetapi tidak ada
-- layar yang mengisinya: setiap barang masuk tercatat "terbayar Rp 0" tanpa
-- jalan untuk melunasinya. Keputusan pemilik produk 2026-09-29:
--
--   1. Pembelian LAMA dianggap LUNAS — dulu memang tidak ada yang melacak
--      utang, jadi daftar utang dimulai bersih; hanya barang masuk baru yang
--      bisa jadi utang.
--   2. Sumber uang dipilih SETIAP pembayaran: "dari laci kasir" (dicatat
--      sebagai uang keluar shift yang sedang buka, supaya hitungan laci tetap
--      cocok) atau "uang lain" (dompet/rekening, laci tidak berubah).
--
--   purchase_payments : catatan tiap pembayaran satu pembelian (saat barang
--                       datang maupun pelunasan kemudian). paid_amount di
--                       purchases tetap angka yang berlaku; tabel ini
--                       jejaknya. Pembayaran dari laci menunjuk gerakan kasnya.

UPDATE purchases SET paid_amount = total WHERE paid_amount < total;

ALTER TABLE purchases ADD CONSTRAINT purchases_paid_amount_range
    CHECK (paid_amount >= 0 AND paid_amount <= total);

CREATE TABLE purchase_payments (
    id               CHAR(26)    PRIMARY KEY,
    tenant_id        CHAR(26)    NOT NULL,
    purchase_id      CHAR(26)    NOT NULL,
    outlet_id        CHAR(26)    NOT NULL,
    amount           BIGINT      NOT NULL CHECK (amount > 0),
    source           TEXT        NOT NULL CHECK (source IN ('drawer', 'other')),
    cash_movement_id CHAR(26),
    note             TEXT,
    paid_at          TIMESTAMPTZ NOT NULL,
    business_date    DATE        NOT NULL,
    created_by       CHAR(26)    NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, purchase_id) REFERENCES purchases (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, cash_movement_id) REFERENCES cash_movements (tenant_id, id) ON DELETE RESTRICT,
    -- Dari laci ⇔ ada gerakan kasnya.
    CHECK ((source = 'drawer') = (cash_movement_id IS NOT NULL))
);
CREATE INDEX idx_purchase_payments_purchase ON purchase_payments (tenant_id, purchase_id);

-- Daftar utang: pembelian yang belum lunas, per outlet.
CREATE INDEX idx_purchases_unpaid ON purchases (tenant_id, outlet_id, due_date)
    WHERE paid_amount < total;

ALTER TABLE purchase_payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE purchase_payments FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON purchase_payments
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));

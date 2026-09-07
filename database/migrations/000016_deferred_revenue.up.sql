-- Migrasi 000016 — pendapatan diterima di muka & pengembalian dana (Fase 7, §5.13, §13.4).
--
-- Uang prabayar N bulan BUKAN pendapatan bulan itu. `deferred_revenue_entries`
-- memecahnya menjadi N baris (satu per bulan pengakuan); pekerjaan harian
-- mengisi `recognized_at` saat bulannya tiba. Laporan pendapatan platform
-- membaca baris yang SUDAH diakui, bukan `subscription_payments` (§13.4).
--
-- `subscription_refunds` mencatat jejak pengembalian dana saat pembatalan di
-- tengah masa prabayar (dihitung ulang pada harga bulanan normal, blueprint
-- aturan 3). Tabel platform — tanpa RLS (lihat 000015).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE deferred_revenue_entries (
    id                      CHAR(26)    PRIMARY KEY,
    subscription_invoice_id CHAR(26)    NOT NULL REFERENCES subscription_invoices (id) ON DELETE CASCADE,
    tenant_id               CHAR(26)    NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    recognition_month       DATE        NOT NULL,   -- tanggal 1 bulan pengakuan
    amount                  BIGINT      NOT NULL,   -- boleh negatif: baris penyesuaian saat batal
    recognized_at           TIMESTAMPTZ,            -- NULL = belum diakui
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (subscription_invoice_id, recognition_month)
);
CREATE INDEX idx_deferred_pending ON deferred_revenue_entries (recognition_month) WHERE recognized_at IS NULL;
CREATE INDEX idx_deferred_recognized ON deferred_revenue_entries (recognition_month) WHERE recognized_at IS NOT NULL;

CREATE TABLE subscription_refunds (
    id                      CHAR(26)    PRIMARY KEY,
    subscription_invoice_id CHAR(26)    NOT NULL REFERENCES subscription_invoices (id) ON DELETE RESTRICT,
    tenant_id               CHAR(26)    NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    amount                  BIGINT      NOT NULL CHECK (amount >= 0),
    months_used             INT         NOT NULL,
    reason                  TEXT,
    refunded_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sub_refunds_invoice ON subscription_refunds (subscription_invoice_id);

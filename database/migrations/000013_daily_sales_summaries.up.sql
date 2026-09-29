-- Migrasi 000013 — agregat laporan harian (Fase 5, §5.14).
--
-- `daily_sales_summaries` adalah tabel RINGKASAN: satu baris per
-- (tenant, outlet, hari usaha, kanal). Dashboard & laporan rentang tanggal
-- MEMBACA tabel ini, tidak pernah SUM(sales) penuh — itu yang menjaga
-- "dashboard < 1 detik pada 100.000 transaksi" (§16 Fase 5 DoD).
--
-- Cara isi:
--   * saat tulis  — checkout menambah baris secara inkremental (UPSERT),
--                    void/retur menghitung ulang hari yang terdampak;
--   * perbaikan   — POST /api/v1/reports/rebuild-summaries membangun ulang
--                    dari tabel `sales` (sumber kebenaran).
--
-- channel_id memakai TEXT, bukan CHAR(26): '' adalah sentinel "tanpa kanal"
-- (transaksi kasir langsung) dan CHAR akan mem-padding-nya jadi 26 spasi
-- sehingga PRIMARY KEY & ON CONFLICT tidak lagi cocok. Tabel `channels`
-- sendiri baru lahir di Fase 11.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE daily_sales_summaries (
    tenant_id       CHAR(26)    NOT NULL,
    outlet_id       CHAR(26)    NOT NULL,
    business_date   DATE        NOT NULL,
    channel_id      TEXT        NOT NULL DEFAULT '',

    sales_count     INTEGER     NOT NULL DEFAULT 0,  -- hanya transaksi 'completed'
    gross_amount    BIGINT      NOT NULL DEFAULT 0,  -- Σ subtotal (sebelum diskon)
    discount_amount BIGINT      NOT NULL DEFAULT 0,
    tax_amount      BIGINT      NOT NULL DEFAULT 0,
    net_amount      BIGINT      NOT NULL DEFAULT 0,  -- Σ total (termasuk service & pembulatan)
    cost_amount     BIGINT      NOT NULL DEFAULT 0,  -- Σ cost_total (harga modal)
    fee_amount      BIGINT      NOT NULL DEFAULT 0,  -- MDR / komisi kanal (Σ sale_payments.fee_amount)
    gross_profit    BIGINT      NOT NULL DEFAULT 0,  -- net_amount - cost_amount - fee_amount

    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, outlet_id, business_date, channel_id),
    FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE CASCADE
);

-- Laporan rentang tanggal memindai per (tenant, tanggal) lintas outlet/kanal.
CREATE INDEX idx_daily_sales_summaries_tenant_date
    ON daily_sales_summaries (tenant_id, business_date);

-- Lapis 2 isolasi tenant: Row Level Security (lihat 000004).
ALTER TABLE daily_sales_summaries ENABLE ROW LEVEL SECURITY;
ALTER TABLE daily_sales_summaries FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON daily_sales_summaries
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));

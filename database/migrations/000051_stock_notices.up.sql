-- Migrasi 000051 — pengingat stok menipis (cmd/stock-reminders).
--
-- Sekali sehari pemilik menerima SATU ringkasan WhatsApp bila ada barang yang
-- BARU habis atau BARU di bawah batas minimum sejak pemeriksaan terakhir.
-- Tiap (toko, barang, jenis) dicatat di sini agar tidak diingatkan ulang
-- setiap hari — dan catatannya DIHAPUS begitu barangnya pulih (dibeli lagi),
-- supaya penurunan berikutnya diingatkan lagi.

CREATE TABLE stock_notices (
    id          CHAR(26)    PRIMARY KEY,
    tenant_id   CHAR(26)    NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    outlet_id   CHAR(26)    NOT NULL,
    product_id  CHAR(26)    NOT NULL,
    kind        TEXT        NOT NULL CHECK (kind IN ('low', 'out')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, outlet_id, product_id, kind),
    FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE CASCADE
);

ALTER TABLE stock_notices ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_notices FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON stock_notices
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));

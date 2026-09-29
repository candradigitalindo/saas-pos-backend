-- Migrasi 000046 — kemasan per barang (konversi satuan: dus → pak → pcs).
--
-- units.conversion (000005) menyimpan konversi GLOBAL ("1 dus = 24 pcs"),
-- padahal isi dus berbeda per barang: dus Indomie 40, dus Aqua 24. Keputusan
-- pemilik produk 2026-09-28: kemasan dicatat PER BARANG dan dipakai untuk
-- JUAL maupun BELI; stok & laporan tetap dalam satuan dasar barang.
--
--   product_units : kemasan satu barang — satuannya (dus), isinya dalam
--                   satuan dasar (conversion > 1), harga jual kemasan (NULL =
--                   isi × harga jual barang), barcode dus (opsional).
--   sale_items / purchase_items : kemasan yang dipakai + konversinya DISALIN
--                   saat transaksi, supaya mengubah isi kemasan kelak tidak
--                   mengubah stok & laporan transaksi lama.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE product_units (
    id           CHAR(26)      PRIMARY KEY,
    tenant_id    CHAR(26)      NOT NULL,
    product_id   CHAR(26)      NOT NULL,
    unit_id      CHAR(26)      NOT NULL,
    conversion   NUMERIC(14,6) NOT NULL CHECK (conversion > 1),
    sell_price   BIGINT        CHECK (sell_price IS NULL OR sell_price > 0),
    barcode      TEXT,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    sync_version BIGINT        NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, unit_id)    REFERENCES units    (tenant_id, id) ON DELETE RESTRICT
);
-- Satu kemasan per satuan per barang (yang aktif) — id tetap stabil saat
-- kemasan diubah, jadi keranjang offline yang merujuknya tidak patah.
CREATE UNIQUE INDEX uq_product_units_unit ON product_units (tenant_id, product_id, unit_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_product_units_barcode ON product_units (tenant_id, barcode)
    WHERE barcode IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_product_units_unit ON product_units (tenant_id, unit_id);

ALTER TABLE product_units ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_units FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON product_units
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));

-- Ikut sinkron offline (kasir menjual & memindai dus tanpa sinyal).
CREATE TRIGGER trg_product_units_sync_version BEFORE INSERT OR UPDATE ON product_units
    FOR EACH ROW EXECUTE FUNCTION bump_sync_version();
CREATE TRIGGER trg_product_units_tombstone AFTER DELETE ON product_units
    FOR EACH ROW EXECUTE FUNCTION record_sync_tombstone();

ALTER TABLE sale_items
    ADD COLUMN product_unit_id CHAR(26),
    ADD COLUMN unit_conversion NUMERIC(14,6) NOT NULL DEFAULT 1 CHECK (unit_conversion > 0);
ALTER TABLE purchase_items
    ADD COLUMN product_unit_id CHAR(26),
    ADD COLUMN unit_conversion NUMERIC(14,6) NOT NULL DEFAULT 1 CHECK (unit_conversion > 0),
    ADD COLUMN unit_name       TEXT NOT NULL DEFAULT '';

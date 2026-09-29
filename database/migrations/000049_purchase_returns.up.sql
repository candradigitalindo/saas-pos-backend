-- Migrasi 000049 — retur ke pemasok.
--
-- Barang dari satu nota dikembalikan ke pemasoknya (rusak, kedaluwarsa, salah
-- kirim). Akibatnya:
--   - stok berkurang (gerakan kind='purchase_return');
--   - TOTAL nota berkurang sebesar nilai retur (purchases.total), sehingga
--     semua perhitungan utang yang sudah ada — total − paid_amount — langsung
--     benar tanpa diubah; nilai yang diretur dicatat di returned_amount;
--   - bila yang sudah dibayar melebihi total baru, pemasok MENGEMBALIKAN
--     selisihnya (keputusan pemilik produk 2026-09-29): ke laci kasir (uang
--     masuk shift) atau ke uang lain. paid_amount diturunkan sebesar itu.

ALTER TABLE purchases ADD COLUMN returned_amount BIGINT NOT NULL DEFAULT 0
    CHECK (returned_amount >= 0);

ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_kind_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_kind_check CHECK (kind IN (
    'sale', 'void', 'refund', 'purchase', 'adjustment', 'transfer_in', 'transfer_out',
    'opname', 'recipe', 'initial', 'purchase_return'));

CREATE TABLE purchase_returns (
    id               CHAR(26)    PRIMARY KEY,
    tenant_id        CHAR(26)    NOT NULL,
    outlet_id        CHAR(26)    NOT NULL,
    purchase_id      CHAR(26)    NOT NULL,
    reason           TEXT        NOT NULL,
    total            BIGINT      NOT NULL CHECK (total > 0),
    refund_amount    BIGINT      NOT NULL DEFAULT 0 CHECK (refund_amount >= 0),
    refund_source    TEXT        CHECK (refund_source IN ('drawer', 'other')),
    cash_movement_id CHAR(26),
    occurred_at      TIMESTAMPTZ NOT NULL,
    business_date    DATE        NOT NULL,
    created_by       CHAR(26)    NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, purchase_id) REFERENCES purchases (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, cash_movement_id) REFERENCES cash_movements (tenant_id, id) ON DELETE RESTRICT,
    -- Ada pengembalian uang ⇔ sumbernya disebut; ke laci ⇔ ada uang masuknya.
    CHECK ((refund_amount > 0) = (refund_source IS NOT NULL)),
    CHECK (refund_source IS DISTINCT FROM 'drawer' OR cash_movement_id IS NOT NULL)
);
CREATE INDEX idx_purchase_returns_purchase ON purchase_returns (tenant_id, purchase_id);

CREATE TABLE purchase_return_items (
    id               CHAR(26)      PRIMARY KEY,
    tenant_id        CHAR(26)      NOT NULL,
    return_id        CHAR(26)      NOT NULL,
    purchase_item_id CHAR(26)      NOT NULL,
    product_id       CHAR(26)      NOT NULL,
    qty              NUMERIC(14,3) NOT NULL CHECK (qty > 0),  -- satuan beli baris nota
    unit_conversion  NUMERIC(14,6) NOT NULL DEFAULT 1,
    unit_cost        BIGINT        NOT NULL,
    line_total       BIGINT        NOT NULL,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, return_id) REFERENCES purchase_returns (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, purchase_item_id) REFERENCES purchase_items (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_purchase_return_items_item ON purchase_return_items (tenant_id, purchase_item_id);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['purchase_returns', 'purchase_return_items'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

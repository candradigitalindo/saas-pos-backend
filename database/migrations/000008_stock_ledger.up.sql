-- Migrasi 000008 — buku besar stok (Fase 3, docs/TECHNICAL-BACKEND.md §5.6).
--
-- `stock_movements` adalah SUMBER KEBENARAN (aturan mengikat #9). `stocks`
-- hanyalah cache saldo yang boleh dihitung ulang kapan saja dari movements.
--
-- Fase 3 hanya menulis movement kind='sale'/'void'/'refund' (dari checkout).
-- kind lain (purchase, opname, transfer, recipe, adjustment) menyusul Fase 4;
-- CHECK sudah memuat seluruh nilainya sekarang.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE stock_movements (
    id            CHAR(26)      PRIMARY KEY,
    tenant_id     CHAR(26)      NOT NULL,
    outlet_id     CHAR(26)      NOT NULL,
    product_id    CHAR(26)      NOT NULL,
    variant_id    CHAR(26),
    kind          TEXT          NOT NULL CHECK (kind IN (
                    'sale','void','refund','purchase','adjustment',
                    'transfer_in','transfer_out','opname','recipe','initial')),
    qty_delta     NUMERIC(14,3) NOT NULL,          -- negatif untuk keluar
    balance_after NUMERIC(14,3) NOT NULL,          -- saldo setelah gerakan ini (kartu stok)
    unit_cost     BIGINT        NOT NULL DEFAULT 0,
    ref_table     TEXT,                            -- 'sales', 'purchases', ...
    ref_id        CHAR(26),
    reason        TEXT,                            -- wajib untuk kind='adjustment'
    occurred_at   TIMESTAMPTZ   NOT NULL,
    business_date DATE          NOT NULL,
    created_by    CHAR(26),
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    sync_version  BIGINT        NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, outlet_id)  REFERENCES outlets  (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, variant_id) REFERENCES product_variants (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_stock_mov_tenant_product ON stock_movements (tenant_id, outlet_id, product_id, occurred_at DESC);
CREATE INDEX idx_stock_mov_tenant_ref     ON stock_movements (tenant_id, ref_table, ref_id);
CREATE INDEX idx_stock_mov_tenant_date    ON stock_movements (tenant_id, business_date);

-- Cache saldo. Tidak ber-id, tidak ber-sync_version (angka di klien = perkiraan,
-- §10). variant_id memakai '' (bukan NULL) agar PK komposit & ON CONFLICT
-- sederhana — keutuhannya dijaga service, bukan FK (§5.16).
--
-- variant_id bertipe TEXT (bukan CHAR(26) seperti §5.6): CHAR mem-blank-pad ''
-- menjadi 26 spasi sehingga sentinel '' rusak untuk pencarian & ON CONFLICT.
CREATE TABLE stocks (
    tenant_id    CHAR(26)      NOT NULL,
    outlet_id    CHAR(26)      NOT NULL,
    product_id   CHAR(26)      NOT NULL,
    variant_id   TEXT          NOT NULL DEFAULT '',
    qty          NUMERIC(14,3) NOT NULL DEFAULT 0,
    reserved_qty NUMERIC(14,3) NOT NULL DEFAULT 0,  -- alokasi kanal online (Fase 11)
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, outlet_id, product_id, variant_id),
    FOREIGN KEY (tenant_id, outlet_id)  REFERENCES outlets  (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE RESTRICT
);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['stock_movements','stocks'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

-- Migrasi 000011 — stok opname / hitung fisik (Fase 4, §5.6).
--
-- Alur: draft → isi item (system_qty di-snapshot, counted_qty diisi) → post
-- (selisih ditulis sebagai gerakan kind='opname', cache disamakan ke counted).
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE stock_opnames (
    id            CHAR(26)    PRIMARY KEY,
    tenant_id     CHAR(26)    NOT NULL,
    outlet_id     CHAR(26)    NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'draft'
                  CHECK (status IN ('draft','posted','canceled')),
    note          TEXT,
    counted_at    TIMESTAMPTZ,
    business_date DATE        NOT NULL,
    created_by    CHAR(26)    NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_stock_opnames_tenant_outlet ON stock_opnames (tenant_id, outlet_id, business_date);

CREATE TABLE stock_opname_items (
    id          CHAR(26)      PRIMARY KEY,
    tenant_id   CHAR(26)      NOT NULL,
    opname_id   CHAR(26)      NOT NULL,
    product_id  CHAR(26)      NOT NULL,
    variant_id  CHAR(26),
    system_qty  NUMERIC(14,3) NOT NULL,   -- saldo sistem saat hitung dimulai
    counted_qty NUMERIC(14,3) NOT NULL,
    diff_qty    NUMERIC(14,3) NOT NULL,   -- counted - system
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, opname_id, product_id, variant_id),
    FOREIGN KEY (tenant_id, opname_id)  REFERENCES stock_opnames (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, product_id) REFERENCES products      (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_stock_opname_items_tenant_opname ON stock_opname_items (tenant_id, opname_id);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['stock_opnames','stock_opname_items'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

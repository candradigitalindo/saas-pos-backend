-- Migrasi 000012 — transfer stok antar outlet (Fase 4, §5.6).
--
-- Alur: draft → send (gerakan kind='transfer_out' di outlet asal) → receive
-- (kind='transfer_in' di outlet tujuan). Baris asli tak diubah nilainya.
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE stock_transfers (
    id             CHAR(26)    PRIMARY KEY,
    tenant_id      CHAR(26)    NOT NULL,
    from_outlet_id CHAR(26)    NOT NULL,
    to_outlet_id   CHAR(26)    NOT NULL,
    status         TEXT        NOT NULL DEFAULT 'draft'
                   CHECK (status IN ('draft','sent','received','canceled')),
    note           TEXT,
    sent_at        TIMESTAMPTZ,
    received_at    TIMESTAMPTZ,
    business_date  DATE        NOT NULL,
    created_by     CHAR(26)    NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    CHECK (from_outlet_id <> to_outlet_id),
    FOREIGN KEY (tenant_id, from_outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, to_outlet_id)   REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_stock_transfers_tenant_from ON stock_transfers (tenant_id, from_outlet_id, business_date);
CREATE INDEX idx_stock_transfers_tenant_to   ON stock_transfers (tenant_id, to_outlet_id, business_date);

CREATE TABLE stock_transfer_items (
    id          CHAR(26)      PRIMARY KEY,
    tenant_id   CHAR(26)      NOT NULL,
    transfer_id CHAR(26)      NOT NULL,
    product_id  CHAR(26)      NOT NULL,
    variant_id  CHAR(26),
    qty         NUMERIC(14,3) NOT NULL CHECK (qty > 0),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, transfer_id) REFERENCES stock_transfers (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, product_id)  REFERENCES products        (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_stock_transfer_items_tenant_transfer ON stock_transfer_items (tenant_id, transfer_id);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['stock_transfers','stock_transfer_items'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

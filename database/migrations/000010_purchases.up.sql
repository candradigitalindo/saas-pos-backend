-- Migrasi 000010 — pembelian / stok masuk (Fase 4, docs/TECHNICAL-BACKEND.md §5.6).
--
-- Menerima barang dari supplier → gerakan stok kind='purchase' + perbarui cache.
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE purchases (
    id              CHAR(26)    PRIMARY KEY,
    tenant_id       CHAR(26)    NOT NULL,
    outlet_id       CHAR(26)    NOT NULL,
    supplier_id     CHAR(26),
    invoice_no      TEXT,
    idempotency_key TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'received'
                    CHECK (status IN ('draft','received','canceled')),
    subtotal        BIGINT      NOT NULL DEFAULT 0,
    discount_amount BIGINT      NOT NULL DEFAULT 0,
    tax_amount      BIGINT      NOT NULL DEFAULT 0,
    total           BIGINT      NOT NULL DEFAULT 0,
    paid_amount     BIGINT      NOT NULL DEFAULT 0,
    due_date        DATE,
    occurred_at     TIMESTAMPTZ NOT NULL,
    business_date   DATE        NOT NULL,
    created_by      CHAR(26)    NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    sync_version    BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, idempotency_key),
    FOREIGN KEY (tenant_id, outlet_id)   REFERENCES outlets   (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, supplier_id) REFERENCES suppliers (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_purchases_tenant_outlet_date ON purchases (tenant_id, outlet_id, business_date);
CREATE INDEX idx_purchases_tenant_supplier ON purchases (tenant_id, supplier_id);

CREATE TABLE purchase_items (
    id          CHAR(26)      PRIMARY KEY,
    tenant_id   CHAR(26)      NOT NULL,
    purchase_id CHAR(26)      NOT NULL,
    product_id  CHAR(26)      NOT NULL,
    variant_id  CHAR(26),
    qty         NUMERIC(14,3) NOT NULL CHECK (qty > 0),
    unit_cost   BIGINT        NOT NULL,
    line_total  BIGINT        NOT NULL,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, purchase_id) REFERENCES purchases (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, product_id)  REFERENCES products  (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_purchase_items_tenant_purchase ON purchase_items (tenant_id, purchase_id);
CREATE INDEX idx_purchase_items_tenant_product ON purchase_items (tenant_id, product_id);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['purchases','purchase_items'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

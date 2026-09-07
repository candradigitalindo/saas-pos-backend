-- Migrasi 000019 — CRM tenant: invoice pelanggan bertermin (Fase 9, §5.9,
-- blueprint E.2).
--
-- Ini invoice PELANGGAN (jangan tertukar dengan `subscription_invoices`
-- tagihan langganan platform). Satu invoice bisa dibayar bertahap
-- (DP / progres / pelunasan). Saat LUNAS, invoice dicatat sebagai satu
-- penjualan di tabel `sales` yang sama dengan POS (`sale_id` terisi) supaya
-- omzet & laba tetap satu pintu — tanpa entri ganda (§13, blueprint E.2).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE invoices (
    id              CHAR(26)    PRIMARY KEY,
    tenant_id       CHAR(26)    NOT NULL,
    number          TEXT        NOT NULL,
    customer_id     CHAR(26)    NOT NULL,
    project_id      CHAR(26),
    quotation_id    CHAR(26),
    owner_id        CHAR(26)    NOT NULL,
    issue_date      DATE        NOT NULL,
    due_date        DATE        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft','sent','partial','paid','overdue','void')),
    subtotal        BIGINT      NOT NULL DEFAULT 0,
    discount_amount BIGINT      NOT NULL DEFAULT 0,
    tax_amount      BIGINT      NOT NULL DEFAULT 0,
    total           BIGINT      NOT NULL DEFAULT 0,
    paid_amount     BIGINT      NOT NULL DEFAULT 0,
    term_label      TEXT,
    sale_id         CHAR(26),   -- diisi saat lunas → tercatat sebagai penjualan
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, customer_id)  REFERENCES customers  (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, project_id)   REFERENCES projects   (tenant_id, id) ON DELETE SET NULL,
    FOREIGN KEY (tenant_id, quotation_id) REFERENCES quotations (tenant_id, id) ON DELETE SET NULL,
    FOREIGN KEY (tenant_id, owner_id)     REFERENCES users      (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, sale_id)      REFERENCES sales      (tenant_id, id) ON DELETE SET NULL
);
CREATE INDEX idx_invoices_tenant_due   ON invoices (tenant_id, due_date) WHERE status IN ('sent','partial','overdue');
CREATE INDEX idx_invoices_tenant_owner ON invoices (tenant_id, owner_id);

CREATE TABLE invoice_items (
    id          CHAR(26)      PRIMARY KEY,
    tenant_id   CHAR(26)      NOT NULL,
    invoice_id  CHAR(26)      NOT NULL,
    product_id  CHAR(26),
    description TEXT          NOT NULL,
    qty         NUMERIC(14,3) NOT NULL,
    unit_price  BIGINT        NOT NULL,
    line_total  BIGINT        NOT NULL,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, invoice_id) REFERENCES invoices (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE SET NULL
);

CREATE TABLE invoice_payments (
    id            CHAR(26)    PRIMARY KEY,
    tenant_id     CHAR(26)    NOT NULL,
    invoice_id    CHAR(26)    NOT NULL,
    amount        BIGINT      NOT NULL CHECK (amount > 0),
    method        TEXT        NOT NULL CHECK (method IN ('cash','qris','transfer','card','ewallet')),
    paid_at       TIMESTAMPTZ NOT NULL,
    business_date DATE        NOT NULL,
    proof_url     TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, invoice_id) REFERENCES invoices (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_invoice_payments_tenant_invoice ON invoice_payments (tenant_id, invoice_id);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['invoices','invoice_items','invoice_payments'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

-- Migrasi 000018 — CRM tenant: penawaran & proyek (Fase 9, §5.9, blueprint E.2).
--
-- Penawaran disetujui → otomatis menjadi proyek (tanpa mengetik ulang). Proyek
-- punya tugas & biaya; biaya proyek dipakai menghitung "laba per proyek".
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE quotations (
    id              CHAR(26)    PRIMARY KEY,
    tenant_id       CHAR(26)    NOT NULL,
    number          TEXT        NOT NULL,
    customer_id     CHAR(26)    NOT NULL,
    deal_id         CHAR(26),
    owner_id        CHAR(26)    NOT NULL,
    valid_until     DATE,
    status          TEXT        NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft','sent','accepted','rejected','expired')),
    subtotal        BIGINT      NOT NULL DEFAULT 0,
    discount_amount BIGINT      NOT NULL DEFAULT 0,
    tax_amount      BIGINT      NOT NULL DEFAULT 0,
    total           BIGINT      NOT NULL DEFAULT 0,
    note            TEXT,
    accepted_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, deal_id)     REFERENCES deals     (tenant_id, id) ON DELETE SET NULL,
    FOREIGN KEY (tenant_id, owner_id)    REFERENCES users     (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_quotations_tenant_owner ON quotations (tenant_id, owner_id);

CREATE TABLE quotation_items (
    id              CHAR(26)      PRIMARY KEY,
    tenant_id       CHAR(26)      NOT NULL,
    quotation_id    CHAR(26)      NOT NULL,
    product_id      CHAR(26),
    description     TEXT          NOT NULL,
    qty             NUMERIC(14,3) NOT NULL,
    unit_price      BIGINT        NOT NULL,
    discount_amount BIGINT        NOT NULL DEFAULT 0,
    line_total      BIGINT        NOT NULL,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, quotation_id) REFERENCES quotations (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, product_id)   REFERENCES products   (tenant_id, id) ON DELETE SET NULL
);

CREATE TABLE projects (
    id             CHAR(26)    PRIMARY KEY,
    tenant_id      CHAR(26)    NOT NULL,
    customer_id    CHAR(26)    NOT NULL,
    quotation_id   CHAR(26),
    owner_id       CHAR(26)    NOT NULL,
    name           TEXT        NOT NULL,
    start_date     DATE,
    due_date       DATE,
    status         TEXT        NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active','on_hold','completed','canceled')),
    contract_value BIGINT      NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, customer_id)  REFERENCES customers  (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, quotation_id) REFERENCES quotations (tenant_id, id) ON DELETE SET NULL,
    FOREIGN KEY (tenant_id, owner_id)     REFERENCES users      (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_projects_tenant_owner ON projects (tenant_id, owner_id);

CREATE TABLE project_tasks (
    id         CHAR(26)    PRIMARY KEY,
    tenant_id  CHAR(26)    NOT NULL,
    project_id CHAR(26)    NOT NULL,
    title      TEXT        NOT NULL,
    due_date   DATE,
    done_at    TIMESTAMPTZ,
    sort_order INT         NOT NULL DEFAULT 0,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE project_expenses (
    id          CHAR(26) PRIMARY KEY,
    tenant_id   CHAR(26) NOT NULL,
    project_id  CHAR(26) NOT NULL,
    description TEXT     NOT NULL,
    amount      BIGINT   NOT NULL CHECK (amount > 0),
    spent_at    DATE     NOT NULL,
    receipt_url TEXT,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects (tenant_id, id) ON DELETE CASCADE
);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['quotations','quotation_items','projects','project_tasks','project_expenses'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

-- Migrasi 000007 — pelanggan & piutang (Fase 3, docs/TECHNICAL-BACKEND.md §5.8).
--
-- Diperlukan sebelum `sales`: sales.customer_id menunjuk ke sini, dan pembayaran
-- ber-metode 'credit' (kasbon) menimbulkan baris `receivables`.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE customers (
    id            CHAR(26)    PRIMARY KEY,
    tenant_id     CHAR(26)    NOT NULL,
    code          TEXT,
    name          TEXT        NOT NULL,
    phone         TEXT,
    email         TEXT,
    address       TEXT,
    type          TEXT        NOT NULL DEFAULT 'person' CHECK (type IN ('person','company','store')),
    price_list_id CHAR(26),                       -- harga khusus pelanggan
    owner_id      CHAR(26),                       -- CRM: sales pemilik data
    credit_limit  BIGINT      NOT NULL DEFAULT 0,
    latitude      NUMERIC(9,6),
    longitude     NUMERIC(9,6),
    note          TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    sync_version  BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id)                REFERENCES tenants (id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, price_list_id) REFERENCES price_lists (tenant_id, id) ON DELETE SET NULL,
    FOREIGN KEY (tenant_id, owner_id)      REFERENCES users (tenant_id, id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX uq_customers_tenant_phone
    ON customers (tenant_id, phone) WHERE deleted_at IS NULL AND phone IS NOT NULL;
CREATE INDEX idx_customers_tenant_owner ON customers (tenant_id, owner_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_customers_tenant_name_trgm ON customers USING gin (name gin_trgm_ops);

CREATE TABLE contact_persons (
    id          CHAR(26)    PRIMARY KEY,
    tenant_id   CHAR(26)    NOT NULL,
    customer_id CHAR(26)    NOT NULL,
    name        TEXT        NOT NULL,
    position    TEXT,
    phone       TEXT,
    email       TEXT,
    is_primary  BOOLEAN     NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_contact_persons_tenant_customer ON contact_persons (tenant_id, customer_id);

CREATE TABLE receivables (
    id           CHAR(26)    PRIMARY KEY,
    tenant_id    CHAR(26)    NOT NULL,
    customer_id  CHAR(26)    NOT NULL,
    source_table TEXT        NOT NULL CHECK (source_table IN ('sales','invoices')),
    source_id    CHAR(26)    NOT NULL,
    amount       BIGINT      NOT NULL CHECK (amount > 0),
    paid_amount  BIGINT      NOT NULL DEFAULT 0,
    due_date     DATE,
    status       TEXT        NOT NULL DEFAULT 'open'
                 CHECK (status IN ('open','partial','paid','written_off')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, source_table, source_id),
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_receivables_tenant_due ON receivables (tenant_id, due_date) WHERE status IN ('open','partial');
CREATE INDEX idx_receivables_tenant_customer ON receivables (tenant_id, customer_id);

CREATE TABLE receivable_payments (
    id            CHAR(26)    PRIMARY KEY,
    tenant_id     CHAR(26)    NOT NULL,
    receivable_id CHAR(26)    NOT NULL,
    amount        BIGINT      NOT NULL CHECK (amount > 0),
    method        TEXT        NOT NULL CHECK (method IN ('cash','qris','transfer','card','ewallet')),
    paid_at       TIMESTAMPTZ NOT NULL,
    business_date DATE        NOT NULL,
    collected_by  CHAR(26),
    proof_url     TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, receivable_id) REFERENCES receivables (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_receivable_payments_tenant_receivable ON receivable_payments (tenant_id, receivable_id);

-- Row Level Security (lapisan 2).
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['customers','contact_persons','receivables','receivable_payments'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

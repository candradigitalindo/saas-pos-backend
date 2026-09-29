-- Migrasi 000009 — transaksi kasir, kas & shift, idempotensi (Fase 3).
--
-- docs/TECHNICAL-BACKEND.md §5.5, §5.7, §5.14. Ini fondasi checkout (§13.1):
-- sale + item + payment + gerakan stok, semuanya dalam satu transaksi.
--
-- Tambahan di luar §5:
--   - outlets.code  : kode singkat outlet untuk prefiks nomor struk (§13.7)
--   - receipt_counters : penghitung nomor struk per outlet per hari usaha,
--     dikunci SELECT ... FOR UPDATE (bukan COUNT(*), §13.7)
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

ALTER TABLE outlets ADD COLUMN code TEXT;
CREATE UNIQUE INDEX uq_outlets_tenant_code
    ON outlets (tenant_id, code) WHERE deleted_at IS NULL AND code IS NOT NULL;

-- ── shifts ─────────────────────────────────────────────────────────────────
CREATE TABLE shifts (
    id            CHAR(26)    PRIMARY KEY,
    tenant_id     CHAR(26)    NOT NULL,
    outlet_id     CHAR(26)    NOT NULL,
    opened_by     CHAR(26)    NOT NULL,
    closed_by     CHAR(26),
    opened_at     TIMESTAMPTZ NOT NULL,
    closed_at     TIMESTAMPTZ,
    business_date DATE        NOT NULL,             -- dari opened_at
    opening_cash  BIGINT      NOT NULL DEFAULT 0,
    expected_cash BIGINT      NOT NULL DEFAULT 0,   -- dihitung sistem saat tutup
    counted_cash  BIGINT,                           -- hasil hitung fisik
    difference    BIGINT,                           -- counted - expected; boleh negatif
    note          TEXT,
    status        TEXT        NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    sync_version  BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, opened_by) REFERENCES users   (tenant_id, id) ON DELETE RESTRICT
);
-- satu outlet hanya boleh punya satu shift terbuka
CREATE UNIQUE INDEX uq_shifts_open_per_outlet ON shifts (tenant_id, outlet_id) WHERE status = 'open';
CREATE INDEX idx_shifts_tenant_outlet_date ON shifts (tenant_id, outlet_id, business_date);

-- ── cash_movements ─────────────────────────────────────────────────────────
CREATE TABLE cash_movements (
    id            CHAR(26)    PRIMARY KEY,
    tenant_id     CHAR(26)    NOT NULL,
    outlet_id     CHAR(26)    NOT NULL,
    shift_id      CHAR(26)    NOT NULL,
    direction     TEXT        NOT NULL CHECK (direction IN ('in','out')),
    amount        BIGINT      NOT NULL CHECK (amount > 0),
    reason        TEXT        NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    business_date DATE        NOT NULL,
    created_by    CHAR(26)    NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    sync_version  BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, shift_id) REFERENCES shifts (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_cash_movements_tenant_shift ON cash_movements (tenant_id, shift_id);

-- ── sales ──────────────────────────────────────────────────────────────────
CREATE TABLE sales (
    id                CHAR(26)    PRIMARY KEY,       -- boleh dibuat KLIEN (ULID)
    tenant_id         CHAR(26)    NOT NULL,
    outlet_id         CHAR(26)    NOT NULL,
    shift_id          CHAR(26),
    channel_id        CHAR(26),                      -- NULL = kasir (Fase 11)
    customer_id       CHAR(26),
    table_id          CHAR(26),                      -- F&B (Fase 5.4 dining_tables)
    receipt_no        TEXT        NOT NULL,
    external_order_id TEXT,
    idempotency_key   TEXT        NOT NULL,
    order_type        TEXT        NOT NULL DEFAULT 'dine_in'
                      CHECK (order_type IN ('dine_in','takeaway','delivery','pickup')),
    status            TEXT        NOT NULL DEFAULT 'completed'
                      CHECK (status IN ('draft','pending','accepted','preparing','ready',
                                        'shipped','completed','rejected','canceled','returned')),
    subtotal          BIGINT      NOT NULL DEFAULT 0,
    discount_amount   BIGINT      NOT NULL DEFAULT 0,
    tax_amount        BIGINT      NOT NULL DEFAULT 0,
    service_amount    BIGINT      NOT NULL DEFAULT 0,
    rounding_amount   BIGINT      NOT NULL DEFAULT 0,
    total             BIGINT      NOT NULL DEFAULT 0,
    paid_amount       BIGINT      NOT NULL DEFAULT 0,
    change_amount     BIGINT      NOT NULL DEFAULT 0,
    cost_total        BIGINT      NOT NULL DEFAULT 0,  -- Σ snapshot modal
    return_of_sale_id CHAR(26),                        -- diisi bila status='returned'
    note              TEXT,
    occurred_at       TIMESTAMPTZ NOT NULL,
    client_created_at TIMESTAMPTZ,
    business_date     DATE        NOT NULL,
    voided_at         TIMESTAMPTZ,
    voided_by         CHAR(26),
    void_reason       TEXT,
    created_by        CHAR(26)    NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    sync_version      BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, outlet_id, receipt_no),
    UNIQUE (tenant_id, idempotency_key),
    FOREIGN KEY (tenant_id, outlet_id)   REFERENCES outlets   (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, shift_id)    REFERENCES shifts    (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, created_by)  REFERENCES users     (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_sales_tenant_outlet_date ON sales (tenant_id, outlet_id, business_date);
CREATE INDEX idx_sales_tenant_occurred    ON sales (tenant_id, occurred_at DESC);
CREATE INDEX idx_sales_tenant_shift       ON sales (tenant_id, shift_id);
CREATE INDEX idx_sales_tenant_status      ON sales (tenant_id, status) WHERE status <> 'completed';

CREATE TABLE sale_items (
    id              CHAR(26)      PRIMARY KEY,
    tenant_id       CHAR(26)      NOT NULL,
    sale_id         CHAR(26)      NOT NULL,
    product_id      CHAR(26)      NOT NULL,
    variant_id      CHAR(26),
    product_name    TEXT          NOT NULL,        -- SNAPSHOT
    unit_name       TEXT          NOT NULL,        -- SNAPSHOT
    qty             NUMERIC(14,3) NOT NULL CHECK (qty > 0),
    unit_price      BIGINT        NOT NULL,        -- SNAPSHOT harga jual
    unit_cost       BIGINT        NOT NULL,        -- SNAPSHOT harga modal
    discount_amount BIGINT        NOT NULL DEFAULT 0,
    tax_amount      BIGINT        NOT NULL DEFAULT 0,
    line_total      BIGINT        NOT NULL,
    note            TEXT,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, sale_id)    REFERENCES sales    (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, variant_id) REFERENCES product_variants (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_sale_items_tenant_sale    ON sale_items (tenant_id, sale_id);
CREATE INDEX idx_sale_items_tenant_product ON sale_items (tenant_id, product_id);

CREATE TABLE sale_payments (
    id         CHAR(26)    PRIMARY KEY,
    tenant_id  CHAR(26)    NOT NULL,
    sale_id    CHAR(26)    NOT NULL,
    method     TEXT        NOT NULL CHECK (method IN ('cash','qris','transfer','card','ewallet','credit')),
    amount     BIGINT      NOT NULL CHECK (amount > 0),
    reference  TEXT,
    fee_amount BIGINT      NOT NULL DEFAULT 0,
    paid_at    TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, sale_id) REFERENCES sales (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_sale_payments_tenant_sale ON sale_payments (tenant_id, sale_id);

-- ── idempotency_keys ───────────────────────────────────────────────────────
CREATE TABLE idempotency_keys (
    id              CHAR(26)    PRIMARY KEY,
    tenant_id       CHAR(26),
    scope           TEXT        NOT NULL,           -- 'sale.create', 'sale.refund', ...
    key             TEXT        NOT NULL,
    request_hash    TEXT        NOT NULL,           -- kunci sama + body beda = 409
    response_status INT,
    response_body   JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX uq_idempotency_scope_key
    ON idempotency_keys (COALESCE(tenant_id, ''), scope, key);
CREATE INDEX idx_idempotency_expires ON idempotency_keys (expires_at);

-- ── receipt_counters ───────────────────────────────────────────────────────
CREATE TABLE receipt_counters (
    tenant_id     CHAR(26) NOT NULL,
    outlet_id     CHAR(26) NOT NULL,
    business_date DATE     NOT NULL,
    next_seq      BIGINT   NOT NULL DEFAULT 1,
    PRIMARY KEY (tenant_id, outlet_id, business_date),
    FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT
);

-- Row Level Security.
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'shifts','cash_movements','sales','sale_items','sale_payments',
    'idempotency_keys','receipt_counters'
  ] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

-- Migrasi 000021 — CRM sales lapangan: target & komisi (Fase 10, §5.9,
-- blueprint E.3).
--
-- `commissions` adalah komisi SALES TENANT (bukan mitra penjual aplikasi —
-- itu `partner_commissions`, Fase 12). Dasarnya nilai TERTAGIH (uang benar
-- benar masuk), bukan terkirim — membayar komisi atas piutang macet adalah
-- kesalahan mahal (blueprint E.3).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE sales_targets (
    id            CHAR(26) PRIMARY KEY,
    tenant_id     CHAR(26) NOT NULL,
    user_id       CHAR(26) NOT NULL,
    period_start  DATE     NOT NULL,
    period_end    DATE     NOT NULL,
    target_amount BIGINT   NOT NULL DEFAULT 0,
    target_visits INT      NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, user_id, period_start),
    FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE commissions (
    id           CHAR(26)     PRIMARY KEY,
    tenant_id    CHAR(26)     NOT NULL,
    user_id      CHAR(26)     NOT NULL,
    period_start DATE         NOT NULL,
    period_end   DATE         NOT NULL,
    base_amount  BIGINT       NOT NULL DEFAULT 0,   -- nilai TERTAGIH, bukan terkirim
    rate         NUMERIC(7,4) NOT NULL,
    amount       BIGINT       NOT NULL DEFAULT 0,
    status       TEXT         NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','approved','paid')),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, user_id, period_start),
    FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_commissions_tenant_status ON commissions (tenant_id, status);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['sales_targets','commissions'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

-- Migrasi 000017 — CRM tenant: sumber prospek, pipeline, deal, aktivitas
-- (Fase 9, §5.9, blueprint Bagian E).
--
-- Semua tabel BERTENANT: `UNIQUE (tenant_id, id)`, FK antar-tabel komposit,
-- index diawali `tenant_id`, Row Level Security (lapis 2). `deals` & `activities`
-- membawa `owner_id` — lapis 3 (visibilitas kepemilikan, §6) diterapkan di
-- repository lewat scopeVisibility.
--
-- `deals` & `activities` juga punya `sync_version` → dapat pemicu bump + batu
-- nisan seperti tabel tersinkron lain (migrasi 000014).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE lead_sources (
    id        CHAR(26) PRIMARY KEY,
    tenant_id CHAR(26) NOT NULL,
    name      TEXT     NOT NULL,
    is_active BOOLEAN  NOT NULL DEFAULT true,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, name),
    FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT
);

CREATE TABLE pipelines (
    id         CHAR(26) PRIMARY KEY,
    tenant_id  CHAR(26) NOT NULL,
    name       TEXT     NOT NULL,
    kind       TEXT     NOT NULL CHECK (kind IN ('freelance','field_sales','general')),
    is_default BOOLEAN  NOT NULL DEFAULT false,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT
);

CREATE TABLE pipeline_stages (
    id          CHAR(26)     PRIMARY KEY,
    tenant_id   CHAR(26)     NOT NULL,
    pipeline_id CHAR(26)     NOT NULL,
    name        TEXT         NOT NULL,
    sort_order  INT          NOT NULL,
    probability NUMERIC(7,4) NOT NULL DEFAULT 0,   -- 0..1
    is_won      BOOLEAN      NOT NULL DEFAULT false,
    is_lost     BOOLEAN      NOT NULL DEFAULT false,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, pipeline_id, sort_order),
    FOREIGN KEY (tenant_id, pipeline_id) REFERENCES pipelines (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE deals (
    id                  CHAR(26)    PRIMARY KEY,
    tenant_id           CHAR(26)    NOT NULL,
    pipeline_id         CHAR(26)    NOT NULL,
    stage_id            CHAR(26)    NOT NULL,
    customer_id         CHAR(26),
    lead_source_id      CHAR(26),
    owner_id            CHAR(26)    NOT NULL,   -- lapis visibilitas ketiga (§6)
    title               TEXT        NOT NULL,
    value               BIGINT      NOT NULL DEFAULT 0,
    expected_close_date DATE,
    status              TEXT        NOT NULL DEFAULT 'open' CHECK (status IN ('open','won','lost')),
    lost_reason         TEXT,
    closed_at           TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ,
    sync_version        BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, pipeline_id)    REFERENCES pipelines       (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, stage_id)       REFERENCES pipeline_stages (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, customer_id)    REFERENCES customers       (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, lead_source_id) REFERENCES lead_sources    (tenant_id, id) ON DELETE SET NULL,
    FOREIGN KEY (tenant_id, owner_id)       REFERENCES users           (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_deals_tenant_owner_status ON deals (tenant_id, owner_id, status);
CREATE INDEX idx_deals_tenant_stage        ON deals (tenant_id, stage_id) WHERE status = 'open';

CREATE TABLE activities (
    id           CHAR(26)    PRIMARY KEY,
    tenant_id    CHAR(26)    NOT NULL,
    kind         TEXT        NOT NULL CHECK (kind IN ('call','chat','meeting','visit','task','note')),
    subject      TEXT        NOT NULL,
    body         TEXT,
    owner_id     CHAR(26)    NOT NULL,
    customer_id  CHAR(26),
    deal_id      CHAR(26),
    due_at       TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    status       TEXT        NOT NULL DEFAULT 'planned' CHECK (status IN ('planned','done','canceled')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    sync_version BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, owner_id)    REFERENCES users     (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, deal_id)     REFERENCES deals     (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_activities_tenant_owner_due ON activities (tenant_id, owner_id, due_at) WHERE status = 'planned';

-- Penomoran dokumen CRM per (tenant, jenis): QUO-000001, INV-000001, ...
CREATE TABLE crm_document_counters (
    tenant_id CHAR(26) NOT NULL,
    doc_type  TEXT     NOT NULL,
    next_seq  BIGINT   NOT NULL DEFAULT 1,
    PRIMARY KEY (tenant_id, doc_type),
    FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE CASCADE
);

-- Index visibilitas kepemilikan pada customers (kolom owner_id sudah ada sejak 000007).
CREATE INDEX IF NOT EXISTS idx_customers_tenant_owner ON customers (tenant_id, owner_id) WHERE deleted_at IS NULL;

-- Lapis 2: Row Level Security.
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['lead_sources','pipelines','pipeline_stages','deals','activities','crm_document_counters'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

-- Pemicu sync_version + batu nisan untuk deals & activities.
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['deals','activities'] LOOP
    EXECUTE format('CREATE TRIGGER trg_%1$s_sync_version BEFORE INSERT OR UPDATE ON %1$I '
                   'FOR EACH ROW EXECUTE FUNCTION bump_sync_version()', t);
    EXECUTE format('CREATE TRIGGER trg_%1$s_tombstone AFTER DELETE ON %1$I '
                   'FOR EACH ROW EXECUTE FUNCTION record_sync_tombstone()', t);
  END LOOP;
END $$;

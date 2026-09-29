-- Migrasi 000020 — CRM sales lapangan: rencana kunjungan & kunjungan
-- (Fase 10, §5.9, blueprint E.3).
--
-- Kunjungan dibuat KLIEN (ULID) dan sering dibuat OFFLINE di lapangan —
-- sinkron lewat /sync/push (op 'visit.upsert', idempoten per id).
--
-- Privasi: titik lokasi HANYA direkam saat check-in/check-out, bukan
-- pelacakan terus-menerus (§E.3, UU PDP).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE visit_plans (
    id         CHAR(26) PRIMARY KEY,
    tenant_id  CHAR(26) NOT NULL,
    owner_id   CHAR(26) NOT NULL,
    plan_date  DATE     NOT NULL,
    status     TEXT     NOT NULL DEFAULT 'planned' CHECK (status IN ('planned','running','done')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, owner_id, plan_date),
    FOREIGN KEY (tenant_id, owner_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE visits (
    id              CHAR(26)     PRIMARY KEY,   -- dibuat KLIEN, sering offline
    tenant_id       CHAR(26)     NOT NULL,
    visit_plan_id   CHAR(26),
    customer_id     CHAR(26)     NOT NULL,
    owner_id        CHAR(26)     NOT NULL,      -- lapis visibilitas ketiga (§6)
    checkin_at      TIMESTAMPTZ,
    checkout_at     TIMESTAMPTZ,
    checkin_lat     NUMERIC(9,6),
    checkin_lng     NUMERIC(9,6),
    photo_url       TEXT,
    result          TEXT         NOT NULL DEFAULT 'pending'
                    CHECK (result IN ('pending','order','no_order','closed','rejected')),
    no_order_reason TEXT,
    sale_id         CHAR(26),                  -- pesanan yang diambil di lokasi
    business_date   DATE         NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    sync_version    BIGINT       NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, customer_id)   REFERENCES customers   (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, owner_id)      REFERENCES users       (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, visit_plan_id) REFERENCES visit_plans (tenant_id, id) ON DELETE SET NULL,
    FOREIGN KEY (tenant_id, sale_id)       REFERENCES sales       (tenant_id, id) ON DELETE SET NULL
);
CREATE INDEX idx_visits_tenant_owner_date ON visits (tenant_id, owner_id, business_date);
CREATE INDEX idx_visits_tenant_plan       ON visits (tenant_id, visit_plan_id);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['visit_plans','visits'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

-- visits ber-sync_version → pemicu bump + batu nisan (lihat 000014).
CREATE TRIGGER trg_visits_sync_version BEFORE INSERT OR UPDATE ON visits
  FOR EACH ROW EXECUTE FUNCTION bump_sync_version();
CREATE TRIGGER trg_visits_tombstone AFTER DELETE ON visits
  FOR EACH ROW EXECUTE FUNCTION record_sync_tombstone();

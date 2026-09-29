-- Migrasi 000030 — HR: absensi, koreksi, cache harian, cuti (Fase 13, §5.11).
--
-- Disiplin sama dengan stok: `attendances` adalah KEBENARAN (buku besar, hanya
-- tambah); `attendance_days` hanyalah CACHE yang boleh dihitung ulang kapan
-- saja dari `attendances` + koreksi disetujui + jadwal + libur + cuti.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE attendances (
    id                CHAR(26)     PRIMARY KEY,   -- dibuat KLIEN (offline mungkin)
    tenant_id         CHAR(26)     NOT NULL,
    employee_id       CHAR(26)     NOT NULL,
    outlet_id         CHAR(26)     NOT NULL,
    kind              TEXT         NOT NULL CHECK (kind IN ('in','out')),
    occurred_at       TIMESTAMPTZ  NOT NULL,      -- UTC
    business_date     DATE         NOT NULL,      -- dihitung server dari zona outlet
    source            TEXT         NOT NULL DEFAULT 'manual'
                      CHECK (source IN ('manual','shift','import','correction')),
    photo_url         TEXT,
    latitude          NUMERIC(9,6),
    longitude         NUMERIC(9,6),
    device_id         TEXT,
    client_created_at TIMESTAMPTZ,
    reason            TEXT,
    approved_by       CHAR(26),
    created_by        CHAR(26)     NOT NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    sync_version      BIGINT       NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, outlet_id)   REFERENCES outlets   (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_attendances_emp_date    ON attendances (tenant_id, employee_id, business_date);
CREATE INDEX idx_attendances_outlet_date ON attendances (tenant_id, outlet_id, business_date);

CREATE TABLE attendance_corrections (
    id              CHAR(26)     PRIMARY KEY,
    tenant_id       CHAR(26)     NOT NULL,
    attendance_id   CHAR(26)     NOT NULL,
    new_occurred_at TIMESTAMPTZ  NOT NULL,
    reason          TEXT         NOT NULL,
    requested_by    CHAR(26)     NOT NULL,
    approved_by     CHAR(26),
    approved_at     TIMESTAMPTZ,
    status          TEXT         NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, attendance_id) REFERENCES attendances (tenant_id, id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX uq_attendance_corr_approved
    ON attendance_corrections (tenant_id, attendance_id) WHERE status = 'approved';

CREATE TABLE attendance_days (
    tenant_id           CHAR(26)    NOT NULL,
    employee_id         CHAR(26)    NOT NULL,
    business_date       DATE        NOT NULL,
    status              TEXT        NOT NULL
                        CHECK (status IN ('present','late','leave','sick','permit','holiday','off','absent')),
    scheduled_start     TIMESTAMPTZ,
    scheduled_end       TIMESTAMPTZ,
    first_in            TIMESTAMPTZ,
    last_out            TIMESTAMPTZ,
    late_minutes        INT         NOT NULL DEFAULT 0,
    early_leave_minutes INT         NOT NULL DEFAULT 0,
    work_minutes        INT         NOT NULL DEFAULT 0,
    overtime_minutes    INT         NOT NULL DEFAULT 0,
    leave_request_id    CHAR(26),
    computed_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, employee_id, business_date),
    FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE leave_requests (
    id             CHAR(26)      PRIMARY KEY,
    tenant_id      CHAR(26)      NOT NULL,
    employee_id    CHAR(26)      NOT NULL,
    kind           TEXT          NOT NULL CHECK (kind IN ('permit','sick','leave','unpaid')),
    start_date     DATE          NOT NULL,
    end_date       DATE          NOT NULL,
    days           NUMERIC(5,1)  NOT NULL,
    reason         TEXT,
    attachment_url TEXT,
    is_paid        BOOLEAN       NOT NULL,   -- SNAPSHOT kebijakan saat disetujui
    status         TEXT          NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','approved','rejected','canceled')),
    approved_by    CHAR(26),
    approved_at    TIMESTAMPTZ,
    reject_reason  TEXT,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    sync_version   BIGINT        NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    CHECK (end_date >= start_date),
    FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_leave_emp_range ON leave_requests (tenant_id, employee_id, start_date, end_date)
    WHERE status = 'approved';

CREATE TABLE leave_balances (
    tenant_id         CHAR(26)     NOT NULL,
    employee_id       CHAR(26)     NOT NULL,
    year              SMALLINT     NOT NULL,
    quota_days        NUMERIC(5,1) NOT NULL DEFAULT 12,
    used_days         NUMERIC(5,1) NOT NULL DEFAULT 0,
    carried_over_days NUMERIC(5,1) NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, employee_id, year),
    FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id) ON DELETE CASCADE
);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['attendances','attendance_corrections','attendance_days','leave_requests','leave_balances'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['attendances','leave_requests'] LOOP
    EXECUTE format('CREATE TRIGGER trg_%1$s_sync_version BEFORE INSERT OR UPDATE ON %1$I '
                   'FOR EACH ROW EXECUTE FUNCTION bump_sync_version()', t);
    EXECUTE format('CREATE TRIGGER trg_%1$s_tombstone AFTER DELETE ON %1$I '
                   'FOR EACH ROW EXECUTE FUNCTION record_sync_tombstone()', t);
  END LOOP;
END $$;

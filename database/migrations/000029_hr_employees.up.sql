-- Migrasi 000029 — HR: karyawan, jadwal kerja, hari libur (Fase 13, §5.11,
-- blueprint Bagian H).
--
-- Nomor 000024–000028 SENGAJA dilewati: dicadangkan untuk Fase 11b (adaptor
-- API kanal) & Fase 12 (program mitra) yang belum dikerjakan karena butuh
-- kemitraan eksternal / pemicu bisnis. Runner migrasi tidak menuntut nomor
-- berurutan.
--
-- Data pribadi karyawan (NIK, rekening, tanggal lahir) tunduk UU PDP: simpan
-- seperlunya, batasi akses (izin hr.*), hapus atas permintaan.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE employees (
    id                  CHAR(26)    PRIMARY KEY,
    tenant_id           CHAR(26)    NOT NULL,
    user_id             CHAR(26),   -- NULL: karyawan tanpa akun aplikasi
    outlet_id           CHAR(26)    NOT NULL,
    employee_no         TEXT,
    full_name           TEXT        NOT NULL,
    phone               TEXT,
    email               TEXT,
    id_number           TEXT,
    npwp                TEXT,
    address             TEXT,
    birth_date          DATE,
    position            TEXT,
    employment_status   TEXT        NOT NULL DEFAULT 'permanent'
                        CHECK (employment_status IN ('permanent','contract','probation','daily')),
    wage_type           TEXT        NOT NULL CHECK (wage_type IN ('monthly','daily','hourly')),
    base_wage           BIGINT      NOT NULL DEFAULT 0,
    payroll_period_type TEXT        NOT NULL DEFAULT 'monthly'
                        CHECK (payroll_period_type IN ('daily','weekly','biweekly','monthly')),
    bank_name           TEXT,
    bank_account_no     TEXT,
    bank_account_name   TEXT,
    joined_at           DATE        NOT NULL,
    resigned_at         DATE,
    is_active           BOOLEAN     NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ,
    sync_version        BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, user_id)   REFERENCES users   (tenant_id, id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX uq_employees_user ON employees (tenant_id, user_id)
    WHERE user_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX uq_employees_no ON employees (tenant_id, employee_no)
    WHERE employee_no IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_employees_tenant_outlet ON employees (tenant_id, outlet_id) WHERE deleted_at IS NULL;

CREATE TABLE work_schedules (
    id                     CHAR(26)    PRIMARY KEY,
    tenant_id              CHAR(26)    NOT NULL,
    employee_id            CHAR(26)    NOT NULL,
    weekday                SMALLINT    NOT NULL CHECK (weekday BETWEEN 0 AND 6),  -- 0 = Minggu
    is_working_day         BOOLEAN     NOT NULL DEFAULT true,
    start_time             TIME,
    end_time               TIME,       -- end < start = melewati tengah malam
    break_minutes          INT         NOT NULL DEFAULT 0,
    late_tolerance_minutes INT         NOT NULL DEFAULT 0,
    effective_from         DATE        NOT NULL,
    effective_to           DATE,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, employee_id, weekday, effective_from),
    CHECK (effective_to IS NULL OR effective_to >= effective_from),
    FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE holidays (
    id           CHAR(26)    PRIMARY KEY,
    tenant_id    CHAR(26)    NOT NULL,
    outlet_id    CHAR(26),   -- NULL = semua outlet
    holiday_date DATE        NOT NULL,
    name         TEXT        NOT NULL,
    is_paid      BOOLEAN     NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);
CREATE UNIQUE INDEX uq_holidays_date ON holidays (tenant_id, COALESCE(outlet_id, ''), holiday_date);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['employees','work_schedules','holidays'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

-- employees ber-sync_version → pemicu bump + batu nisan (lihat 000014).
CREATE TRIGGER trg_employees_sync_version BEFORE INSERT OR UPDATE ON employees
  FOR EACH ROW EXECUTE FUNCTION bump_sync_version();
CREATE TRIGGER trg_employees_tombstone AFTER DELETE ON employees
  FOR EACH ROW EXECUTE FUNCTION record_sync_tombstone();

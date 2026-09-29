-- Migrasi 000031 — HR: aturan gaji, periode, slip, penyesuaian, kasbon
-- (Fase 13, §5.11, §13.6).
--
-- Yang membuat penggajian akurat bukan rumusnya, melainkan URUTAN YANG TETAP,
-- SNAPSHOT di setiap baris slip, dan PENGUNCIAN. Menghitung periode yang sama
-- dua kali dari data yang sama menghasilkan angka identik sampai rupiah
-- terakhir (§13.6).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE payroll_rules (
    id             CHAR(26)     PRIMARY KEY,
    tenant_id      CHAR(26)     NOT NULL,
    code           TEXT         NOT NULL,
    name           TEXT         NOT NULL,
    type           TEXT         NOT NULL CHECK (type IN ('tunjangan_tetap','potongan_telat','potongan_alpa',
                                'upah_lembur','bonus_kehadiran','bonus_target','potongan_kasbon','komponen_manual')),
    category       TEXT         NOT NULL CHECK (category IN ('earning','deduction')),
    params         JSONB        NOT NULL,   -- divalidasi per tipe di service; tak pernah di-query SQL
    target_type    TEXT         NOT NULL DEFAULT 'all' CHECK (target_type IN ('all','role','employee')),
    target_id      CHAR(26),
    priority       INT          NOT NULL DEFAULT 100,
    effective_from DATE         NOT NULL,
    effective_to   DATE,
    is_active      BOOLEAN      NOT NULL DEFAULT true,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code, effective_from),
    CHECK (effective_to IS NULL OR effective_to >= effective_from),
    CHECK ((target_type = 'all') = (target_id IS NULL))
);
CREATE INDEX idx_payroll_rules_active ON payroll_rules (tenant_id, effective_from, effective_to) WHERE is_active;

CREATE TABLE payroll_periods (
    id              CHAR(26)     PRIMARY KEY,
    tenant_id       CHAR(26)     NOT NULL,
    outlet_id       CHAR(26),
    period_type     TEXT         NOT NULL CHECK (period_type IN ('daily','weekly','biweekly','monthly')),
    start_date      DATE         NOT NULL,
    end_date        DATE         NOT NULL,
    pay_date        DATE,
    status          TEXT         NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft','calculated','locked','paid','canceled')),
    total_gross     BIGINT       NOT NULL DEFAULT 0,
    total_deduction BIGINT       NOT NULL DEFAULT 0,
    total_net       BIGINT       NOT NULL DEFAULT 0,
    calculated_at   TIMESTAMPTZ,
    locked_at       TIMESTAMPTZ,
    locked_by       CHAR(26),
    paid_at         TIMESTAMPTZ,
    created_by      CHAR(26)     NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    CHECK (end_date >= start_date)
);
CREATE UNIQUE INDEX uq_payroll_periods
    ON payroll_periods (tenant_id, COALESCE(outlet_id, ''), period_type, start_date);

CREATE TABLE payslips (
    id                CHAR(26)     PRIMARY KEY,
    tenant_id         CHAR(26)     NOT NULL,
    payroll_period_id CHAR(26)     NOT NULL,
    employee_id       CHAR(26)     NOT NULL,
    gross_amount      BIGINT       NOT NULL DEFAULT 0,
    deduction_amount  BIGINT       NOT NULL DEFAULT 0,
    net_amount        BIGINT       NOT NULL DEFAULT 0 CHECK (net_amount >= 0),
    carried_debt      BIGINT       NOT NULL DEFAULT 0,
    present_days      NUMERIC(5,1) NOT NULL DEFAULT 0,
    late_count        INT          NOT NULL DEFAULT 0,
    absent_days       NUMERIC(5,1) NOT NULL DEFAULT 0,
    leave_days        NUMERIC(5,1) NOT NULL DEFAULT 0,
    overtime_minutes  INT          NOT NULL DEFAULT 0,
    status            TEXT         NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','locked','paid')),
    paid_at           TIMESTAMPTZ,
    payment_method    TEXT,
    note              TEXT,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, payroll_period_id, employee_id),
    FOREIGN KEY (tenant_id, payroll_period_id) REFERENCES payroll_periods (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, employee_id)       REFERENCES employees       (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE payslip_lines (
    id              CHAR(26)      PRIMARY KEY,
    tenant_id       CHAR(26)      NOT NULL,
    payslip_id      CHAR(26)      NOT NULL,
    rule_id         CHAR(26),
    name            TEXT          NOT NULL,   -- SNAPSHOT
    category        TEXT          NOT NULL CHECK (category IN ('earning','deduction')),
    rule_type       TEXT,                     -- SNAPSHOT
    params_snapshot JSONB,                    -- SNAPSHOT
    basis_note      TEXT,                     -- "3 kali telat x Rp10.000"
    quantity        NUMERIC(14,3),
    amount          BIGINT        NOT NULL,
    sort_order      INT           NOT NULL DEFAULT 0,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, payslip_id) REFERENCES payslips      (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, rule_id)    REFERENCES payroll_rules (tenant_id, id) ON DELETE SET NULL
);
CREATE INDEX idx_payslip_lines_payslip ON payslip_lines (tenant_id, payslip_id);

CREATE TABLE payroll_adjustments (
    id                 CHAR(26)     PRIMARY KEY,
    tenant_id          CHAR(26)     NOT NULL,
    employee_id        CHAR(26)     NOT NULL,
    origin_period_id   CHAR(26)     NOT NULL,   -- periode yang salah
    target_period_id   CHAR(26),                -- NULL = periode berikutnya
    name               TEXT         NOT NULL,
    category           TEXT         NOT NULL CHECK (category IN ('earning','deduction')),
    amount             BIGINT       NOT NULL CHECK (amount > 0),
    reason             TEXT         NOT NULL,
    applied_payslip_id CHAR(26),
    created_by         CHAR(26)     NOT NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, employee_id)      REFERENCES employees       (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, origin_period_id) REFERENCES payroll_periods (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_payroll_adj_pending ON payroll_adjustments (tenant_id, employee_id) WHERE applied_payslip_id IS NULL;

CREATE TABLE employee_advances (
    id                 CHAR(26)     PRIMARY KEY,
    tenant_id          CHAR(26)     NOT NULL,
    employee_id        CHAR(26)     NOT NULL,
    amount             BIGINT       NOT NULL CHECK (amount > 0),
    remaining          BIGINT       NOT NULL CHECK (remaining >= 0),
    installment_amount BIGINT       NOT NULL CHECK (installment_amount > 0),
    reason             TEXT,
    status             TEXT         NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending','approved','disbursed','settled','canceled')),
    approved_by        CHAR(26),
    approved_at        TIMESTAMPTZ,
    disbursed_at       TIMESTAMPTZ,
    cash_movement_id   CHAR(26),
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    CHECK (remaining <= amount),
    FOREIGN KEY (tenant_id, employee_id)      REFERENCES employees      (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, cash_movement_id) REFERENCES cash_movements (tenant_id, id) ON DELETE SET NULL
);
CREATE INDEX idx_advances_open ON employee_advances (tenant_id, employee_id)
    WHERE status = 'disbursed' AND remaining > 0;

CREATE TABLE advance_repayments (
    id            CHAR(26)     PRIMARY KEY,
    tenant_id     CHAR(26)     NOT NULL,
    advance_id    CHAR(26)     NOT NULL,
    payslip_id    CHAR(26),
    amount        BIGINT       NOT NULL CHECK (amount > 0),
    paid_at       TIMESTAMPTZ  NOT NULL,
    business_date DATE         NOT NULL,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, advance_id, payslip_id),
    FOREIGN KEY (tenant_id, advance_id) REFERENCES employee_advances (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, payslip_id) REFERENCES payslips          (tenant_id, id) ON DELETE RESTRICT
);

-- Gaji & pencairan kasbon masuk arus kas. shift_id jadi NULLABLE: pembayaran
-- ini tidak terikat shift kasir.
ALTER TABLE cash_movements
    ADD COLUMN ref_table TEXT CHECK (ref_table IN ('payroll_periods','employee_advances')),
    ADD COLUMN ref_id    CHAR(26),
    ALTER COLUMN shift_id DROP NOT NULL;

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['payroll_rules','payroll_periods','payslips','payslip_lines',
                           'payroll_adjustments','employee_advances','advance_repayments'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;

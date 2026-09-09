-- Migrasi 000028 — Program Mitra: komisi, pencairan, target, materi, pelatihan,
-- sengketa (Fase 12, §5.12, blueprint G.2/G.4/G.5).
--
-- `partner_commissions`: satu baris per (referral × faktur langganan × bulan).
-- `UNIQUE (referral_id, subscription_invoice_id, period_month)` = idempoten;
-- mesin komisi meng-UPSERT dan tidak menyentuh baris 'approved'/'paid'
-- (deterministik, sepadan dengan mesin gaji Fase 13).
--
-- `partner_payouts`: pencairan periodik. bruto − clawback − pajak = neto
-- (blueprint G.2 #6).
--
-- Jejak audit akses mitra ke data merchant TIDAK dibuat di sini — memakai
-- `audit_logs` (§5.14, migrasi 000032) dengan actor_type='partner_user'.
--
-- Modul PLATFORM — tanpa RLS, FK tunggal (§5.17).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE partner_commissions (
    id                      CHAR(26)     PRIMARY KEY,
    partner_id              CHAR(26)     NOT NULL REFERENCES partners (id) ON DELETE RESTRICT,
    referral_id             CHAR(26)     NOT NULL REFERENCES partner_referrals (id) ON DELETE RESTRICT,
    subscription_invoice_id CHAR(26)     NOT NULL REFERENCES subscription_invoices (id) ON DELETE RESTRICT,
    period_month            DATE         NOT NULL,   -- bulan yang dikomisikan (tanggal 1)
    base_amount             BIGINT       NOT NULL,   -- nilai yang BENAR-BENAR diterima
    rate                    NUMERIC(7,4) NOT NULL,
    amount                  BIGINT       NOT NULL,
    status                  TEXT         NOT NULL DEFAULT 'held'
                            CHECK (status IN ('held','approved','paid','clawed_back','canceled')),
    payout_id               CHAR(26),
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (referral_id, subscription_invoice_id, period_month)
);
CREATE INDEX idx_partner_commissions_partner_status ON partner_commissions (partner_id, status);
CREATE INDEX idx_partner_commissions_referral       ON partner_commissions (referral_id);
CREATE INDEX idx_partner_commissions_invoice        ON partner_commissions (subscription_invoice_id);

CREATE TABLE partner_payouts (
    id                 CHAR(26)    PRIMARY KEY,
    partner_id         CHAR(26)    NOT NULL REFERENCES partners (id) ON DELETE RESTRICT,
    period_start       DATE        NOT NULL,
    period_end         DATE        NOT NULL,
    gross_amount       BIGINT      NOT NULL,
    tax_amount         BIGINT      NOT NULL DEFAULT 0,
    clawback_amount    BIGINT      NOT NULL DEFAULT 0,
    net_amount         BIGINT      NOT NULL,
    status             TEXT        NOT NULL DEFAULT 'draft'
                       CHECK (status IN ('draft','approved','paid','failed')),
    paid_at            TIMESTAMPTZ,
    transfer_proof_url TEXT,
    tax_slip_url       TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (partner_id, period_start)
);

ALTER TABLE partner_commissions
    ADD CONSTRAINT fk_partner_commissions_payout
    FOREIGN KEY (payout_id) REFERENCES partner_payouts (id) ON DELETE SET NULL;
CREATE INDEX idx_partner_commissions_payout ON partner_commissions (payout_id);

CREATE TABLE partner_targets (
    id                 CHAR(26) PRIMARY KEY,
    partner_id         CHAR(26) NOT NULL REFERENCES partners (id) ON DELETE CASCADE,
    period_start       DATE     NOT NULL,
    period_end         DATE     NOT NULL,
    target_merchants   INT      NOT NULL DEFAULT 0,
    achieved_merchants INT      NOT NULL DEFAULT 0,
    UNIQUE (partner_id, period_start)
);

CREATE TABLE partner_materials (
    id          CHAR(26)    PRIMARY KEY,
    title       TEXT        NOT NULL,
    kind        TEXT        NOT NULL CHECK (kind IN ('brochure','video','template','pricelist')),
    file_url    TEXT        NOT NULL,
    version     INT         NOT NULL DEFAULT 1,
    min_tier_id CHAR(26) REFERENCES partner_tiers (id) ON DELETE SET NULL,
    is_active   BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_partner_materials_tier ON partner_materials (min_tier_id);

CREATE TABLE partner_trainings (
    id          CHAR(26) PRIMARY KEY,
    title       TEXT     NOT NULL,
    content_url TEXT,
    is_required BOOLEAN  NOT NULL DEFAULT false,
    sort_order  INT      NOT NULL DEFAULT 0
);

CREATE TABLE partner_training_records (
    partner_user_id CHAR(26) NOT NULL REFERENCES partner_users (id) ON DELETE CASCADE,
    training_id     CHAR(26) NOT NULL REFERENCES partner_trainings (id) ON DELETE CASCADE,
    completed_at    TIMESTAMPTZ,
    score           INT,
    PRIMARY KEY (partner_user_id, training_id)   -- tabel penghubung murni (§5.17)
);
CREATE INDEX idx_partner_training_records_training ON partner_training_records (training_id);

CREATE TABLE partner_disputes (
    id                  CHAR(26)    PRIMARY KEY,
    claimant_partner_id CHAR(26)    NOT NULL REFERENCES partners (id) ON DELETE CASCADE,
    tenant_id           CHAR(26) REFERENCES tenants (id) ON DELETE SET NULL,
    lead_id             CHAR(26) REFERENCES partner_leads (id) ON DELETE SET NULL,
    reason              TEXT        NOT NULL,
    status              TEXT        NOT NULL DEFAULT 'open'
                        CHECK (status IN ('open','accepted','rejected')),
    decided_by          CHAR(26),
    decided_at          TIMESTAMPTZ,
    decision_note       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_partner_disputes_claimant ON partner_disputes (claimant_partner_id, status);
CREATE INDEX idx_partner_disputes_tenant   ON partner_disputes (tenant_id);
CREATE INDEX idx_partner_disputes_lead     ON partner_disputes (lead_id);

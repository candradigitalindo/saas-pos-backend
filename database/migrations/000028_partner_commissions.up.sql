-- Migrasi 000028 — Program Mitra: komisi, pencairan, sengketa, jejak audit
-- (Fase 12, blueprint G.2/G.4/G.5/G.8).
--
-- `partner_commissions`: satu baris per (faktur langganan DIBAYAR × mitra).
-- `UNIQUE (subscription_invoice_id, partner_id)` = hitung ulang idempoten —
-- mesin komisi meng-UPSERT dan tidak menyentuh baris yang sudah 'approved'/'paid'
-- (deterministik, sepadan dengan mesin gaji Fase 13).
--
-- `partner_payouts`: pencairan periodik. bruto − clawback − pajak = neto
-- (blueprint G.2 #6). `UNIQUE (partner_id, period_start, period_end)`.
--
-- `partner_merchant_access_log`: setiap kali mitra melihat data merchant
-- binaannya (yang terbatas), dicatat di sini (blueprint G.8).
--
-- Modul PLATFORM — tanpa RLS.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE partner_commissions (
    id                      CHAR(26)     PRIMARY KEY,
    partner_id              CHAR(26)     NOT NULL REFERENCES partners (id) ON DELETE RESTRICT,
    tenant_id               CHAR(26)     NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    subscription_invoice_id CHAR(26)     NOT NULL REFERENCES subscription_invoices (id) ON DELETE RESTRICT,
    period_start            DATE         NOT NULL,
    period_end              DATE         NOT NULL,
    base_amount             BIGINT       NOT NULL,   -- subscription_invoices.paid_amount
    rate                    NUMERIC(7,4) NOT NULL,
    amount                  BIGINT       NOT NULL,   -- round(base * rate)
    status                  TEXT         NOT NULL DEFAULT 'held'
                            CHECK (status IN ('held','approved','paid','clawed_back')),
    payout_id               CHAR(26),
    note                    TEXT,
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (subscription_invoice_id, partner_id)
);
CREATE INDEX idx_partner_commissions_partner ON partner_commissions (partner_id, status);
CREATE INDEX idx_partner_commissions_payout  ON partner_commissions (payout_id);

CREATE TABLE partner_payouts (
    id              CHAR(26)    PRIMARY KEY,
    partner_id      CHAR(26)    NOT NULL REFERENCES partners (id) ON DELETE RESTRICT,
    period_start    DATE        NOT NULL,
    period_end      DATE        NOT NULL,
    gross_amount    BIGINT      NOT NULL,
    clawback_amount BIGINT      NOT NULL DEFAULT 0,
    tax_amount      BIGINT      NOT NULL DEFAULT 0,
    net_amount      BIGINT      NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft','paid','void')),
    transfer_proof  TEXT,
    paid_at         TIMESTAMPTZ,
    note            TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (partner_id, period_start, period_end)
);

CREATE TABLE partner_disputes (
    id          CHAR(26)    PRIMARY KEY,
    partner_id  CHAR(26)    NOT NULL REFERENCES partners (id) ON DELETE CASCADE,
    tenant_id   CHAR(26) REFERENCES tenants (id) ON DELETE SET NULL,
    referral_id CHAR(26) REFERENCES partner_referrals (id) ON DELETE SET NULL,
    reason      TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'open'
                CHECK (status IN ('open','upheld','rejected')),
    resolution  TEXT,
    resolved_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE partner_merchant_access_log (
    id              BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    partner_id      CHAR(26)    NOT NULL,
    partner_user_id CHAR(26)    NOT NULL,
    tenant_id       CHAR(26)    NOT NULL,
    action          TEXT        NOT NULL,   -- 'view_merchant_status'
    at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_partner_access_log_partner ON partner_merchant_access_log (partner_id, at);
